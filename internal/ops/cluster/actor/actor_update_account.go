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
package actor

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
	"github.com/chideat/valkey-operator/internal/config"
	cops "github.com/chideat/valkey-operator/internal/ops/cluster"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/actor"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/security/acl"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/types/user"
	"github.com/chideat/valkey-operator/pkg/version"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ actor.Actor = (*actorUpdateAccount)(nil)

func init() {
	actor.Register(core.ValkeyCluster, NewUpdateAccountActor)
}

func NewUpdateAccountActor(client kubernetes.ClientSet, logger logr.Logger) actor.Actor {
	return &actorUpdateAccount{
		client: client,
		logger: logger,
	}
}

type actorUpdateAccount struct {
	client kubernetes.ClientSet

	logger logr.Logger
}

// SupportedCommands
func (a *actorUpdateAccount) SupportedCommands() []actor.Command {
	return []actor.Command{cops.CommandUpdateAccount}
}

func (a *actorUpdateAccount) Version() *semver.Version {
	return semver.MustParse("3.14.0")
}

// Do
//
// 对于账户更新，需要尽量保持在一个 reconcile 里完成，否则会出现一个实例多种密码的情况
// operator 账户为内置账户，不支持修改密码, 如果出现账户不一致的情况，可通过重启来解决
//
// 由于 redis 6.0 支持了 ACL，从 5.0 升级到 6.0，会创建ACL账户。
// 更新逻辑：
// 1. 同步 acl configmap
// 2. 同步实例账户，如果同步实例账户失败，会清理 Pod
// 前置条件
// 1. 不支持 Redis 版本回退
// 2. 更新密码不能更新 secret，需要新建 secret
func (a *actorUpdateAccount) Do(ctx context.Context, val types.Instance) *actor.ActorResult {
	cluster := val.(types.ClusterInstance)
	logger := val.Logger().WithValues("actor", cops.CommandUpdateAccount.String())

	logger.Info("start update account", "cluster", cluster.GetName())

	var (
		err error

		users       = cluster.Users()
		defaultUser = users.GetDefaultUser()
		opUser      = users.GetOpUser()
	)

	if defaultUser == nil {
		defaultUser, _ = user.NewUser("", user.RoleDeveloper, nil, cluster.Version().IsACLSupported())
	}

	var (
		currentSecretName string = defaultUser.Password.GetSecretName()
		newSecretName     string = cluster.Definition().Spec.Access.DefaultPasswordSecret
	)

	isAclEnabled := (opUser.Role == user.RoleOperator)

	name := clusterbuilder.GenerateClusterACLConfigMapName(cluster.GetName())
	oldCm, err := a.client.GetConfigMap(ctx, cluster.GetNamespace(), name)
	if err != nil && !errors.IsNotFound(err) {
		logger.Error(err, "load configmap failed", "target", name)
		return actor.NewResultWithError(cops.CommandRequeue, err)
	} else if oldCm == nil {
		// sync acl configmap
		oldCm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				Namespace:       cluster.GetNamespace(),
				Labels:          cluster.GetLabels(),
				OwnerReferences: util.BuildOwnerReferences(cluster.Definition()),
			},
			Data: users.Encode(true),
		}

		// create acl with old password
		// create redis acl file, after restart, the password is updated
		if err := a.client.CreateConfigMap(ctx, cluster.GetNamespace(), oldCm); err != nil {
			logger.Error(err, "create acl configmap failed", "target", oldCm.Name)
			return actor.NewResultWithError(cops.CommandRequeue, err)
		}

		// wait for resource sync
		time.Sleep(time.Second * 3)
		if oldCm, err = a.client.GetConfigMap(ctx, cluster.GetNamespace(), name); err != nil {
			if !errors.IsNotFound(err) {
				logger.Error(err, "get configmap failed")
			}
			logger.Error(err, "get configmap failed", "target", name)
			return actor.NewResultWithError(cops.CommandRequeue, err)
		}
	}

	isUpdated := false
	users = append(users[0:0], defaultUser)
	if cluster.Version().IsACLSupported() {
		if !isAclEnabled {
			secretName := clusterbuilder.GenerateClusterACLOperatorSecretName(cluster.GetName())
			ownRefs := util.BuildOwnerReferences(cluster.Definition())
			if opUser, err := acl.NewOperatorUser(ctx, a.client,
				secretName, cluster.GetNamespace(), ownRefs, cluster.Version().IsACLSupported()); err != nil {

				logger.Error(err, "create operator user failed")
				return actor.NewResult(cops.CommandRequeue)
			} else {
				users = append(users, opUser)
				isUpdated = true
			}
			opRedisUser := clusterbuilder.GenerateClusterRedisUser(cluster, opUser)
			if err := a.client.CreateIfNotExistsRedisUser(ctx, opRedisUser); err != nil {
				logger.Error(err, "create operator redis user failed")
				return actor.NewResult(cops.CommandRequeue)
			}
			cluster.SendEventf(corev1.EventTypeNormal, config.EventCreateUser, "created operator user to enable acl")
		} else {
			if newOpUser, err := acl.NewOperatorUser(ctx, a.client,
				opUser.Password.SecretName, cluster.GetNamespace(), nil, cluster.Version().IsACLSupported()); err != nil {
				logger.Error(err, "create operator user failed")
				return actor.NewResult(cops.CommandRequeue)
			} else {
				opRedisUser := clusterbuilder.GenerateClusterRedisUser(cluster, newOpUser)
				if err := a.client.CreateOrUpdateRedisUser(ctx, opRedisUser); err != nil {
					logger.Error(err, "update operator redis user failed")
					return actor.NewResult(cops.CommandRequeue)
				}
				cluster.SendEventf(corev1.EventTypeNormal, config.EventCreateUser, "created/updated operator user")

				opUser.Rules = newOpUser.Rules
				users = append(users, opUser)

				isUpdated = true
			}
		}

		defaultRedisUser := clusterbuilder.GenerateClusterRedisUser(cluster, defaultUser)
		defaultRedisUser.Annotations[config.ACLSupportedVersionAnnotationKey] = cluster.Version().String()
		if oldDefaultRU, err := a.client.GetRedisUser(ctx, cluster.GetNamespace(), defaultRedisUser.GetName()); errors.IsNotFound(err) {
			if err := a.client.CreateIfNotExistsRedisUser(ctx, defaultRedisUser); err != nil {
				logger.Error(err, "update default redis user failed")
				return actor.NewResult(cops.CommandRequeue)
			}
			cluster.SendEventf(corev1.EventTypeNormal, config.EventCreateUser, "created default user")
		} else if err != nil {
			logger.Error(err, "get default redisuser failed")
			return actor.NewResult(cops.CommandRequeue)
		} else if cluster.Version().IsACLSupported() {
			oldVersion := version.ValkeyVersion(oldDefaultRU.Annotations[config.ACLSupportedVersionAnnotationKey])
			// COMP: if old version not support acl2, and new version is supported, update acl rules for compatibility
			if !oldVersion.IsACLSupported() {
				fields := strings.Fields(oldDefaultRU.Spec.AclRules)
				if !slices.Contains(fields, "&*") && !slices.Contains(fields, "allchannels") {
					oldDefaultRU.Spec.AclRules = fmt.Sprintf("%s &*", oldDefaultRU.Spec.AclRules)
				}
				if oldDefaultRU.Annotations == nil {
					oldDefaultRU.Annotations = make(map[string]string)
				}
				oldDefaultRU.Annotations[config.ACLSupportedVersionAnnotationKey] = cluster.Version().String()
				if err := a.client.UpdateRedisUser(ctx, oldDefaultRU); err != nil {
					logger.Error(err, "update default redis user failed")
					return actor.NewResult(cops.CommandRequeue)
				}
				cluster.SendEventf(corev1.EventTypeNormal, config.EventUpdateUser, "migrate default user acl rules to support channels")
			}
		}
	}

	if !reflect.DeepEqual(users.Encode(true), oldCm.Data) {
		isUpdated = true
	}
	if isUpdated {
		for k, v := range users.Encode(true) {
			oldCm.Data[k] = v
		}
		if err := a.client.CreateOrUpdateConfigMap(ctx, cluster.GetNamespace(), oldCm); err != nil {
			logger.Error(err, "update acl configmap failed", "target", oldCm.Name)
			return actor.NewResultWithError(cops.CommandRequeue, err)
		}
		if err := cluster.Refresh(ctx); err != nil {
			logger.Error(err, "refresh resource failed")
			return actor.NewResultWithError(cops.CommandRequeue, err)
		}
	}

	var (
		isACLAppliedInPods = true
		isAllACLSupported  = true
	)

	// if not all nodes ready, refuse to update the password
	// this is used to ensure succeed upgrade from no acl => acl
	if !cluster.IsReady() {
		return actor.NewResult(cops.CommandHealPod)
	}
	// only update password when all pod is ready
	// here only check the startup pods, not check the termination and pending pods
	for _, shard := range cluster.Shards() {
		for _, node := range shard.Nodes() {
			if !node.CurrentVersion().IsACLSupported() {
				isAllACLSupported = false
				break
			}
			// check if acl have been applied to container
			if !node.IsACLApplied() {
				isACLAppliedInPods = false
			}
		}
		if !isAllACLSupported {
			break
		}
	}

	defaultUser = users.GetDefaultUser()
	opUser = users.GetOpUser()
	logger.V(3).Info("update account ready",
		"isACLAppliedInPods", isACLAppliedInPods,
		"isAllACLSupported", isAllACLSupported,
		"isAclEnabled", isAclEnabled,
		"acl", cluster.Version().IsACLSupported(),
		"exists", cluster.IsACLUserExists(),
	)
	if cluster.Version().IsACLSupported() && isAllACLSupported {
		if newSecretName != currentSecretName {
			if isAclEnabled {
				// hotconfig with redis acl/password
				// if some node update failed, then the pod should be deleted to restarted (TODO)
				for _, node := range cluster.Nodes() {
					if node.ContainerStatus() == nil || !node.ContainerStatus().Ready ||
						node.IsTerminating() {
						continue
					}

					if err := node.Setup(ctx, formatACLSetCommand(defaultUser)); err != nil {
						logger.Error(err, "update acl config failed")
					}
				}

				// update default user password
				ru := clusterbuilder.GenerateClusterRedisUser(cluster, defaultUser)
				oldRu, err := a.client.GetRedisUser(ctx, cluster.GetNamespace(), ru.Name)
				if err == nil && !reflect.DeepEqual(oldRu.Spec.PasswordSecrets, ru.Spec.PasswordSecrets) {
					oldRu.Spec.PasswordSecrets = ru.Spec.PasswordSecrets
					if err := a.client.UpdateRedisUser(ctx, oldRu); err != nil {
						logger.Error(err, "update default user redisUser failed")
					}
				} else if errors.IsNotFound(err) {
					if err := a.client.CreateIfNotExistsRedisUser(ctx, ru); err != nil {
						logger.Error(err, "create default user redisUser failed")
					}
					cluster.SendEventf(corev1.EventTypeNormal, config.EventCreateUser, "created default user when update password")
				}
			} else {
				// this is low probability event

				// added acl account and restart
				// after the instance upgrade from old version to 6.0 supported version,
				margs := [][]interface{}{}
				for _, user := range users {
					margs = append(margs, formatACLSetCommand(user))
				}
				margs = append(
					margs,
					[]interface{}{"config", "set", "masteruser", opUser.Name},
					[]interface{}{"config", "set", "masterauth", opUser.Password.String()},
				)
				for _, node := range cluster.Nodes() {
					if node.ContainerStatus() == nil || !node.ContainerStatus().Ready ||
						node.IsTerminating() {
						continue
					}

					if err := node.Setup(ctx, margs...); err != nil {
						logger.Error(err, "update acl config failed")
					}
				}
				cluster.SendEventf(corev1.EventTypeNormal, config.EventUpdatePassword, "updated instance password and injected acl users")
				// then requeue to refresh cluster info
				a.logger.Info("=== requeue to refresh cluster info ===(acl)")
				return actor.NewResultWithValue(cops.CommandRequeue, time.Second)
			}
		} else if !isAclEnabled && isACLAppliedInPods {
			// added acl account and restart
			// after the instance upgrade from old version to 6.0 supported version,
			margs := [][]interface{}{}
			for _, user := range users {
				margs = append(margs, formatACLSetCommand(user))
			}
			margs = append(
				margs,
				[]interface{}{"config", "set", "masteruser", opUser.Name},
				[]interface{}{"config", "set", "masterauth", opUser.Password.String()},
			)
			allAclUpdated := true
			for _, node := range cluster.Nodes() {
				if node.ContainerStatus() == nil || !node.ContainerStatus().Ready ||
					node.IsTerminating() {
					allAclUpdated = false
					continue
				}

				if err := node.Setup(ctx, margs...); err != nil {
					allAclUpdated = false
					logger.Error(err, "update acl config failed")
				}
			}
			if allAclUpdated {
				if err := a.client.CreateOrUpdateConfigMap(ctx, cluster.GetNamespace(), oldCm); err != nil {
					logger.Error(err, "update acl configmap failed", "target", oldCm.Name)
					return actor.NewResultWithError(cops.CommandRequeue, err)
				}
				cluster.SendEventf(corev1.EventTypeNormal, config.EventUpdatePassword, "applied acl to all pods, switch to operator user for cluster auth")
			}
			a.logger.Info("=== requeue to refresh cluster info ===(no acl)")
			// then requeue to refresh cluster info
			return actor.NewResultWithValue(cops.CommandRequeue, time.Second)
		}
	} else {
		if newSecretName != currentSecretName {
			var newSecret *corev1.Secret
			if newSecretName != "" {
				if newSecret, err = a.client.GetSecret(ctx, cluster.GetNamespace(), newSecretName); errors.IsNotFound(err) {
					logger.Error(err, "get cluster secret failed", "target", newSecretName)
					return actor.NewResultWithError(cops.CommandRequeue, fmt.Errorf("secret %s not found", newSecretName))

				} else if err != nil {
					logger.Error(err, "get cluster secret failed", "target", newSecretName)
					return actor.NewResultWithError(cops.CommandRequeue, err)
				}
			}
			defaultUser.Password, _ = user.NewPassword(newSecret)

			_ = cluster.UpdateStatus(ctx, types.Any, "updating password")
			// update masterauth and requirepass, and restart (ensure_resource do this)
			// hotconfig with redis acl/password
			updateMasterAuth := []interface{}{"config", "set", "masterauth", defaultUser.Password.String()}
			updateRequirePass := []interface{}{"config", "set", "requirepass", defaultUser.Password.String()}
			cmd := []string{"sh", "-c", fmt.Sprintf(`echo -n '%s' > /tmp/newpass`, defaultUser.Password.String())}
			for _, shard := range cluster.Shards() {
				for _, node := range shard.Nodes() {
					if !node.IsReady() || node.IsTerminating() {
						continue
					}

					// Retry hard
					if err := util.RetryOnTimeout(func() error {
						_, _, err := a.client.Exec(ctx, node.GetNamespace(), node.GetName(), clusterbuilder.ServerContainerName, cmd)
						return err
					}, 5); err != nil {
						logger.Error(err, "patch new secret to pod failed", "pod", node.GetName())
					}

					// for old version, use default user for auth
					if err := node.Setup(ctx, updateMasterAuth, updateRequirePass); err != nil {
						logger.Error(err, "update acl config failed")
					}
				}
			}

			for k, v := range users.Encode(true) {
				oldCm.Data[k] = v
			}
			if err := a.client.CreateOrUpdateConfigMap(ctx, cluster.GetNamespace(), oldCm); err != nil {
				logger.Error(err, "update acl configmap failed", "target", oldCm.Name)
				return actor.NewResultWithError(cops.CommandRequeue, err)
			}

			if err := cluster.Refresh(ctx); err != nil {
				logger.Error(err, "refresh resource failed")
			}

			// kill all replica clients from master to force replicas use new password reconnect to master
			for _, shard := range cluster.Shards() {
				master := shard.Master()
				if master == nil {
					continue
				}

				logger.Info("force replica clients to reconnect to master", "master", master.GetName())
				// NOTE: require redis 5.0
				if err := master.Setup(ctx, []interface{}{"client", "kill", "type", "replica"}); err != nil {
					logger.Error(err, "kill replica client failed", "master", master.GetName())
				}
				time.Sleep(time.Second * 5)
			}
			cluster.SendEventf(corev1.EventTypeNormal, config.EventUpdatePassword, "updated instance password")

			return actor.NewResult(cops.CommandRequeue)
		}
	}

	if cluster.Version().IsACLSupported() && !isAllACLSupported {
		return actor.NewResult(cops.CommandEnsureResource)
	}
	return nil
}

// formatACLSetCommand
//
// only acl 1 supported
func formatACLSetCommand(u *user.User) []interface{} {
	if u == nil {
		return nil
	}

	// keep in mind that the user.Name is "default" for default user
	// when update command,password,keypattern, must reset them all
	args := []interface{}{"acl", "setuser", u.Name, "reset"}
	for _, rule := range u.Rules {
		for _, field := range strings.Fields(rule.Encode()) {
			args = append(args, field)
		}
	}
	passwd := u.Password.String()
	if passwd == "" {
		args = append(args, "nopass")
	} else {
		args = append(args, fmt.Sprintf(">%s", passwd))
	}
	return append(args, "on")
}
