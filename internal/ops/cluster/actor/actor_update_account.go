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
	"reflect"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/builder/aclbuilder"
	"github.com/chideat/valkey-operator/internal/config"
	cops "github.com/chideat/valkey-operator/internal/ops/cluster"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/actor"
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
	return semver.MustParse("0.1.0")
}

// Do
func (a *actorUpdateAccount) Do(ctx context.Context, val types.Instance) *actor.ActorResult {
	cluster := val.(types.ClusterInstance)
	logger := val.Logger().WithValues("actor", cops.CommandUpdateAccount.String())

	logger.Info("start update account", "cluster", cluster.GetName())

	var (
		err error

		users  = cluster.Users()
		opUser = users.GetOpUser()
	)

	name := aclbuilder.GenerateACLConfigMapName(cluster.Arch(), cluster.GetName())
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
		// create valkey acl file, after restart, the password is updated
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
	users = users[0:0]
	if opUser == nil {
		secretName := aclbuilder.GenerateACLOperatorSecretName(cluster.Arch(), cluster.GetName())
		ownRefs := util.BuildOwnerReferences(cluster.Definition())
		if opUser, err := acl.NewOperatorUser(ctx, a.client, secretName, cluster.GetNamespace(), ownRefs); err != nil {
			logger.Error(err, "create operator user failed")
			return actor.NewResult(cops.CommandRequeue)
		} else {
			users = append(users, opUser)
			isUpdated = true
		}
		user := aclbuilder.GenerateUser(cluster, opUser)
		if err := a.client.CreateIfNotExistsUser(ctx, user); err != nil {
			logger.Error(err, "create operator user failed")
			return actor.NewResult(cops.CommandRequeue)
		}
		cluster.SendEventf(corev1.EventTypeNormal, config.EventCreateUser, "created operator user to enable acl")
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
		return actor.Requeue()
	}
	return nil
}
