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

package sentinelbuilder

import (
	"fmt"

	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/samber/lo"
)

const (
	SentinelConfigFileName = "sentinel.conf"
)

func GetCommonLabels(name string, extra ...map[string]string) map[string]string {
	return lo.Assign(lo.Assign(extra...), getPublicLabels(name))
}

func getPublicLabels(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/component":                 RedisArchRoleSEN,
		"app.kubernetes.io/managed-by":                "redis-operator",
		builder.InstanceNameLabel:                     name,
		builder.InstanceTypeLabel:                     "redis-failover",
		"redissentinels.databases.spotahome.com/name": name,
	}
}

func GenerateSelectorLabels(component, name string) map[string]string {
	// NOTE: not use "middleware.instance/name" and "middleware.instance/type" for compatibility for old instances
	return map[string]string{
		"app.kubernetes.io/name":                      name,
		"redissentinels.databases.spotahome.com/name": name,
		"app.kubernetes.io/component":                 component,
	}
}

func GetSentinelConfigMapName(name string) string {
	// NOTE: use rfs-server-xxx for compatibility for old name "rfs-xxx" maybe needed
	return fmt.Sprintf("rfs-server-%s", name)
}

func GetSentinelStatefulSetName(sentinelName string) string {
	return fmt.Sprintf("rfs-%s", sentinelName)
}

func GetSentinelServiceName(sentinelName string) string {
	return fmt.Sprintf("rfs-%s", sentinelName)
}

func GetSentinelHeadlessServiceName(sentinelName string) string {
	// NOTE: use rfs-server-xxx-hl for compatibility for old name "rfs-xxx-hl"
	// this naming method may cause name conflict with <name>-hl
	return fmt.Sprintf("rfs-%s-hl", sentinelName)
}

func GetSentinelNodeServiceName(sentinelName string, i int) string {
	return fmt.Sprintf("rfs-%s-%d", sentinelName, i)
}
