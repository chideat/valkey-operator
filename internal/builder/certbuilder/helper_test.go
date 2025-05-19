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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestGenerateTLSEnvs(t *testing.T) {
	envVars := GenerateTLSEnvs()

	// Check the number of environment variables
	assert.Len(t, envVars, 4, "Should return 4 environment variables")

	// Create a map for easier testing
	envMap := make(map[string]string)
	for _, env := range envVars {
		envMap[env.Name] = env.Value
	}

	// Verify each environment variable
	assert.Equal(t, "true", envMap[TLSEnabledKey], "TLS_ENABLED should be 'true'")
	assert.Equal(t, "/tls/ca.crt", envMap[TLSCaFileKey], "TLS_CA_CERT_FILE should be '/tls/ca.crt'")
	assert.Equal(t, "/tls/tls.key", envMap[TLSKeyFileKey], "TLS_CLIENT_KEY_FILE should be '/tls/tls.key'")
	assert.Equal(t, "/tls/tls.crt", envMap[TLSCertFileKey], "TLS_CLIENT_CERT_FILE should be '/tls/tls.crt'")

	// Verify the actual structure of the environment variables
	expectedEnvs := []corev1.EnvVar{
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

	assert.ElementsMatch(t, expectedEnvs, envVars, "Environment variables should match expected values")
}
