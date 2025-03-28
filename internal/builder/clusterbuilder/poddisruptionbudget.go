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
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/types"
)

func GeneratePodDisruptionBudget(cluster types.ClusterInstance, index int) *policyv1.PodDisruptionBudget {
	var (
		name      = ClusterStatefulSetName(cluster.GetName(), index)
		selectors = GenerateClusterStatefulSetSelectors(cluster.GetName(), index)
		labels    = GenerateClusterStatefulSetLabels(cluster.GetName(), index)
	)

	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          labels,
			Name:            name,
			Namespace:       cluster.GetNamespace(),
			OwnerReferences: util.BuildOwnerReferences(cluster),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: ptr.To(intstr.FromInt(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectors,
			},
		},
	}
}
