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
	"time"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
	"github.com/chideat/valkey-operator/internal/config"
	cops "github.com/chideat/valkey-operator/internal/ops/cluster"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/actor"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/slot"
	"github.com/chideat/valkey-operator/pkg/types"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
)

var _ actor.Actor = (*actorCleanResource)(nil)

func init() {
	actor.Register(core.ValkeyCluster, NewCleanResourceActor)
}

func NewCleanResourceActor(client kubernetes.ClientSet, logger logr.Logger) actor.Actor {
	return &actorCleanResource{
		client: client,
		logger: logger,
	}
}

type actorCleanResource struct {
	client kubernetes.ClientSet
	logger logr.Logger
}

func (a *actorCleanResource) SupportedCommands() []actor.Command {
	return []actor.Command{cops.CommandCleanResource}
}

func (a *actorCleanResource) Version() *semver.Version {
	return semver.MustParse("3.14.0")
}

// Do
func (a *actorCleanResource) Do(ctx context.Context, val types.Instance) *actor.ActorResult {
	logger := val.Logger().WithValues("actor", cops.CommandCleanResource.String())

	cluster := val.(types.ClusterInstance)
	cr := cluster.Definition()

	var (
		failedNodeId = []string{}
		shards       = cluster.Shards()
	)

	for _, shard := range shards {
		if shard.Index() >= int(cr.Spec.Replicas.Shards) {
			// if not empty, break clean. should clean by order
			if !(shard.Slots().Count(slot.SlotAssigned) == 0 &&
				shard.Slots().Count(slot.SlotMigrating) == 0 &&
				shard.Slots().Count(slot.SlotImporting) == 0) {

				break
			}

			for _, node := range shard.Nodes() {
				failedNodeId = append(failedNodeId, node.ID())
			}
		}
	}

	if len(failedNodeId) > 0 {
		var margs [][]interface{}
		for _, id := range failedNodeId {
			margs = append(margs, []interface{}{"cluster", "forget", id})
		}
		for i := 0; i < int(cr.Spec.Replicas.Shards) && len(cluster.Shards()) >= int(cr.Spec.Replicas.Shards); i++ {
			shard := cluster.Shards()[i]
			for _, node := range shard.Nodes() {
				if err := node.Setup(ctx, margs...); err != nil {
					logger.Error(err, "forget invalid node failed", "node", node.ID())
				}
			}
		}

		time.Sleep(time.Second * 5)

		for _, shard := range shards {
			if shard.Index() >= int(cr.Spec.Replicas.Shards) {
				// if not empty, break clean. should clean by order
				if !(shard.Slots().Count(slot.SlotAssigned) == 0 &&
					shard.Slots().Count(slot.SlotMigrating) == 0 &&
					shard.Slots().Count(slot.SlotImporting) == 0) {

					break
				}

				// delete the shard
				name := clusterbuilder.ClusterStatefulSetName(cr.GetName(), shard.Index())
				logger.Info("clean resource", "statefulset", name)
				// NOTE: DeleteStatefulSet with GracePeriodSeconds=0
				if err := a.client.DeleteStatefulSet(ctx, cr.GetNamespace(), name, client.GracePeriodSeconds(0)); err != nil {
					a.logger.Error(err, "delete statefulset failed", "target", util.ObjectKey(cr.GetNamespace(), name))
				}

				for _, node := range shard.Nodes() {
					// delete pods
					if err := a.client.DeletePod(ctx, node.GetNamespace(), node.GetName(), client.GracePeriodSeconds(0)); err != nil {
						a.logger.Error(err, "force delete pod failed", "target", util.ObjectKey(cr.GetNamespace(), node.GetName()))
					}

					// delete cm
					name := "sync-" + node.GetName()
					if err := a.client.DeleteConfigMap(ctx, node.GetNamespace(), name); err != nil {
						a.logger.Error(err, "delete pod related nodes.conf configmap failed", "target", util.ObjectKey(cr.GetNamespace(), name))
					}
				}
				cluster.SendEventf(corev1.EventTypeNormal, config.EventCleanResource, "cleaned useless shard %s", name)
			}
		}
		return actor.NewResult(cops.CommandRequeue)
	}
	return nil
}
