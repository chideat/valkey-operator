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
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

type Address string

func (a Address) parse() (string, int, error) {
	addr := string(a)
	lastColonIndex := strings.LastIndex(addr, ":")
	if lastColonIndex == -1 {
		return "", 0, fmt.Errorf("Invalid IP:Port format")
	}

	ip := strings.TrimSuffix(strings.TrimPrefix(addr[:lastColonIndex], "["), "]")
	port, err := strconv.Atoi(addr[lastColonIndex+1:])
	if err != nil {
		return "", 0, fmt.Errorf("Invalid port number: %v", err)
	}
	return ip, port, nil
}

func (a Address) Host() string {
	host, _, _ := a.parse()
	return host
}

func (a Address) Port() int {
	_, port, _ := a.parse()
	return port
}

func (a Address) String() string {
	host, port, _ := a.parse()
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// DialAddress returns addr in the form net.Dial accepts.
//
// Valkey prints IPv6 literals bare in CLUSTER NODES, nodes.conf and INFO
// ("fd00::1:6379"), and Go's dialer rejects that form with "too many colons
// in address" — it wants "[fd00::1]:6379". When addr parses as an IPv6
// literal followed by a port the host is bracketed; hostnames, IPv4,
// already-bracketed literals and anything unparseable are returned
// unchanged, so a dial error still names exactly what the caller passed.
// A bare IPv6 literal with no port is inherently ambiguous — its last group
// reads as the port — and is not dialable either way.
func DialAddress(addr string) string {
	host, port, err := Address(addr).parse()
	if err != nil {
		return addr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !ip.Is6() {
		return addr
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}
