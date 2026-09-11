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

package valkey

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddress_Parse(t *testing.T) {
	tests := []struct {
		name     string
		addr     Address
		wantIP   string
		wantPort int
		wantErr  bool
	}{
		{
			name:     "valid IPv4 address",
			addr:     Address("192.168.1.1:6379"),
			wantIP:   "192.168.1.1",
			wantPort: 6379,
			wantErr:  false,
		},
		{
			name:     "valid IPv6 address",
			addr:     Address("[2001:db8::1]:6379"),
			wantIP:   "2001:db8::1",
			wantPort: 6379,
			wantErr:  false,
		},
		{
			name:    "invalid address format",
			addr:    Address("invalid"),
			wantErr: true,
		},
		{
			name:    "invalid port number",
			addr:    Address("192.168.1.1:invalid"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIP, gotPort, err := tt.addr.parse()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantIP, gotIP)
			assert.Equal(t, tt.wantPort, gotPort)
		})
	}
}

func TestAddress_Host(t *testing.T) {
	tests := []struct {
		name     string
		addr     Address
		wantHost string
	}{
		{
			name:     "IPv4 address",
			addr:     Address("192.168.1.1:6379"),
			wantHost: "192.168.1.1",
		},
		{
			name:     "IPv6 address",
			addr:     Address("[2001:db8::1]:6379"),
			wantHost: "2001:db8::1",
		},
		{
			name:     "invalid address",
			addr:     Address("invalid"),
			wantHost: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.addr.Host()
			assert.Equal(t, tt.wantHost, got)
		})
	}
}

func TestAddress_Port(t *testing.T) {
	tests := []struct {
		name     string
		addr     Address
		wantPort int
	}{
		{
			name:     "valid port",
			addr:     Address("192.168.1.1:6379"),
			wantPort: 6379,
		},
		{
			name:     "invalid address",
			addr:     Address("invalid"),
			wantPort: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.addr.Port()
			assert.Equal(t, tt.wantPort, got)
		})
	}
}

func TestAddress_String(t *testing.T) {
	tests := []struct {
		name    string
		addr    Address
		wantStr string
	}{
		{
			name:    "IPv4 address",
			addr:    Address("192.168.1.1:6379"),
			wantStr: "192.168.1.1:6379",
		},
		{
			name:    "IPv6 address",
			addr:    Address("[2001:db8::1]:6379"),
			wantStr: "[2001:db8::1]:6379",
		},
		{
			name:    "invalid address",
			addr:    Address("invalid"),
			wantStr: ":0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.addr.String()
			assert.Equal(t, tt.wantStr, got)
		})
	}
}

func TestDialAddress(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{"bare IPv6 as CLUSTER NODES prints it", "fd00:10:244::a1:6379", "[fd00:10:244::a1]:6379"},
		{"bare IPv6 loopback", "::1:6379", "[::1]:6379"},
		{"bare IPv6 with zone", "fe80::1%eth0:6379", "[fe80::1%eth0]:6379"},
		{"already bracketed", "[fd00:10:244::a1]:6379", "[fd00:10:244::a1]:6379"},
		{"IPv4", "10.0.0.1:6379", "10.0.0.1:6379"},
		{"hostname", "local.inject:6379", "local.inject:6379"},
		{"service name", "rfr-x-readwrite.ns.svc:6379", "rfr-x-readwrite.ns.svc:6379"},
		{"port only", ":6379", ":6379"},
		{"IPv6 without a port", "fd00:10:244::a1", "fd00:10:244::a1"},
		{"IPv6 without a port, digits last", "fd00::1", "fd00::1"},
		{"port-less IPv6 whose last group is numeric reads as host:port", "2001:db8::1:2:3:4", "[2001:db8::1:2:3]:4"},
		{"non-numeric port", "fd00::1:abc", "fd00::1:abc"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DialAddress(tt.addr); got != tt.want {
				t.Errorf("DialAddress(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}

// The runner dials ClusterNode.Addr verbatim, so the constructor is where a
// bare IPv6 literal has to become dialable.
func TestNewValkeyClient_NormalisesDialAddress(t *testing.T) {
	c := NewValkeyClient("fd00:10:244::a1:6379", AuthConfig{})
	defer c.Close()
	vc, ok := c.(*valkeyClient)
	if !ok {
		t.Fatalf("unexpected client type %T", c)
	}
	if want := "[fd00:10:244::a1]:6379"; vc.addr != want {
		t.Fatalf("stored addr = %q, want %q", vc.addr, want)
	}
}
