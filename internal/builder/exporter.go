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

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/chideat/valkey-operator/pkg/types/user"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	UserEnvName     = "REDIS_USER"
	PasswordEnvName = "REDIS_PASSWORD"

	ExporterPortNumber    int32 = 9121
	ExporterTelemetryPath       = "/metrics"
)

func BuildExporterContainer(obj metav1.Object, exporter *core.Exporter, user *user.User, enableTLS bool) *corev1.Container {
	if exporter == nil {
		return nil
	}

	image := exporter.Image
	if image == "" {
		image = config.GetValkeyExporterImage(obj)
	}

	cmd := []string{
		"/redis_exporter",
		"--web.listen-address", fmt.Sprintf(":%d", ExporterPortNumber),
		"--web.telemetry-path", ExporterTelemetryPath,
	}

	resources := exporter.Resources
	if resources == nil {
		resources = &corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
		}
	}
	container := corev1.Container{
		Name:            "exporter",
		Command:         cmd,
		Image:           image,
		ImagePullPolicy: GetPullPolicy(exporter.ImagePullPolicy),
		Ports: []corev1.ContainerPort{
			{
				Name:          "exporter",
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: ExporterPortNumber,
			},
		},
		Resources:       *resources,
		SecurityContext: GetContainerSecurityContext(exporter.SecurityContext),
	}

	container.Env = append(container.Env,
		corev1.EnvVar{Name: UserEnvName, Value: user.Name},
	)
	if secretName := user.Password.GetSecretName(); secretName != "" {
		container.Env = append(container.Env,
			corev1.EnvVar{Name: PasswordEnvName, ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "password",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
				},
			}},
		)
	}

	if enableTLS {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      ValkeyTLSVolumeName,
			MountPath: ValkeyTLSVolumeDefaultMountPath,
		})

		// TODO: oliver006/redis_exporter not upgrade to valkey yet
		container.Env = append(container.Env, []corev1.EnvVar{
			{
				Name:  "REDIS_EXPORTER_TLS_CLIENT_KEY_FILE",
				Value: "/tls/tls.key",
			},
			{
				Name:  "REDIS_EXPORTER_TLS_CLIENT_CERT_FILE",
				Value: "/tls/tls.crt",
			},
			{
				Name:  "REDIS_EXPORTER_TLS_CA_CERT_FILE",
				Value: "/tls/ca.crt",
			},
			{
				Name:  "REDIS_EXPORTER_SKIP_TLS_VERIFICATION",
				Value: "true",
			},
			{
				Name:  "REDIS_ADDR",
				Value: fmt.Sprintf("valkeys://%s:%d", config.LocalInjectName, DefaultValkeyServerPort),
			},
		}...)
	} else {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "REDIS_ADDR",
			Value: fmt.Sprintf("valkey://%s:%d", config.LocalInjectName, DefaultValkeyServerPort),
		})
	}
	return &container
}
