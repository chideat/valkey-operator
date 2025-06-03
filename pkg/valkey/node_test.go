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

package valkey

import (
	"reflect"
	"testing"
)

func TestParseNodeFromClusterNode(t *testing.T) {
	type args struct {
		line string
	}
	tests := []struct {
		name    string
		args    args
		want    *ClusterNode
		wantErr bool
	}{
		{
			name: "new",
			args: args{line: "33b1262d41a4d9c27a78eef522c84999b064ce7f :6379@16379 myself,master - 0 0 0 connected"},
			want: &ClusterNode{
				Id:        "33b1262d41a4d9c27a78eef522c84999b064ce7f",
				Addr:      "",
				RawFlag:   "myself,master",
				BusPort:   "16379",
				AuxFields: ClusterNodeAuxFields{raw: ":6379@16379"},
				Role:      "master",
				MasterId:  "",
				PingSend:  0,
				PongRecv:  0,
				Epoch:     0,
				LinkState: "connected",
				slots:     []string{},
				rawInfo:   "33b1262d41a4d9c27a78eef522c84999b064ce7f :6379@16379 myself,master - 0 0 0 connected",
			},
			wantErr: false,
		},
		{
			name: "with aux fields",
			args: args{line: "343eca6406a48f95388682965bc19e37fcd300ca 192.168.130.82:31397@32669,nodename=node1,tcp-port=6379,tls-port=0,shard-id=a300430f8783c8a604dede6b8afabdc36230f038 master - 0 1743648449265 2 connected 5462-10922"},
			want: &ClusterNode{
				Id:      "343eca6406a48f95388682965bc19e37fcd300ca",
				Addr:    "192.168.130.82:31397",
				RawFlag: "master",
				BusPort: "32669",
				AuxFields: ClusterNodeAuxFields{
					ShardID:  "a300430f8783c8a604dede6b8afabdc36230f038",
					NodeName: "node1",
					TCPPort:  6379,
					TLSPort:  0,
					raw:      "192.168.130.82:31397@32669,nodename=node1,tcp-port=6379,tls-port=0,shard-id=a300430f8783c8a604dede6b8afabdc36230f038",
				},
				Role:      "master",
				MasterId:  "",
				PingSend:  0,
				PongRecv:  1743648449265,
				Epoch:     2,
				LinkState: "connected",
				slots:     []string{"5462-10922"},
				rawInfo:   "343eca6406a48f95388682965bc19e37fcd300ca 192.168.130.82:31397@32669,nodename=node1,tcp-port=6379,tls-port=0,shard-id=a300430f8783c8a604dede6b8afabdc36230f038 master - 0 1743648449265 2 connected 5462-10922",
			},
			wantErr: false,
		},
		{
			name: "with invalid aux fields",
			args: args{line: "343eca6406a48f95388682965bc19e37fcd300ca 192.168.130.82:31397@32669,node1,tcp-port=6379,tls-port=0,shard-id=a300430f8783c8a604dede6b8afabdc36230f038 master - 0 1743648449265 2 connected 5462-10922"},
			want: &ClusterNode{
				Id:      "343eca6406a48f95388682965bc19e37fcd300ca",
				Addr:    "192.168.130.82:31397",
				RawFlag: "master",
				BusPort: "32669",
				AuxFields: ClusterNodeAuxFields{
					ShardID: "a300430f8783c8a604dede6b8afabdc36230f038",
					TCPPort: 6379,
					TLSPort: 0,
					raw:     "192.168.130.82:31397@32669,node1,tcp-port=6379,tls-port=0,shard-id=a300430f8783c8a604dede6b8afabdc36230f038",
				},
				Role:      "master",
				MasterId:  "",
				PingSend:  0,
				PongRecv:  1743648449265,
				Epoch:     2,
				LinkState: "connected",
				slots:     []string{"5462-10922"},
				rawInfo:   "343eca6406a48f95388682965bc19e37fcd300ca 192.168.130.82:31397@32669,node1,tcp-port=6379,tls-port=0,shard-id=a300430f8783c8a604dede6b8afabdc36230f038 master - 0 1743648449265 2 connected 5462-10922",
			},
			wantErr: false,
		},
		{
			name:    "with error addr",
			args:    args{line: "343eca6406a48f95388682965bc19e37fcd300ca 192.168.130.82:31397,nodename=node1,tcp-port=6379,tls-port=0,shard-id=a300430f8783c8a604dede6b8afabdc36230f038 master - 0 1743648449265 2 connected 5462-10922"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseNodeFromClusterNode(tt.args.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseNodeFromClusterNode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseNodeFromClusterNode() = %v, want %v", got, tt.want)
				return
			}
			if !tt.wantErr && got.AuxFields.Raw() != tt.want.AuxFields.Raw() {
				t.Errorf("ClusterNodeAuxFields.Raw() = %v, want %v", got.AuxFields.Raw(), tt.want.AuxFields.Raw())
			}
		})
	}
}

func TestClusterNode_IsSelf(t *testing.T) {
	tests := []struct {
		name   string
		fields *ClusterNode
		want   bool
	}{
		{
			name: "isSelf",
			fields: &ClusterNode{
				Id:        "33b1262d41a4d9c27a78eef522c84999b064ce7f",
				Addr:      "",
				RawFlag:   "myself,master",
				MasterId:  "",
				PingSend:  0,
				PongRecv:  0,
				Epoch:     0,
				LinkState: "connected",
			},
			want: true,
		},
		{
			name:   "nil",
			fields: nil,
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var n *ClusterNode
			if tt.fields != nil {
				n = &ClusterNode{
					Id:        tt.fields.Id,
					Addr:      tt.fields.Addr,
					RawFlag:   tt.fields.RawFlag,
					MasterId:  tt.fields.MasterId,
					PingSend:  tt.fields.PingSend,
					PongRecv:  tt.fields.PongRecv,
					Epoch:     tt.fields.Epoch,
					LinkState: tt.fields.LinkState,
					slots:     tt.fields.slots,
					Role:      tt.fields.Role,
				}
			}
			if got := n.IsSelf(); got != tt.want {
				t.Errorf("ClusterNode.IsSelf() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterNodes_Self(t *testing.T) {
	node := ClusterNode{
		Id:        "33b1262d41a4d9c27a78eef522c84999b064ce7f",
		Addr:      "",
		RawFlag:   "myself,master",
		MasterId:  "",
		PingSend:  0,
		PongRecv:  0,
		Epoch:     0,
		LinkState: "connected",
	}
	tests := []struct {
		name string
		ns   ClusterNodes
		want *ClusterNode
	}{
		{
			name: "self",
			ns:   []*ClusterNode{&node},
			want: &node,
		},
		{
			name: "no self",
			ns: []*ClusterNode{
				{
					Id:        "33b1262d41a4d9c27a78eef522c84999b064ce7f",
					Addr:      "",
					RawFlag:   "master",
					MasterId:  "",
					PingSend:  0,
					PongRecv:  0,
					Epoch:     0,
					LinkState: "connected",
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ns.Self(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ClusterNodes.Self() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterNode_IsFailed(t *testing.T) {
	tests := []struct {
		name   string
		fields ClusterNode
		want   bool
	}{
		{
			name: "failed node",
			fields: ClusterNode{
				RawFlag: "master,fail",
			},
			want: true,
		},
		{
			name: "healthy node",
			fields: ClusterNode{
				RawFlag: "master",
			},
			want: false,
		},
		{
			name: "nil node",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var n *ClusterNode
			if tt.name != "nil node" {
				n = &ClusterNode{
					RawFlag: tt.fields.RawFlag,
				}
			}
			if got := n.IsFailed(); got != tt.want {
				t.Errorf("ClusterNode.IsFailed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterNode_IsConnected(t *testing.T) {
	tests := []struct {
		name   string
		fields ClusterNode
		want   bool
	}{
		{
			name: "connected node",
			fields: ClusterNode{
				LinkState: "connected",
			},
			want: true,
		},
		{
			name: "disconnected node",
			fields: ClusterNode{
				LinkState: "disconnected",
			},
			want: false,
		},
		{
			name: "nil node",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var n *ClusterNode
			if tt.name != "nil node" {
				n = &ClusterNode{
					LinkState: tt.fields.LinkState,
				}
			}
			if got := n.IsConnected(); got != tt.want {
				t.Errorf("ClusterNode.IsConnected() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterNode_IsJoined(t *testing.T) {
	tests := []struct {
		name   string
		fields ClusterNode
		want   bool
	}{
		{
			name: "joined node",
			fields: ClusterNode{
				Addr: "192.168.1.1:6379",
			},
			want: true,
		},
		{
			name: "not joined node",
			fields: ClusterNode{
				Addr: "",
			},
			want: false,
		},
		{
			name: "nil node",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var n *ClusterNode
			if tt.name != "nil node" {
				n = &ClusterNode{
					Addr: tt.fields.Addr,
				}
			}
			if got := n.IsJoined(); got != tt.want {
				t.Errorf("ClusterNode.IsJoined() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterNode_Slots(t *testing.T) {
	tests := []struct {
		name   string
		fields ClusterNode
		want   bool // true if slots should not be nil
	}{
		{
			name: "master with slots",
			fields: ClusterNode{
				Role:  MasterRole,
				slots: []string{"0-1000", "2000-3000"},
			},
			want: true,
		},
		{
			name: "slave node",
			fields: ClusterNode{
				Role:  SlaveRole,
				slots: []string{"0-1000"},
			},
			want: false,
		},
		{
			name: "master without slots",
			fields: ClusterNode{
				Role:  MasterRole,
				slots: []string{},
			},
			want: false,
		},
		{
			name: "nil node",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var n *ClusterNode
			if tt.name != "nil node" {
				n = &ClusterNode{
					Role:  tt.fields.Role,
					slots: tt.fields.slots,
				}
			}
			got := n.Slots()
			if (got != nil) != tt.want {
				t.Errorf("ClusterNode.Slots() = %v, want non-nil: %v", got, tt.want)
			}
		})
	}
}

func TestClusterNode_Raw(t *testing.T) {
	tests := []struct {
		name    string
		fields  ClusterNode
		wantRaw string
	}{
		{
			name: "node with raw info",
			fields: ClusterNode{
				rawInfo: "33b1262d41a4d9c27a78eef522c84999b064ce7f :6379@16379 master",
			},
			wantRaw: "33b1262d41a4d9c27a78eef522c84999b064ce7f :6379@16379 master",
		},
		{
			name: "node without raw info",
			fields: ClusterNode{
				rawInfo: "",
			},
			wantRaw: "",
		},
		{
			name:    "nil node",
			wantRaw: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var n *ClusterNode
			if tt.name != "nil node" {
				n = &ClusterNode{
					rawInfo: tt.fields.rawInfo,
				}
			}
			if got := n.Raw(); got != tt.wantRaw {
				t.Errorf("ClusterNode.Raw() = %v, want %v", got, tt.wantRaw)
			}
		})
	}
}

func TestClusterNodes_Get(t *testing.T) {
	node1 := &ClusterNode{Id: "node1"}
	node2 := &ClusterNode{Id: "node2"}
	nodes := ClusterNodes{node1, node2}

	tests := []struct {
		name string
		ns   ClusterNodes
		id   string
		want *ClusterNode
	}{
		{
			name: "existing node",
			ns:   nodes,
			id:   "node1",
			want: node1,
		},
		{
			name: "non-existing node",
			ns:   nodes,
			id:   "node3",
			want: nil,
		},
		{
			name: "empty nodes",
			ns:   ClusterNodes{},
			id:   "node1",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ns.Get(tt.id); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ClusterNodes.Get() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterNodes_Replicas(t *testing.T) {
	master := &ClusterNode{Id: "master1"}
	replica1 := &ClusterNode{Id: "replica1", MasterId: "master1"}
	replica2 := &ClusterNode{Id: "replica2", MasterId: "master1"}
	nodes := ClusterNodes{master, replica1, replica2}

	tests := []struct {
		name string
		ns   ClusterNodes
		id   string
		want []*ClusterNode
	}{
		{
			name: "master with replicas",
			ns:   nodes,
			id:   "master1",
			want: []*ClusterNode{replica1, replica2},
		},
		{
			name: "master without replicas",
			ns:   nodes,
			id:   "master2",
			want: nil,
		},
		{
			name: "empty nodes",
			ns:   ClusterNodes{},
			id:   "master1",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ns.Replicas(tt.id); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ClusterNodes.Replicas() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterNodes_Masters(t *testing.T) {
	master1 := &ClusterNode{Id: "master1", Role: MasterRole, slots: []string{"0-1000"}}
	master2 := &ClusterNode{Id: "master2", Role: MasterRole, slots: []string{"1001-2000"}}
	slave := &ClusterNode{Id: "slave1", Role: SlaveRole}
	emptyMaster := &ClusterNode{Id: "empty", Role: MasterRole}
	nodes := ClusterNodes{master1, master2, slave, emptyMaster}

	tests := []struct {
		name string
		ns   ClusterNodes
		want []*ClusterNode
	}{
		{
			name: "multiple masters",
			ns:   nodes,
			want: []*ClusterNode{master1, master2},
		},
		{
			name: "empty nodes",
			ns:   ClusterNodes{},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ns.Masters(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ClusterNodes.Masters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterNodes_Marshal(t *testing.T) {
	node := &ClusterNode{
		Id:        "node1",
		Addr:      "192.168.1.1:6379",
		BusPort:   "16379",
		RawFlag:   "master",
		Role:      MasterRole,
		MasterId:  "",
		PingSend:  0,
		PongRecv:  0,
		Epoch:     1,
		LinkState: "connected",
		slots:     []string{"0-1000"},
		AuxFields: ClusterNodeAuxFields{
			ShardID:  "shard1",
			NodeName: "node1",
			TCPPort:  6379,
			TLSPort:  6380,
		},
	}
	nodes := ClusterNodes{node}

	tests := []struct {
		name    string
		ns      ClusterNodes
		wantErr bool
	}{
		{
			name:    "valid nodes",
			ns:      nodes,
			wantErr: false,
		},
		{
			name:    "empty nodes",
			ns:      ClusterNodes{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.ns.Marshal()
			if (err != nil) != tt.wantErr {
				t.Errorf("ClusterNodes.Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) == 0 && len(tt.ns) > 0 {
					t.Errorf("ClusterNodes.Marshal() returned empty data for non-empty nodes")
				}
			}
		})
	}
}

func TestParseNodes(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    int // number of nodes expected
		wantErr bool
	}{
		{
			name: "valid nodes without host",
			data: `33b1262d41a4d9c27a78eef522c84999b064ce7f :6379@16379 myself,master - 0 0 0 connected
44c1262d41a4d9c27a78eef522c84999b064ce7f :6380@16380 slave 33b1262d41a4d9c27a78eef522c84999b064ce7f 0 0 0 connected
vars currentEpoch 5 lastVoteEpoch 0`,
			want:    2,
			wantErr: false,
		},
		{
			name: "valid nodes",
			data: `b4cb5b221aedca8b7380f51bc3a10195e0211066 192.168.138.234:30643@31899,,tls-port=0,shard-id=48a6e32a9da7c8d484ff932ce0bb9f540a930ed9 slave ba1577ec8d5037199b4e9fe7661ae569995713d9 0 1748928991000 9872 connected
ba1577ec8d5037199b4e9fe7661ae569995713d9 192.168.142.242:32412@30910,,tls-port=0,shard-id=48a6e32a9da7c8d484ff932ce0bb9f540a930ed9 master - 0 1748928992000 9872 connected 0-5461
c71c6a6de4e3309e40669720863da816d0d28835 192.168.142.242:30689@30476,,tls-port=0,shard-id=ad7120bc83b9491e179c79b747ca60c610285648 slave ffc9f22c82a69d6d56699513ac5a2eb55a1ab267 0 1748928993000 9864 connected
6a8a3f4037ce81183cf2f0dbade7422f4a553514 192.168.142.242:31195@32311,,tls-port=0,shard-id=d435d01b17038215998f66759e1800b977453e17 slave d386323275dfce0e0b8c4c645a20c5892fa47566 0 1748928993246 9865 connected
d386323275dfce0e0b8c4c645a20c5892fa47566 192.168.138.234:31012@30749,,tls-port=0,shard-id=d435d01b17038215998f66759e1800b977453e17 master - 0 1748928992186 9865 connected 10923-16383
ffc9f22c82a69d6d56699513ac5a2eb55a1ab267 192.168.138.234:32320@30296,,tls-port=0,shard-id=ad7120bc83b9491e179c79b747ca60c610285648 myself,master - 0 1748928992000 9864 connected 5462-10922
vars currentEpoch 9872 lastVoteEpoch 9872`,
			want:    6,
			wantErr: false,
		},
		{
			name: "valid cluster nodes",
			data: `b4cb5b221aedca8b7380f51bc3a10195e0211066 192.168.138.234:30643@31899 slave ba1577ec8d5037199b4e9fe7661ae569995713d9 0 1748929224000 9872 connected
ba1577ec8d5037199b4e9fe7661ae569995713d9 192.168.142.242:32412@30910 master - 0 1748929226515 9872 connected 0-5461
c71c6a6de4e3309e40669720863da816d0d28835 192.168.142.242:30689@30476 slave ffc9f22c82a69d6d56699513ac5a2eb55a1ab267 0 1748929225512 9864 connected
6a8a3f4037ce81183cf2f0dbade7422f4a553514 192.168.142.242:31195@32311 slave d386323275dfce0e0b8c4c645a20c5892fa47566 0 1748929225000 9865 connected
d386323275dfce0e0b8c4c645a20c5892fa47566 192.168.138.234:31012@30749 master - 0 1748929223000 9865 connected 10923-16383
ffc9f22c82a69d6d56699513ac5a2eb55a1ab267 192.168.138.234:32320@30296 myself,master - 0 1748929225000 9864 connected 5462-10922`,
			want:    6,
			wantErr: false,
		},
		{
			name:    "empty data",
			data:    "",
			want:    0,
			wantErr: false,
		},
		{
			name:    "invalid node format",
			data:    "invalid node data",
			wantErr: true,
		},
		{
			name:    "cluster nodes test",
			data:    "b4cb5b221aedca8b7380f51bc3a10195e0211066 192.168.138.234:30643@31899 slave ba1577ec8d5037199b4e9fe7661ae569995713d9 0 1748929519136 9872 connected\nba1577ec8d5037199b4e9fe7661ae569995713d9 192.168.142.242:32412@30910 master - 0 1748929520141 9872 connected 0-5461\nc71c6a6de4e3309e40669720863da816d0d28835 192.168.142.242:30689@30476 slave ffc9f22c82a69d6d56699513ac5a2eb55a1ab267 0 1748929519000 9864 connected\n6a8a3f4037ce81183cf2f0dbade7422f4a553514 192.168.142.242:31195@32311 slave d386323275dfce0e0b8c4c645a20c5892fa47566 0 1748929521146 9865 connected\nd386323275dfce0e0b8c4c645a20c5892fa47566 192.168.138.234:31012@30749 master - 0 1748929519000 9865 connected 10923-16383\nffc9f22c82a69d6d56699513ac5a2eb55a1ab267 192.168.138.234:32320@30296 myself,master - 0 1748929518000 9864 connected 5462-10922\n",
			want:    6,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseNodes(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseNodes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != tt.want {
				t.Errorf("ParseNodes() returned %v nodes, want %v", len(got), tt.want)
			}
		})
	}
}
