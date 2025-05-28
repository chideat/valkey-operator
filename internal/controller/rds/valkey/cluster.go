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

package valkey

import (
	"reflect"

	"github.com/chideat/valkey-operator/api/core"
	rdsv1alpha1 "github.com/chideat/valkey-operator/api/rds/v1alpha1"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ApplyPolicy string

const (
	Unsupported  ApplyPolicy = "Unsupported"
	RestartApply ApplyPolicy = "RestartApply"
)

func GetValkeyConfigsApplyPolicyByVersion(ver string) map[string]ApplyPolicy {
	data := map[string]ApplyPolicy{
		"databases":           RestartApply,
		"rename-command":      RestartApply,
		"rdbchecksum":         RestartApply,
		"tcp-backlog":         RestartApply,
		"io-threads":          RestartApply,
		"io-threads-do-reads": RestartApply,
	}
	return data
}

func GenerateValkeyCluster(instance *rdsv1alpha1.Valkey) (*v1alpha1.Cluster, error) {
	customConfig := instance.Spec.CustomConfigs
	if customConfig == nil {
		customConfig = map[string]string{}
	}
	if instance.Spec.PodAnnotations == nil {
		instance.Spec.PodAnnotations = map[string]string{}
	}

	image := config.GetValkeyImageByVersion(instance.Spec.Version)
	var (
		labels   = clusterbuilder.GenerateClusterLabels(instance.Name, instance.Labels)
		exporter *core.Exporter
	)

	if exp := instance.Spec.Exporter; exp == nil || !exp.Disable {
		if instance.Spec.Exporter != nil {
			exporter = &exp.Exporter
		} else {
			exporter = &core.Exporter{}
		}
		if exporter.Image == "" {
			exporter.Image = config.GetValkeyExporterImage(nil)
		}
		if exporter.Resources == nil || exporter.Resources.Limits.Cpu().IsZero() || exporter.Resources.Limits.Memory().IsZero() {
			exporter.Resources = &corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("300Mi"),
				},
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("300Mi"),
				},
			}
		}
	}

	shardsConf := instance.Spec.Replicas.ShardsConfig
	if instance.Spec.Replicas.Shards != int32(len(shardsConf)) {
		shardsConf = nil
	}

	cluster := &v1alpha1.Cluster{
		ObjectMeta: v12.ObjectMeta{
			Name:            instance.Name,
			Namespace:       instance.Namespace,
			Labels:          labels,
			OwnerReferences: util.BuildOwnerReferences(instance),
		},
		Spec: v1alpha1.ClusterSpec{
			Image: image,
			Replicas: v1alpha1.ClusterReplicas{
				Shards:          instance.Spec.Replicas.Shards,
				ReplicasOfShard: instance.Spec.Replicas.ReplicasOfShard,
				ShardsConfig:    shardsConf,
			},
			Resources:       instance.Spec.Resources,
			CustomConfigs:   customConfig,
			CustomAffinity:  instance.Spec.CustomAffinity,
			AffinityPolicy:  instance.Spec.AffinityPolicy,
			NodeSelector:    instance.Spec.NodeSelector,
			Tolerations:     instance.Spec.Tolerations,
			SecurityContext: instance.Spec.SecurityContext,
			PodAnnotations:  instance.Spec.PodAnnotations,
			Access:          instance.Spec.Access,
			Exporter:        exporter,
			Storage:         instance.Spec.Storage,
		},
	}

	if instance.Spec.Storage != nil && instance.Spec.Storage.StorageClassName != nil &&
		(instance.Spec.Storage.Capacity == nil || instance.Spec.Storage.Capacity.IsZero()) {
		size := resource.NewQuantity(instance.Spec.Resources.Limits.Memory().Value()*2, resource.BinarySI)
		instance.Spec.Storage.Capacity = size
	}
	return cluster, nil
}

func ShouldUpdateCluster(cluster, newCluster *v1alpha1.Cluster, logger logr.Logger) bool {
	if !reflect.DeepEqual(cluster.Labels, newCluster.Labels) ||
		!reflect.DeepEqual(cluster.Annotations, newCluster.Annotations) {
		return true
	}

	return !reflect.DeepEqual(cluster.Spec, newCluster.Spec)
}

func ClusterIsUp(cluster *v1alpha1.Cluster) bool {
	return cluster.Status.Phase == v1alpha1.ClusterPhaseReady &&
		cluster.Status.ServiceStatus == v1alpha1.ClusterInService
}
