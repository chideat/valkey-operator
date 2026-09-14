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

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/chideat/valkey-operator/internal/builder"
)

var tlsDirAssignment = regexp.MustCompile(`(?m)^TLS_DIR="([^"]*)"`)

// The entrypoint scripts read the certificates out of the volume the builders
// mount, so the path is agreed between a Go constant and three shell scripts
// with nothing connecting them. run_failover.sh had drifted to "/tmp" -- the
// value its neighbouring config-file variables use -- and every TLS failover
// and replication instance died on startup with
//
//	# Failed to load certificate: /tmp/tls.crt: No such file or directory
//	# Failed to configure TLS. Check logs for more info.
//
// leaving the node unable to serve and the instance stuck in Initializing.
// Nothing failed at build time, and a non-TLS instance never reads the value.
func TestEntrypointScriptsReadCertificatesFromTheMountedVolume(t *testing.T) {
	scripts, err := filepath.Glob("run_*.sh")
	if err != nil {
		t.Fatalf("glob entrypoint scripts: %v", err)
	}
	if len(scripts) == 0 {
		t.Fatal("no entrypoint scripts found; this test is looking in the wrong place")
	}

	for _, script := range scripts {
		t.Run(filepath.Base(script), func(t *testing.T) {
			content, err := os.ReadFile(script)
			if err != nil {
				t.Fatalf("read %s: %v", script, err)
			}

			match := tlsDirAssignment.FindSubmatch(content)
			if match == nil {
				// A script that never configures TLS has nothing to agree with.
				if !regexp.MustCompile(`TLS_DIR`).Match(content) {
					t.Skipf("%s does not configure TLS", script)
				}
				t.Fatalf("%s uses TLS_DIR but never assigns it", script)
			}

			if got := string(match[1]); got != builder.ValkeyTLSVolumeDefaultMountPath {
				t.Errorf("TLS_DIR = %q, want %q: the certificates are mounted at %q, "+
					"so the server cannot load them from anywhere else",
					got, builder.ValkeyTLSVolumeDefaultMountPath,
					builder.ValkeyTLSVolumeDefaultMountPath)
			}
		})
	}
}
