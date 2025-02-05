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
	"github.com/chideat/valkey-operator/internal/config"
	cops "github.com/chideat/valkey-operator/internal/ops/cluster"
	"github.com/chideat/valkey-operator/pkg/actor"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/slot"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
)

var _ actor.Actor = (*actorEnsureSlots)(nil)

func init() {
	actor.Register(core.ValkeyCluster, NewEnsureSlotsActor)
}

func NewEnsureSlotsActor(client kubernetes.ClientSet, logger logr.Logger) actor.Actor {
	return &actorEnsureSlots{
		client: client,
		logger: logger,
	}
}

type actorEnsureSlots struct {
	client kubernetes.ClientSet

	logger logr.Logger
}

// SupportedCommands
func (a *actorEnsureSlots) SupportedCommands() []actor.Command {
	return []actor.Command{cops.CommandEnsureSlots}
}

func (a *actorEnsureSlots) Version() *semver.Version {
	return semver.MustParse("0.1.0")
}

// Do
// 该 Actor 处理槽分配与槽丢失的情况, 槽的迁移由 Rebanalce + sidecar 处理
// 处理逻辑如下：
// 判断是否有 master 节点存在，如果无 master 节点，则 failover
// 槽的分配和迁移只根据 cr.status.shards 中预分配的信息来，如果发现某个槽丢失了，以下逻辑会自动添加回来
// 特殊情况：
//
//	如果发现某个槽被意外移到了其他节点，该槽不会被移动回来，operator 不会处理这个情况，并保留这个状态。在下一个 Reconcile 中，operator 会在cluster inservice 时，刷新 cr.status.shards 信息
func (a *actorEnsureSlots) Do(ctx context.Context, val types.Instance) *actor.ActorResult {
	cluster := val.(types.ClusterInstance)
	cr := cluster.Definition()
	logger := val.Logger().WithValues("actor", cops.CommandEnsureSlots.String())

	// force refresh the cluster
	if err := cluster.Refresh(ctx); err != nil {
		logger.Error(err, "refresh cluster info failed")
		return actor.NewResultWithError(cops.CommandRequeue, err)
	}
	if len(cluster.Shards()) == 0 {
		return actor.NewResult(cops.CommandEnsureResource)
	}

	// check is slots fullfilled
	var (
		allSlots    = slot.NewSlots()
		shardsSlots = map[int]types.ClusterShard{}
	)
	for i, shard := range cluster.Shards() {
		if i != shard.Index() {
			return actor.NewResult(cops.CommandEnsureResource)
		}
		allSlots = allSlots.Union(shard.Slots())
		shardsSlots[shard.Index()] = shard
	}
	if allSlots.IsFullfilled() {
		return nil
	}

	var (
		failedShards        []types.ClusterShard
		importingSlotTarget = map[int]types.ClusterShard{}
	)

	// Only masters will slots can vote for the slave to promote to be a master
	for _, statusShard := range cr.Status.Shards {
		var shard types.ClusterShard
		for _, cs := range cluster.Shards() {
			if cs.Index() == int(statusShard.Index) {
				shard = cs
				break
			}
		}
		for _, status := range statusShard.Slots {
			if status.Status == slot.SlotImporting.String() {
				imslots, _ := slot.LoadSlots(status.Slots)
				for _, slot := range imslots.Slots() {
					importingSlotTarget[slot] = shard
				}
			}
		}
	}

	// check shard master info
	// NOTE: here not check the case when the last statefulset is removed
	for i := 0; i < len(cluster.Shards()); i++ {
		shard := cluster.Shards()[i]
		if shard.Index() != i {
			// some shard missing, fix them first. this should not happen here
			return actor.NewResult(cops.CommandEnsureResource)
		}

		// check if master exists
		if shard.Master() == nil {
			if len(shard.Replicas()) == 0 {
				// pod missing
				return actor.NewResult(cops.CommandHealPod)
			}
			failedShards = append(failedShards, shard)
		}
	}

	if len(failedShards) > 0 {
		for _, shard := range failedShards {
			if err := shard.Refresh(ctx); err != nil {
				logger.Error(err, "refresh shard info failed", "shard", shard.GetName())
				continue
			}
			if shard.Master() != nil {
				continue
			}

			for _, node := range shard.Replicas() {
				if !node.IsReady() && node.Role() != core.NodeRoleReplica {
					continue
				}

				// disable takeover when shard in importing or migrating
				if a.doFailover(ctx, node, 10, !shard.IsImporting() && !shard.IsMigrating(), logger) == nil {
					cluster.SendEventf(corev1.EventTypeWarning, config.EventFailover, "healed shard %s with new master %s", shard.GetName(), node.GetName())
					if err := shard.Refresh(ctx); err != nil {
						logger.Error(err, "refresh shard info failed", "shard", shard.GetName())
						return actor.NewResultWithError(cops.CommandRequeue, err)
					} else if shard.Master() == nil {
						logger.Error(fmt.Errorf("failover failed"), "no master found after ENSURE failover", "shard", shard.GetName())
						return actor.NewResult(cops.CommandRequeue)
					}
				}
				break
			}
			// continue do failover with other shards
		}
		return actor.NewResult(cops.CommandRequeue)
	}

	needRefresh := false
	for _, shardStatus := range cr.Status.Shards {
		assignedSlots := slot.NewSlots()
		for _, status := range shardStatus.Slots {
			switch status.Status {
			case slot.SlotAssigned.String():
				_ = assignedSlots.Set(status.Slots, slot.SlotAssigned)
			case slot.SlotMigrating.String():
				// if the slot is migrating, the operator will not assgin it back
				_ = assignedSlots.Set(status.Slots, slot.SlotUnassigned)
			}
		}
		shard := shardsSlots[int(shardStatus.Index)]
		shardSlots := shard.Slots()
		node := shard.Master()
		if err := node.Refresh(ctx); err != nil {
			logger.Error(err, "refresh node failed")
			return actor.NewResultWithError(cops.CommandRequeue, err)
		}
		if node.Role() != core.NodeRoleMaster {
			continue
		}

		missingSlots := assignedSlots.Sub(shardSlots).Sub(allSlots)
		missingSlotsIndex := missingSlots.Slots()
		logger.Info("missing slots", "shardIndex", shardStatus.Index, "slots", shardStatus.Slots)
		for _, slotIndex := range missingSlotsIndex {
			if importShard := importingSlotTarget[slotIndex]; importShard != nil && importShard.Master() != nil && importShard.Master().ID() != "" {
				// set the slot to importing
				args := []interface{}{"CLUSTER", "SETSLOT", slotIndex, "NODE", shard.Master().ID()}
				if err := importShard.Master().Setup(ctx, args); err == nil {
					_ = missingSlots.Set(slotIndex, slot.SlotUnassigned)
					for _, shard := range cluster.Shards() {
						if shard.Index() == importShard.Index() {
							continue
						}
						if shard.Master() != nil {
							if err := shard.Master().Setup(ctx, args); err != nil {
								logger.Error(err, "set slot assigned failed", "shard", shard.GetName(), "node", node.GetName(), "slot", slotIndex)
							}
						}
					}
					logger.Info("set slot assignment fixed", "shard", shard.GetName(), "node", node.GetName(), "slot", slotIndex)
				} else {
					logger.Error(err, "set slot assignment failed", "shard", shard.GetName(), "slot", slotIndex)
				}
			}
		}

		missingSlotsIndex = missingSlots.Slots()
		if len(missingSlotsIndex) > 0 {
			if len(missingSlotsIndex) < shardSlots.Count(slot.SlotAssigned) {
				logger.Info("WARNING: as if the slots have been moved unexpectedly, this may case the cluster run in an unbalance state")
			}
			var (
				args          = []interface{}{"CLUSTER", "ADDSLOTS"}
				assginedSlots = slot.NewSlots()
			)
			for _, s := range missingSlotsIndex {
				args = append(args, s)
				_ = assginedSlots.Set(s, slot.SlotAssigned)
			}
			if err := node.Setup(ctx, args); err != nil {
				logger.Error(err, "assign slots to shard failed", "shard", shard.GetName())
				return actor.NewResultWithError(cops.CommandRequeue, err)
			}
			cluster.SendEventf(corev1.EventTypeNormal, config.EventAssignSlots, "setup/fix missing slots %v to node %s", assginedSlots.String(), node.GetName())
			needRefresh = true
		}
	}

	if needRefresh {
		// force refresh the cluster
		if err := cluster.Refresh(ctx); err != nil {
			logger.Error(err, "refresh cluster info failed")
			return actor.NewResultWithError(cops.CommandRequeue, err)
		}
	}
	return nil
}

func (a *actorEnsureSlots) doFailover(ctx context.Context, node types.ValkeyNode, retry int, ensure bool, logger logr.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	args := []interface{}{"CLUSTER", "FAILOVER", "FORCE"}
	for i := 0; i < retry+1; i++ {
		logger.Info("do shard failover", "node", node.GetName(), "action", args[2])
		if err := node.Setup(ctx, args); err != nil {
			logger.Error(err, "do failover failed", "node", node.GetName())
			return err
		}

		for j := 0; j < 3; j++ {
			time.Sleep(time.Second * 2)
			if err := node.Refresh(ctx); err != nil {
				logger.Error(err, "refresh node info failed")
				return err
			}
			if node.Role() == core.NodeRoleMaster || (node.Role() == core.NodeRoleReplica && node.IsMasterLinkUp()) {
				return nil
			}
		}
	}

	if ensure {
		if err := node.Refresh(ctx); err != nil {
			logger.Error(err, "refresh node info failed")
			return err
		}

		args[2] = "TAKEOVER"
		logger.Info("do shard failover", "node", node.GetName(), "action", args[2])
		if err := node.Setup(ctx, args); err != nil {
			logger.Error(err, "do failover failed", "node", node.GetName())
			return err
		}

		for j := 0; j < 3; j++ {
			time.Sleep(time.Second * 2)
			if err := node.Refresh(ctx); err != nil {
				logger.Error(err, "refresh node info failed")
				return err
			}
			if node.Role() == core.NodeRoleMaster || (node.Role() == core.NodeRoleReplica && node.IsMasterLinkUp()) {
				return nil
			}
		}
		return fmt.Errorf("takeover failver failed")
	} else {
		return fmt.Errorf("force failover failed")
	}
}
