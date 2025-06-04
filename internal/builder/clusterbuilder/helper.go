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

package clusterbuilder

import (
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/samber/lo"
)

// generateSelectors
// NOTE: this labels are const, take care of edit this
func generateSelectors(name string) map[string]string {
	return map[string]string{
		builder.InstanceTypeLabelKey: string(core.ValkeyCluster),
		builder.InstanceNameLabelKey: name,
		builder.ManagedByLabelKey:    config.AppName,
		builder.AppComponentLabelKey: string(core.ValkeyCluster),
		builder.AppNameLabelKey:      name,
	}
}

// GenerateClusterStatefulSetSelectors
func GenerateClusterStatefulSetSelectors(name string, index int) map[string]string {
	labels := generateSelectors(name)
	if index >= 0 {
		labels["statefulset"] = ClusterStatefulSetName(name, index)
	}
	return labels
}

// GenerateClusterStatefulSetLabels
func GenerateClusterStatefulSetLabels(name string, index int, extra ...map[string]string) map[string]string {
	labels := generateSelectors(name)
	if index >= 0 {
		labels["statefulset"] = ClusterStatefulSetName(name, index)
	}
	return labels
}

// GenerateClusterStaticLabels
// extra labels will override by the static labels
func GenerateClusterLabels(name string, extra map[string]string) map[string]string {
	return lo.Assign(generateSelectors(name), extra)
}
