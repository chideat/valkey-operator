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
