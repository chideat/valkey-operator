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
	"fmt"
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/pkg/types/user"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestValkeyExporterContainer(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *v1alpha1.Cluster
		user     *user.User
		expected corev1.Container
	}{
		{
			name: "Basic test",
			cluster: &v1alpha1.Cluster{
				Spec: v1alpha1.ClusterSpec{
					Exporter: &core.Exporter{
						Image: "valkey-exporter:latest",
						Resources: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("200m"),
								corev1.ResourceMemory: resource.MustParse("200Mi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("200m"),
								corev1.ResourceMemory: resource.MustParse("200Mi"),
							},
						},
					},
				},
			},
			user: &user.User{
				Name: user.DefaultOperatorUserName,
				Role: user.RoleOperator,
				Password: &user.Password{
					SecretName: "secret-name",
				},
			},
			expected: corev1.Container{
				Name: "exporter",
				Command: []string{
					"/redis_exporter",
					"--web.listen-address", fmt.Sprintf(":%d", ExporterPortNumber),
					"--web.telemetry-path", ExporterTelemetryPath},
				Image:           "valkey-exporter:latest",
				ImagePullPolicy: corev1.PullIfNotPresent,
				Ports: []corev1.ContainerPort{
					{
						Name:          "exporter",
						Protocol:      corev1.ProtocolTCP,
						ContainerPort: ExporterPortNumber,
					},
				},
				Env: []corev1.EnvVar{
					{Name: UserEnvName, Value: user.DefaultOperatorUserName},
					{Name: PasswordEnvName, ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: "password",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "secret-name",
							},
						},
					}},
					{Name: "REDIS_ADDR", Value: "valkey://local.inject:6379"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("200Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("200Mi"),
					},
				},
			},
		},
		{
			name: "test with tls enabled",
			cluster: &v1alpha1.Cluster{
				Spec: v1alpha1.ClusterSpec{
					Access: core.InstanceAccess{
						EnableTLS: true,
					},
					Exporter: &core.Exporter{
						Image: "valkey-exporter:latest",
						Resources: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("200m"),
								corev1.ResourceMemory: resource.MustParse("200Mi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("200m"),
								corev1.ResourceMemory: resource.MustParse("200Mi"),
							},
						},
					},
				},
			},
			user: &user.User{
				Name: user.DefaultOperatorUserName,
				Role: user.RoleOperator,
				Password: &user.Password{
					SecretName: "secret-name",
				},
			},
			expected: corev1.Container{
				Name: "exporter",
				Command: []string{
					"/redis_exporter",
					"--web.listen-address",
					fmt.Sprintf(":%d", ExporterPortNumber),
					"--web.telemetry-path", ExporterTelemetryPath},
				Image:           "valkey-exporter:latest",
				ImagePullPolicy: corev1.PullIfNotPresent,
				Ports: []corev1.ContainerPort{
					{
						Name:          "exporter",
						Protocol:      corev1.ProtocolTCP,
						ContainerPort: ExporterPortNumber,
					},
				},
				Env: []corev1.EnvVar{
					{Name: UserEnvName, Value: user.DefaultOperatorUserName},
					{Name: PasswordEnvName, ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: "password",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "secret-name",
							},
						},
					}},
					{Name: "REDIS_EXPORTER_TLS_CLIENT_KEY_FILE", Value: "/tls/tls.key"},
					{Name: "REDIS_EXPORTER_TLS_CLIENT_CERT_FILE", Value: "/tls/tls.crt"},
					{Name: "REDIS_EXPORTER_TLS_CA_CERT_FILE", Value: "/tls/ca.crt"},
					{Name: "REDIS_EXPORTER_SKIP_TLS_VERIFICATION", Value: "true"},
					{Name: "REDIS_ADDR", Value: "valkeys://local.inject:6379"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("200Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("200Mi"),
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: ValkeyTLSVolumeName, MountPath: ValkeyTLSVolumeDefaultMountPath},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := BuildExporterContainer(tt.cluster, tt.cluster.Spec.Exporter, tt.user, tt.cluster.Spec.Access.EnableTLS)
			assert.Equal(t, tt.expected.Name, container.Name)
			assert.Equal(t, tt.expected.Command, container.Command)
			assert.Equal(t, tt.expected.Args, container.Args)
			assert.Equal(t, tt.expected.Image, container.Image)
			assert.Equal(t, tt.expected.ImagePullPolicy, container.ImagePullPolicy)
			assert.Equal(t, tt.expected.Ports, container.Ports)
			assert.ElementsMatch(t, tt.expected.Env, container.Env)
			assert.Equal(t, tt.expected.Resources, container.Resources)
			assert.ElementsMatch(t, tt.expected.VolumeMounts, container.VolumeMounts)
		})
	}
}
