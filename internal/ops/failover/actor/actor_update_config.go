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
	"maps"

	"github.com/Masterminds/semver/v3"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/actor"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/failoverbuilder"
	ops "github.com/chideat/valkey-operator/internal/ops/failover"
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
	return semver.MustParse("0.1.0")
}

func (a *actorUpdateConfigMap) Do(ctx context.Context, val types.Instance) *actor.ActorResult {
	logger := val.Logger().WithValues("actor", ops.CommandUpdateConfig.String())

	st := val.(types.FailoverInstance)
	newCm, err := failoverbuilder.GenerateConfigMap(st)
	if err != nil {
		return actor.NewResultWithError(ops.CommandRequeue, err)
	}
	oldCm, err := a.client.GetConfigMap(ctx, newCm.GetNamespace(), newCm.GetName())
	if errors.IsNotFound(err) || oldCm.Data[builder.ValkeyConfigKey] == "" {
		return actor.NewResultWithError(ops.CommandEnsureResource, fmt.Errorf("configmap %s not found", newCm.GetName()))
	} else if err != nil {
		return actor.RequeueWithError(err)
	}

	var (
		newConf builder.ValkeyConfig
		oldConf builder.ValkeyConfig
	)
	if lastAppliedConf := oldCm.Annotations[builder.LastAppliedConfigAnnotationKey]; lastAppliedConf != "" {
		newConf, _ = builder.LoadValkeyConfig(newCm.Data[builder.ValkeyConfigKey])
		oldConf, _ = builder.LoadValkeyConfig(lastAppliedConf)
	} else {
		newConf, _ = builder.LoadValkeyConfig(newCm.Data[builder.ValkeyConfigKey])
		oldConf, _ = builder.LoadValkeyConfig(oldCm.Data[builder.ValkeyConfigKey])
	}

	added, changed, deleted := oldConf.Diff(newConf)
	maps.Copy(changed, added)

	if len(deleted)+len(changed) > 0 {
		conf := newCm.DeepCopy()
		if lastAppliedConf := oldCm.Annotations[builder.LastAppliedConfigAnnotationKey]; lastAppliedConf != "" {
			conf.Annotations[builder.LastAppliedConfigAnnotationKey] = lastAppliedConf
		} else {
			conf.Annotations[builder.LastAppliedConfigAnnotationKey] = oldCm.Data[builder.ValkeyConfigKey]
		}

		// update configmap with last applied config
		if err := a.client.UpdateConfigMap(ctx, conf.GetNamespace(), conf); err != nil {
			logger.Error(err, "update config failed", "target", client.ObjectKeyFromObject(conf))
			return actor.NewResultWithError(ops.CommandRequeue, err)
		}
	}

	if len(changed) > 0 {
		foundRestartApplyConfig := false
		for key := range changed {
			if policy := builder.ValkeyConfigRestartPolicy[key]; policy == builder.RequireRestart {
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
			var margs [][]any
			for key, vals := range changed {
				logger.V(2).Info("hot config ", "key", key, "value", vals.String())
				margs = append(margs, []any{"config", "set", key, vals.String()})
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

			if isUpdateFailed {
				return actor.NewResultWithError(ops.CommandRequeue, err)
			}
		}
	}

	// update configmap without last applied config
	if err := a.client.UpdateConfigMap(ctx, newCm.GetNamespace(), newCm); err != nil {
		logger.Error(err, "update config failed", "target", client.ObjectKeyFromObject(newCm))
		return actor.NewResultWithError(ops.CommandRequeue, err)
	}

	return nil
}
