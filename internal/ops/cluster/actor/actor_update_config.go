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

	"github.com/Masterminds/semver/v3"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/actor"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
	cops "github.com/chideat/valkey-operator/internal/ops/cluster"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ actor.Actor = (*actorUpdateConfig)(nil)

func init() {
	actor.Register(core.ValkeyCluster, NewUpdateConfigActor)
}

func NewUpdateConfigActor(client kubernetes.ClientSet, logger logr.Logger) actor.Actor {
	return &actorUpdateConfig{
		client: client,
		logger: logger,
	}
}

type actorUpdateConfig struct {
	client kubernetes.ClientSet

	logger logr.Logger
}

// SupportedCommands
func (a *actorUpdateConfig) SupportedCommands() []actor.Command {
	return []actor.Command{cops.CommandUpdateConfig}
}

func (a *actorUpdateConfig) Version() *semver.Version {
	return semver.MustParse("0.1.0")
}

// Do
//
// two type config: hotconfig and restartconfig
// use cm to check the difference of the config
func (a *actorUpdateConfig) Do(ctx context.Context, val types.Instance) *actor.ActorResult {
	logger := val.Logger().WithValues("actor", cops.CommandUpdateConfig.String())

	cluster := val.(types.ClusterInstance)
	newCm, _ := clusterbuilder.NewConfigMapForCR(cluster)
	oldCm, err := a.client.GetConfigMap(ctx, newCm.Namespace, newCm.Name)
	if err != nil && !errors.IsNotFound(err) {
		logger.Error(err, "get configmap failed", "target", client.ObjectKeyFromObject(newCm))
		return actor.NewResultWithError(cops.CommandRequeue, err)
	} else if oldCm == nil || oldCm.Data[builder.ValkeyConfigKey] == "" {
		if err = a.client.CreateConfigMap(ctx, cluster.GetNamespace(), newCm); err != nil {
			logger.Error(err, "create configmap failed", "target", client.ObjectKeyFromObject(newCm))
			return actor.NewResultWithError(cops.CommandRequeue, err)
		}
		return nil
	}

	// check if config changed
	newConf, _ := clusterbuilder.LoadValkeyConfig(newCm.Data[builder.ValkeyConfigKey])
	oldConf, _ := clusterbuilder.LoadValkeyConfig(oldCm.Data[builder.ValkeyConfigKey])
	added, changed, deleted := oldConf.Diff(newConf)

	if len(deleted) > 0 || len(added) > 0 || len(changed) > 0 {
		// NOTE: update configmap first may cause the hot config fail for it will not retry again
		if err := a.client.UpdateConfigMap(ctx, cluster.GetNamespace(), newCm); err != nil {
			logger.Error(err, "update config failed", "target", client.ObjectKeyFromObject(newCm))
			return actor.NewResultWithError(cops.CommandRequeue, err)
		}
	}

	for k, v := range added {
		changed[k] = v
	}
	if len(changed) == 0 {
		return nil
	}

	foundRestartApplyConfig := false
	for key := range changed {
		if policy := clusterbuilder.ValkeyConfigRestartPolicy[key]; policy == clusterbuilder.RequireRestart {
			foundRestartApplyConfig = true
			break
		}
	}

	if foundRestartApplyConfig {
		logger.Info("rolling restart all shard")
		// NOTE: the restart is done by RDS
		// rolling update all statefulset
		// if err := cluster.Restart(ctx); err != nil {
		// 	logger.Error(err, "restart instance failed")
		// }
		return actor.NewResult(cops.CommandEnsureResource)
	} else {
		var margs [][]interface{}
		for key, vals := range changed {
			logger.V(2).Info("hot config ", "key", key, "value", vals.String())
			margs = append(margs, []interface{}{"config", "set", key, vals.String()})
		}

		var (
			isUpdateFailed = false
			err            error
		)
		for _, node := range cluster.Nodes() {
			if node.ContainerStatus() == nil || !node.ContainerStatus().Ready ||
				node.IsTerminating() {
				continue
			}
			if err = node.Setup(ctx, margs...); err != nil {
				isUpdateFailed = true
				break
			}
		}

		if !isUpdateFailed {
			return actor.NewResultWithError(cops.CommandRequeue, err)
		}
	}
	return nil
}
