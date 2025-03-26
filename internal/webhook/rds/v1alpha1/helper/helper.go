/*
Copyright 2023 The RedisOperator Authors.

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

func CalculateNodeCount(arch core.Arch, masterCount int32, replicaCount int32) int {
	switch arch {
	case core.ValkeyCluster:
		return int(masterCount) * int(replicaCount+1)
	case core.ValkeyFailover:
		return int(masterCount) + int(replicaCount)
	case core.ValkeyReplica:
		return 1
	default:
		return 0
	}
}
