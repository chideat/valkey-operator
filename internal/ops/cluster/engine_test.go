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

package cluster

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"github.com/chideat/valkey-operator/api/v1alpha1"
	clusterv1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/redis/cluster"
	"github.com/chideat/valkey-operator/pkg/slot"
	"github.com/chideat/valkey-operator/pkg/types"
	"k8s.io/utils/ptr"
)

func Test_buildStatusOfShards(t *testing.T) {
	slotsA1, _ := slot.LoadSlots("0-5461")
	slotsA2, _ := slot.LoadSlots("5462-10922")
	slotsA3, _ := slot.LoadSlots("10923-16383")

	slotsB1, _ := slot.LoadSlots("0-4095")
	slotsB2, _ := slot.LoadSlots("5462-9557")
	slotsB3, _ := slot.LoadSlots("10923-15018")
	slotsB4, _ := slot.LoadSlots("4096-5461,9558-10922,15019-16383")

	type args struct {
		cluster types.RedisClusterInstance
		slots   []*slot.Slots
	}
	tests := []struct {
		name    string
		args    args
		wantRet []*clusterv1.ClusterShards
	}{
		{
			name: "newassign",
			args: args{
				cluster: &cluster.RedisCluster{
					Cluster: v1alpha1.Cluster{
						Spec: v1alpha1.ClusterSpec{
							Replicas: v1alpha1.ClusterReplicas{
								Shards: 3,
							},
						},
					},
				},
				slots: []*slot.Slots{slotsA1, slotsA2, slotsA3},
			},
			wantRet: []*clusterv1.ClusterShards{
				{
					Index: 0,
					Slots: []*clusterv1.ClusterShardsSlotStatus{
						{
							ShardIndex: ptr.To(int32(0)),
							Status:     slot.SlotAssigned.String(),
							Slots:      "0-5461",
						},
					},
				},
				{
					Index: 1,
					Slots: []*clusterv1.ClusterShardsSlotStatus{
						{
							ShardIndex: ptr.To(int32(1)),
							Status:     slot.SlotAssigned.String(),
							Slots:      "5462-10922",
						},
					},
				},
				{
					Index: 2,
					Slots: []*clusterv1.ClusterShardsSlotStatus{
						{
							ShardIndex: ptr.To(int32(2)),
							Status:     slot.SlotAssigned.String(),
							Slots:      "10923-16383",
						},
					},
				},
			},
		},
		{
			name: "scale 3=>4",
			args: args{
				cluster: &cluster.RedisCluster{
					Cluster: v1alpha1.Cluster{
						Spec: v1alpha1.ClusterSpec{
							Replicas: v1alpha1.ClusterReplicas{
								Shards: 4,
							},
						},
						Status: v1alpha1.ClusterStatus{
							Shards: []*clusterv1.ClusterShards{
								{
									Index: 0,
									Slots: []*clusterv1.ClusterShardsSlotStatus{
										{
											ShardIndex: ptr.To(int32(0)),
											Status:     slot.SlotAssigned.String(),
											Slots:      "0-5461",
										},
									},
								},
								{
									Index: 1,
									Slots: []*clusterv1.ClusterShardsSlotStatus{
										{
											ShardIndex: ptr.To(int32(1)),
											Status:     slot.SlotAssigned.String(),
											Slots:      "5462-10922",
										},
									},
								},
								{
									Index: 2,
									Slots: []*clusterv1.ClusterShardsSlotStatus{
										{
											ShardIndex: ptr.To(int32(2)),
											Status:     slot.SlotAssigned.String(),
											Slots:      "10923-16383",
										},
									},
								},
							},
						},
					},
				},
				slots: []*slot.Slots{slotsB1, slotsB2, slotsB3, slotsB4},
			},
			wantRet: []*clusterv1.ClusterShards{
				{
					Index: 0,
					Slots: []*clusterv1.ClusterShardsSlotStatus{
						{
							ShardIndex: ptr.To(int32(0)),
							Status:     slot.SlotAssigned.String(),
							Slots:      "0-5461",
						},
						{
							ShardIndex: ptr.To(int32(3)),
							Status:     slot.SlotMigrating.String(),
							Slots:      "4096-5461",
						},
					},
				},
				{
					Index: 1,
					Slots: []*clusterv1.ClusterShardsSlotStatus{
						{
							ShardIndex: ptr.To(int32(1)),
							Status:     slot.SlotAssigned.String(),
							Slots:      "5462-10922",
						},
						{
							ShardIndex: ptr.To(int32(3)),
							Status:     slot.SlotMigrating.String(),
							Slots:      "9558-10922",
						},
					},
				},
				{
					Index: 2,
					Slots: []*clusterv1.ClusterShardsSlotStatus{
						{
							ShardIndex: ptr.To(int32(2)),
							Status:     slot.SlotAssigned.String(),
							Slots:      "10923-16383",
						},
						{
							ShardIndex: ptr.To(int32(3)),
							Status:     slot.SlotMigrating.String(),
							Slots:      "15019-16383",
						},
					},
				},
				{
					Index: 3,
					Slots: []*clusterv1.ClusterShardsSlotStatus{
						{
							ShardIndex: ptr.To(int32(0)),
							Status:     slot.SlotImporting.String(),
							Slots:      "4096-5461",
						},
						{
							ShardIndex: ptr.To(int32(1)),
							Status:     slot.SlotImporting.String(),
							Slots:      "9558-10922",
						},

						{
							ShardIndex: ptr.To(int32(2)),
							Status:     slot.SlotImporting.String(),
							Slots:      "15019-16383",
						},
					},
				},
			},
		},
		{
			name: "update status",
			args: args{
				cluster: &cluster.RedisCluster{
					Cluster: v1alpha1.Cluster{
						Spec: v1alpha1.ClusterSpec{
							Replicas: v1alpha1.ClusterReplicas{
								Shards: 4,
							},
						},
						Status: v1alpha1.ClusterStatus{
							Shards: []*clusterv1.ClusterShards{
								{
									Index: 0,
									Slots: []*clusterv1.ClusterShardsSlotStatus{
										{
											ShardIndex: ptr.To(int32(0)),
											Status:     slot.SlotAssigned.String(),
											Slots:      "0-5461",
										},
										{
											ShardIndex: ptr.To(int32(3)),
											Status:     slot.SlotMigrating.String(),
											Slots:      "4096-5461",
										},
									},
								},
								{
									Index: 1,
									Slots: []*clusterv1.ClusterShardsSlotStatus{
										{
											ShardIndex: ptr.To(int32(1)),
											Status:     slot.SlotAssigned.String(),
											Slots:      "5462-10922",
										},
										{
											ShardIndex: ptr.To(int32(3)),
											Status:     slot.SlotMigrating.String(),
											Slots:      "9558-10922",
										},
									},
								},
								{
									Index: 2,
									Slots: []*clusterv1.ClusterShardsSlotStatus{
										{
											ShardIndex: ptr.To(int32(2)),
											Status:     slot.SlotAssigned.String(),
											Slots:      "10923-16383",
										},
										{
											ShardIndex: ptr.To(int32(3)),
											Status:     slot.SlotMigrating.String(),
											Slots:      "15019-16383",
										},
									},
								},
								{
									Index: 3,
									Slots: []*clusterv1.ClusterShardsSlotStatus{
										{
											ShardIndex: ptr.To(int32(0)),
											Status:     slot.SlotImporting.String(),
											Slots:      "4096-5461",
										},
										{
											ShardIndex: ptr.To(int32(1)),
											Status:     slot.SlotImporting.String(),
											Slots:      "9558-10922",
										},

										{
											ShardIndex: ptr.To(int32(2)),
											Status:     slot.SlotImporting.String(),
											Slots:      "15019-16383",
										},
									},
								},
							},
						},
					},
				},
			},
			wantRet: []*clusterv1.ClusterShards{
				{
					Index: 0,
					Slots: []*clusterv1.ClusterShardsSlotStatus{
						{
							ShardIndex: ptr.To(int32(0)),
							Status:     slot.SlotAssigned.String(),
							Slots:      "0-5461",
						},
						{
							ShardIndex: ptr.To(int32(3)),
							Status:     slot.SlotMigrating.String(),
							Slots:      "4096-5461",
						},
					},
				},
				{
					Index: 1,
					Slots: []*clusterv1.ClusterShardsSlotStatus{
						{
							ShardIndex: ptr.To(int32(1)),
							Status:     slot.SlotAssigned.String(),
							Slots:      "5462-10922",
						},
						{
							ShardIndex: ptr.To(int32(3)),
							Status:     slot.SlotMigrating.String(),
							Slots:      "9558-10922",
						},
					},
				},
				{
					Index: 2,
					Slots: []*clusterv1.ClusterShardsSlotStatus{
						{
							ShardIndex: ptr.To(int32(2)),
							Status:     slot.SlotAssigned.String(),
							Slots:      "10923-16383",
						},
						{
							ShardIndex: ptr.To(int32(3)),
							Status:     slot.SlotMigrating.String(),
							Slots:      "15019-16383",
						},
					},
				},
				{
					Index: 3,
					Slots: []*clusterv1.ClusterShardsSlotStatus{
						{
							ShardIndex: ptr.To(int32(0)),
							Status:     slot.SlotImporting.String(),
							Slots:      "4096-5461",
						},
						{
							ShardIndex: ptr.To(int32(1)),
							Status:     slot.SlotImporting.String(),
							Slots:      "9558-10922",
						},

						{
							ShardIndex: ptr.To(int32(2)),
							Status:     slot.SlotImporting.String(),
							Slots:      "15019-16383",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRet := buildStatusOfShards(tt.args.cluster, tt.args.slots)
			for _, item := range gotRet {
				sort.SliceStable(item.Slots, func(i, j int) bool {
					return *item.Slots[i].ShardIndex < *item.Slots[j].ShardIndex
				})
			}
			if !reflect.DeepEqual(gotRet, tt.wantRet) {
				dataRet, _ := json.Marshal(gotRet)
				dataWant, _ := json.Marshal(tt.wantRet)
				t.Errorf("buildStatusOfShards() = %s, want %s", dataRet, dataWant)
			}
		})
	}
}
