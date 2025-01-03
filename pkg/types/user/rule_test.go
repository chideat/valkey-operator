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

package user

import (
	"testing"
)

func TestNewRule(t *testing.T) {
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
			name:    "allcommands",
			raw:     "allcommands",
			want:    "+@all",
			wantErr: false,
		},
		{
			name:    "nocommands",
			raw:     "nocommands",
			want:    "-@all",
			wantErr: false,
		},
		{
			name:    "allchannels",
			raw:     "allchannels",
			want:    "&*",
			wantErr: false,
		},
		{
			name:    "read key",
			raw:     "%R~test",
			want:    "%R~test",
			wantErr: false,
		},
		{
			name:    "write key",
			raw:     "%W~test",
			want:    "%W~test",
			wantErr: false,
		},
		{
			name:    "test",
			raw:     "allkeys +@all -flushall -flushdb",
			want:    "+@all -flushall -flushdb ~*",
			wantErr: false,
		},
		{
			name:    "notdangerous",
			raw:     "allkeys +@all -@dangerous",
			want:    "+@all -@dangerous ~*",
			wantErr: false,
		},
		{
			name:    "readwrite",
			raw:     "allkeys -@all +@write +@read -@dangerous",
			want:    "-@all +@write +@read -@dangerous ~*",
			wantErr: false,
		},
		{
			name:    "readonly",
			raw:     "allkeys -@all +@read -keys",
			want:    "-@all +@read -keys ~*",
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
			want:    "-@admin +config|get ~*",
			wantErr: false,
		},
		{
			name:    "disable cmd enable subcommand",
			raw:     "allkeys -config +config|get",
			want:    "-config +config|get ~*",
			wantErr: false,
		},
		{
			name:    "fixed acl",
			raw:     "+@all -acl +acl|setuser -flushall -flushdb -keys ~* &*",
			want:    "+@all -acl -flushall -flushdb -keys +acl|setuser ~* &*",
			wantErr: false,
		},
		{
			name:    "withpassword",
			raw:     "allkeys +@all >admin@123",
			wantErr: true,
		},
		{
			name:    "sanitize-payload",
			raw:     "allkeys +@all sanitize-payload",
			wantErr: true,
		},
		{
			name:    "skip-sanitize-payload",
			raw:     "allkeys +@all skip-sanitize-payload",
			wantErr: true,
		},
		{
			name:    "not allowed category",
			raw:     "+@test",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := NewRule(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if rule != nil {
				if rule.Encode() != tt.want {
					t.Errorf("NewRule() = %v, want %v", rule, tt.want)
				}
			}
		})
	}
}

func TestRule_Format(t *testing.T) {
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
			want:    "+@all -@dangerous ~*",
			wantErr: false,
		},
		{
			name:    "readwrite",
			raw:     "allkeys -@all +@write +@read -@dangerous",
			want:    "-@all +@write +@read -@dangerous ~*",
			wantErr: false,
		},
		{
			name:    "readonly",
			raw:     "allkeys -@all +@read -keys",
			want:    "-@all +@read -keys ~*",
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
			want:    "-@admin +config|get ~*",
			wantErr: false,
		},
		{
			name:    "disable cmd enable subcommand",
			raw:     "allkeys -config +config|get",
			want:    "-config +config|get ~*",
			wantErr: false,
		},
		{
			name:    "fixed acl",
			raw:     "+@all -acl +acl|setuser -flushall -flushdb -keys ~* &*",
			want:    "+@all -acl -flushall -flushdb -keys +acl|setuser ~* &*",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := NewRule(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRule() error = %v, wantErr: %v", err, tt.wantErr)
				return
			}
			if rule != nil {
				if got := rule.Encode(); got != tt.want {
					t.Errorf("Rule.Encode() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestRule_Validate(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		disableAcl bool
		wantErr    bool
	}{
		{
			name:       "test",
			raw:        "allkeys +@all -flushall -flushdb",
			disableAcl: true,
			wantErr:    true,
		},
		{
			name:       "notdangerous",
			raw:        "allkeys +@all -@dangerous",
			disableAcl: true,
			wantErr:    false,
		},
		{
			name:       "readwrite",
			raw:        "allkeys -@all +@write +@read -@dangerous",
			disableAcl: true,
			wantErr:    false,
		},
		{
			name:       "readonly",
			raw:        "allkeys -@all +@read -keys",
			disableAcl: true,
			wantErr:    false,
		},
		{
			name:       "administrator",
			raw:        "allkeys +@all -acl",
			disableAcl: true,
			wantErr:    false,
		},
		{
			name:       "support subcommand",
			raw:        "allkeys -@admin +config|get",
			disableAcl: true,
			wantErr:    false,
		},
		{
			name:       "disable cmd enable subcommand",
			raw:        "allkeys -config +config|get",
			disableAcl: true,
			wantErr:    false,
		},
		{
			name:       "default user for 7.0",
			raw:        "+@all -acl -flushall -flushdb -keys ~* &*",
			disableAcl: true,
			wantErr:    false,
		},
		{
			name:       "fixed acl",
			raw:        "+@all -acl +acl|setuser -flushall -flushdb -keys ~* &*",
			disableAcl: true,
			wantErr:    true,
		},
		{
			name:       "fixed acl",
			raw:        "+@all -acl +acl|setuser -flushall -flushdb -keys ~* &*",
			disableAcl: true,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewRule(tt.raw)
			if err != nil {
				t.Errorf("NewRule() error = %v, ", err)
				return
			}
			if err := r.Validate(tt.disableAcl); (err != nil) != tt.wantErr {
				t.Errorf("Rule.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
