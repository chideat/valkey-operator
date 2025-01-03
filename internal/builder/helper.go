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

	corev1 "k8s.io/api/core/v1"
)

// GenerateCertName
func GenerateCertName(name string) string {
	return name + "-cert"
}

func GetRedisSSLSecretName(name string) string {
	return fmt.Sprintf("%s-tls", name)
}

// GetServiceDNSName
func GetServiceDNSName(serviceName, namespace string) string {
	return fmt.Sprintf("%s.%s.svc", serviceName, namespace)
}

const (
	RedisStorageVolumeName = "redis-data"
)

func EmptyVolume() *corev1.Volume {
	return &corev1.Volume{
		Name: RedisStorageVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}
