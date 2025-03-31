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
	"github.com/chideat/valkey-operator/internal/config"
	ops "github.com/chideat/valkey-operator/internal/ops/sentinel"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
)

var _ actor.Actor = (*actorHealMonitor)(nil)

func init() {
	actor.Register(core.ValkeySentinel, NewHealMonitorActor)
}

func NewHealMonitorActor(client kubernetes.ClientSet, logger logr.Logger) actor.Actor {
	return &actorHealMonitor{
		client: client,
		logger: logger,
	}
}

type actorHealMonitor struct {
	client kubernetes.ClientSet
	logger logr.Logger
}

func (a *actorHealMonitor) SupportedCommands() []actor.Command {
	return []actor.Command{ops.CommandHealMonitor}
}

func (a *actorHealMonitor) Version() *semver.Version {
	return semver.MustParse("0.1.0")
}

func (a *actorHealMonitor) Do(ctx context.Context, val types.Instance) *actor.ActorResult {
	logger := val.Logger().WithValues("actor", ops.CommandHealMonitor.String())
	inst := val.(types.SentinelInstance)

	// NOTE: only try to heal sentinel monitoring clusters when all nodes of sentinel is ready
	if !inst.Replication().IsReady() {
		logger.Info("resource is not ready")
		return actor.NewResult(ops.CommandHealPod)
	}

	clusters, err := inst.Clusters(ctx)
	if err != nil {
		logger.Error(err, "failed to get monitoring clusters")
		return actor.NewResult(ops.CommandHealMonitor)
	}

	unknownSentinels, _ := ops.FindUnknownSentinel(ctx, inst, logger)
	if len(unknownSentinels) > 0 {
		var (
			user = inst.Users().GetOpUser()
			tls  = inst.TLSConfig()
		)
		for name, nodes := range unknownSentinels {
			// set sentinels
			for _, node := range inst.Nodes() {
				args := []any{"SENTINEL", "REMOVE", name}
				if err := node.Setup(ctx, args); err != nil {
					logger.Error(err, "failed to remove sentinel", "name", name)
				} else {
					logger.Info("remove monitoring cluster with sentinel", "name", name, "node", node.GetName())
				}

			}
			for _, addr := range nodes {
				logger.Info("found unknown sentinel", "name", name, "addr", addr)
				// connect to unknown sentinel and reset with name
				func() {
					cli := valkey.NewValkeyClient(addr, valkey.AuthConfig{
						Username:  user.Name,
						Password:  user.GetPassword().String(),
						TLSConfig: tls,
					})
					defer cli.Close()

					if _, err := cli.Do(ctx, "SENTINEL", "REMOVE", name); err != nil {
						logger.Error(err, "failed to remove monitoring cluster with sentinel", "name", name, "addr", addr)
					} else {
						logger.Info("remove monitoring cluster with sentinel", "name", name, "addr", addr)
					}
				}()
			}
			inst.SendEventf(corev1.EventTypeWarning, config.EventCleanResource,
				"force reset sentinels %s, belong with unknown sentinels", name)
		}
	}

	// list all sentinels
	for _, name := range clusters {
		needReset := func() bool {
			for _, node := range inst.Nodes() {
				if needReset, _ := ops.NeedResetSentinel(ctx, name, node, logger); needReset {
					return true
				}
			}
			return false
		}()

		if needReset {
			// reset all nodes
			for _, node := range inst.Nodes() {
				args := []any{"SENTINEL", "RESET", name}
				if err := node.Setup(ctx, args); err != nil {
					logger.Error(err, "failed to reset sentinel", "name", name)
				}
			}
			inst.SendEventf(corev1.EventTypeWarning, config.EventCleanResource,
				"force reset sentinels %s", name)
		}
	}
	return nil

}
