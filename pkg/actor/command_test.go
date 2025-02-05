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
*/package actor

import (
	"reflect"
	"testing"

	"github.com/chideat/valkey-operator/api/core"
)

func Test_opsCommand_String(t *testing.T) {
	type fields struct {
		arch    core.Arch
		command string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "string",
			fields: fields{
				arch:    core.ValkeyCluster,
				command: "CommandEnsureResource",
			},
			want: "CommandEnsureResource",
		},
		{
			name: "without arch",
			fields: fields{
				command: "CommandEnsureResource",
			},
			want: "CommandEnsureResource",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &opsCommand{
				arch:    tt.fields.arch,
				command: tt.fields.command,
			}
			if got := c.String(); got != tt.want {
				t.Errorf("opsCommand.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewCommand(t *testing.T) {
	type args struct {
		arch    core.Arch
		command string
	}
	tests := []struct {
		name string
		args args
		want Command
	}{
		{
			name: "new command",
			args: args{
				arch:    core.ValkeyCluster,
				command: "CommandEnsureResource",
			},
			want: &opsCommand{arch: core.ValkeyCluster, command: "CommandEnsureResource"},
		},
		{
			name: "without arch command",
			args: args{
				command: "CommandRequeue",
			},
			want: &opsCommand{command: "CommandRequeue"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewCommand(tt.args.arch, tt.args.command); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}
