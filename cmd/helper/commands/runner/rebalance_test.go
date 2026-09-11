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

package runner

import "testing"

func TestGetLocalAddressByIPFamily(t *testing.T) {
	const def = "10.0.0.9"
	tests := []struct {
		name   string
		family string
		bind   []string
		want   string
	}{
		{
			// The bind list run_cluster.sh writes on an IPv6 instance: the pod
			// address, then the loopback literal.
			name:   "IPv6 pod address next to the ::1 literal",
			family: "IPv6",
			bind:   []string{"fd00:10:244::a1", "::1"},
			want:   "fd00:10:244::a1",
		},
		{
			name:   "IPv4 pod address next to the 127.0.0.1 literal",
			family: "IPv4",
			bind:   []string{"10.244.1.5", "127.0.0.1"},
			want:   "10.244.1.5",
		},
		{
			// Dual stack with IPv6 preferred: the entrypoint lists the preferred
			// address first and status.podIP after it.
			name:   "dual stack picks the preferred family",
			family: "IPv6",
			bind:   []string{"fd00:10:244::a1", "10.244.1.5", "::1"},
			want:   "fd00:10:244::a1",
		},
		{
			name:   "dual stack picks IPv4 when preferred",
			family: "IPv4",
			bind:   []string{"10.244.1.5", "fd00:10:244::a1", "127.0.0.1"},
			want:   "10.244.1.5",
		},
		{
			// The loopback literal is never a candidate, whatever its position.
			// An earlier version matched the family against the unfiltered list
			// and handed back ::1 here.
			name:   "the loopback literal never satisfies the family",
			family: "IPv6",
			bind:   []string{"10.244.1.5", "::1"},
			want:   def,
		},
		{
			name:   "loopback first is still skipped",
			family: "IPv6",
			bind:   []string{"::1", "fd00:10:244::a1"},
			want:   "fd00:10:244::a1",
		},
		{
			name:   "no preference takes the first non-loopback entry",
			family: "",
			bind:   []string{"127.0.0.1", "10.244.1.5", "fd00:10:244::a1"},
			want:   "10.244.1.5",
		},
		{
			// What an entrypoint older than this fix bound: the alias by name.
			name:   "the legacy alias name is skipped like a loopback",
			family: "IPv4",
			bind:   []string{"10.244.1.5", "local.inject"},
			want:   "10.244.1.5",
		},
		{
			name:   "a hostname never matches a family",
			family: "IPv4",
			bind:   []string{"valkey.example", "10.244.1.5"},
			want:   "10.244.1.5",
		},
		{
			name:   "only loopbacks falls back",
			family: "IPv4",
			bind:   []string{"127.0.0.1", "::1", "localhost"},
			want:   def,
		},
		{
			name:   "empty list falls back",
			family: "IPv6",
			bind:   nil,
			want:   def,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := append([]string(nil), tt.bind...)
			if got := getLocalAddressByIPFamily(tt.family, in, def); got != tt.want {
				t.Errorf("getLocalAddressByIPFamily(%q, %v) = %q, want %q", tt.family, tt.bind, got, tt.want)
			}
		})
	}
}
