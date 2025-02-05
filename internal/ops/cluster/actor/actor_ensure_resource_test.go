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
	"testing"

	"github.com/chideat/valkey-operator/internal/builder"
)

func Test_parsePodShardAndIndex(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name      string
		args      args
		wantShard int
		wantIndex int
		wantErr   bool
	}{
		{
			name:      "name ok",
			args:      args{name: "vkc-valkey-1-1"},
			wantShard: 1,
			wantIndex: 1,
			wantErr:   false,
		},
		{
			name:      "name ok",
			args:      args{name: "vkc----valkey-0-0"},
			wantShard: 0,
			wantIndex: 0,
			wantErr:   false,
		},
		{
			name:      "name error",
			args:      args{name: "vkc-valkey-1"},
			wantShard: -1,
			wantIndex: -1,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotShard, gotIndex, err := builder.ParsePodShardAndIndex(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("%s parsePodShardAndIndex() error = %v, wantErr %v", tt.name, err, tt.wantErr)
				return
			}
			if gotShard != tt.wantShard {
				t.Errorf("%s parsePodShardAndIndex() gotShard = %v, want %v", tt.name, gotShard, tt.wantShard)
			}
			if gotIndex != tt.wantIndex {
				t.Errorf("%s parsePodShardAndIndex() gotIndex = %v, want %v", tt.name, gotIndex, tt.wantIndex)
			}
		})
	}
}
