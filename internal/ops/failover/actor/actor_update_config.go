/*
Copyright 2023 The RedisOperator Authors.

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

	"github.com/Masterminds/semver/v3"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
	"github.com/chideat/valkey-operator/internal/builder/failoverbuilder"
	ops "github.com/chideat/valkey-operator/internal/ops/failover"
	"github.com/chideat/valkey-operator/pkg/actor"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ actor.Actor = (*actorUpdateConfigMap)(nil)

func init() {
	actor.Register(core.ValkeyFailover, NewSentinelUpdateConfig)
}

func NewSentinelUpdateConfig(client kubernetes.ClientSet, logger logr.Logger) actor.Actor {
	return &actorUpdateConfigMap{
		client: client,
		logger: logger,
	}
}

type actorUpdateConfigMap struct {
	client kubernetes.ClientSet
	logger logr.Logger
}

func (a *actorUpdateConfigMap) SupportedCommands() []actor.Command {
	return []actor.Command{ops.CommandUpdateConfig}
}

func (a *actorUpdateConfigMap) Version() *semver.Version {
	return semver.MustParse("3.18.0")
}

func (a *actorUpdateConfigMap) Do(ctx context.Context, val types.RedisInstance) *actor.ActorResult {
	logger := val.Logger().WithValues("actor", ops.CommandUpdateConfig.String())

	st := val.(types.RedisFailoverInstance)
	selectors := st.Selector()
	newCm, err := failoverbuilder.NewRedisConfigMap(st, selectors)
	if err != nil {
		return actor.NewResultWithError(ops.CommandRequeue, err)
	}
	oldCm, err := a.client.GetConfigMap(ctx, newCm.GetNamespace(), newCm.GetName())
	if errors.IsNotFound(err) || oldCm.Data[clusterbuilder.RedisConfKey] == "" {
		return actor.NewResultWithError(ops.CommandEnsureResource, fmt.Errorf("configmap %s not found", newCm.GetName()))
	} else if err != nil {
		return actor.NewResultWithError(ops.CommandRequeue, err)
	}
	newConf, _ := clusterbuilder.LoadRedisConfig(newCm.Data[clusterbuilder.RedisConfKey])
	oldConf, _ := clusterbuilder.LoadRedisConfig(oldCm.Data[clusterbuilder.RedisConfKey])
	added, changed, deleted := oldConf.Diff(newConf)
	if len(deleted) > 0 || len(added) > 0 || len(changed) > 0 {
		// NOTE: update configmap first may cause the hot config fail for it will not retry again
		if err := a.client.UpdateConfigMap(ctx, newCm.GetNamespace(), newCm); err != nil {
			logger.Error(err, "update config failed", "target", client.ObjectKeyFromObject(newCm))
			return actor.NewResultWithError(ops.CommandRequeue, err)
		}
	}
	for k, v := range added {
		changed[k] = v
	}
	foundRestartApplyConfig := false
	for key := range changed {
		if policy := clusterbuilder.RedisConfigRestartPolicy[key]; policy == clusterbuilder.RequireRestart {
			foundRestartApplyConfig = true
			break
		}
	}
	if foundRestartApplyConfig {
		err := st.Restart(ctx)
		if err != nil {
			logger.Error(err, "restart redis failed")
			return actor.NewResultWithError(ops.CommandRequeue, err)
		}
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
		for _, node := range st.Nodes() {
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
			return actor.NewResultWithError(ops.CommandRequeue, err)
		}
	}
	return nil
}
