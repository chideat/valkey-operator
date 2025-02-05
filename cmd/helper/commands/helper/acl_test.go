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
	"reflect"
	"testing"

	"github.com/chideat/valkey-operator/pkg/types/user"
)

func Test_formatACLSetCommand(t *testing.T) {
	type args struct {
		user *user.User
	}
	tests := []struct {
		name     string
		args     args
		wantArgs []string
	}{
		{
			name: "default",
			args: args{
				user: &user.User{
					Name: "default",
					Role: user.RoleDeveloper,
					Rules: []*user.Rule{
						{
							DisallowedCommands: []string{"flushall", "flushdb"},
						},
					},
				},
			},
			wantArgs: []string{"user", "default", "-flushall", "-flushdb", "nopass", "on"},
		},
		{
			name: "custom1",
			args: args{
				user: &user.User{
					Name: "custom1",
					Role: user.RoleDeveloper,
					Rules: []*user.Rule{
						{
							Categories:           []string{"read"},
							DisallowedCategories: []string{"all"},
							DisallowedCommands:   []string{"keys"},
							KeyPatterns:          []string{"*"},
						},
					},
				},
			},
			wantArgs: []string{"user", "custom1", "-@all", "+@read", "-keys", "~*", "nopass", "on"},
		},
		{
			name: "custom2",
			args: args{
				user: &user.User{
					Name: "custom2",
					Role: user.RoleDeveloper,
					Rules: []*user.Rule{
						{
							AllowedCommands:    []string{"cluster"},
							DisallowedCommands: []string{"cluster|setslot", "cluster|nodes"},
							KeyPatterns:        []string{"*"},
						},
					},
				},
			},
			wantArgs: []string{"user", "custom2", "+cluster", "-cluster|setslot", "-cluster|nodes", "~*", "nopass", "on"},
		},
		{
			name: "custom3",
			args: args{
				user: &user.User{
					Name: "custom3",
					Role: user.RoleDeveloper,
					Rules: []*user.Rule{
						{
							DisallowedCommands: []string{"cluster|setslot", "cluster|nodes"},
							KeyPatterns:        []string{"*"},
						},
					},
				},
			},
			wantArgs: []string{"user", "custom3", "-cluster|setslot", "-cluster|nodes", "~*", "nopass", "on"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotArgs := formatACLSetCommand(tt.args.user); !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Errorf("formatACLSetCommand() = %v, want %v", gotArgs, tt.wantArgs)
			}
		})
	}
}
