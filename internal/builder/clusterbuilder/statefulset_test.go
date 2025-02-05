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

package clusterbuilder

import (
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	v1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/pkg/types/user"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestVolumeMounts(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *v1alpha1.Cluster
		user     *user.User
		expected []corev1.VolumeMount
	}{
		{
			name: "Basic test",
			cluster: &v1alpha1.Cluster{
				Spec: v1alpha1.ClusterSpec{},
			},
			user: &user.User{
				Password: &user.Password{},
			},
			expected: []corev1.VolumeMount{
				{Name: ConfigVolumeName, MountPath: ConfigVolumeMountPath},
				{Name: StorageVolumeName, MountPath: StorageVolumeMountPath},
				{Name: ValkeyOptVolumeName, MountPath: ValkeyOptVolumeMountPath},
				{Name: ValkeyTempVolumeName, MountPath: ValkeyTempVolumeMountPath},
			},
		},
		{
			name: "With password",
			cluster: &v1alpha1.Cluster{
				Spec: v1alpha1.ClusterSpec{},
			},
			user: &user.User{
				Password: &user.Password{
					SecretName: "secret-name",
				},
			},
			expected: []corev1.VolumeMount{
				{Name: ConfigVolumeName, MountPath: ConfigVolumeMountPath},
				{Name: StorageVolumeName, MountPath: StorageVolumeMountPath},
				{Name: ValkeyOptVolumeName, MountPath: ValkeyOptVolumeMountPath},
				{Name: ValkeyTempVolumeName, MountPath: ValkeyTempVolumeMountPath},
				{Name: ValkeyPasswordVolumeName, MountPath: PasswordVolumeMountPath},
			},
		},
		{
			name: "With TLS",
			cluster: &v1alpha1.Cluster{
				Spec: v1alpha1.ClusterSpec{
					Access: core.InstanceAccess{
						EnableTLS: true,
					},
				},
			},
			user: &user.User{
				Password: &user.Password{},
			},
			expected: []corev1.VolumeMount{
				{Name: ConfigVolumeName, MountPath: ConfigVolumeMountPath},
				{Name: StorageVolumeName, MountPath: StorageVolumeMountPath},
				{Name: ValkeyOptVolumeName, MountPath: ValkeyOptVolumeMountPath},
				{Name: ValkeyTempVolumeName, MountPath: ValkeyTempVolumeMountPath},
				{Name: TLSVolumeName, MountPath: TLSVolumeMountPath},
			},
		},
		{
			name: "With password and TLS",
			cluster: &v1alpha1.Cluster{
				Spec: v1alpha1.ClusterSpec{
					Access: core.InstanceAccess{
						EnableTLS: true,
					},
				},
			},
			user: &user.User{
				Password: &user.Password{
					SecretName: "secret-name",
				},
			},
			expected: []corev1.VolumeMount{
				{Name: ConfigVolumeName, MountPath: ConfigVolumeMountPath},
				{Name: StorageVolumeName, MountPath: StorageVolumeMountPath},
				{Name: ValkeyOptVolumeName, MountPath: ValkeyOptVolumeMountPath},
				{Name: ValkeyTempVolumeName, MountPath: ValkeyTempVolumeMountPath},
				{Name: ValkeyPasswordVolumeName, MountPath: PasswordVolumeMountPath},
				{Name: TLSVolumeName, MountPath: TLSVolumeMountPath},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			volumeMounts := buildVolumeMounts(tt.cluster, tt.user)
			assert.ElementsMatch(t, tt.expected, volumeMounts)
		})
	}
}
