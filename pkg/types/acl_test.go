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

package types

import (
	"reflect"
	"testing"

	"github.com/chideat/valkey-operator/pkg/types/user"
	core "github.com/chideat/valkey-operator/pkg/types/user"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUsers_GetOpUser(t *testing.T) {
	t.Run("empty Users slice", func(t *testing.T) {
		users := Users{}
		if got := users.GetOpUser(); got != nil {
			t.Errorf("Users.GetOpUser() = %v, want %v", got, nil)
		}
	})

	t.Run("Users with operator", func(t *testing.T) {
		users := Users{
			&core.User{Name: "op", Role: core.RoleOperator},
		}
		if got := users.GetOpUser(); got == nil || got.Role != core.RoleOperator {
			t.Errorf("Users.GetOpUser() = %v, want %v", got, core.RoleOperator)
		}
	})

	t.Run("Users with only developers", func(t *testing.T) {
		users := Users{
			&core.User{Name: "dev", Role: core.RoleDeveloper},
		}
		if got := users.GetOpUser(); got == nil || got.Role != core.RoleDeveloper {
			t.Errorf("Users.GetOpUser() = %v, want %v", got, core.RoleDeveloper)
		}
	})
}

func TestUsers_GetDefaultUser(t *testing.T) {
	t.Run("empty Users slice", func(t *testing.T) {
		users := Users{}
		if got := users.GetDefaultUser(); got != nil {
			t.Errorf("Users.GetDefaultUser() = %v, want %v", got, nil)
		}
	})

	t.Run("Users with default user", func(t *testing.T) {
		users := Users{
			&core.User{Name: core.DefaultUserName, Role: core.RoleDeveloper},
		}
		if got := users.GetDefaultUser(); got == nil || got.Name != core.DefaultUserName {
			t.Errorf("Users.GetDefaultUser() = %v, want %v", got, core.DefaultUserName)
		}
	})

	t.Run("Users with no default user", func(t *testing.T) {
		users := Users{
			&core.User{Name: "custom", Role: core.RoleDeveloper},
		}
		if got := users.GetDefaultUser(); got != nil {
			t.Errorf("Users.GetDefaultUser() = %v, want %v", got, nil)
		}
	})
}

func TestUsers_Encode(t *testing.T) {
	t.Run("empty Users slice", func(t *testing.T) {
		users := Users{}
		if got := users.Encode(false); len(got) != 0 {
			t.Errorf("Users.Encode() = %v, want %v", got, map[string]string{})
		}
	})

	t.Run("Users with valid users", func(t *testing.T) {
		users := Users{
			&core.User{Name: "user1", Role: core.RoleDeveloper},
		}
		got := users.Encode(false)
		if len(got) != 1 {
			t.Errorf("Users.Encode() = %v, want %v", len(got), 1)
		}
	})
}

func TestUsers_IsChanged(t *testing.T) {
	t.Run("different lengths", func(t *testing.T) {
		users1 := Users{
			&core.User{Name: "user1", Role: core.RoleDeveloper},
		}
		users2 := Users{}
		if got := users1.IsChanged(users2); got != true {
			t.Errorf("Users.IsChanged() = %v, want %v", got, true)
		}
	})

	t.Run("identical Users slices", func(t *testing.T) {
		users1 := Users{
			&core.User{Name: "user1", Role: core.RoleDeveloper},
		}
		users2 := Users{
			&core.User{Name: "user1", Role: core.RoleDeveloper},
		}
		if got := users1.IsChanged(users2); got != false {
			t.Errorf("Users.IsChanged() = %v, want %v", got, false)
		}
	})

	t.Run("different Users slices", func(t *testing.T) {
		users1 := Users{
			&core.User{Name: "user1", Role: core.RoleDeveloper},
		}
		users2 := Users{
			&core.User{Name: "user2", Role: core.RoleDeveloper},
		}
		if got := users1.IsChanged(users2); got != true {
			t.Errorf("Users.IsChanged() = %v, want %v", got, true)
		}
	})
}

func TestPatchClusterClientRequiredRules(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{
			name:    "test",
			raw:     "allkeys +@all -flushall -flushdb",
			want:    "+@all -flushall -flushdb ~*",
			wantErr: false,
		},
		{
			name:    "notdangerous",
			raw:     "allkeys +@all -@dangerous",
			want:    "+@all -@dangerous +cluster|slots +cluster|nodes +cluster|info +cluster|keyslot +cluster|getkeysinslot +cluster|countkeysinslot ~*",
			wantErr: false,
		},
		{
			name:    "readwrite",
			raw:     "allkeys -@all +@write +@read -@dangerous",
			want:    "-@all +@write +@read -@dangerous +cluster|slots +cluster|nodes +cluster|info +cluster|keyslot +cluster|getkeysinslot +cluster|countkeysinslot ~*",
			wantErr: false,
		},
		{
			name:    "readonly",
			raw:     "allkeys -@all +@read -keys",
			want:    "-@all +@read -keys +cluster|slots +cluster|nodes +cluster|info +cluster|keyslot +cluster|getkeysinslot +cluster|countkeysinslot ~*",
			wantErr: false,
		},
		{
			name:    "administrator",
			raw:     "allkeys +@all -acl",
			want:    "+@all -acl ~*",
			wantErr: false,
		},
		{
			name:    "support subcommand",
			raw:     "allkeys -@admin +config|get",
			want:    "-@admin +config|get +cluster|slots +cluster|nodes +cluster|info +cluster|keyslot +cluster|getkeysinslot +cluster|countkeysinslot ~*",
			wantErr: false,
		},
		{
			name:    "disable cmd enable subcommand",
			raw:     "allkeys -config +config|get",
			want:    "-config +config|get +cluster|slots +cluster|nodes +cluster|info +cluster|keyslot +cluster|getkeysinslot +cluster|countkeysinslot ~*",
			wantErr: false,
		},
		{
			name:    "fixed acl",
			raw:     "+@all -acl +acl|setuser -flushall -flushdb -keys ~* &*",
			want:    "+@all -acl -flushall -flushdb -keys +acl|setuser ~* &*",
			wantErr: false,
		},
		{
			name:    "disable cluster",
			raw:     "+@all -cluster ~* &*",
			want:    "+@all -cluster +cluster|slots +cluster|nodes +cluster|info +cluster|keyslot +cluster|getkeysinslot +cluster|countkeysinslot ~* &*",
			wantErr: false,
		},
		{
			name:    "disable cluster subcommand",
			raw:     "+@all -cluster|nodes ~* &*",
			want:    "+@all ~* &*",
			wantErr: false,
		},
		{
			name:    "enable cluster subcommand",
			raw:     "+@all +cluster -cluster|nodes ~* &*",
			want:    "+@all +cluster ~* &*",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := user.NewRule(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if rule != nil {
				rule = PatchClusterClientRequiredRules(rule)
				if got := rule.Encode(); got != tt.want {
					t.Errorf("PatchValkeyClusterClientRequiredRules() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestPatchPubsubRules(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{
			name:    "empty",
			raw:     "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "channel enabled indirect",
			raw:     "allkeys +@all -flushall -flushdb",
			want:    "+@all -flushall -flushdb ~* &*",
			wantErr: false,
		},
		{
			name:    "channel not enabled",
			raw:     "allkeys +get +set -flushall -flushdb",
			want:    "+get +set -flushall -flushdb ~*",
			wantErr: false,
		},
		{
			name:    "channel enabled",
			raw:     "allkeys +get +set -flushall -flushdb &*",
			want:    "+get +set -flushall -flushdb ~* &*",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := user.NewRule(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if rule != nil {
				rule = PatchPubsubRules(rule)
				if got := rule.Encode(); got != tt.want {
					t.Errorf("PatchValkeyPubsubRules() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

var (
	validSecret = &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Data: map[string][]byte{
			"password": []byte("password"),
		},
	}
	invalidSecret = &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Data: map[string][]byte{},
	}
	validPassword, _ = user.NewPassword(validSecret)
)

func TestNewOperatorUser(t *testing.T) {
	type args struct {
		secret *v1.Secret
	}
	tests := []struct {
		name    string
		args    args
		want    *user.User
		wantErr bool
	}{
		{
			name: "without secret",
			args: args{
				secret: nil,
			},
			want: &user.User{
				Name: user.DefaultOperatorUserName,
				Role: user.RoleOperator,
				Rules: []*user.Rule{
					{Categories: []string{"all"}, DisallowedCommands: []string{"keys"}, KeyPatterns: []string{"*"}, Channels: []string{"*"}},
				},
			},
			wantErr: false,
		},
		{
			name: "with secret",
			args: args{
				secret: validSecret,
			},
			want: &user.User{
				Name: user.DefaultOperatorUserName,
				Role: user.RoleOperator,
				Rules: []*user.Rule{
					{Categories: []string{"all"}, DisallowedCommands: []string{"keys"}, KeyPatterns: []string{"*"}, Channels: []string{"*"}},
				},
				Password: validPassword,
			},
			wantErr: false,
		},
		{
			name: "with invalid secret",
			args: args{
				secret: invalidSecret,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewOperatorUser(tt.args.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOperatorUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewOperatorUser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewSentinelUser(t *testing.T) {
	type args struct {
		name   string
		role   user.UserRole
		secret *v1.Secret
	}
	tests := []struct {
		name    string
		args    args
		want    *user.User
		wantErr bool
	}{
		{
			name: "custom user without secret",
			args: args{
				name:   "",
				role:   user.RoleDeveloper,
				secret: nil,
			},
			want: &user.User{
				Name: "",
				Role: user.RoleDeveloper,
			},
			wantErr: false,
		},
		{
			name: "custom user with secret",
			args: args{
				name:   "test",
				role:   user.RoleDeveloper,
				secret: validSecret,
			},
			want: &user.User{
				Name:     "test",
				Role:     user.RoleDeveloper,
				Password: validPassword,
			},
			wantErr: false,
		},
		{
			name: "with invalid secret, enable acl2",
			args: args{
				name:   "test1",
				role:   user.RoleDeveloper,
				secret: invalidSecret,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewSentinelUser(tt.args.name, tt.args.role, tt.args.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSentinelUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewSentinelUser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewUserFromValkeyUser(t *testing.T) {
	type args struct {
		username string
		ruleStr  string
		pwd      *user.Password
	}
	tests := []struct {
		name    string
		args    args
		want    *user.User
		wantErr bool
	}{
		{
			name: "default user",
			args: args{
				username: user.DefaultUserName,
				ruleStr:  "+@all -flushall -flushdb -keys ~*",
				pwd:      nil,
			},
			want: &user.User{
				Name: user.DefaultUserName,
				Role: user.RoleDeveloper,
				Rules: []*user.Rule{
					{Categories: []string{"all"}, DisallowedCommands: []string{"flushall", "flushdb", "keys"}, KeyPatterns: []string{"*"}},
				},
			},
		},
		{
			name: "invald user",
			args: args{
				username: user.DefaultUserName,
				ruleStr:  "+@all -flushall -flushdb -keys ~* +@test",
				pwd:      nil,
			},
			wantErr: true,
		},
		{
			name: "operator user",
			args: args{
				username: user.DefaultOperatorUserName,
				ruleStr:  "+@all -keys ~*",
				pwd:      validPassword,
			},
			want: &user.User{
				Name: user.DefaultOperatorUserName,
				Role: user.RoleOperator,
				Rules: []*user.Rule{
					{Categories: []string{"all"}, DisallowedCommands: []string{"keys"}, KeyPatterns: []string{"*"}},
				},
				Password: validPassword,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewUserFromValkeyUser(tt.args.username, tt.args.ruleStr, tt.args.pwd)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewUserFromValkeyUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewUserFromValkeyUser() = %v, want %v", got, tt.want)
			}
		})
	}
}
