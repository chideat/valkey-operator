/*
Copyright 2024 chideat.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package user

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/types/user"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder/aclbuilder"
	"github.com/chideat/valkey-operator/internal/valkey/cluster"
	"github.com/chideat/valkey-operator/internal/valkey/failover"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
)

type UserHandler struct {
	k8sClient     kubernetes.ClientSet
	eventRecorder record.EventRecorder
	logger        logr.Logger
}

func NewUserHandler(k8sservice kubernetes.ClientSet, eventRecorder record.EventRecorder, logger logr.Logger) *UserHandler {
	return &UserHandler{
		k8sClient:     k8sservice,
		eventRecorder: eventRecorder,
		logger:        logger.WithName("UserHandler"),
	}
}

func (r *UserHandler) Delete(ctx context.Context, inst v1alpha1.User, logger logr.Logger) error {
	logger.V(3).Info("delete user", "user instance name", inst.Name,
		"instance", inst.Spec.InstanceName, "type", inst.Spec.Arch)
	if inst.Spec.Username == user.DefaultUserName || inst.Spec.Username == user.DefaultOperatorUserName {
		return nil
	}

	vkName := inst.Spec.InstanceName
	cmName := aclbuilder.GenerateACLConfigMapName(inst.Spec.Arch, vkName)
	// Read inside the retry: the update carries the version this read returned, so a write
	// racing with it conflicts and the removal is replayed against the newer content instead
	// of one of the two writers silently losing its change.
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		configMap, err := r.k8sClient.GetConfigMap(ctx, inst.Namespace, cmName)
		if err != nil {
			return err
		}
		if _, ok := configMap.Data[inst.Spec.Username]; !ok {
			return nil
		}
		delete(configMap.Data, inst.Spec.Username)
		return r.k8sClient.UpdateConfigMap(ctx, inst.Namespace, configMap)
	}); err != nil && !errors.IsNotFound(err) {
		logger.Error(err, "delete user from configmap failed", "configmap", cmName)
		return err
	}

	switch inst.Spec.Arch {
	case core.ValkeyCluster:
		logger.V(3).Info("cluster", "instance", vkName)
		rc, err := r.k8sClient.GetCluster(ctx, inst.Namespace, vkName)
		if errors.IsNotFound(err) {
			return nil
		} else if err != nil {
			return err
		}

		rcm, err := cluster.NewCluster(ctx, r.k8sClient, r.eventRecorder, rc, logger)
		if err != nil {
			return err
		}
		if !rcm.IsReady() {
			logger.V(3).Info("instance is not ready", "instance", vkName)
			return fmt.Errorf("instance is not ready")
		}

		for _, node := range rcm.Nodes() {
			err := node.Setup(ctx, []any{"ACL", "DELUSER", inst.Spec.Username})
			if err != nil {
				logger.Error(err, "acl del user failed", "node", node.GetName())
				return err
			}
			logger.V(3).Info("acl del user success", "node", node.GetName())
		}
	case core.ValkeyFailover, core.ValkeyReplica:
		logger.V(3).Info("sentinel", "instane", vkName)
		rf, err := r.k8sClient.GetFailover(ctx, inst.Namespace, vkName)
		if errors.IsNotFound(err) {
			return nil
		} else if err != nil {
			return err
		}

		rfm, err := failover.NewFailover(ctx, r.k8sClient, r.eventRecorder, rf, logger)
		if err != nil {
			return err
		}
		if !rfm.IsReady() {
			logger.V(3).Info("instance is not ready", "instance", vkName)
			return fmt.Errorf("instance is not ready")
		}
		for _, node := range rfm.Nodes() {
			err := node.Setup(ctx, []any{"ACL", "DELUSER", inst.Spec.Username})
			if err != nil {
				logger.Error(err, "acl del user failed", "node", node.GetName())
				return err
			}
			logger.V(3).Info("acl del user success", "node", node.GetName())
		}
	}
	return nil
}

// upsertUserInACLConfigMap writes one user's entry into the ACL ConfigMap, re-reading the
// object on every attempt so that entries owned by other writers — the other Users and the
// instance's own default/operator accounts — are carried over from the current object rather
// than from a snapshot that may be several API round-trips old.
func (r *UserHandler) upsertUserInACLConfigMap(ctx context.Context, namespace, name, username, entry string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		configMap, err := r.k8sClient.GetConfigMap(ctx, namespace, name)
		if err != nil {
			return err
		}
		if configMap.Data == nil {
			configMap.Data = map[string]string{}
		}
		configMap.Data[username] = entry
		return r.k8sClient.UpdateConfigMap(ctx, namespace, configMap)
	})
}

func (r *UserHandler) Do(ctx context.Context, inst v1alpha1.User, logger logr.Logger) error {
	if inst.Annotations == nil {
		inst.Annotations = map[string]string{}
	}

	passwords := []string{}
	userPassword := &user.Password{}
	for _, secretName := range inst.Spec.PasswordSecrets {
		secret, err := r.k8sClient.GetSecret(ctx, inst.Namespace, secretName)
		if err != nil {
			return err
		}
		passwords = append(passwords, string(secret.Data["password"]))
		userPassword = &user.Password{
			SecretName: secretName,
		}
	}

	logger.V(3).Info("reconcile user", "user name", inst.Name, "type", inst.Spec.Arch)
	vkName := inst.Spec.InstanceName
	cmName := aclbuilder.GenerateACLConfigMapName(inst.Spec.Arch, vkName)

	switch inst.Spec.Arch {
	case core.ValkeyCluster:
		logger.V(3).Info("cluster", "instance", vkName)
		rc, err := r.k8sClient.GetCluster(ctx, inst.Namespace, vkName)
		if err != nil {
			return err
		}

		rcm, err := cluster.NewCluster(ctx, r.k8sClient, r.eventRecorder, rc, logger)
		if err != nil {
			return err
		}
		if !rcm.IsReady() {
			logger.V(3).Info("instance is not ready", "instance", vkName)
			return fmt.Errorf("instance is not ready")
		}

		rule, err := user.NewRule(inst.Spec.AclRules)
		if err != nil {
			logger.V(3).Info("rule parse failed", "rule", inst.Spec.AclRules)
			return err
		}
		rule = types.PatchClusterClientRequiredRules(rule)
		aclRules := rule.Encode()

		userObj, err := types.NewUserFromValkeyUser(inst.Spec.Username, aclRules, userPassword)
		if err != nil {
			return err
		}
		info, err := json.Marshal(userObj)
		if err != nil {
			return err
		}

		if inst.Spec.AccountType != v1alpha1.SystemAccount {
			for _, node := range rcm.Nodes() {
				_, err := node.SetACLUser(ctx, inst.Spec.Username, passwords, aclRules)
				if err != nil {
					logger.Error(err, "acl set user failed", "node", node.GetName())
					return err
				}
				logger.V(3).Info("acl set user success", "node", node.GetName())
			}
		} else {
			logger.V(3).Info("skip system account online update", "username", inst.Spec.Username)
		}

		if err := r.upsertUserInACLConfigMap(ctx, inst.Namespace, cmName, inst.Spec.Username, string(info)); err != nil {
			logger.Error(err, "update configmap failed", "configmap", cmName)
			return err
		}
	case core.ValkeyFailover, core.ValkeyReplica:
		logger.V(3).Info("sentinel", "instance", vkName)
		rf, err := r.k8sClient.GetFailover(ctx, inst.Namespace, vkName)
		if err != nil {
			return err
		}
		rfm, err := failover.NewFailover(ctx, r.k8sClient, r.eventRecorder, rf, logger)
		if err != nil {
			return err
		}
		if !rfm.IsReady() {
			logger.V(3).Info("instance is not ready", "instance", vkName)
			return fmt.Errorf("instance is not ready")
		}
		userObj, err := types.NewUserFromValkeyUser(inst.Spec.Username, inst.Spec.AclRules, userPassword)
		if err != nil {
			return err
		}
		info, err := json.Marshal(userObj)
		if err != nil {
			return err
		}
		if inst.Spec.AccountType != v1alpha1.SystemAccount {
			for _, node := range rfm.Nodes() {
				_, err := node.SetACLUser(ctx, inst.Spec.Username, passwords, inst.Spec.AclRules)
				if err != nil {
					logger.Error(err, "acl set user failed", "node", node.GetName())
					return err
				}
				logger.V(3).Info("acl set user success", "node", node.GetName())
			}
		} else {
			logger.V(3).Info("skip system account online update", "username", inst.Spec.Username)
		}

		if err := r.upsertUserInACLConfigMap(ctx, inst.Namespace, cmName, inst.Spec.Username, string(info)); err != nil {
			logger.Error(err, "update configmap failed", "configmap", cmName)
			return err
		}
	}
	return nil
}
