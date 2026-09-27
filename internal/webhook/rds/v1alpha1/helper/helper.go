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

package helper

import (
	"regexp"
	"strconv"

	"github.com/chideat/valkey-operator/api/core"
)

var (
	shardIndexReg = regexp.MustCompile(`^.*-(\d+)$`)
)

func parseShardIndex(name string) int {
	if shardIndexReg.MatchString(name) {
		matches := shardIndexReg.FindStringSubmatch(name)
		if len(matches) == 2 {
			val, _ := strconv.ParseInt(matches[1], 10, 32)
			return int(val)
		}
	}
	return 0
}

// CalculateNodeCount returns the number of Valkey pods the operator runs for an
// instance: replicasOfShard pods in each shard. The failover and replica
// architectures have a single shard.
func CalculateNodeCount(arch core.Arch, shards int32, replicasOfShard int32) int {
	switch arch {
	case core.ValkeyCluster:
		return int(shards) * int(replicasOfShard)
	case core.ValkeyFailover, core.ValkeyReplica:
		return int(replicasOfShard)
	default:
		return 0
	}
}
