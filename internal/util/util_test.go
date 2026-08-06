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

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUnifyValueUnit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no unit",
			input:    "100",
			expected: "100",
		},
		{
			name:     "byte unit",
			input:    "100b",
			expected: "100",
		},
		{
			name:     "kilobyte unit k",
			input:    "100k",
			expected: "100000",
		},
		{
			name:     "kilobyte unit kb",
			input:    "100kb",
			expected: "102400",
		},
		{
			name:     "megabyte unit m",
			input:    "10m",
			expected: "10000000",
		},
		{
			name:     "megabyte unit mb",
			input:    "10mb",
			expected: "10485760",
		},
		{
			name:     "gigabyte unit g",
			input:    "1g",
			expected: "1000000000",
		},
		{
			name:     "gigabyte unit gb",
			input:    "1gb",
			expected: "1073741824",
		},
		{
			name:     "multiple values with units",
			input:    "1g 512mb",
			expected: "1000000000 536870912",
		},
		{
			name:     "mixed case",
			input:    "10KB 5Mb",
			expected: "10240 5242880",
		},
		{
			name:     "mixed with non-memory values",
			input:    "maxmemory 1gb",
			expected: "maxmemory 1073741824",
		},
		{
			name:     "invalid memory unit",
			input:    "abc 123",
			expected: "abc 123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UnifyValueUnit(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertMemoryUnit(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:     "bytes",
			input:    "100b",
			expected: "100",
		},
		{
			name:     "kilobytes k",
			input:    "10k",
			expected: "10000",
		},
		{
			name:     "kilobytes kb",
			input:    "10kb",
			expected: "10240",
		},
		{
			name:     "megabytes m",
			input:    "5m",
			expected: "5000000",
		},
		{
			name:     "megabytes mb",
			input:    "5mb",
			expected: "5242880",
		},
		{
			name:     "gigabytes g",
			input:    "1g",
			expected: "1000000000",
		},
		{
			name:     "gigabytes gb",
			input:    "1gb",
			expected: "1073741824",
		},
		{
			name:        "invalid number",
			input:       "abc",
			expectError: true,
		},
		{
			name:        "invalid format",
			input:       "10.5k",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertMemoryUnit(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestMapSigGenerator(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]string
		salt  string
	}{
		{
			name:  "empty map",
			input: map[string]string{},
			salt:  "salt",
		},
		{
			name: "single key-value",
			input: map[string]string{
				"key": "value",
			},
			salt: "salt",
		},
		{
			name: "multiple key-values",
			input: map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
			salt: "salt",
		},
		{
			name: "same values different salt",
			input: map[string]string{
				"key": "value",
			},
			salt: "different-salt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mapSigGenerator(tt.input, tt.salt)
			assert.NoError(t, err)
			assert.NotEmpty(t, result)

			// Generate again to ensure deterministic result
			result2, err := mapSigGenerator(tt.input, tt.salt)
			assert.NoError(t, err)
			assert.Equal(t, result, result2)

			// Test with different salt should produce different result
			if tt.salt != "different-salt" {
				resultDiffSalt, err := mapSigGenerator(tt.input, "different-salt")
				assert.NoError(t, err)
				assert.NotEqual(t, result, resultDiffSalt)
			}
		})
	}
}

func TestGenerateObjectSig(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		salt        string
		expectEmpty bool
	}{
		{
			name:        "nil input",
			input:       nil,
			salt:        "salt",
			expectEmpty: true,
		},
		{
			name:  "string input",
			input: "test-string",
			salt:  "salt",
		},
		{
			name:  "byte slice input",
			input: []byte("test-bytes"),
			salt:  "salt",
		},
		{
			name: "ConfigMap input",
			input: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-configmap",
					Namespace: "default",
				},
				Data: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			salt: "salt",
		},
		{
			name: "Secret input",
			input: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"key1": []byte("value1"),
					"key2": []byte("value2"),
				},
			},
			salt: "salt",
		},
		{
			name:  "unsupported type",
			input: 123,
			salt:  "salt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GenerateObjectSig(tt.input, tt.salt)

			if tt.input == 123 {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				if tt.expectEmpty {
					assert.Empty(t, result)
				} else {
					assert.NotEmpty(t, result)

					// Test deterministic result
					result2, err := GenerateObjectSig(tt.input, tt.salt)
					assert.NoError(t, err)
					assert.Equal(t, result, result2)
				}
			}
		})
	}
}
