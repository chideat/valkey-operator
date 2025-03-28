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
	"github.com/chideat/valkey-operator/internal/builder/failoverbuilder"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/chideat/valkey-operator/internal/util"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateFailover(instance *rdsv1alpha1.Valkey) (*v1alpha1.Failover, error) {
	image := config.GetValkeyImageByVersion(instance.Spec.Version)

	if instance.Spec.Arch == core.ValkeyFailover {
		if instance.Spec.Sentinel == nil {
			instance.Spec.Sentinel = &v1alpha1.SentinelSettings{}
		}
	} else {
		instance.Spec.Sentinel = nil
	}

	var (
		access   = instance.Spec.Access.DeepCopy()
		exporter *core.Exporter
		sentinel = instance.Spec.Sentinel.DeepCopy()

		annotations = map[string]string{}
	)

	if exp := instance.Spec.Exporter; exp == nil || !exp.Disable {
		exporter = &exp.Exporter
		if exporter.Image == "" {
			exporter.Image = config.GetValkeyExporterImage(nil)
		}
		if exporter.Resources.Limits.Cpu().IsZero() || exporter.Resources.Limits.Memory().IsZero() {
			exporter.Resources = &corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("384Mi"),
				},
			}
		}
	}

	if sentinel != nil {
		if sentinel.SentinelReference == nil {
			sentinel.Image = image

			if len(sentinel.NodeSelector) == 0 {
				sentinel.NodeSelector = instance.Spec.NodeSelector
			}
			if sentinel.Tolerations == nil {
				sentinel.Tolerations = instance.Spec.Tolerations
			}
			if sentinel.SecurityContext == nil {
				sentinel.SecurityContext = instance.Spec.SecurityContext
			}
			sentinel.PodAnnotations = lo.Assign(sentinel.PodAnnotations, instance.Spec.PodAnnotations)
		}
	} else {
		annotations["standalone"] = "true"
	}

	failover := &v1alpha1.Failover{
		ObjectMeta: metav1.ObjectMeta{
			Name:            instance.Name,
			Namespace:       instance.Namespace,
			Labels:          failoverbuilder.GenerateSelectorLabels(instance.Name),
			Annotations:     annotations,
			OwnerReferences: util.BuildOwnerReferences(instance),
		},
		Spec: v1alpha1.FailoverSpec{
			Image:          image,
			Replicas:       int32(*&instance.Spec.Replicas.ReplicasOfShard),
			Resources:      *instance.Spec.Resources,
			CustomConfigs:  instance.Spec.CustomConfigs,
			PodAnnotations: lo.Assign(instance.Spec.PodAnnotations),
			Exporter:       exporter,
			Access:         *access,
			Storage:        instance.Spec.Storage.DeepCopy(),

			Affinity:        instance.Spec.CustomAffinity,
			NodeSelector:    instance.Spec.NodeSelector,
			Tolerations:     instance.Spec.Tolerations,
			SecurityContext: instance.Spec.SecurityContext,
			Sentinel:        sentinel,
		},
	}

	// if storgae class specified, setup the default pvc size
	if failover.Spec.Storage != nil && failover.Spec.Storage.StorageClassName != nil &&
		(failover.Spec.Storage.Capacity == nil || failover.Spec.Storage.Capacity.IsZero()) {
		cap := resource.NewQuantity(instance.Spec.Resources.Limits.Memory().Value()*2, resource.BinarySI)
		instance.Spec.Storage.Capacity = cap
	}
	return failover, nil
}

func ShouldUpdateFailover(failover, newFailover *v1alpha1.Failover, logger logr.Logger) bool {
	if !reflect.DeepEqual(newFailover.Annotations, failover.Annotations) ||
		!reflect.DeepEqual(newFailover.Labels, failover.Labels) {
		return true
	}
	return !reflect.DeepEqual(failover.Spec, newFailover.Spec)
}
