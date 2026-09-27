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

package helper

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"gotest.tools/v3/assert"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
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
			portSequence:  "9-10,4-6,3",
			expectedPorts: []int32{3, 4, 5, 6, 9, 10},
			expectedError: nil,
		},
		{
			name:          "Duplicate port",
			portSequence:  "9-10,4-6,5",
			expectedPorts: nil,
			expectedError: fmt.Errorf("duplicate port 5 found"),
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

func TestParsePortsRejectsOutOfRangePorts(t *testing.T) {
	// A port above 65535 used to survive the range branch: the loop counter was
	// narrowed with int32(i) and no ceiling was applied, so "1-70000" silently
	// produced ports no listener could ever bind.
	for _, input := range []string{
		"1-70000",      // range end above uint16
		"70000-70010",  // range start above uint16
		"70000",        // single port above uint16
		"1-2147483647", // range end at the int32 boundary
	} {
		t.Run(input, func(t *testing.T) {
			ports, err := ParsePorts(input)
			assert.Assert(t, err != nil, "%q must be rejected, got ports %v", input, ports)
			assert.Assert(t, ports == nil)
		})
	}
}

func TestParsePortsAcceptsBoundaryPorts(t *testing.T) {
	for _, input := range []string{"65535", "65530-65535", "1", "6379,16379"} {
		t.Run(input, func(t *testing.T) {
			ports, err := ParsePorts(input)
			assert.NilError(t, err)
			assert.Assert(t, len(ports) > 0)
			for _, p := range ports {
				assert.Assert(t, p > 0 && p <= 65535, "port %d out of range", p)
			}
		})
	}
}

// TestPortsPatternMatchesParsePorts guards the contract between the CRDs and
// ParsePorts: the pattern of every access.ports field must accept the formats
// ParsePorts reads, and reject what it cannot read.
//
// The pattern used to require "a:b" pairs, which ParsePorts rejects, while it
// rejected the ports and ranges ParsePorts reads. No value passed both, so
// access.ports could not be set. The patterns are read out of the generated
// CRDs and the Helm chart's copy of them, so they cannot drift from the parser.
func TestPortsPatternMatchesParsePorts(t *testing.T) {
	cases := []struct {
		ports string
		want  bool
	}{
		{"30000", true},
		{"30000,30001", true},
		{"30000-30001", true},
		{"30000,30002-30004", true},
		{"3,4-6,7,9-10", true},
		{"30000:30000", false},
		{"30000:30000,30001:30001", false},
		{"30000,", false},
		{",30000", false},
		{"30000-", false},
		{"30000--30001", false},
		{"30000-30001-30002", false},
		{"30000, 30001", false},
	}
	for _, tc := range cases {
		_, err := ParsePorts(tc.ports)
		assert.Equal(t, err == nil, tc.want, "ParsePorts(%q) returned %v", tc.ports, err)
	}

	patterns := map[string]string{}
	for _, dir := range []string{"../../../config/crd/bases", "../../../charts/valkey-operator/crds"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
		assert.NilError(t, err)
		for _, file := range files {
			data, err := os.ReadFile(file)
			assert.NilError(t, err)
			var crd apiextensionsv1.CustomResourceDefinition
			assert.NilError(t, yaml.Unmarshal(data, &crd), file)
			for _, version := range crd.Spec.Versions {
				collectPortsPatterns(version.Schema.OpenAPIV3Schema, file+" "+version.Name, patterns)
			}
		}
	}
	// spec.access and spec.sentinel.access of the Valkey and Failover CRDs, and
	// spec.access of the Cluster and Sentinel CRDs, in both copies.
	assert.Equal(t, len(patterns), 12, "access.ports fields found: %v", patterns)

	for field, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		assert.NilError(t, err, field)
		for _, tc := range cases {
			assert.Equal(t, re.MatchString(tc.ports), tc.want, "%s: pattern %q on %q", field, pattern, tc.ports)
		}
	}
}

// collectPortsPatterns records the pattern of every string property named
// ports under schema, keyed by where it was found.
func collectPortsPatterns(schema *apiextensionsv1.JSONSchemaProps, path string, found map[string]string) {
	if schema == nil {
		return
	}
	for name, prop := range schema.Properties {
		if name == "ports" && prop.Type == "string" {
			found[path+".ports"] = prop.Pattern
		}
		collectPortsPatterns(&prop, path+"."+name, found)
	}
	if schema.Items != nil {
		collectPortsPatterns(schema.Items.Schema, path+"[]", found)
	}
}
