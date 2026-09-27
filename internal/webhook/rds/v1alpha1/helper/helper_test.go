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

package helper

import (
	"testing"

	"github.com/chideat/valkey-operator/api/core"
)

func Test_parseShardIndex(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "drc-test-0-0",
			args: args{
				name: "drc-test-0-0",
			},
			want: 0,
		},
		{
			name: "drc-test-0-999",
			args: args{
				name: "drc-test-0-999",
			},
			want: 999,
		},
		{
			name: "rfr-test",
			args: args{
				name: "rfr-test",
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseShardIndex(tt.args.name); got != tt.want {
				t.Errorf("parseShardIndex() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCalculateNodeCount pins the count to the pods the operator runs:
// replicasOfShard pods in each shard, because the cluster StatefulSets and the
// Failover of the failover and replica architectures get replicasOfShard
// replicas. Admission asks for one node port per pod, and the count used to add
// a master to every shard, so it refused every access.ports that fit.
func TestCalculateNodeCount(t *testing.T) {
	type args struct {
		arch            core.Arch
		shards          int32
		replicasOfShard int32
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "cluster, 3 shards of 2 pods",
			args: args{
				arch:            core.ValkeyCluster,
				shards:          (int32(3)),
				replicasOfShard: (int32(2)),
			},
			want: 6,
		},
		{
			name: "cluster, 3 shards of 1 pod",
			args: args{
				arch:            core.ValkeyCluster,
				shards:          (int32(3)),
				replicasOfShard: (int32(1)),
			},
			want: 3,
		},
		{
			name: "failover, 2 pods",
			args: args{
				arch:            core.ValkeyFailover,
				shards:          (int32(1)),
				replicasOfShard: (int32(2)),
			},
			want: 2,
		},
		{
			name: "replica, 1 pod",
			args: args{
				arch:            core.ValkeyReplica,
				shards:          (int32(1)),
				replicasOfShard: (int32(1)),
			},
			want: 1,
		},
		{
			name: "replica, 3 pods",
			args: args{
				arch:            core.ValkeyReplica,
				shards:          (int32(1)),
				replicasOfShard: (int32(3)),
			},
			want: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateNodeCount(tt.args.arch, tt.args.shards, tt.args.replicasOfShard); got != tt.want {
				t.Errorf("CalculateNodeCount() = %v, want %v", got, tt.want)
			}
		})
	}
}
