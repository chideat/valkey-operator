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

package user

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
)

func TestNewUser(t *testing.T) {
	type args struct {
		name        string
		role        UserRole
		secret      *v1.Secret
		acl2Support bool
	}
	tests := []struct {
		name    string
		args    args
		want    *User
		wantErr bool
	}{
		{
			name: "default user, disable acl2",
			args: args{
				name:        "",
				role:        RoleDeveloper,
				secret:      nil,
				acl2Support: false,
			},
			want: &User{
				Name: DefaultUserName,
				Role: RoleDeveloper,
				Rules: []*Rule{
					{Categories: []string{"all"}, DisallowedCommands: []string{"acl", "flushall", "flushdb", "keys"}, KeyPatterns: []string{"*"}},
				},
			},
			wantErr: false,
		},
		{
			name: "default user with name and disable acl2",
			args: args{
				name:        DefaultUserName,
				role:        RoleDeveloper,
				secret:      nil,
				acl2Support: false,
			},
			want: &User{
				Name: DefaultUserName,
				Role: RoleDeveloper,
				Rules: []*Rule{
					{Categories: []string{"all"}, DisallowedCommands: []string{"acl", "flushall", "flushdb", "keys"}, KeyPatterns: []string{"*"}},
				},
			},
			wantErr: false,
		},
		{
			name: "default user, enable acl2",
			args: args{
				name:        "",
				role:        RoleDeveloper,
				secret:      nil,
				acl2Support: true,
			},
			want: &User{
				Name: DefaultUserName,
				Role: RoleDeveloper,
				Rules: []*Rule{
					{Categories: []string{"all"}, DisallowedCommands: []string{"acl", "flushall", "flushdb", "keys"},
						KeyPatterns: []string{"*"}, Channels: []string{"*"}},
				},
			},
			wantErr: false,
		},
		{
			name: "custom user, disable acl2",
			args: args{
				name:        "debug",
				role:        RoleDeveloper,
				secret:      nil,
				acl2Support: false,
			},
			want: &User{
				Name: "debug",
				Role: RoleDeveloper,
				Rules: []*Rule{
					{Categories: []string{"all"}, KeyPatterns: []string{"*"}},
				},
			},
			wantErr: false,
		},
		{
			name: "custom user, enable acl2",
			args: args{
				name:        "debug",
				role:        RoleDeveloper,
				secret:      nil,
				acl2Support: true,
			},
			want: &User{
				Name: "debug",
				Role: RoleDeveloper,
				Rules: []*Rule{
					{Categories: []string{"all"}, KeyPatterns: []string{"*"}, Channels: []string{"*"}},
				},
			},
			wantErr: false,
		},
		{
			name: "operator user, enable acl2",
			args: args{
				name:        DefaultOperatorUserName,
				role:        RoleOperator,
				secret:      nil,
				acl2Support: true,
			},
			want: &User{
				Name: DefaultOperatorUserName,
				Role: RoleOperator,
				Rules: []*Rule{
					{Categories: []string{"all"}, KeyPatterns: []string{"*"}, Channels: []string{"*"}},
				},
			},
			wantErr: false,
		},
		{
			name: "default user with secret",
			args: args{
				name:        DefaultUserName,
				role:        RoleDeveloper,
				secret:      validSecret,
				acl2Support: true,
			},
			want: &User{
				Name: DefaultUserName,
				Role: RoleDeveloper,
				Rules: []*Rule{
					{Categories: []string{"all"}, DisallowedCommands: []string{"acl", "flushall", "flushdb", "keys"},
						KeyPatterns: []string{"*"}, Channels: []string{"*"}},
				},
				Password: &Password{
					SecretName: "test",
					secret:     validSecret,
					data:       "password",
				},
			},
			wantErr: false,
		},
		{
			name: "with invalid secret, enable acl2",
			args: args{
				name:        DefaultUserName,
				role:        RoleDeveloper,
				secret:      invalidSecret,
				acl2Support: true,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewUser(tt.args.name, tt.args.role, tt.args.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewUser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUser_GetPassword(t *testing.T) {
	type fields struct {
		Name     string
		Role     UserRole
		Password *Password
		Rules    []*Rule
	}
	tests := []struct {
		name   string
		fields fields
		want   *Password
	}{
		{
			name:   "without password",
			fields: fields{},
			want:   nil,
		},
		{
			name: "with password",
			fields: fields{
				Password: &Password{
					SecretName: "test",
					secret:     validSecret,
					data:       "password",
				},
			},
			want: &Password{
				SecretName: "test",
				secret:     validSecret,
				data:       "password",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &User{
				Name:     tt.fields.Name,
				Role:     tt.fields.Role,
				Password: tt.fields.Password,
				Rules:    tt.fields.Rules,
			}
			if got := u.GetPassword(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("User.GetPassword() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUser_AppendRule(t *testing.T) {
	type fields struct {
		Name     string
		Role     UserRole
		Password *Password
		Rules    []*Rule
	}
	type args struct {
		rules []*Rule
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "nil append",
			fields: fields{
				Rules: []*Rule{},
			},
			args:    args{rules: []*Rule{}},
			wantErr: false,
		},
		{
			name: "append",
			fields: fields{
				Rules: []*Rule{},
			},
			args:    args{rules: []*Rule{{Categories: []string{"all"}, DisallowedCommands: []string{"keys"}, KeyPatterns: []string{"*"}}}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &User{
				Name:     tt.fields.Name,
				Role:     tt.fields.Role,
				Password: tt.fields.Password,
				Rules:    tt.fields.Rules,
			}
			if err := u.AppendRule(tt.args.rules...); (err != nil) != tt.wantErr {
				t.Errorf("User.AppendRule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUser_Validate(t *testing.T) {
	type fields struct {
		Name     string
		Role     UserRole
		Password *Password
		Rules    []*Rule
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "default user",
			fields: fields{
				Name: DefaultUserName,
				Role: RoleDeveloper,
				Rules: []*Rule{
					{Categories: []string{"all"}, DisallowedCommands: []string{"acl", "flushall", "flushdb", "keys"}, KeyPatterns: []string{"*"}},
				},
			},
			wantErr: false,
		},
		{
			name: "operator user",
			fields: fields{
				Name: DefaultOperatorUserName,
				Role: RoleOperator,
				Rules: []*Rule{
					{Categories: []string{"all"}, DisallowedCommands: []string{"keys"}, KeyPatterns: []string{"*"}},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid username",
			fields: fields{
				Name: "adwf_13123r",
				Role: RoleOperator,
				Rules: []*Rule{
					{Categories: []string{"all"}, DisallowedCommands: []string{"keys"}, KeyPatterns: []string{"*"}},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid user role",
			fields: fields{
				Name: "adwf",
				Role: "test",
				Rules: []*Rule{
					{Categories: []string{"all"}, DisallowedCommands: []string{"keys"}, KeyPatterns: []string{"*"}},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &User{
				Name:     tt.fields.Name,
				Role:     tt.fields.Role,
				Password: tt.fields.Password,
				Rules:    tt.fields.Rules,
			}
			if err := u.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("User.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUser_String(t *testing.T) {
	type fields struct {
		Name     string
		Role     UserRole
		Password *Password
		Rules    []*Rule
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "default user",
			fields: fields{
				Name: DefaultUserName,
				Role: RoleDeveloper,
				Rules: []*Rule{
					{Categories: []string{"all"}, DisallowedCommands: []string{"acl", "flushall", "flushdb", "keys"}, KeyPatterns: []string{"*"}},
				},
			},
			want: "default Developer +@all -acl -flushall -flushdb -keys ~*",
		},
		{
			name: "operator user",
			fields: fields{
				Name: DefaultOperatorUserName,
				Role: RoleOperator,
				Rules: []*Rule{
					{Categories: []string{"all"}, DisallowedCommands: []string{"keys"}, KeyPatterns: []string{"*"}},
				},
			},
			want: "operator Operator +@all -keys ~*",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &User{
				Name:     tt.fields.Name,
				Role:     tt.fields.Role,
				Password: tt.fields.Password,
				Rules:    tt.fields.Rules,
			}
			if got := u.String(); got != tt.want {
				t.Errorf("User.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewPassword(t *testing.T) {
	type args struct {
		secret *v1.Secret
	}
	tests := []struct {
		name    string
		args    args
		want    *Password
		wantErr bool
	}{
		{
			name: "valid secret",
			args: args{
				secret: validSecret,
			},
			want: &Password{
				SecretName: "test",
				secret:     validSecret,
				data:       "password",
			},
			wantErr: false,
		},
		{
			name: "invalid secret",
			args: args{
				secret: invalidSecret,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewPassword(tt.args.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewPassword() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPassword_GetSecretName(t *testing.T) {
	type fields struct {
		SecretName string
		secret     *v1.Secret
		data       string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name:   "without secret",
			fields: fields{},
			want:   "",
		},
		{
			name: "with secret",
			fields: fields{
				SecretName: "test",
				secret:     validSecret,
				data:       "password",
			},
			want: "test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Password{
				SecretName: tt.fields.SecretName,
				secret:     tt.fields.secret,
				data:       tt.fields.data,
			}
			if got := p.GetSecretName(); got != tt.want {
				t.Errorf("Password.GetSecretName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPassword_SetSecret(t *testing.T) {
	type args struct {
		secret *v1.Secret
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "valid secret",
			args: args{
				secret: validSecret,
			},
			wantErr: false,
		},
		{
			name: "invalid secret",
			args: args{
				secret: invalidSecret,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Password{}
			if err := p.SetSecret(tt.args.secret); (err != nil) != tt.wantErr {
				t.Errorf("Password.SetSecret() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPassword_Secret(t *testing.T) {
	type fields struct {
		SecretName string
		secret     *v1.Secret
		data       string
	}
	tests := []struct {
		name   string
		fields fields
		want   *v1.Secret
	}{
		{
			name:   "without secret",
			fields: fields{},
			want:   nil,
		},
		{
			name: "with secret",
			fields: fields{
				SecretName: "test",
				secret:     validSecret,
				data:       "password",
			},
			want: validSecret,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Password{
				SecretName: tt.fields.SecretName,
				secret:     tt.fields.secret,
				data:       tt.fields.data,
			}
			if got := p.Secret(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Password.Secret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPassword_String(t *testing.T) {
	type fields struct {
		SecretName string
		secret     *v1.Secret
		data       string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name:   "without secret",
			fields: fields{},
			want:   "",
		},
		{
			name: "with secret",
			fields: fields{
				SecretName: "test",
				secret:     validSecret,
				data:       "password",
			},
			want: "password",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Password{
				SecretName: tt.fields.SecretName,
				secret:     tt.fields.secret,
				data:       tt.fields.data,
			}
			if got := p.String(); got != tt.want {
				t.Errorf("Password.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
