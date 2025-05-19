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

package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetValkeyVersion(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "empty image",
			image:    "",
			expected: "",
		},
		{
			name:     "image without tag",
			image:    "valkey/valkey",
			expected: "",
		},
		{
			name:     "image with simple tag",
			image:    "valkey/valkey:1.0.0",
			expected: "1.0.0",
		},
		{
			name:     "image with complex tag",
			image:    "valkey/valkey:1.0.0-alpine",
			expected: "1.0.0",
		},
		{
			name:     "image with registry",
			image:    "registry.example.com/valkey/valkey:2.0.0",
			expected: "2.0.0",
		},
		{
			name:     "image with tag and suffix",
			image:    "valkey/valkey:3.0.0-beta1",
			expected: "3.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetValkeyVersion(tt.image)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetenv(t *testing.T) {
	// Save the original environment variable values
	originalEnv := make(map[string]string)
	varsToTest := []string{"TEST_ENV_VAR", "TEST_EMPTY_VAR", "TEST_NONEXISTENT_VAR"}
	for _, v := range varsToTest {
		originalEnv[v] = os.Getenv(v)
	}

	// Restore environment variables after the test
	defer func() {
		for k, v := range originalEnv {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// Set test environment variables
	os.Setenv("TEST_ENV_VAR", "test-value")
	os.Setenv("TEST_EMPTY_VAR", "")
	os.Unsetenv("TEST_NONEXISTENT_VAR")

	tests := []struct {
		name     string
		envVar   string
		defaults []string
		expected string
	}{
		{
			name:     "environment variable exists",
			envVar:   "TEST_ENV_VAR",
			defaults: []string{"default-value"},
			expected: "test-value",
		},
		{
			name:     "environment variable exists but empty, with default",
			envVar:   "TEST_EMPTY_VAR",
			defaults: []string{"default-value"},
			expected: "default-value",
		},
		{
			name:     "environment variable doesn't exist, with default",
			envVar:   "TEST_NONEXISTENT_VAR",
			defaults: []string{"default-value"},
			expected: "default-value",
		},
		{
			name:     "environment variable doesn't exist, with multiple defaults",
			envVar:   "TEST_NONEXISTENT_VAR",
			defaults: []string{"default1", "default2"},
			expected: "default1",
		},
		{
			name:     "environment variable doesn't exist, with empty default",
			envVar:   "TEST_NONEXISTENT_VAR",
			defaults: []string{""},
			expected: "",
		},
		{
			name:     "environment variable doesn't exist, with no defaults",
			envVar:   "TEST_NONEXISTENT_VAR",
			defaults: []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Getenv(tt.envVar, tt.defaults...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetFullImageURL(t *testing.T) {
	// Save the original environment variable value
	originalRegistry := os.Getenv("DEFAULT_REGISTRY")

	// Restore environment variable after the test
	defer func() {
		if originalRegistry == "" {
			os.Unsetenv("DEFAULT_REGISTRY")
		} else {
			os.Setenv("DEFAULT_REGISTRY", originalRegistry)
		}
	}()

	tests := []struct {
		name           string
		registry       string
		path           string
		tag            string
		expectedResult string
	}{
		{
			name:           "with registry",
			registry:       "registry.example.com",
			path:           "valkey/valkey",
			tag:            "1.0.0",
			expectedResult: "registry.example.com/valkey/valkey:1.0.0",
		},
		{
			name:           "without registry",
			registry:       "",
			path:           "valkey/valkey",
			tag:            "2.0.0",
			expectedResult: "valkey/valkey:2.0.0",
		},
		{
			name:           "with different path",
			registry:       "registry.example.com",
			path:           "custom/path",
			tag:            "latest",
			expectedResult: "registry.example.com/custom/path:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("DEFAULT_REGISTRY", tt.registry)
			result := GetFullImageURL(tt.path, tt.tag)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestGetValkeyImageByVersion(t *testing.T) {
	// Save original environment variables
	originalImageName := os.Getenv("VALKEY_IMAGE_NAME")
	originalRegistry := os.Getenv("DEFAULT_REGISTRY")

	// Restore environment variables after test
	defer func() {
		if originalImageName == "" {
			os.Unsetenv("VALKEY_IMAGE_NAME")
		} else {
			os.Setenv("VALKEY_IMAGE_NAME", originalImageName)
		}

		if originalRegistry == "" {
			os.Unsetenv("DEFAULT_REGISTRY")
		} else {
			os.Setenv("DEFAULT_REGISTRY", originalRegistry)
		}
	}()

	tests := []struct {
		name           string
		imageName      string
		registry       string
		version        string
		expectedResult string
	}{
		{
			name:           "default image name",
			imageName:      "",
			registry:       "",
			version:        "1.0.0",
			expectedResult: "valkey/valkey:1.0.0",
		},
		{
			name:           "custom image name",
			imageName:      "custom/valkey",
			registry:       "",
			version:        "2.0.0",
			expectedResult: "custom/valkey:2.0.0",
		},
		{
			name:           "with registry",
			imageName:      "",
			registry:       "registry.example.com",
			version:        "3.0.0",
			expectedResult: "registry.example.com/valkey/valkey:3.0.0",
		},
		{
			name:           "custom image name and registry",
			imageName:      "custom/valkey",
			registry:       "registry.example.com",
			version:        "latest",
			expectedResult: "registry.example.com/custom/valkey:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("VALKEY_IMAGE_NAME", tt.imageName)
			os.Setenv("DEFAULT_REGISTRY", tt.registry)

			result := GetValkeyImageByVersion(tt.version)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestBuildImageVersionKey(t *testing.T) {
	tests := []struct {
		name     string
		typ      string
		expected string
	}{
		{
			name:     "valkey-helper type",
			typ:      "valkey-helper",
			expected: "buf.red/imageversions-valkey-helper",
		},
		{
			name:     "exporter type",
			typ:      "exporter",
			expected: "buf.red/imageversions-exporter",
		},
		{
			name:     "empty type",
			typ:      "",
			expected: "buf.red/imageversions-",
		},
		{
			name:     "custom type",
			typ:      "custom-test",
			expected: "buf.red/imageversions-custom-test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildImageVersionKey(tt.typ)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetOperatorVersion(t *testing.T) {
	// Save original environment variable
	originalVersion := os.Getenv("OPERATOR_VERSION")

	// Restore environment variable after test
	defer func() {
		if originalVersion == "" {
			os.Unsetenv("OPERATOR_VERSION")
		} else {
			os.Setenv("OPERATOR_VERSION", originalVersion)
		}
	}()

	tests := []struct {
		name     string
		version  string
		expected string
	}{
		{
			name:     "with set version",
			version:  "1.0.0",
			expected: "1.0.0",
		},
		{
			name:     "with empty version",
			version:  "",
			expected: "latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("OPERATOR_VERSION", tt.version)
			result := GetOperatorVersion()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetValkeyHelperImage(t *testing.T) {
	// Save original environment variables
	originalImageName := os.Getenv("OPERATOR_IMAGE_NAME")
	originalVersion := os.Getenv("OPERATOR_VERSION")
	originalRegistry := os.Getenv("DEFAULT_REGISTRY")

	// Restore environment variables after test
	defer func() {
		if originalImageName == "" {
			os.Unsetenv("OPERATOR_IMAGE_NAME")
		} else {
			os.Setenv("OPERATOR_IMAGE_NAME", originalImageName)
		}

		if originalVersion == "" {
			os.Unsetenv("OPERATOR_VERSION")
		} else {
			os.Setenv("OPERATOR_VERSION", originalVersion)
		}

		if originalRegistry == "" {
			os.Unsetenv("DEFAULT_REGISTRY")
		} else {
			os.Setenv("DEFAULT_REGISTRY", originalRegistry)
		}
	}()

	// Setup environment for tests
	os.Setenv("OPERATOR_IMAGE_NAME", "chideat/valkey-operator")
	os.Setenv("OPERATOR_VERSION", "latest")
	os.Unsetenv("DEFAULT_REGISTRY")

	tests := []struct {
		name           string
		registry       string
		annotations    map[string]string
		expectedResult string
	}{
		{
			name:           "default image without annotation",
			registry:       "",
			annotations:    nil,
			expectedResult: "chideat/valkey-operator:latest",
		},
		{
			name:     "image from annotation",
			registry: "",
			annotations: map[string]string{
				BuildImageVersionKey("valkey-helper"): "custom/helper:1.0.0",
			},
			expectedResult: "custom/helper:1.0.0",
		},
		{
			name:           "with custom registry",
			registry:       "registry.example.com",
			annotations:    nil,
			expectedResult: "registry.example.com/chideat/valkey-operator:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("DEFAULT_REGISTRY", tt.registry)

			// Create a mock object with annotations if needed
			var obj metav1.Object
			if tt.annotations != nil {
				obj = &metav1.ObjectMeta{
					Annotations: tt.annotations,
				}
			}

			result := GetValkeyHelperImage(obj)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestGetValkeyExporterImage(t *testing.T) {
	// Save original environment variables
	originalImageName := os.Getenv("DEFAULT_EXPORTER_IMAGE_NAME")
	originalVersion := os.Getenv("DEFAULT_EXPORTER_VERSION")
	originalRegistry := os.Getenv("DEFAULT_REGISTRY")

	// Restore environment variables after test
	defer func() {
		if originalImageName == "" {
			os.Unsetenv("DEFAULT_EXPORTER_IMAGE_NAME")
		} else {
			os.Setenv("DEFAULT_EXPORTER_IMAGE_NAME", originalImageName)
		}

		if originalVersion == "" {
			os.Unsetenv("DEFAULT_EXPORTER_VERSION")
		} else {
			os.Setenv("DEFAULT_EXPORTER_VERSION", originalVersion)
		}

		if originalRegistry == "" {
			os.Unsetenv("DEFAULT_REGISTRY")
		} else {
			os.Setenv("DEFAULT_REGISTRY", originalRegistry)
		}
	}()

	// Setup environment for tests
	os.Unsetenv("DEFAULT_REGISTRY")

	tests := []struct {
		name           string
		imageName      string
		version        string
		registry       string
		annotations    map[string]string
		expectedResult string
	}{
		{
			name:           "default settings",
			imageName:      "",
			version:        "",
			registry:       "",
			annotations:    nil,
			expectedResult: "oliver006/redis_exporter:v1.67.0-alpine",
		},
		{
			name:      "image from annotation",
			imageName: "",
			version:   "",
			registry:  "",
			annotations: map[string]string{
				BuildImageVersionKey("exporter"): "custom/exporter:1.0.0",
			},
			expectedResult: "custom/exporter:1.0.0",
		},
		{
			name:           "custom image name and version",
			imageName:      "custom/exporter",
			version:        "2.0.0",
			registry:       "",
			annotations:    nil,
			expectedResult: "custom/exporter:2.0.0",
		},
		{
			name:           "with custom registry",
			imageName:      "",
			version:        "",
			registry:       "registry.example.com",
			annotations:    nil,
			expectedResult: "registry.example.com/oliver006/redis_exporter:v1.67.0-alpine",
		},
		{
			name:           "custom all settings",
			imageName:      "custom/exporter",
			version:        "3.0.0",
			registry:       "registry.example.com",
			annotations:    nil,
			expectedResult: "registry.example.com/custom/exporter:3.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("DEFAULT_EXPORTER_IMAGE_NAME", tt.imageName)
			os.Setenv("DEFAULT_EXPORTER_VERSION", tt.version)
			os.Setenv("DEFAULT_REGISTRY", tt.registry)

			// Create a mock object with annotations if needed
			var obj metav1.Object
			if tt.annotations != nil {
				obj = &metav1.ObjectMeta{
					Annotations: tt.annotations,
				}
			}

			result := GetValkeyExporterImage(obj)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
