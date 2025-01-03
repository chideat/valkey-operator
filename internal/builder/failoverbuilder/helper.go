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
package failoverbuilder

import (
	"fmt"

	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
)

const (
	BaseName               = "rf"
	SentinelName           = "s"
	SentinelRoleName       = "sentinel"
	SentinelConfigFileName = "sentinel.conf"
	RedisConfigFileName    = "redis.conf"
	RedisName              = "r"
	RedisShutdownName      = "r-s"
	RedisReadinessName     = "r-readiness"
	RedisRoleName          = "redis"
	RedisMasterName        = "mymaster"

	// TODO: reserverd for compatibility, remove in 3.22
	RedisSentinelSVCHostKey = "RFS_REDIS_SERVICE_HOST"
	RedisSentinelSVCPortKey = "RFS_REDIS_SERVICE_PORT_SENTINEL"
)

// variables refering to the redis exporter port
const (
	ExporterPort                  = 9121
	SentinelExporterPort          = 9355
	SentinelPort                  = "26379"
	ExporterPortName              = "http-metrics"
	RedisPort                     = 6379
	RedisPortString               = "6379"
	RedisPortName                 = "redis"
	ExporterContainerName         = "redis-exporter"
	SentinelExporterContainerName = "sentinel-exporter"
)

// label
const (
	LabelInstanceName     = "app.kubernetes.io/name"
	LabelPartOf           = "app.kubernetes.io/part-of"
	LabelRedisConfig      = "redis.middleware.alauda.io/config"
	LabelRedisConfigValue = "true"
	LabelRedisRole        = "redis.middleware.alauda.io/role"
)

// Redis arch
const (
	Standalone = "standalone"
	Sentinel   = "sentinel"
	Cluster    = "cluster"
)

// Redis role
const (
	Master = "master"
	Slave  = "slave"
)

func GetCommonLabels(name string, extra ...map[string]string) map[string]string {
	labels := getPublicLabels(name)
	for _, item := range extra {
		for k, v := range item {
			labels[k] = v
		}
	}
	return labels
}

func getPublicLabels(name string) map[string]string {
	return map[string]string{
		"redisfailovers.databases.spotahome.com/name": name,
		"app.kubernetes.io/managed-by":                "redis-operator",
		builder.InstanceNameLabel:                     name,
		builder.InstanceTypeLabel:                     "redis-failover",
	}
}

func GenerateSelectorLabels(component, name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/part-of":   "redis-failover",
		"app.kubernetes.io/component": component,
		"app.kubernetes.io/name":      name,
	}
}

func GetFailoverStatefulSetName(sentinelName string) string {
	return fmt.Sprintf("rfr-%s", sentinelName)
}

// GetFailoverDeploymentName
// Deprecated in favor of standalone sentinel
func GetFailoverDeploymentName(sentinelName string) string {
	return fmt.Sprintf("rfs-%s", sentinelName)
}

// Redis standalone annotation
const (
	AnnotationStandaloneLoadFilePath = "redis-standalone/filepath"
	AnnotationStandaloneInitStorage  = "redis-standalone/storage-type"
	AnnotationStandaloneInitPvcName  = "redis-standalone/pvc-name"
	AnnotationStandaloneInitHostPath = "redis-standalone/hostpath"
)

func NeedStandaloneInit(rf *v1alpha1.Failover) bool {
	if rf.Annotations[AnnotationStandaloneInitStorage] != "" &&
		(rf.Annotations[AnnotationStandaloneInitPvcName] != "" || rf.Annotations[AnnotationStandaloneInitHostPath] != "") &&
		rf.Annotations[AnnotationStandaloneLoadFilePath] != "" {
		return true
	}
	return false
}

func GenerateName(typeName, metaName string) string {
	return fmt.Sprintf("%s%s-%s", BaseName, typeName, metaName)
}

func GetRedisName(rf *v1alpha1.Failover) string {
	return GenerateName(RedisName, rf.Name)
}

func GetFailoverNodePortServiceName(rf *v1alpha1.Failover, index int) string {
	name := GetFailoverStatefulSetName(rf.Name)
	return fmt.Sprintf("%s-%d", name, index)
}

func GetRedisShutdownName(rf *v1alpha1.Failover) string {
	return GenerateName(RedisShutdownName, rf.Name)
}

func GetRedisNameExporter(rf *v1alpha1.Failover) string {
	return GenerateName(fmt.Sprintf("%s%s", RedisName, "e"), rf.Name)
}

func GetRedisNodePortSvc(rf *v1alpha1.Failover) string {
	return GenerateName(fmt.Sprintf("%s-%s", RedisName, "n"), rf.Name)
}

func GetRedisSecretName(rf *v1alpha1.Failover) string {
	return GenerateName(fmt.Sprintf("%s-%s", RedisName, "p"), rf.Name)
}

func GetRedisShutdownConfigMapName(rf *v1alpha1.Failover) string {
	return GenerateName(fmt.Sprintf("%s-%s", RedisName, "s"), rf.Name)
}

func GetCronJobName(redisName, scheduleName string) string {
	return fmt.Sprintf("%s-%s", redisName, scheduleName)
}

// Sentinel
func GetSentinelReadinessConfigmap(name string) string {
	return GenerateName(fmt.Sprintf("%s-%s", SentinelName, "r"), name)
}

func GetSentinelConfigmap(name string) string {
	return GenerateName(fmt.Sprintf("%s-%s", SentinelName, "r"), name)
}

func GetSentinelName(name string) string {
	return GenerateName(SentinelName, name)
}

func GetSentinelHeadlessSvc(name string) string {
	return GenerateName(SentinelName, fmt.Sprintf("%s-%s", name, "hl"))
}
