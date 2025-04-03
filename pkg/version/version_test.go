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

package version

import (
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/stretchr/testify/assert"
)

func TestParseValkeyVersion(t *testing.T) {
	type args struct {
		v string
	}
	tests := []struct {
		name    string
		args    args
		want    ValkeyVersion
		wantErr bool
	}{
		{
			name:    "patch version",
			args:    args{v: "7.4-alpine.000b26a0c3b6"},
			want:    ValkeyVersion("7.4"),
			wantErr: false,
		},
		{
			name:    "patch invalid version",
			args:    args{v: "abcdefg"},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseValkeyVersion(tt.args.v)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseValkeyVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if got != tt.want {
				t.Errorf("ParseValkeyVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseValkeyVersionFromImage(t *testing.T) {
	type args struct {
		u string
	}
	tests := []struct {
		name    string
		args    args
		want    ValkeyVersion
		wantErr bool
	}{
		{
			name:    "7.4-alpine",
			args:    args{u: "valkey:7.4-alpine"},
			want:    ValkeyVersion("7.4"),
			wantErr: false,
		},
		{
			name:    "7.4-alpine.xxx",
			args:    args{u: "valkey:7.4-alpine.000b26a0c3b6"},
			want:    ValkeyVersion("7.4"),
			wantErr: false,
		},
		{
			name:    "invalid image",
			args:    args{u: "valkey-7.4-alpine.000b26a0c3b6"},
			want:    "",
			wantErr: true,
		},
		{
			name:    "latest",
			args:    args{u: "valkey:latest"},
			want:    ValkeyVersion("8.0"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseValkeyVersionFromImage(tt.args.u)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseValkeyVersionFromImage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseValkeyVersionFromImage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValkeyVersion_String(t *testing.T) {
	tests := []struct {
		name string
		v    ValkeyVersion
		want string
	}{
		{
			name: "empty",
			v:    "",
			want: "",
		},
		{
			name: "version",
			v:    "8.0",
			want: "8.0",
		},
		{
			name: "default",
			v:    DefaultValKeyVersion,
			want: "8.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.String(); got != tt.want {
				t.Errorf("ValkeyVersion.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValkeyVersion_CustomConfigs(t *testing.T) {
	tests := []struct {
		name string
		v    ValkeyVersion
		arch core.Arch
		want map[string]string
	}{
		{
			name: "empty version",
			v:    "",
			arch: core.ValkeyCluster,
			want: nil,
		},
		{
			name: "cluster arch",
			v:    "8.0",
			arch: core.ValkeyCluster,
			want: map[string]string{
				"ignore-warnings":                 "ARM64-COW-BUG",
				"cluster-allow-replica-migration": "no",
				"cluster-migration-barrier":       "10",
			},
		},
		{
			name: "non-cluster arch",
			v:    "8.0",
			arch: core.ValkeySentinel,
			want: map[string]string{
				"ignore-warnings": "ARM64-COW-BUG",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.CustomConfigs(tt.arch)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValkeyVersion_Compare(t *testing.T) {
	tests := []struct {
		name    string
		v       ValkeyVersion
		other   ValkeyVersion
		want    int
		wantErr bool
	}{
		{
			name:    "equal versions",
			v:       "8.0",
			other:   "8.0",
			want:    0,
			wantErr: false,
		},
		{
			name:    "greater version",
			v:       "8.1",
			other:   "8.0",
			want:    1,
			wantErr: false,
		},
		{
			name:    "lesser version",
			v:       "7.0",
			other:   "8.0",
			want:    -1,
			wantErr: false,
		},
		{
			name:    "empty versions",
			v:       "",
			other:   "",
			want:    0,
			wantErr: false,
		},
		{
			name:    "empty vs version",
			v:       "",
			other:   "8.0",
			want:    -1,
			wantErr: false,
		},
		{
			name:    "version vs empty",
			v:       "8.0",
			other:   "",
			want:    1,
			wantErr: false,
		},
		{
			name:    "invalid version",
			v:       "invalid",
			other:   "8.0",
			want:    -2,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.Compare(tt.other)
			if got != tt.want {
				t.Errorf("ValkeyVersion.Compare() = %v, want %v", got, tt.want)
			}
		})
	}
}
