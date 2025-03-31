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

package failover

import (
	"os"
	"testing"

	"github.com/go-logr/logr"
)

func Test_loadAnnounceAddress(t *testing.T) {
	logger := logr.Discard()

	type args struct {
		filepath string
		data     string
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{
				filepath: "/tmp/slave-abc123",
				data:     ``,
			},
			want: "",
		},
		{
			name: "replica config",
			args: args{
				filepath: "/tmp/replica-abc123",
				data: `
replica-announce-ip 192.168.138.159
replica-announce-port 31095`,
			},
			want: "192.168.138.159:31095",
		},
		{
			name: "slave config without lead space",
			args: args{
				filepath: "/tmp/slave-abc123",
				data: `replica-announce-ip 192.168.138.159
replica-announce-port 31095`,
			},
			want: "192.168.138.159:31095",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(tt.args.filepath, []byte(tt.args.data), 0644); err != nil {
				t.Errorf("failed to write file: %v", err)
			}
			if got := loadAnnounceAddress(tt.args.filepath, logger); got != tt.want {
				t.Errorf("loadAnnounceAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}
