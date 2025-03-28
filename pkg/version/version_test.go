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
	"reflect"
	"testing"

	"github.com/chideat/valkey-operator/api/core"
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

func TestCustomConfigs(t *testing.T) {
	tests := []struct {
		name string
		v    ValkeyVersion
		arch core.Arch
		want map[string]string
	}{
		{
			name: "Valkey 7.4 with ARM64",
			v:    ValkeyVersion("7.4"),
			arch: core.ValkeyCluster,
			want: map[string]string{
				"ignore-warnings":                 "ARM64-COW-BUG",
				"cluster-allow-replica-migration": "no",
				"cluster-migration-barrier":       "10",
			},
		},
		{
			name: "Valkey 8.0 with ARM64",
			v:    ValkeyVersion("8.0"),
			arch: core.ValkeyCluster,
			want: map[string]string{
				"ignore-warnings":                 "ARM64-COW-BUG",
				"cluster-allow-replica-migration": "no",
				"cluster-migration-barrier":       "10",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.CustomConfigs(tt.arch); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CustomConfigs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValkeyVersionCompare(t *testing.T) {
	tests := []struct {
		name string
		v1   ValkeyVersion
		v2   ValkeyVersion
		want int
	}{
		{
			name: "v1 major greater than v2",
			v1:   "7.0",
			v2:   "6.0",
			want: 1,
		},
		{
			name: "v1 major less than v2",
			v1:   "6.0",
			v2:   "7.0",
			want: -1,
		},
		{
			name: "v1 minor greater than v2",
			v1:   "7.2",
			v2:   "7.0",
			want: 1,
		},
		{
			name: "v1 minor less than v2",
			v1:   "7.0",
			v2:   "7.2",
			want: -1,
		},
		{
			name: "equal versions",
			v1:   "6.0",
			v2:   "6.0",
			want: 0,
		},
		{
			name: "both empty versions",
			v1:   ValkeyVersionUnknown,
			v2:   ValkeyVersionUnknown,
			want: 0,
		},
		{
			name: "v1 empty",
			v1:   ValkeyVersionUnknown,
			v2:   "6.0",
			want: -1,
		},
		{
			name: "v2 empty",
			v1:   "6.0",
			v2:   ValkeyVersionUnknown,
			want: 1,
		},
		{
			name: "invalid v1",
			v1:   "invalid",
			v2:   "6.0",
			want: -2,
		},
		{
			name: "invalid v2",
			v1:   "6.0",
			v2:   "invalid",
			want: -2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v1.Compare(tt.v2); got != tt.want {
				t.Errorf("ValkeyVersion.Compare() = %v, want %v", got, tt.want)
			}
		})
	}
}
