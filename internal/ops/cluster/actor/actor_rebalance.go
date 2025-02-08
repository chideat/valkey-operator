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
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/chideat/valkey-operator/api/core"
	cops "github.com/chideat/valkey-operator/internal/ops/cluster"
	"github.com/chideat/valkey-operator/pkg/actor"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/slot"
	"github.com/chideat/valkey-operator/pkg/types"
	vkcli "github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/go-logr/logr"
)

var _ actor.Actor = (*actorRebalance)(nil)

func init() {
	actor.Register(core.ValkeyCluster, NewRebalanceActor)
}

func NewRebalanceActor(client kubernetes.ClientSet, logger logr.Logger) actor.Actor {
	return &actorRebalance{
		client: client,
		logger: logger,
	}
}

type actorRebalance struct {
	client kubernetes.ClientSet

	logger logr.Logger
}

func (a *actorRebalance) Version() *semver.Version {
	return semver.MustParse("0.1.0")
}

// SupportedCommands
func (a *actorRebalance) SupportedCommands() []actor.Command {
	return []actor.Command{cops.CommandRebalance}
}

func (a *actorRebalance) moveSlot(ctx context.Context, destNode, srcNode types.ValkeyNode, slot int) *actor.ActorResult {
	destId := destNode.ID()
	sourceId := srcNode.ID()
	if err := destNode.Setup(ctx, []interface{}{"CLUSTER", "SETSLOT", slot, "IMPORTING", sourceId}); err != nil {
		a.logger.Error(err, "setup importing failed", "slot", slot, "source", sourceId, "dest", destId)
		return actor.NewResultWithError(cops.CommandRequeue, err)
	}
	if err := srcNode.Setup(ctx, []interface{}{"CLUSTER", "SETSLOT", slot, "MIGRATING", destId}); err != nil {
		a.logger.Error(err, "setup migrating failed", "slot", slot, "source", sourceId, "dest", destId)
		return actor.NewResultWithError(cops.CommandRequeue, err)
	}
	return nil
}

func (a *actorRebalance) stableSlot(ctx context.Context, node types.ValkeyNode, slots ...int) *actor.ActorResult {
	var args [][]any
	for _, slot := range slots {
		args = append(args, []any{"CLUSTER", "SETSLOT", slot, "STABLE"})
	}
	if err := node.Setup(ctx, args...); err != nil {
		a.logger.Error(err, "stable slots failed", "slots", slots)
		return actor.NewResultWithError(cops.CommandRequeue, err)
	}
	return nil
}

func (a *actorRebalance) findNodeWithMostSlots(nodes ...types.ValkeyNode) types.ValkeyNode {
	if len(nodes) == 0 {
		return nil
	}
	if len(nodes) == 1 {
		return nodes[0]
	}

	var (
		nodeWithMostSlots = nodes[0]
		nodeWithImporting types.ValkeyNode
	)
	// find the node with most slots
	for _, node := range nodes[1:] {
		if node.Slots().Count(slot.SlotImporting) > 0 {
			nodeWithImporting = node
			break
		}
		if node.Slots().Count(slot.SlotAssigned) > nodeWithMostSlots.Slots().Count(slot.SlotAssigned) {
			nodeWithMostSlots = node
		}
	}

	if nodeWithImporting != nil {
		return nodeWithImporting
	}
	return nodeWithMostSlots
}

type SlotMigrateStatus struct {
	Slot          int
	SourceShard   int
	SourceLabeled bool // true if source shard is labeled in node
	DestShard     int
	DestLabeled   bool // true if dest shard is labeled in node
}

// Do
//
// 关于槽迁移，参考：https://redis.io/commands/cluster-setslot/
// Valkey 的槽迁移比较恶心，要先标记状态，然后手动迁移数据，然后再去除标记，最后在广播
//
// 槽迁移实现
// 由于槽迁移是一个标记然后后台任务一直执行的过程，为了槽迁移的健壮性，将槽迁移的任务进行拆分
// 1. operator 部分：operator 只负责标记哪些槽要迁移
// 2. sidecar: sidercar 用于按照标记信息迁移槽，并在数据迁移完成之后，清理标记
// 3. 即使在槽迁移过程中 node 重启或者关机(可能会数据丢失)，operator 会重新标记，sidecar 会重新进行迁移
func (a *actorRebalance) Do(ctx context.Context, val types.Instance) *actor.ActorResult {
	cluster := val.(types.ClusterInstance)
	logger := val.Logger().WithValues("actor", cops.CommandRebalance.String())

	if err := cluster.Refresh(ctx); err != nil {
		logger.V(3).Error(err, "refresh cluster failed")
		return actor.NewResultWithError(cops.CommandRequeue, err)
	}

	if !cluster.IsReady() {
		logger.Info("cluster is not ready")
		return actor.NewResult(cops.CommandEnsureResource)
	}

	// check if slots fullfilled
	var (
		allSlots    = slot.NewSlots()
		shardsSlots = map[int]types.ClusterShard{}
		nodes       = map[string]types.ValkeyNode{}
	)
	for _, shard := range cluster.Shards() {
		allSlots = allSlots.Union(shard.Slots())
		shardsSlots[shard.Index()] = shard

		for _, node := range shard.Nodes() {
			if node.ID() != "" {
				nodes[node.ID()] = node
			}
		}
	}

	// NOTE: 这里的实现基于实例的槽信息的一致性，所以必须强制要求槽无错误
	// 同时还要求 cr.status.shards 中记录的槽信息记录的一致性
	if !allSlots.IsFullfilled() {
		// check if some shard got multi master, if so, do slot migrate
		var nodes []types.ValkeyNode
		for _, shard := range cluster.Shards() {
			nodes = nodes[0:0]
			for _, node := range shard.Nodes() {
				if node.Role() == core.NodeRoleMaster && node.Slots().Count(slot.SlotAssigned) > 0 {
					nodes = append(nodes, node)
				}
			}
			if len(nodes) > 1 {
				// do slot migrate, check use with node as the final node
				destNode := a.findNodeWithMostSlots(nodes...)
				for _, node := range nodes {
					if node == destNode {
						continue
					}
					for _, slotId := range node.Slots().Slots() {
						if result := a.moveSlot(ctx, destNode, node, slotId); result != nil {
							return result
						}
					}
				}
			}
		}
		logger.V(3).Info("slots not fullfilled, try to ensure slots")
		return actor.NewResult(cops.CommandEnsureSlots)
	}

	// check if there is a situation where the migration object is not a master due to failover
	nodesWithWrongMigrateFlag := map[types.ValkeyNode][]int{}
	for _, shard := range cluster.Shards() {
		nodeSlots := shard.Slots()
		for _, slot := range nodeSlots.SlotsByStatus(slot.SlotMigrating) {
			if _, nodeId := nodeSlots.MoveingStatus(slot); nodeId == "" {
				continue
			} else if node := nodes[nodeId]; node != nil && node.Role() != core.NodeRoleMaster {
				nodesWithWrongMigrateFlag[shard.Master()] = append(nodesWithWrongMigrateFlag[shard.Master()], slot)
			}
		}
		for _, slot := range nodeSlots.SlotsByStatus(slot.SlotImporting) {
			if _, nodeId := nodeSlots.MoveingStatus(slot); nodeId == "" {
				continue
			} else if node := nodes[nodeId]; node != nil && node.Role() != core.NodeRoleMaster {
				nodesWithWrongMigrateFlag[shard.Master()] = append(nodesWithWrongMigrateFlag[shard.Master()], slot)
			}
		}
	}
	if len(nodesWithWrongMigrateFlag) > 0 {
		for node, slots := range nodesWithWrongMigrateFlag {
			logger.V(3).Info("reset node with wrong slots migrate flag", "node", node.ID(), "slots", slots)
			if result := a.stableSlot(ctx, node, slots...); result != nil {
				return result
			}
		}
		return actor.NewResult(cops.CommandJoinNode)
	}

	// check if there exists master node cluster_state:fail
	// if so, stable all importing slots
	for _, shard := range cluster.Shards() {
		master := shard.Master()
		slotsIndex := master.Slots().SlotsByStatus(slot.SlotImporting)
		if len(slotsIndex) > 0 && master.ClusterInfo().ClusterState == vkcli.ClusterStateFail {
			logger.Error(fmt.Errorf("importing master node %s cluster_state is fail", master.ID()), "master node cluster_state is fail")
			if result := a.stableSlot(ctx, master, slotsIndex...); result != nil {
				return result
			}
			return actor.NewResult(cops.CommandRequeue)
		}
	}

	var (
		migrateAggMapping = map[int]*SlotMigrateStatus{}
		migrateAgg        = []*SlotMigrateStatus{}
	)
	// 幂等标记槽的迁移状态
	for _, shardStatus := range cluster.Definition().Status.Shards {
		shard := shardsSlots[int(shardStatus.Index)]
		currentSlots := shard.Slots()

		for _, status := range shardStatus.Slots {
			as := slot.NewSlotAssignStatusFromString(status.Status)
			if as == slot.SlotAssigned || as == slot.SlotUnassigned {
				continue
			}

			// this should not happen
			if status.ShardIndex == nil {
				return actor.NewResultWithError(cops.CommandRequeue, fmt.Errorf("nil shard field"))
			}

			slots := slot.NewSlots()
			_ = slots.Set(status.Slots, slot.SlotAssigned)
			if as == slot.SlotMigrating {
				for _, slotIndex := range slots.Slots() {
					if currentSlots.Status(slotIndex) != slot.SlotAssigned &&
						currentSlots.Status(slotIndex) != slot.SlotMigrating {
						continue
					}

					if status := migrateAggMapping[slotIndex]; status == nil {
						status = &SlotMigrateStatus{
							Slot:          slotIndex,
							SourceShard:   shard.Index(),
							SourceLabeled: currentSlots.Status(slotIndex) == slot.SlotMigrating,
							DestShard:     -1,
						}
						migrateAggMapping[slotIndex] = status
						migrateAgg = append(migrateAgg, status)
					} else {
						status.Slot = slotIndex
						status.SourceShard = shard.Index()
						status.SourceLabeled = currentSlots.Status(slotIndex) == slot.SlotMigrating
					}
				}
			} else if as == slot.SlotImporting {
				for _, slotIndex := range slots.Slots() {
					if currentSlots.Status(slotIndex) != slot.SlotUnassigned &&
						currentSlots.Status(slotIndex) != slot.SlotImporting {
						continue
					}

					if status := migrateAggMapping[slotIndex]; status == nil {
						status = &SlotMigrateStatus{
							Slot:        slotIndex,
							SourceShard: -1,
							DestShard:   shard.Index(),
							DestLabeled: currentSlots.Status(slotIndex) == slot.SlotImporting,
						}
						migrateAggMapping[slotIndex] = status
						migrateAgg = append(migrateAgg, status)
					} else {
						status.Slot = slotIndex
						status.DestShard = shard.Index()
						status.DestLabeled = currentSlots.Status(slotIndex) == slot.SlotImporting
					}
				}
			}
		}
	}

	// TODO: before migrate, check if the new masters are healthy

	count := 0
	for _, status := range migrateAgg {
		if status.SourceShard == -1 || status.DestShard == -1 {
			// BUG: cr.status.shards 记录的槽信息出现不一致性
			a.logger.Info("slots info record in status is not consistent", "slot", status.Slot)
			continue
		}

		// ignored if already labeled
		if status.SourceLabeled && status.DestLabeled {
			continue
		}

		sourceNode := shardsSlots[status.SourceShard].Master()
		destNode := shardsSlots[status.DestShard].Master()

		// setup importing first
		if err := a.moveSlot(ctx, destNode, sourceNode, status.Slot); err != nil {
			return err
		}
		count += 1
	}

	if count > 0 {
		return actor.NewResultWithValue(cops.CommandRequeue, time.Second*5)
	}
	return nil
}
