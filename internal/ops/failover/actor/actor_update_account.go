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
	"maps"
	"reflect"

	"github.com/Masterminds/semver/v3"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/actor"
	"github.com/chideat/valkey-operator/internal/builder/aclbuilder"
	"github.com/chideat/valkey-operator/internal/config"
	ops "github.com/chideat/valkey-operator/internal/ops/failover"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/security/acl"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ actor.Actor = (*actorUpdateAccount)(nil)

func init() {
	actor.Register(core.ValkeyFailover, NewUpdateAccountActor)
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

func (a *actorUpdateAccount) Version() *semver.Version {
	return semver.MustParse("0.1.0")
}

// SupportedCommands
func (a *actorUpdateAccount) SupportedCommands() []actor.Command {
	return []actor.Command{ops.CommandUpdateAccount}
}

func (a *actorUpdateAccount) Do(ctx context.Context, val types.Instance) *actor.ActorResult {
	logger := val.Logger().WithValues("actor", ops.CommandUpdateAccount.String())

	var (
		inst      = val.(types.FailoverInstance)
		users     = inst.Users()
		opUser    = users.GetOpUser()
		ownRefs   = util.BuildOwnerReferences(inst.Definition())
		isUpdated = false
	)

	name := aclbuilder.GenerateACLConfigMapName(inst.Arch(), inst.GetName())
	oldCm, err := a.client.GetConfigMap(ctx, inst.GetNamespace(), name)
	if err != nil && !errors.IsNotFound(err) {
		logger.Error(err, "load configmap failed", "target", name)
		return actor.NewResultWithError(ops.CommandRequeue, err)
	} else if oldCm != nil && oldCm.GetDeletionTimestamp() != nil {
		logger.Info("configmap is being deleted, skip update", "target", name)
		return actor.NewResult(ops.CommandRequeue)
	} else {
		// sync acl configmap
		oldCm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				Namespace:       inst.GetNamespace(),
				Labels:          inst.GetLabels(),
				OwnerReferences: util.BuildOwnerReferences(inst.Definition()),
			},
			Data: users.Encode(true),
		}
		isUpdated = true

		// // create acl with old password
		// // create acl file, after restart, the password is updated
		// if err := a.client.CreateConfigMap(ctx, inst.GetNamespace(), oldCm); err != nil {
		// 	logger.Error(err, "create acl configmap failed", "target", oldCm.Name)
		// 	return actor.NewResultWithError(ops.CommandRequeue, err)
		// }

		// // wait for resource sync
		// time.Sleep(time.Second * 1)
		// if oldCm, err = a.client.GetConfigMap(ctx, inst.GetNamespace(), name); err != nil {
		// 	logger.Error(err, "get configmap failed", "target", name)
		// 	return actor.NewResultWithError(ops.CommandRequeue, err)
		// }
	}

	logger.Info("update account", "namespace", inst.GetNamespace(), "name", inst.GetName(), "aclConfigMap", oldCm.Name, "opUser", opUser == nil)

	if opUser == nil {
		secretName := aclbuilder.GenerateACLOperatorSecretName(inst.Arch(), inst.GetName())
		opUser, err = acl.NewOperatorUser(ctx, a.client, secretName, inst.GetNamespace(), ownRefs)
		if err != nil {
			logger.Error(err, "create operator user failed")
			return actor.NewResult(ops.CommandRequeue)
		} else {
			users = append(users, opUser)
			isUpdated = true
		}

		opVKUser := aclbuilder.GenerateUser(inst, opUser)
		if err := a.client.CreateOrUpdateUser(ctx, opVKUser); err != nil {
			logger.Error(err, "create operator user failed")
			return actor.NewResult(ops.CommandRequeue)
		}
		inst.SendEventf(corev1.EventTypeNormal, config.EventCreateUser, "created operator user to enable acl")
	}

	if !reflect.DeepEqual(users.Encode(true), oldCm.Data) {
		isUpdated = true
	}
	if isUpdated {
		maps.Copy(oldCm.Data, users.Encode(true))
		if err := a.client.CreateOrUpdateConfigMap(ctx, inst.GetNamespace(), oldCm); err != nil {
			logger.Error(err, "update acl configmap failed", "target", oldCm.Name)
			return actor.NewResultWithError(ops.CommandRequeue, err)
		}
		return actor.Requeue()
	}
	return nil
}
