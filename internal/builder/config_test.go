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
	"reflect"
	"testing"
)

func TestIsForbiddenValkeyConfig(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"requirepass", true},
		{"masterauth", true},
		{"masteruser", true},
		// the 8.0+ spellings — aliases of the above, so equally able to carry a password
		{"primaryauth", true},
		{"primaryuser", true},
		{"tls-key-file-pass", true},
		{"tls-client-key-file-pass", true},
		{"aclfile", true},

		// Valkey matches directive names case-insensitively, so a differently-cased key
		// must not slip past the filter and reach a rendered config file.
		{"RequirePass", true},
		{"MASTERAUTH", true},
		{"PrimaryAuth", true},
		{"  requirepass  ", true},

		// ordinary tunables stay allowed
		{"maxmemory", false},
		{"maxmemory-policy", false},
		{"tcp-backlog", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := IsForbiddenValkeyConfig(tt.key); got != tt.want {
				t.Errorf("IsForbiddenValkeyConfig(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestValkeyConfigValuesString(t *testing.T) {
	tests := []struct {
		name   string
		values *ValkeyConfigValues
		want   string
	}{
		{
			name:   "Nil values",
			values: nil,
			want:   "",
		},
		{
			name:   "Empty values",
			values: &ValkeyConfigValues{},
			want:   "",
		},
		{
			name:   "Single value",
			values: &ValkeyConfigValues{"test"},
			want:   "test",
		},
		{
			name:   "Multiple values",
			values: &ValkeyConfigValues{"b", "c", "a"},
			want:   "a b c", // Should be sorted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.values.String(); got != tt.want {
				t.Errorf("ValkeyConfigValues.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadValkeyConfig(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    ValkeyConfig
		wantErr bool
	}{
		{
			name:    "Empty config",
			data:    "",
			want:    ValkeyConfig{},
			wantErr: false,
		},
		{
			name:    "Config with comments",
			data:    "# This is a comment\n",
			want:    ValkeyConfig{},
			wantErr: false,
		},
		{
			name:    "Config with empty lines",
			data:    "\n\n\n",
			want:    ValkeyConfig{},
			wantErr: false,
		},
		{
			name:    "Config with invalid format",
			data:    "invalid format",
			want:    ValkeyConfig{"invalid": ValkeyConfigValues{"format"}},
			wantErr: false,
		},
		{
			name: "Valid config with single value per key",
			data: "maxmemory 100mb\nsave 900 1",
			want: ValkeyConfig{
				"maxmemory": ValkeyConfigValues{"100mb"},
				"save":      ValkeyConfigValues{"900 1"},
			},
			wantErr: false,
		},
		{
			name: "Valid config with quoted value",
			data: `maxmemory "100mb"`,
			want: ValkeyConfig{
				"maxmemory": ValkeyConfigValues{"100mb"},
			},
			wantErr: false,
		},
		{
			name: "Valid config with multiple values for same key",
			data: "save 900 1\nsave 300 10\nsave 60 10000",
			want: ValkeyConfig{
				"save": ValkeyConfigValues{"900 1", "300 10", "60 10000"},
			},
			wantErr: false,
		},
		{
			name: "Config with forbidden settings",
			data: "bind 0.0.0.0\nport 6379\nmaxmemory 100mb",
			want: ValkeyConfig{
				"maxmemory": ValkeyConfigValues{"100mb"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadValkeyConfig(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadValkeyConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadValkeyConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

// compareNilOrEqual compares two maps, considering nil and empty map as equal
func compareNilOrEqual(t *testing.T, label string, got, want map[string]ValkeyConfigValues) {
	if (got == nil && len(want) == 0) || (want == nil && len(got) == 0) {
		// Both are effectively empty, so they're equal
		return
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ValkeyConfig.Diff() %s = %v, want %v", label, got, want)
	}
}

func TestValkeyConfigDiff(t *testing.T) {
	tests := []struct {
		name        string
		old         ValkeyConfig
		new         ValkeyConfig
		wantAdded   map[string]ValkeyConfigValues
		wantChanged map[string]ValkeyConfigValues
		wantDeleted map[string]ValkeyConfigValues
	}{
		{
			name:        "Both configs empty",
			old:         ValkeyConfig{},
			new:         ValkeyConfig{},
			wantAdded:   nil,
			wantChanged: nil,
			wantDeleted: nil,
		},
		{
			name: "Old config empty",
			old:  ValkeyConfig{},
			new: ValkeyConfig{
				"maxmemory": ValkeyConfigValues{"100mb"},
			},
			wantAdded: ValkeyConfig{
				"maxmemory": ValkeyConfigValues{"100mb"},
			},
			wantChanged: nil,
			wantDeleted: nil,
		},
		{
			name: "New config empty",
			old: ValkeyConfig{
				"maxmemory": ValkeyConfigValues{"100mb"},
			},
			new:         ValkeyConfig{},
			wantAdded:   nil,
			wantChanged: nil,
			wantDeleted: ValkeyConfig{
				"maxmemory": ValkeyConfigValues{"100mb"},
			},
		},
		{
			name: "Identical configs",
			old: ValkeyConfig{
				"maxmemory": ValkeyConfigValues{"100mb"},
				"save":      ValkeyConfigValues{"900 1"},
			},
			new: ValkeyConfig{
				"maxmemory": ValkeyConfigValues{"100mb"},
				"save":      ValkeyConfigValues{"900 1"},
			},
			wantAdded:   nil,
			wantChanged: nil,
			wantDeleted: nil,
		},
		{
			name: "Added keys",
			old: ValkeyConfig{
				"maxmemory": ValkeyConfigValues{"100mb"},
			},
			new: ValkeyConfig{
				"maxmemory": ValkeyConfigValues{"100mb"},
				"save":      ValkeyConfigValues{"900 1"},
			},
			wantAdded: map[string]ValkeyConfigValues{
				"save": {"900 1"},
			},
			wantChanged: map[string]ValkeyConfigValues{},
			wantDeleted: map[string]ValkeyConfigValues{},
		},
		{
			name: "Changed keys",
			old: ValkeyConfig{
				"maxmemory": ValkeyConfigValues{"100mb"},
				"save":      ValkeyConfigValues{"900 1"},
			},
			new: ValkeyConfig{
				"maxmemory": ValkeyConfigValues{"200mb"},
				"save":      ValkeyConfigValues{"900 1"},
			},
			wantAdded: map[string]ValkeyConfigValues{},
			wantChanged: map[string]ValkeyConfigValues{
				"maxmemory": {"200mb"},
			},
			wantDeleted: map[string]ValkeyConfigValues{},
		},
		{
			name: "Deleted keys",
			old: ValkeyConfig{
				"maxmemory": ValkeyConfigValues{"100mb"},
				"save":      ValkeyConfigValues{"900 1"},
			},
			new: ValkeyConfig{
				"maxmemory": ValkeyConfigValues{"100mb"},
			},
			wantAdded:   map[string]ValkeyConfigValues{},
			wantChanged: map[string]ValkeyConfigValues{},
			wantDeleted: map[string]ValkeyConfigValues{
				"save": {"900 1"},
			},
		},
		{
			name: "Added, changed and deleted keys",
			old: ValkeyConfig{
				"maxmemory":        ValkeyConfigValues{"100mb"},
				"save":             ValkeyConfigValues{"900 1"},
				"maxmemory-policy": ValkeyConfigValues{"allkeys-lru"},
			},
			new: ValkeyConfig{
				"maxmemory":        ValkeyConfigValues{"200mb"},
				"appendonly":       ValkeyConfigValues{"yes"},
				"maxmemory-policy": ValkeyConfigValues{"allkeys-lru"},
			},
			wantAdded: map[string]ValkeyConfigValues{
				"appendonly": {"yes"},
			},
			wantChanged: map[string]ValkeyConfigValues{
				"maxmemory": {"200mb"},
			},
			wantDeleted: map[string]ValkeyConfigValues{
				"save": {"900 1"},
			},
		},
		{
			name: "Same value but different order in values",
			old: ValkeyConfig{
				"save": ValkeyConfigValues{"900 1", "300 10"},
			},
			new: ValkeyConfig{
				"save": ValkeyConfigValues{"300 10", "900 1"},
			},
			wantAdded:   nil,
			wantChanged: nil,
			wantDeleted: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			added, changed, deleted := tt.old.Diff(tt.new)
			compareNilOrEqual(t, "added", added, tt.wantAdded)
			compareNilOrEqual(t, "changed", changed, tt.wantChanged)
			compareNilOrEqual(t, "deleted", deleted, tt.wantDeleted)
		})
	}
}
