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
			}
		})
	}
}

func TestClusterNode_IsSelf(t *testing.T) {
	tests := []struct {
		name   string
		fields ClusterNode
		want   bool
	}{
		{
			name: "isSelf",
			fields: ClusterNode{
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &ClusterNode{
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ns.Self(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ClusterNodes.Self() = %v, want %v", got, tt.want)
			}
		})
	}
}
