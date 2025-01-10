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
package failoverbuilder

import (
	"os"
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/pkg/types/user"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestCreateValkeyExporterContainer(t *testing.T) {
	tests := []struct {
		name     string
		rf       *v1alpha1.Failover
		opUser   *user.User
		expected corev1.Container
	}{
		{
			name: "Basic test",
			rf: &v1alpha1.Failover{
				Spec: v1alpha1.FailoverSpec{
					Exporter: v1alpha1.ValkeyExporter{
						Exporter: core.Exporter{
							Enabled: true,
							Image:   "redis-exporter:latest",
						},
					},
				},
			},
			opUser: &user.User{
				Name: "default",
				Password: &user.Password{
					SecretName: "secret-name",
				},
			},
			expected: corev1.Container{
				Name:            exporterContainerName,
				Command:         []string{"/redis_exporter"},
				Image:           "redis-exporter:latest",
				ImagePullPolicy: corev1.PullIfNotPresent,
				Env: []corev1.EnvVar{
					{Name: "VALKEY_ALIAS", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
					{Name: "VALKEY_USER", Value: ""},
					{Name: "VALKEY_PASSWORD", ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key:                  "password",
							LocalObjectReference: corev1.LocalObjectReference{Name: "secret-name"},
						},
					}},
					{Name: "VALKEY_ADDR", Value: "valkey://local.inject:6379"},
				},
				Ports: []corev1.ContainerPort{
					{Name: "metrics", ContainerPort: 9121, Protocol: corev1.ProtocolTCP},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("200Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
				SecurityContext: builder.GetSecurityContext(nil),
			},
		},
		{
			name: "With TLS",
			rf: &v1alpha1.Failover{
				Spec: v1alpha1.FailoverSpec{
					Exporter: v1alpha1.ValkeyExporter{
						Exporter: core.Exporter{
							Enabled: true,
							Image:   "redis-exporter:latest",
						},
					},
				},
			},
			opUser: &user.User{
				Name: "default",
				Password: &user.Password{
					SecretName: "secret-name",
				},
			},
			expected: corev1.Container{
				Name:            exporterContainerName,
				Command:         []string{"/redis_exporter"},
				Image:           "redis-exporter:latest",
				ImagePullPolicy: corev1.PullIfNotPresent,
				Env: []corev1.EnvVar{
					{Name: "VALKEY_ALIAS", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
					{Name: "VALKEY_USER", Value: ""},
					{Name: "VALKEY_PASSWORD", ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key:                  "password",
							LocalObjectReference: corev1.LocalObjectReference{Name: "secret-name"},
						},
					}},
					{Name: "VALKEY_EXPORTER_TLS_CLIENT_KEY_FILE", Value: "/tls/tls.key"},
					{Name: "VALKEY_EXPORTER_TLS_CLIENT_CERT_FILE", Value: "/tls/tls.crt"},
					{Name: "VALKEY_EXPORTER_TLS_CA_CERT_FILE", Value: "/tls/ca.crt"},
					{Name: "VALKEY_EXPORTER_SKIP_TLS_VERIFICATION", Value: "true"},
					{Name: "VALKEY_ADDR", Value: "valkeys://local.inject:6379"},
				},
				Ports: []corev1.ContainerPort{
					{Name: "metrics", ContainerPort: 9121, Protocol: corev1.ProtocolTCP},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("200Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: ValkeyTLSVolumeName, MountPath: "/tls"},
				},
				SecurityContext: builder.GetSecurityContext(nil),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := createValkeyExporterContainer(tt.rf, tt.opUser)
			assert.Equal(t, tt.expected.Name, container.Name)
			assert.Equal(t, tt.expected.Image, container.Image)
			assert.Equal(t, tt.expected.ImagePullPolicy, container.ImagePullPolicy)
			assert.Equal(t, tt.expected.Ports, container.Ports)
			assert.ElementsMatch(t, tt.expected.Env, container.Env)
			assert.Equal(t, tt.expected.Resources, container.Resources)
			assert.Equal(t, tt.expected.SecurityContext, container.SecurityContext)
			assert.ElementsMatch(t, tt.expected.VolumeMounts, container.VolumeMounts)
		})
	}
}

func TestCreateStandaloneInitContainer(t *testing.T) {
	tests := []struct {
		name     string
		rf       *v1alpha1.Failover
		expected corev1.Container
	}{
		{
			name: "Default settings",
			rf: &v1alpha1.Failover{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						AnnotationStandaloneLoadFilePath: "dump.rdb",
					},
				},
			},
			expected: corev1.Container{
				Name:            "standalone-pod",
				Image:           "valkey-tools:latest",
				ImagePullPolicy: corev1.PullIfNotPresent,
				VolumeMounts: []corev1.VolumeMount{
					{Name: "valkey-data", MountPath: "/data"},
					{Name: "valkey-standalone", MountPath: "/tmp-data"},
				},
				Command: []string{"sh", "-c", "if [ -e '/data/dump.rdb' ]; then\necho 'valkey storage file exist,skip' \nelse \necho 'copy valkey storage file' && cp /tmp-data/dump.rdb /data/dump.rdb && chown 999:1000 /data/dump.rdb && chmod 644 /data/dump.rdb \nfi"},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
				SecurityContext: &corev1.SecurityContext{
					Privileged: ptr.To(false),
				},
			},
		},
		{
			name: "Hostpath storage",
			rf: &v1alpha1.Failover{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						AnnotationStandaloneLoadFilePath: "appendonly.aof",
						AnnotationStandaloneInitStorage:  "hostpath",
					},
				},
			},
			expected: corev1.Container{
				Name:            "standalone-pod",
				Image:           "valkey-tools:latest",
				ImagePullPolicy: corev1.PullIfNotPresent,
				VolumeMounts: []corev1.VolumeMount{
					{Name: "valkey-data", MountPath: "/data"},
					{Name: "valkey-standalone", MountPath: "/tmp-data"},
				},
				Command: []string{"sh", "-c", "if [ -e '/data/appendonly.aof' ]; then\necho 'valkey storage file exist,skip' \nelse \necho 'copy valkey storage file' && cp /tmp-data/appendonly.aof /data/appendonly.aof && chown 999:1000 /data/appendonly.aof && chmod 644 /data/appendonly.aof \nfi"},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
				SecurityContext: &corev1.SecurityContext{
					Privileged:   ptr.To(false),
					RunAsGroup:   ptr.To(int64(0)),
					RunAsUser:    ptr.To(int64(0)),
					RunAsNonRoot: ptr.To(false),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("VALKEY_TOOLS_IMAGE", "valkey-tools:latest")

			container := createStandaloneInitContainer(tt.rf)
			assert.Equal(t, tt.expected.Name, container.Name)
			assert.Equal(t, tt.expected.Image, container.Image)
			assert.Equal(t, tt.expected.ImagePullPolicy, container.ImagePullPolicy)
			assert.ElementsMatch(t, tt.expected.VolumeMounts, container.VolumeMounts)
			assert.Equal(t, tt.expected.Command, container.Command)
			assert.Equal(t, tt.expected.Resources, container.Resources)
			assert.Equal(t, tt.expected.SecurityContext, container.SecurityContext)
		})
	}
}
