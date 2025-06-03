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

package builder

import (
	"reflect"
	"testing"
	"time"

	"github.com/chideat/valkey-operator/api/core"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func TestChecksumKey(t *testing.T) {
	tests := []struct {
		name string
		typ  string
		want string
	}{
		{
			name: "lowercase type",
			typ:  "config",
			want: ChecksumLabelKey + "-config",
		},
		{
			name: "uppercase type",
			typ:  "CONFIG",
			want: ChecksumLabelKey + "-config",
		},
		{
			name: "mixed case type",
			typ:  "ConFig",
			want: ChecksumLabelKey + "-config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ChecksumKey(tt.typ); got != tt.want {
				t.Errorf("ChecksumKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourcePrefix(t *testing.T) {
	tests := []struct {
		name string
		arch core.Arch
		want string
	}{
		{
			name: "ValkeyCluster",
			arch: core.ValkeyCluster,
			want: "drc",
		},
		{
			name: "ValkeySentinel",
			arch: core.ValkeySentinel,
			want: "rfs",
		},
		{
			name: "Default",
			arch: core.Arch("other"),
			want: "rfr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResourcePrefix(tt.arch); got != tt.want {
				t.Errorf("ResourcePrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalhostAlias(t *testing.T) {
	tests := []struct {
		name   string
		family corev1.IPFamily
		want   corev1.HostAlias
	}{
		{
			name:   "IPv4",
			family: corev1.IPv4Protocol,
			want: corev1.HostAlias{
				IP:        "127.0.0.1",
				Hostnames: []string{"local.inject"},
			},
		},
		{
			name:   "IPv6",
			family: corev1.IPv6Protocol,
			want: corev1.HostAlias{
				IP:        "::1",
				Hostnames: []string{"local.inject"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LocalhostAlias(tt.family); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LocalhostAlias() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPullPolicy(t *testing.T) {
	tests := []struct {
		name     string
		policies []corev1.PullPolicy
		want     corev1.PullPolicy
	}{
		{
			name:     "No policies",
			policies: []corev1.PullPolicy{},
			want:     corev1.PullIfNotPresent,
		},
		{
			name:     "Empty policies",
			policies: []corev1.PullPolicy{"", ""},
			want:     corev1.PullIfNotPresent,
		},
		{
			name:     "First policy set",
			policies: []corev1.PullPolicy{corev1.PullAlways, corev1.PullNever},
			want:     corev1.PullAlways,
		},
		{
			name:     "Skip empty policies",
			policies: []corev1.PullPolicy{"", corev1.PullAlways},
			want:     corev1.PullAlways,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetPullPolicy(tt.policies...); got != tt.want {
				t.Errorf("GetPullPolicy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPodSecurityContext(t *testing.T) {
	defaultGroupId := int64(1000)
	customFsGroup := int64(2000)

	tests := []struct {
		name   string
		secctx *corev1.PodSecurityContext
		want   *corev1.PodSecurityContext
	}{
		{
			name:   "Nil security context",
			secctx: nil,
			want:   &corev1.PodSecurityContext{FSGroup: &defaultGroupId},
		},
		{
			name:   "Empty security context",
			secctx: &corev1.PodSecurityContext{},
			want:   &corev1.PodSecurityContext{},
		},
		{
			name:   "Security context with custom FSGroup",
			secctx: &corev1.PodSecurityContext{FSGroup: &customFsGroup},
			want:   &corev1.PodSecurityContext{FSGroup: &customFsGroup},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPodSecurityContext(tt.secctx)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetPodSecurityContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetSecurityContext(t *testing.T) {
	defaultUserId, defaultGroupId := int64(999), int64(1000)
	customUserId, customGroupId := int64(1234), int64(5678)
	runAsNonRoot := true
	readOnlyRootFilesystem := true

	tests := []struct {
		name   string
		secctx *corev1.PodSecurityContext
		want   *corev1.SecurityContext
	}{
		{
			name:   "Nil security context",
			secctx: nil,
			want: &corev1.SecurityContext{
				RunAsUser:              &defaultUserId,
				RunAsGroup:             &defaultGroupId,
				RunAsNonRoot:           ptr.To(true),
				ReadOnlyRootFilesystem: ptr.To(true),
			},
		},
		{
			name:   "Empty security context",
			secctx: &corev1.PodSecurityContext{},
			want: &corev1.SecurityContext{
				RunAsUser:              &defaultUserId,
				RunAsGroup:             &defaultGroupId,
				RunAsNonRoot:           ptr.To(true),
				ReadOnlyRootFilesystem: ptr.To(true),
			},
		},
		{
			name: "Security context with custom values",
			secctx: &corev1.PodSecurityContext{
				RunAsUser:    &customUserId,
				RunAsGroup:   &customGroupId,
				RunAsNonRoot: &runAsNonRoot,
			},
			want: &corev1.SecurityContext{
				RunAsUser:              &customUserId,
				RunAsGroup:             &customGroupId,
				RunAsNonRoot:           &runAsNonRoot,
				ReadOnlyRootFilesystem: &readOnlyRootFilesystem,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetSecurityContext(tt.secctx)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSecurityContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetContainerSecurityContext(t *testing.T) {
	defaultUserId, defaultGroupId := int64(999), int64(1000)
	customUserId, customGroupId := int64(1234), int64(5678)
	runAsNonRoot := true
	readOnlyRootFilesystem := true

	tests := []struct {
		name   string
		secctx *corev1.SecurityContext
		want   *corev1.SecurityContext
	}{
		{
			name:   "Nil security context",
			secctx: nil,
			want: &corev1.SecurityContext{
				RunAsUser:              &defaultUserId,
				RunAsGroup:             &defaultGroupId,
				RunAsNonRoot:           ptr.To(true),
				ReadOnlyRootFilesystem: ptr.To(true),
			},
		},
		{
			name:   "Empty security context",
			secctx: &corev1.SecurityContext{},
			want: &corev1.SecurityContext{
				RunAsUser:              &defaultUserId,
				RunAsGroup:             &defaultGroupId,
				RunAsNonRoot:           ptr.To(true),
				ReadOnlyRootFilesystem: ptr.To(true),
			},
		},
		{
			name: "Security context with custom values",
			secctx: &corev1.SecurityContext{
				RunAsUser:    &customUserId,
				RunAsGroup:   &customGroupId,
				RunAsNonRoot: &runAsNonRoot,
			},
			want: &corev1.SecurityContext{
				RunAsUser:              &customUserId,
				RunAsGroup:             &customGroupId,
				RunAsNonRoot:           &runAsNonRoot,
				ReadOnlyRootFilesystem: &readOnlyRootFilesystem,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetContainerSecurityContext(tt.secctx)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetContainerSecurityContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParsePodIndex(t *testing.T) {
	tests := []struct {
		name    string
		podName string
		want    int
		wantErr bool
	}{
		{
			name:    "Valid pod name",
			podName: "valkey-0",
			want:    0,
			wantErr: false,
		},
		{
			name:    "Valid pod name with multiple segments",
			podName: "valkey-cluster-1",
			want:    1,
			wantErr: false,
		},
		{
			name:    "Invalid pod name - too few segments",
			podName: "valkey",
			want:    -1,
			wantErr: true,
		},
		{
			name:    "Invalid pod name - non-numeric index",
			podName: "valkey-abc",
			want:    -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePodIndex(tt.podName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePodIndex() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParsePodIndex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParsePodShardAndIndex(t *testing.T) {
	tests := []struct {
		name    string
		podName string
		wantS   int
		wantI   int
		wantErr bool
	}{
		{
			name:    "Valid pod name",
			podName: "valkey-0-0",
			wantS:   0,
			wantI:   0,
			wantErr: false,
		},
		{
			name:    "Valid pod name with multiple segments",
			podName: "valkey-cluster-2-3",
			wantS:   2,
			wantI:   3,
			wantErr: false,
		},
		{
			name:    "Invalid pod name - too few segments",
			podName: "valkey-0",
			wantS:   -1,
			wantI:   -1,
			wantErr: true,
		},
		{
			name:    "Invalid pod name - non-numeric shard",
			podName: "valkey-abc-0",
			wantS:   -1,
			wantI:   -1,
			wantErr: true,
		},
		{
			name:    "Invalid pod name - non-numeric index",
			podName: "valkey-0-abc",
			wantS:   -1,
			wantI:   -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotS, gotI, err := ParsePodShardAndIndex(tt.podName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePodShardAndIndex() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotS != tt.wantS {
				t.Errorf("ParsePodShardAndIndex() gotS = %v, want %v", gotS, tt.wantS)
			}
			if gotI != tt.wantI {
				t.Errorf("ParsePodShardAndIndex() gotI = %v, want %v", gotI, tt.wantI)
			}
		})
	}
}

func TestMergeAnnotations(t *testing.T) {
	now := time.Now()
	older := now.Add(-1 * time.Hour)
	newer := now.Add(1 * time.Hour)

	tests := []struct {
		name string
		t    map[string]string
		s    map[string]string
		want map[string]string
	}{
		{
			name: "nil target",
			t:    nil,
			s:    map[string]string{"key": "value"},
			want: map[string]string{"key": "value"},
		},
		{
			name: "nil source",
			t:    map[string]string{"key": "value"},
			s:    nil,
			want: map[string]string{"key": "value"},
		},
		{
			name: "merge regular keys",
			t:    map[string]string{"key1": "value1"},
			s:    map[string]string{"key2": "value2"},
			want: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name: "source overwrites target",
			t:    map[string]string{"key": "old"},
			s:    map[string]string{"key": "new"},
			want: map[string]string{"key": "new"},
		},
		{
			name: "restart annotation - target empty, source has value",
			t:    map[string]string{RestartAnnotationKey: ""},
			s:    map[string]string{RestartAnnotationKey: now.Format(time.RFC3339Nano)},
			want: map[string]string{RestartAnnotationKey: now.Format(time.RFC3339Nano)},
		},
		{
			name: "restart annotation - source newer than target",
			t:    map[string]string{RestartAnnotationKey: older.Format(time.RFC3339Nano)},
			s:    map[string]string{RestartAnnotationKey: newer.Format(time.RFC3339Nano)},
			want: map[string]string{RestartAnnotationKey: newer.Format(time.RFC3339Nano)},
		},
		{
			name: "restart annotation - target newer than source",
			t:    map[string]string{RestartAnnotationKey: newer.Format(time.RFC3339Nano)},
			s:    map[string]string{RestartAnnotationKey: older.Format(time.RFC3339Nano)},
			want: map[string]string{RestartAnnotationKey: newer.Format(time.RFC3339Nano)},
		},
		{
			name: "restart annotation - invalid time format",
			t:    map[string]string{RestartAnnotationKey: "invalid-time"},
			s:    map[string]string{RestartAnnotationKey: now.Format(time.RFC3339Nano)},
			want: map[string]string{RestartAnnotationKey: now.Format(time.RFC3339Nano)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeAnnotations(tt.t, tt.s)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MergeAnnotations() = %v, want %v", got, tt.want)
			}
		})
	}
}
