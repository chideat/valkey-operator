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

package cluster

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func Test_fixClusterNodesConf(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []byte
	}{
		{
			// A freshly seeded node carries a node-id but no shard-id; it must
			// pass through untouched (the operator must not inject a shard-id).
			name: "new init node without shard-id preserved",
			data: []byte(`267600f4b192a940a20759aa0ebeee22f41d69e6 :0@0 myself,master - 0 0 0 connected
vars currentEpoch 0 lastVoteEpoch 0`),
			want: []byte(`267600f4b192a940a20759aa0ebeee22f41d69e6 :0@0 myself,master - 0 0 0 connected
vars currentEpoch 0 lastVoteEpoch 0`),
		},
		{
			// A recovered cluster config: every node's valkey-assigned shard-id
			// (including the distinct per-shard ids) must be preserved verbatim.
			name: "6 nodes shard-ids preserved",
			data: []byte(`f89575a0d78cdc25b5ea1886bb88f1d979026c7d 192.168.132.183:32295@31500,,tls-port=0,shard-id=aaaa1111 slave c8b765997335f66f892ca6840f7f0b6df8200638 0 1709546093510 1 connected
c8b765997335f66f892ca6840f7f0b6df8200638 192.168.132.208:30471@30566,,tls-port=0,shard-id=aaaa1111 master - 0 1709546095505 1 connected 10923-16383
4cc7fd15a841f081f8c956b0432f75baa170ea97 192.168.132.208:30969@30670,,tls-port=0,shard-id=bbbb2222 slave a8fa11eb3c3cc0c8115b8fe191d1e8ce92b857a5 0 1709546094000 2 connected
e8cd1219f9d712f7a1002962625f4f6ab46e4a69 192.168.132.209:31741@30771,,tls-port=0,shard-id=cccc3333 myself,master - 0 1709546091000 3 connected 0-5461
c4db03ea65954e1c2ced6135b8622360b5bf6ca7 192.168.132.183:31176@32087,,tls-port=0,shard-id=cccc3333 slave e8cd1219f9d712f7a1002962625f4f6ab46e4a69 0 1709546092000 3 connected
a8fa11eb3c3cc0c8115b8fe191d1e8ce92b857a5 192.168.132.183:30550@31597,,tls-port=0,shard-id=bbbb2222 master - 0 1709546094511 2 connected 5462-10922
vars currentEpoch 5 lastVoteEpoch 0`),
			want: []byte(`f89575a0d78cdc25b5ea1886bb88f1d979026c7d 192.168.132.183:32295@31500,,tls-port=0,shard-id=aaaa1111 slave c8b765997335f66f892ca6840f7f0b6df8200638 0 1709546093510 1 connected
c8b765997335f66f892ca6840f7f0b6df8200638 192.168.132.208:30471@30566,,tls-port=0,shard-id=aaaa1111 master - 0 1709546095505 1 connected 10923-16383
4cc7fd15a841f081f8c956b0432f75baa170ea97 192.168.132.208:30969@30670,,tls-port=0,shard-id=bbbb2222 slave a8fa11eb3c3cc0c8115b8fe191d1e8ce92b857a5 0 1709546094000 2 connected
e8cd1219f9d712f7a1002962625f4f6ab46e4a69 192.168.132.209:31741@30771,,tls-port=0,shard-id=cccc3333 myself,master - 0 1709546091000 3 connected 0-5461
c4db03ea65954e1c2ced6135b8622360b5bf6ca7 192.168.132.183:31176@32087,,tls-port=0,shard-id=cccc3333 slave e8cd1219f9d712f7a1002962625f4f6ab46e4a69 0 1709546092000 3 connected
a8fa11eb3c3cc0c8115b8fe191d1e8ce92b857a5 192.168.132.183:30550@31597,,tls-port=0,shard-id=bbbb2222 master - 0 1709546094511 2 connected 5462-10922
vars currentEpoch 5 lastVoteEpoch 0`),
		},
		{
			// A crash can leave a partial/garbage node line (no "connected" state
			// or < 8 fields); such lines are dropped, valid lines are kept.
			name: "drop crash-corrupted node line",
			data: []byte(`c8b765997335f66f892ca6840f7f0b6df8200638 192.168.132.208:30471@30566,,tls-port=0,shard-id=aaaa1111 master - 0 1709546095505 1 connected 10923-16383
deadbeef garbage line from crash
a8fa11eb3c3cc0c8115b8fe191d1e8ce92b857a5 192.168.132.183:30550@31597,,tls-port=0,shard-id=bbbb2222 master - 0 1709546094511 2 connected 5462-10922
vars currentEpoch 5 lastVoteEpoch 0`),
			want: []byte(`c8b765997335f66f892ca6840f7f0b6df8200638 192.168.132.208:30471@30566,,tls-port=0,shard-id=aaaa1111 master - 0 1709546095505 1 connected 10923-16383
a8fa11eb3c3cc0c8115b8fe191d1e8ce92b857a5 192.168.132.183:30550@31597,,tls-port=0,shard-id=bbbb2222 master - 0 1709546094511 2 connected 5462-10922
vars currentEpoch 5 lastVoteEpoch 0`),
		},
		{
			// A truncated epoch line (currentEpoch only) is normalized to the
			// full "currentEpoch N lastVoteEpoch N" form valkey expects.
			name: "normalize truncated epoch line",
			data: []byte(`c8b765997335f66f892ca6840f7f0b6df8200638 192.168.132.208:30471@30566,,tls-port=0,shard-id=aaaa1111 myself,master - 0 1709546095505 1 connected 0-16383
vars currentEpoch 7`),
			want: []byte(`c8b765997335f66f892ca6840f7f0b6df8200638 192.168.132.208:30471@30566,,tls-port=0,shard-id=aaaa1111 myself,master - 0 1709546095505 1 connected 0-16383
vars currentEpoch 7 lastVoteEpoch 7`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixClusterNodesConf(tt.data)
			gotVals := strings.Split(string(got), "\n")
			wantVals := strings.Split(string(tt.want), "\n")
			sort.Strings(gotVals)
			sort.Strings(wantVals)
			if !reflect.DeepEqual(gotVals, wantVals) {
				t.Errorf("fixClusterNodesConf() = (%d)\n%s\n## want (%d)\n%s",
					len(got), strings.Join(gotVals, "\n"), len(tt.want), strings.Join(wantVals, "\n"))
			}
		})
	}
}

func Test_generateValkeyCluterNodeRecord(t *testing.T) {
	got := string(generateValkeyCluterNodeRecord("default", "drc-c77-0-0"))

	// Regression guard for valkey-io/valkey#2811: the seeded record must NOT
	// carry a shard-id (a pre-written shard-id makes an empty replica look like
	// an empty primary to peers and permanently breaks failover on valkey 9.1).
	if strings.Contains(got, "shard-id") {
		t.Fatalf("seeded node record must not contain a shard-id, got:\n%s", got)
	}
	// It must still seed a deterministic node-id so the pod keeps a stable
	// identity across restarts.
	if !strings.Contains(got, "myself,master") {
		t.Fatalf("seeded node record must declare myself,master, got:\n%s", got)
	}
	nodeID := strings.Fields(got)[0]
	if len(nodeID) != 40 {
		t.Fatalf("expected a 40-char node-id, got %q", nodeID)
	}
	// Determinism: same namespace/pod -> same node-id.
	if again := string(generateValkeyCluterNodeRecord("default", "drc-c77-0-0")); again != got {
		t.Fatalf("node record not deterministic:\n%s\n!=\n%s", got, again)
	}
}
