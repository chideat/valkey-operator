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

func TestCalculateNodeCount(t *testing.T) {
	type args struct {
		arch         core.Arch
		masterCount  int32
		replicaCount int32
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "redis cluster with replicas",
			args: args{
				arch:         core.ValkeyCluster,
				masterCount:  (int32(3)),
				replicaCount: (int32(1)),
			},
			want: 6,
		},
		{
			name: "redis cluster without replicas",
			args: args{
				arch:        core.ValkeyCluster,
				masterCount: (int32(3)),
			},
			want: 3,
		},
		{
			name: "redis sentinel/standalone with replicas",
			args: args{
				arch:         core.ValkeyFailover,
				masterCount:  (int32(1)),
				replicaCount: (int32(2)),
			},
			want: 3,
		},
		{
			name: "redis sentinel/standalone without replicas",
			args: args{
				arch:        core.ValkeyReplica,
				masterCount: (int32(1)),
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateNodeCount(tt.args.arch, tt.args.masterCount, tt.args.replicaCount); got != tt.want {
				t.Errorf("CalculateNodeCount() = %v, want %v", got, tt.want)
			}
		})
	}
}
