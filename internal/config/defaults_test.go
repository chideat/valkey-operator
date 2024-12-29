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

package config

import (
	"os"
	"testing"
)

func TestGetRedisVersion(t *testing.T) {
	testCases := []struct {
		input          string
		expectedOutput string
	}{
		{"redis:3.0-alpine", "3.0"},
		{"redis:4.0.14", "4.0.14"},
		{"redis", ""},
		{"", ""},
	}

	for _, tc := range testCases {
		output := GetRedisVersion(tc.input)
		if output != tc.expectedOutput {
			t.Errorf("Unexpected output for input '%s'. Expected '%s', but got '%s'", tc.input, tc.expectedOutput, output)
		}
	}
}

func TestGetDefaultRedisImage(t *testing.T) {
	os.Setenv("DEFAULT_REDIS_IMAGE", "redis:3.2-alpine")
	expectedOutput := "redis:3.2-alpine"
	output := GetDefaultRedisImage()
	if output != expectedOutput {
		t.Errorf("Unexpected output for GetDefaultRedisImage(). Expected '%s', but got '%s'", expectedOutput, output)
	}
}
