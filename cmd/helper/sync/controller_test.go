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

package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExceedsConfigMapSizeLimit(t *testing.T) {
	// the limit must be ~1Mi, NOT ~1Gi (the historical bug used 1024*1024*1024-4096)
	assert.Equal(t, 1024*1024-4096, maxConfigMapDataSize)

	tests := []struct {
		name     string
		dataSize int
		expected bool
	}{
		{name: "small payload is under the limit", dataSize: 1024, expected: false},
		{name: "just below the 1Mi limit", dataSize: maxConfigMapDataSize - 1, expected: false},
		{name: "exactly at the limit triggers the guard", dataSize: maxConfigMapDataSize, expected: true},
		{name: "above the limit triggers the guard", dataSize: maxConfigMapDataSize + 1, expected: true},
		{name: "a 2Mi payload that used to slip past the broken 1Gi guard now triggers", dataSize: 2 * 1024 * 1024, expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, exceedsConfigMapSizeLimit(tt.dataSize))
		})
	}
}
