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

package sentinelbuilder

import (
	"github.com/chideat/valkey-operator/internal/builder/overwrite"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/types"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func GeneratePodDisruptionBudget(inst types.SentinelInstance) (*policyv1.PodDisruptionBudget, error) {
	sen := inst.Definition()
	maxUnavailable := intstr.FromInt(int(sen.Spec.Replicas) / 2)
	selectors := GenerateSelectorLabels(sen.Name)
	labels := GenerateCommonLabels(sen.Name)

	name := SentinelStatefulSetName(sen.Name)
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          labels,
			Name:            name,
			Namespace:       sen.GetNamespace(),
			OwnerReferences: util.BuildOwnerReferences(sen),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: &maxUnavailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectors,
			},
		},
	}

	pdb, problems, err := overwrite.PodDisruptionBudget(pdb, sen.Spec.Overwrites)
	if err != nil {
		return nil, err
	}
	overwrite.Report(inst, pdb.Name, problems)
	return pdb, nil
}
