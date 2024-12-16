/*
Copyright 2023 The RedisOperator Authors.

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

package helper

import (
	"fmt"
	"testing"

	"gotest.tools/v3/assert"
	v1 "k8s.io/api/core/v1"
)

func TestParseSequencePorts(t *testing.T) {
	testCases := []struct {
		name          string
		portSequence  string
		expectedPorts []int32
		expectedError error
	}{
		{
			name:          "Basic range",
			portSequence:  "1-3",
			expectedPorts: []int32{1, 2, 3},
			expectedError: nil,
		},
		{
			name:          "Single value",
			portSequence:  "5",
			expectedPorts: []int32{5},
			expectedError: nil,
		},
		{
			name:          "Mixed ranges and single values",
			portSequence:  "3,4-6,7,9-10",
			expectedPorts: []int32{3, 4, 5, 6, 7, 9, 10},
			expectedError: nil,
		},
		{
			name:          "Invalid format",
			portSequence:  "4-6,7-",
			expectedPorts: nil,
			expectedError: fmt.Errorf("strconv.Atoi: parsing \"\": invalid syntax"),
		},
		{
			name:          "Unsorted and overlapping",
			portSequence:  "9-10,7,4-6,3,5-8",
			expectedPorts: []int32{3, 4, 5, 6, 7, 8, 9, 10},
			expectedError: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ports, err := ParsePorts(tc.portSequence)

			if tc.expectedError != nil && err == nil {
				t.Errorf("Expected error, got nil")
			}

			if tc.expectedError == nil && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tc.expectedError != nil && err != nil && tc.expectedError.Error() != err.Error() {
				t.Errorf("Expected error '%v', got '%v'", tc.expectedError, err)
			}

			if len(tc.expectedPorts) != len(ports) {
				t.Errorf("Expected ports length %v, got %v", len(tc.expectedPorts), len(ports))
			}

			for i, port := range tc.expectedPorts {
				if port != ports[i] {
					t.Errorf("Expected port %v at position %v, got %v", port, i, ports[i])
				}
			}
		})
	}
}

func TestGetDefaultIPFamily(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected v1.IPFamily
	}{
		{
			name:     "Empty IP",
			ip:       "",
			expected: "",
		},
		{
			name:     "Valid IPv6",
			ip:       "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			expected: v1.IPv6Protocol,
		},
		{
			name:     "Valid IPv6",
			ip:       "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			expected: v1.IPv6Protocol,
		},
		{
			name:     "Invalid IP",
			ip:       "invalid-ip",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetDefaultIPFamily(tt.ip)
			assert.Equal(t, tt.expected, result)
		})
	}
}
