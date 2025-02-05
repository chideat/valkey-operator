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
package certbuilder

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	TLSEnabledKey  = "TLS_ENABLED"
	TLSCaFileKey   = "TLS_CA_CERT_FILE"
	TLSKeyFileKey  = "TLS_CLIENT_KEY_FILE"
	TLSCertFileKey = "TLS_CLIENT_CERT_FILE"
)

func GenerateTLSEnvs() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  TLSEnabledKey,
			Value: "true",
		},
		{
			Name:  TLSCaFileKey,
			Value: "/tls/ca.crt",
		},
		{
			Name:  TLSKeyFileKey,
			Value: "/tls/tls.key",
		},
		{
			Name:  TLSCertFileKey,
			Value: "/tls/tls.crt",
		},
	}
}
