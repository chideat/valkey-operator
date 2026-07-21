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
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/pkg/slot"
	"github.com/chideat/valkey-operator/pkg/types"
	vkcli "github.com/chideat/valkey-operator/pkg/valkey"
)

// fakeNode implements only the ValkeyNode methods the predicates use; the embedded
// interface satisfies the rest (those methods are never called here).
type fakeNode struct {
	types.ValkeyNode
	id           string
	clusterState string
	joined       bool
	terminating  bool
	role         core.NodeRole
	slots        *slot.Slots
}

func (f *fakeNode) ID() string          { return f.id }
func (f *fakeNode) IsTerminating() bool { return f.terminating }
func (f *fakeNode) IsJoined() bool      { return f.joined }
func (f *fakeNode) Role() core.NodeRole { return f.role }
func (f *fakeNode) Slots() *slot.Slots  { return f.slots }
func (f *fakeNode) ClusterInfo() vkcli.ClusterNodeInfo {
	return vkcli.ClusterNodeInfo{ClusterState: f.clusterState}
}

type fakeShard struct {
	types.ClusterShard
	nodes []types.ValkeyNode
}

func (f *fakeShard) Nodes() []types.ValkeyNode { return f.nodes }

type fakeCluster struct {
	types.ClusterInstance
	shards []types.ClusterShard
}

func (f *fakeCluster) Shards() []types.ClusterShard { return f.shards }

func mustSlots(t *testing.T, s string) *slot.Slots {
	t.Helper()
	sl, err := slot.LoadSlots(s)
	if err != nil {
		t.Fatalf("LoadSlots(%q): %v", s, err)
	}
	return sl
}

// disconnectedMaster is a master that owns slots but cannot see its peers (post-restart).
func disconnectedMaster(id string, slots *slot.Slots) *fakeNode {
	return &fakeNode{id: id, role: core.NodeRoleMaster, slots: slots, joined: false, clusterState: "fail"}
}

func TestShardAssignedSlots(t *testing.T) {
	s := mustSlots(t, "0-5461")
	tests := []struct {
		name      string
		shard     types.ClusterShard
		wantCount int
	}{
		{"disconnected master keeps its slots", &fakeShard{nodes: []types.ValkeyNode{disconnectedMaster("a", s)}}, 5462},
		{"empty master (fresh) has none", &fakeShard{nodes: []types.ValkeyNode{&fakeNode{id: "a", role: core.NodeRoleMaster}}}, 0},
		{"replica contributes nothing", &fakeShard{nodes: []types.ValkeyNode{&fakeNode{id: "r", role: core.NodeRoleReplica, slots: s}}}, 0},
		{"nil shard", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShardAssignedSlots(tt.shard).Count(slot.SlotAssigned); got != tt.wantCount {
				t.Errorf("ShardAssignedSlots count = %d, want %d", got, tt.wantCount)
			}
		})
	}
}

func TestShardHasMaster(t *testing.T) {
	s := mustSlots(t, "0-5461")
	tests := []struct {
		name  string
		shard types.ClusterShard
		want  bool
	}{
		{"disconnected master owning slots", &fakeShard{nodes: []types.ValkeyNode{disconnectedMaster("a", s)}}, true},
		{"master without slots (fresh)", &fakeShard{nodes: []types.ValkeyNode{&fakeNode{id: "a", role: core.NodeRoleMaster}}}, false},
		{"replica only", &fakeShard{nodes: []types.ValkeyNode{&fakeNode{id: "r", role: core.NodeRoleReplica, slots: s}}}, false},
		{"nil shard", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShardHasMaster(tt.shard); got != tt.want {
				t.Errorf("ShardHasMaster = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDisconnectedButIntact(t *testing.T) {
	s1 := mustSlots(t, "0-5461")
	s2 := mustSlots(t, "5462-10922")
	s3 := mustSlots(t, "10923-16383")

	connectedMaster := func(id string, sl *slot.Slots) *fakeNode {
		return &fakeNode{id: id, role: core.NodeRoleMaster, slots: sl, joined: true, clusterState: "ok"}
	}
	shard := func(n ...types.ValkeyNode) *fakeShard { return &fakeShard{nodes: n} }

	tests := []struct {
		name   string
		shards []types.ClusterShard
		want   bool
	}{
		{
			name: "post-restart all disconnected, slots intact -> true",
			shards: []types.ClusterShard{
				shard(disconnectedMaster("a", s1)), shard(disconnectedMaster("b", s2)), shard(disconnectedMaster("c", s3)),
			},
			want: true,
		},
		{
			name: "healthy connected cluster -> false (not disconnected)",
			shards: []types.ClusterShard{
				shard(connectedMaster("a", s1)), shard(connectedMaster("b", s2)), shard(connectedMaster("c", s3)),
			},
			want: false,
		},
		{
			name: "partial-convergence: one still disconnected, slots intact -> true",
			shards: []types.ClusterShard{
				shard(connectedMaster("a", s1)), shard(connectedMaster("b", s2)), shard(disconnectedMaster("c", s3)),
			},
			want: true,
		},
		{
			name: "truly dead master (slots uncovered) -> false (allow failover)",
			shards: []types.ClusterShard{
				shard(disconnectedMaster("a", s1)), shard(disconnectedMaster("b", s2)),
				shard(&fakeNode{id: "c", role: core.NodeRoleReplica, joined: false, clusterState: "fail"}),
			},
			want: false,
		},
		{
			name:   "fresh cluster, no slots yet -> false",
			shards: []types.ClusterShard{shard(&fakeNode{id: "a", role: core.NodeRoleMaster, joined: false, clusterState: "fail"})},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDisconnectedButIntact(&fakeCluster{shards: tt.shards}); got != tt.want {
				t.Errorf("IsDisconnectedButIntact = %v, want %v", got, tt.want)
			}
		})
	}
}
