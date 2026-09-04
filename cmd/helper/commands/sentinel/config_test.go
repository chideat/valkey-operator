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

package sentinel

import "testing"

func TestInternalIPFromBind(t *testing.T) {
	tests := []struct {
		name     string
		bind     string
		ipFamily string
		want     string
	}{
		{
			// The bind list run_failover.sh writes on an IPv6 instance.
			name:     "IPv6 pod address next to the ::1 literal",
			bind:     "fd00:10:244::a1 ::1",
			ipFamily: "IPv6",
			want:     "fd00:10:244::a1",
		},
		{
			name:     "IPv4 pod address next to the 127.0.0.1 literal",
			bind:     "10.244.1.5 127.0.0.1",
			ipFamily: "IPv4",
			want:     "10.244.1.5",
		},
		{
			name:     "dual stack picks the preferred family",
			bind:     "fd00:10:244::a1 10.244.1.5 ::1",
			ipFamily: "IPv4",
			want:     "10.244.1.5",
		},
		{
			// What an entrypoint older than this fix bound. The alias name used
			// to reach netip.MustParseAddr and panic the merge whenever the
			// entries before it did not match the family.
			name:     "the legacy alias name is skipped, not parsed",
			bind:     "10.244.1.5 local.inject",
			ipFamily: "IPv6",
			want:     "",
		},
		{
			name:     "a loopback literal is never chosen",
			bind:     "127.0.0.1 ::1",
			ipFamily: "IPv6",
			want:     "",
		},
		{
			name:     "no preference matches nothing",
			bind:     "10.244.1.5 127.0.0.1",
			ipFamily: "",
			want:     "",
		},
		{
			name:     "empty bind",
			bind:     "",
			ipFamily: "IPv4",
			want:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := internalIPFromBind(tt.bind, tt.ipFamily); got != tt.want {
				t.Errorf("internalIPFromBind(%q, %q) = %q, want %q", tt.bind, tt.ipFamily, got, tt.want)
			}
		})
	}
}
