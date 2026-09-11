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

package commands

import (
	"fmt"
	"net"
	"strings"

	"github.com/chideat/valkey-operator/pkg/valkey"
)

// ParseMonitorURI returns the addresses named by a monitor URI of the form
// "sentinel://host:port,host:port,...", which is how the failover builder
// writes MONITOR_URI into the valkey pods.
//
// It deliberately does not use url.Parse. The host part is a comma-joined
// list, which is not a URL authority: url.Parse tolerates that for IPv4 and
// hostnames only because it never validates them, but it does validate an
// IPv6 literal and rejects the second bracketed one as "invalid IP-literal".
// The builder brackets every IPv6 sentinel (net.JoinHostPort), so on an IPv6
// cluster every URI naming two or more sentinels failed to parse, and both
// the boot-time master lookup and the preStop failover skipped their work.
//
// Each element comes back in dialable form: a bare IPv6 literal is
// bracketed, hostnames and IPv4 pass through, and anything that is not
// host:port is an error naming the offending element.
func ParseMonitorURI(uri string) ([]string, error) {
	rest := strings.TrimSpace(uri)
	if i := strings.Index(rest, "://"); i >= 0 {
		rest = rest[i+3:]
	}
	var addrs []string
	for _, part := range strings.Split(rest, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		addr := valkey.DialAddress(part)
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid address %q in monitor uri %q: %w", part, uri, err)
		}
		if host == "" || port == "" {
			return nil, fmt.Errorf("invalid address %q in monitor uri %q: host and port are both required", part, uri)
		}
		addrs = append(addrs, addr)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("monitor uri %q names no address", uri)
	}
	return addrs, nil
}
