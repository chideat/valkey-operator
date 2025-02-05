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
	"testing"
)

func TestGetValkeyVersion(t *testing.T) {
	testCases := []struct {
		input          string
		expectedOutput string
	}{
		{"valkey:3.0-alpine", "3.0"},
		{"valkey:4.0.14", "4.0.14"},
		{"valkey", ""},
		{"", ""},
	}

	for _, tc := range testCases {
		output := GetValkeyVersion(tc.input)
		if output != tc.expectedOutput {
			t.Errorf("Unexpected output for input '%s'. Expected '%s', but got '%s'", tc.input, tc.expectedOutput, output)
		}
	}
}
