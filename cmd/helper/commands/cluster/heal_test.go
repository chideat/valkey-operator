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
			// A freshly seeded node: node-id plus its own per-pod shard-id. Both
			// must pass through untouched — shard-id is valkey's to re-home once
			// the node replicates, but it has to be present or valkey never adds
			// the node to server.cluster->shards.
			name: "new init node with seeded shard-id preserved",
			data: []byte(`267600f4b192a940a20759aa0ebeee22f41d69e6 :0@0,,shard-id=8fbc4e6a1d2f3b5c7e9a0d1f2b3c4d5e6f708192 myself,master - 0 0 0 connected
vars currentEpoch 0 lastVoteEpoch 0`),
			want: []byte(`267600f4b192a940a20759aa0ebeee22f41d69e6 :0@0,,shard-id=8fbc4e6a1d2f3b5c7e9a0d1f2b3c4d5e6f708192 myself,master - 0 0 0 connected
vars currentEpoch 0 lastVoteEpoch 0`),
		},
		{
			// Nodes seeded by an operator predating the shard-id seed still load;
			// the sanitizer must not start injecting one on recovery either.
			name: "legacy init node without shard-id preserved",
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

	// It must seed a deterministic node-id so the pod keeps a stable identity
	// across restarts.
	if !strings.Contains(got, "myself,master") {
		t.Fatalf("seeded node record must declare myself,master, got:\n%s", got)
	}
	fields := strings.Fields(got)
	nodeID := fields[0]
	if len(nodeID) != 40 {
		t.Fatalf("expected a 40-char node-id, got %q", nodeID)
	}
	// Determinism: same namespace/pod -> same record.
	if again := string(generateValkeyCluterNodeRecord("default", "drc-c77-0-0")); again != got {
		t.Fatalf("node record not deterministic:\n%s\n!=\n%s", got, again)
	}

	// The record must carry a shard-id. valkey adds a node to
	// server.cluster->shards only from the "shard-id=" aux handler — the
	// clusterLoadConfig fallback for a primary "without a persisted shard_id"
	// never fires, because createClusterNode has already filled shard_id with
	// random hex and auxShardIdPresent() only measures strlen of that. A seeded
	// primary with no shard-id is therefore absent from its own CLUSTER SHARDS
	// reply along with the slots it owns, and valkey-go (which reads CLUSTER
	// SHARDS on servers >= 8) fails every key in that range with "the slot has
	// no valkey node".
	shardID := ""
	for _, part := range strings.Split(fields[1], ",") {
		if v, ok := strings.CutPrefix(part, "shard-id="); ok {
			shardID = v
		}
	}
	if shardID == "" {
		t.Fatalf("seeded node record must carry a shard-id, got:\n%s", got)
	}
	// valkey's verifyClusterNodeId rejects anything that is not 40 hex chars,
	// and a rejected aux field aborts nodes.conf loading entirely.
	if len(shardID) != 40 {
		t.Fatalf("expected a 40-char shard-id, got %q", shardID)
	}
	for _, r := range shardID {
		if !strings.ContainsRune("0123456789abcdef", r) {
			t.Fatalf("shard-id must be lowercase hex, got %q", shardID)
		}
	}
	if shardID == nodeID {
		t.Fatalf("shard-id must not duplicate the node-id, both %q", shardID)
	}

	// Regression guard for the valkey 9.1 failover breakage (valkey-io/valkey#2811):
	// the pods of one shard must NOT start out sharing a shard-id. When they did,
	// the replica's first post-REPLICATE announcement took valkey's same-shard
	// stale-packet branch and was dropped forever, so peers kept seeing it as an
	// empty primary and failover elections drew zero votes. Per-pod uniqueness
	// keeps that announcement on the cross-shard-move path.
	peer := string(generateValkeyCluterNodeRecord("default", "drc-c77-0-1"))
	peerShardID := ""
	for _, part := range strings.Split(strings.Fields(peer)[1], ",") {
		if v, ok := strings.CutPrefix(part, "shard-id="); ok {
			peerShardID = v
		}
	}
	if peerShardID == shardID {
		t.Fatalf("pods of the same shard must not share a seeded shard-id, both %q", shardID)
	}

	// The seeded line must survive the recovery sanitizer untouched.
	if fixed := string(fixClusterNodesConf([]byte(got))); fixed != got {
		t.Fatalf("fixClusterNodesConf altered the seeded record:\n%s\n!=\n%s", fixed, got)
	}
}
