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
	corev1 "k8s.io/api/core/v1"
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

	const hash = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08" // sha256("test")
	plainPwd, err := user.NewPassword(&corev1.Secret{Data: map[string][]byte{user.PasswordSecretKey: []byte("Str0ng!Passw0rd")}})
	if err != nil {
		t.Fatal(err)
	}
	hashedPwd, err := user.NewPassword(&corev1.Secret{Data: map[string][]byte{user.PasswordSHA256SecretKey: []byte(hash)}})
	if err != nil {
		t.Fatal(err)
	}
	tests = append(tests,
		struct {
			name     string
			args     args
			wantArgs []string
		}{
			name:     "plaintext password",
			args:     args{user: &user.User{Name: "app", Role: user.RoleDeveloper, Password: plainPwd, Rules: []*user.Rule{{KeyPatterns: []string{"*"}}}}},
			wantArgs: []string{"user", "app", "~*", ">Str0ng!Passw0rd", "on"},
		},
		struct {
			name     string
			args     args
			wantArgs []string
		}{
			name:     "sha256 password",
			args:     args{user: &user.User{Name: "app", Role: user.RoleDeveloper, Password: hashedPwd, Rules: []*user.Rule{{KeyPatterns: []string{"*"}}}}},
			wantArgs: []string{"user", "app", "~*", "#" + hash, "on"},
		},
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotArgs := formatACLSetCommand(tt.args.user); !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Errorf("formatACLSetCommand() = %v, want %v", gotArgs, tt.wantArgs)
			}
		})
	}
}
