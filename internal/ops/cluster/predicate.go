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

package cluster

import (
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/pkg/slot"
	"github.com/chideat/valkey-operator/pkg/types"
)

// ShardAssignedSlots returns the union of slots owned by the shard's master-role
// nodes, read from each node's OWN cluster-nodes view (node.Slots()) WITHOUT requiring
// the node to currently see its peers. Unlike types.ClusterShard.Slots() — which is
// gated on node.IsJoined() and returns nil while peers are unreachable — this reflects
// the slots a shard actually owns even after a full restart when peers are still
// unreachable (stale nodes.conf IPs). The engine uses it so it does not mistake a
// disconnected-but-intact shard for a slotless one and trigger spurious slot
// reassignment.
func ShardAssignedSlots(shard types.ClusterShard) *slot.Slots {
	slots := slot.NewSlots()
	if shard == nil {
		return slots
	}
	for _, node := range shard.Nodes() {
		if node.Role() == core.NodeRoleMaster {
			if ns := node.Slots(); ns != nil {
				slots = slots.Union(ns)
			}
		}
	}
	return slots
}

// ShardHasMaster reports whether the shard has a master-role node that owns slots
// (assigned or importing), regardless of peer connectivity (IsJoined). It distinguishes
// a disconnected-but-intact master (its peers are unreachable, but it still owns its
// slots) from a genuinely missing master (dead pod / all replicas), for which failover
// remains appropriate.
func ShardHasMaster(shard types.ClusterShard) bool {
	if shard == nil {
		return false
	}
	for _, node := range shard.Nodes() {
		if node.Role() == core.NodeRoleMaster &&
			(node.Slots().Count(slot.SlotAssigned) > 0 || node.Slots().Count(slot.SlotImporting) > 0) {
			return true
		}
	}
	return false
}

// IsDisconnectedButIntact reports whether the cluster has lost gossip connectivity
// (some node reports cluster_state != ok and cannot see its peers) while every slot is
// still owned locally by some master. In this state the only safe action is to (re-)MEET
// and wait for reconnection: reassigning slots or forcing failover on mutually-unreachable
// nodes creates epoch/slot conflicts that prevent convergence (and risk split-brain).
//
// A genuine slot loss (a dead master leaving slots uncovered) fails the IsFullfilled()
// check and is excluded, so legitimate failover/assignment still runs. Connected
// operations (rebalance / scale) have IsJoined()==true for all nodes and are excluded by
// the hasDisconnected term.
func IsDisconnectedButIntact(cluster types.ClusterInstance) bool {
	if cluster == nil {
		return false
	}
	hasDisconnected := false
	allSlots := slot.NewSlots()
	for _, shard := range cluster.Shards() {
		for _, node := range shard.Nodes() {
			if node.ID() == "" || node.IsTerminating() {
				continue
			}
			if !node.IsJoined() && node.ClusterInfo().ClusterState != "ok" {
				hasDisconnected = true
			}
			if node.Role() == core.NodeRoleMaster {
				if ns := node.Slots(); ns != nil {
					allSlots = allSlots.Union(ns)
				}
			}
		}
	}
	return hasDisconnected && allSlots.IsFullfilled()
}
