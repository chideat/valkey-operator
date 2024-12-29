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

package redis

import (
	"reflect"
	"testing"

	"github.com/chideat/valkey-operator/api/core"
)

func TestParseRedisVersion(t *testing.T) {
	type args struct {
		v string
	}
	tests := []struct {
		name    string
		args    args
		want    RedisVersion
		wantErr bool
	}{
		{
			name:    "patch version",
			args:    args{v: "6.0-alpine.000b26a0c3b6"},
			want:    RedisVersion6,
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
			got, err := ParseRedisVersion(tt.args.v)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRedisVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if got != tt.want {
				t.Errorf("ParseRedisVersion() = %v, want %v", got, tt.want)
			}
			if got.String() != "6.0" {
				t.Errorf("ParseRedisVersion().String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseRedisVersionFromImage(t *testing.T) {
	type args struct {
		u string
	}
	tests := []struct {
		name    string
		args    args
		want    RedisVersion
		wantErr bool
	}{
		{
			name:    "4.0-alpine",
			args:    args{u: "build-harbor.alauda.cn/middleware/redis:4.0-alpine"},
			want:    RedisVersion4,
			wantErr: false,
		},
		{
			name:    "4.0-alpine.xxx",
			args:    args{u: "build-harbor.alauda.cn/middleware/redis:4.0-alpine.000b26a0c3b6"},
			want:    RedisVersion4,
			wantErr: false,
		},
		{
			name:    "5.0-alpine",
			args:    args{u: "build-harbor.alauda.cn/middleware/redis:5.0-alpine"},
			want:    RedisVersion5,
			wantErr: false,
		},
		{
			name:    "5.0-alpine.xxx",
			args:    args{u: "build-harbor.alauda.cn/middleware/redis:5.0-alpine.000b26a0c3b6"},
			want:    RedisVersion5,
			wantErr: false,
		},
		{
			name:    "invalid image",
			args:    args{u: "build-harbor.alauda.cn/middleware/redis5.0-alpine.000b26a0c3b6"},
			want:    "",
			wantErr: true,
		},
		{
			name:    "latest",
			args:    args{u: "build-harbor.alauda.cn/middleware/redis:latest"},
			want:    RedisVersion6,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRedisVersionFromImage(tt.args.u)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRedisVersionFromImage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseRedisVersionFromImage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTLSSupported(t *testing.T) {
	tests := []struct {
		name string
		v    RedisVersion
		want bool
	}{
		{
			name: "TLS supported 7.2",
			v:    RedisVersion7_2,
			want: true,
		},
		{
			name: "TLS supported 7.0",
			v:    RedisVersion7,
			want: true,
		},
		{
			name: "TLS supported 6.0",
			v:    RedisVersion6,
			want: true,
		},
		{
			name: "TLS not supported 5.0",
			v:    RedisVersion5,
			want: false,
		},
		{
			name: "TLS not supported 4.0",
			v:    RedisVersion4,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.IsTLSSupported(); got != tt.want {
				t.Errorf("IsTLSSupported() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsACLSupported(t *testing.T) {
	tests := []struct {
		name string
		v    RedisVersion
		want bool
	}{
		{
			name: "ACL supported 7.2",
			v:    RedisVersion7_2,
			want: true,
		},
		{
			name: "ACL supported 7.0",
			v:    RedisVersion7,
			want: true,
		},
		{
			name: "ACL supported 6.0",
			v:    RedisVersion6,
			want: true,
		},
		{
			name: "ACL not supported 5.0",
			v:    RedisVersion5,
			want: false,
		},
		{
			name: "ACL not supported 4.0",
			v:    RedisVersion4,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.IsACLSupported(); got != tt.want {
				t.Errorf("IsACLSupported() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsACL2Supported(t *testing.T) {
	tests := []struct {
		name string
		v    RedisVersion
		want bool
	}{
		{
			name: "ACL2 supported 7.2",
			v:    RedisVersion7_2,
			want: true,
		},
		{
			name: "ACL2 supported 7.0",
			v:    RedisVersion7,
			want: true,
		},
		{
			name: "ACL2 not supported 6.0",
			v:    RedisVersion6,
			want: false,
		},
		{
			name: "ACL2 not supported 5.0",
			v:    RedisVersion5,
			want: false,
		},
		{
			name: "ACL2 not supported 4.0",
			v:    RedisVersion4,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.IsACL2Supported(); got != tt.want {
				t.Errorf("IsACL2Supported() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsClusterShardSupported(t *testing.T) {
	tests := []struct {
		name string
		v    RedisVersion
		want bool
	}{
		{
			name: "Cluster shard supported 7.2",
			v:    RedisVersion7_2,
			want: true,
		},
		{
			name: "Cluster shard supported 7.0",
			v:    RedisVersion7,
			want: true,
		},
		{
			name: "Cluster shard not supported 6.0",
			v:    RedisVersion6,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.IsClusterShardSupported(); got != tt.want {
				t.Errorf("IsClusterShardSupported() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCustomConfigs(t *testing.T) {
	tests := []struct {
		name string
		v    RedisVersion
		arch RedisArch
		want map[string]string
	}{
		{
			name: "Redis 5 with ARM64",
			v:    RedisVersion5,
			arch: core.ValkeyCluster,
			want: map[string]string{
				"ignore-warnings":           "ARM64-COW-BUG",
				"cluster-migration-barrier": "10",
			},
		},
		{
			name: "Redis 7 with ARM64",
			v:    RedisVersion7,
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
