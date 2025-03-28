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
