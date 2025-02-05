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
	if configMap, err := r.k8sClient.GetConfigMap(ctx, inst.Namespace, cmName); err != nil {
		if !errors.IsNotFound(err) {
			logger.Error(err, "delete user from configmap failed")
			return err
		}
	} else if _, ok := configMap.Data[inst.Spec.Username]; ok {
		delete(configMap.Data, inst.Spec.Username)
		if err := r.k8sClient.UpdateConfigMap(ctx, inst.Namespace, configMap); err != nil {
			logger.Error(err, "delete user from configmap failed", "configmap", cmName)
			return err
		}
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
			err := node.Setup(ctx, []interface{}{"ACL", "DELUSER", inst.Spec.Username})
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
			err := node.Setup(ctx, []interface{}{"ACL", "DELUSER", inst.Spec.Username})
			if err != nil {
				logger.Error(err, "acl del user failed", "node", node.GetName())
				return err
			}
			logger.V(3).Info("acl del user success", "node", node.GetName())
		}
	}
	return nil
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

		aclRules := inst.Spec.AclRules
		rule, err := user.NewRule(inst.Spec.AclRules)
		if err != nil {
			logger.V(3).Info("rule parse failed", "rule", inst.Spec.AclRules)
			return err
		}
		rule = types.PatchClusterClientRequiredRules(rule)
		aclRules = rule.Encode()

		userObj, err := types.NewUserFromValkeyUser(inst.Spec.Username, aclRules, userPassword)
		if err != nil {
			return err
		}
		info, err := json.Marshal(userObj)
		if err != nil {
			return err
		}

		configmap, err := r.k8sClient.GetConfigMap(ctx, inst.Namespace, cmName)
		if err != nil {
			return err
		}
		configmap.Data[inst.Spec.Username] = string(info)

		if inst.Spec.AccountType != v1alpha1.System {
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

		if err := r.k8sClient.UpdateConfigMap(ctx, inst.Namespace, configmap); err != nil {
			logger.Error(err, "update configmap failed", "configmap", configmap.Name)
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
		configmap, err := r.k8sClient.GetConfigMap(ctx, inst.Namespace, cmName)
		if err != nil {
			return err
		}
		userObj, err := types.NewUserFromValkeyUser(inst.Spec.Username, inst.Spec.AclRules, userPassword)
		if err != nil {
			return err
		}
		info, err := json.Marshal(userObj)
		if err != nil {
			return err
		}
		configmap.Data[inst.Spec.Username] = string(info)

		if inst.Spec.AccountType != v1alpha1.System {
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

		if err := r.k8sClient.UpdateConfigMap(ctx, inst.Namespace, configmap); err != nil {
			logger.Error(err, "update configmap failed", "configmap", configmap.Name)
			return err
		}
	}
	return nil
}
