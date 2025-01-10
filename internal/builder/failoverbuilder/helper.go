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
	ValkeyConfigFileName   = "valkey.conf"
	ValkeyName             = "r"
	ValkeyShutdownName     = "r-s"
	ValkeyReadinessName    = "r-readiness"
	ValkeyRoleName         = "valkey"
	ValkeyMasterName       = "mymaster"

	// TODO: reserverd for compatibility, remove in 3.22
	ValkeySentinelSVCHostKey = "RFS_VALKEY_SERVICE_HOST"
	ValkeySentinelSVCPortKey = "RFS_VALKEY_SERVICE_PORT_SENTINEL"
)

// variables refering to the exporter port
const (
	ExporterPort                  = 9121
	SentinelExporterPort          = 9355
	SentinelPort                  = "26379"
	ExporterPortName              = "http-metrics"
	ValkeyPort                    = 6379
	ValkeyPortString              = "6379"
	ValkeyPortName                = "valkey"
	ExporterContainerName         = "valkey-exporter"
	SentinelExporterContainerName = "sentinel-exporter"
)

// label
const (
	LabelInstanceName      = "app.kubernetes.io/name"
	LabelPartOf            = "app.kubernetes.io/part-of"
	LabelValkeyConfig      = "valkey.middleware.alauda.io/config"
	LabelValkeyConfigValue = "true"
	LabelValkeyRole        = "valkey.middleware.alauda.io/role"
)

// Valkey arch
const (
	Standalone = "standalone"
	Sentinel   = "sentinel"
	Cluster    = "cluster"
)

// Valkey role
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
		"valkeyfailovers.databases.spotahome.com/name": name,
		"app.kubernetes.io/managed-by":                 "valkey-operator",
		builder.InstanceNameLabel:                      name,
		builder.InstanceTypeLabel:                      "valkey-failover",
	}
}

func GenerateSelectorLabels(component, name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/part-of":   "valkey-failover",
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

// Valkey standalone annotation
const (
	AnnotationStandaloneLoadFilePath = "valkey-standalone/filepath"
	AnnotationStandaloneInitStorage  = "valkey-standalone/storage-type"
	AnnotationStandaloneInitPvcName  = "valkey-standalone/pvc-name"
	AnnotationStandaloneInitHostPath = "valkey-standalone/hostpath"
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

func GetValkeyName(rf *v1alpha1.Failover) string {
	return GenerateName(ValkeyName, rf.Name)
}

func GetFailoverNodePortServiceName(rf *v1alpha1.Failover, index int) string {
	name := GetFailoverStatefulSetName(rf.Name)
	return fmt.Sprintf("%s-%d", name, index)
}

func GetValkeyShutdownName(rf *v1alpha1.Failover) string {
	return GenerateName(ValkeyShutdownName, rf.Name)
}

func GetValkeyNameExporter(rf *v1alpha1.Failover) string {
	return GenerateName(fmt.Sprintf("%s%s", ValkeyName, "e"), rf.Name)
}

func GetValkeyNodePortSvc(rf *v1alpha1.Failover) string {
	return GenerateName(fmt.Sprintf("%s-%s", ValkeyName, "n"), rf.Name)
}

func GetValkeySecretName(rf *v1alpha1.Failover) string {
	return GenerateName(fmt.Sprintf("%s-%s", ValkeyName, "p"), rf.Name)
}

func GetValkeyShutdownConfigMapName(rf *v1alpha1.Failover) string {
	return GenerateName(fmt.Sprintf("%s-%s", ValkeyName, "s"), rf.Name)
}

func GetCronJobName(valkeyName, scheduleName string) string {
	return fmt.Sprintf("%s-%s", valkeyName, scheduleName)
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
