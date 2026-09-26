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
	"fmt"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/overwrite"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func SentinelHeadlessServiceName(sentinelName string) string {
	return fmt.Sprintf("%s-%s", builder.ResourcePrefix(core.ValkeySentinel), sentinelName)
}

func SentinelPodServiceName(sentinelName string, i int) string {
	return fmt.Sprintf("%s-%d", SentinelStatefulSetName(sentinelName), i)
}

func GenerateSentinelHeadlessService(inst types.SentinelInstance) (*corev1.Service, error) {
	sen := inst.Definition()
	name := SentinelHeadlessServiceName(sen.Name)
	namespace := sen.Namespace

	selectors := GenerateSelectorLabels(sen.Name)
	labels := GenerateCommonLabels(sen.Name)
	protocol, ptype := builder.IPFamilySpec(sen.Spec.Access.IPFamilyPrefer)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          labels,
			Annotations:     sen.Spec.Access.Annotations,
			OwnerReferences: util.BuildOwnerReferences(sen),
		},
		Spec: corev1.ServiceSpec{
			IPFamilies:     protocol,
			IPFamilyPolicy: ptype,
			Type:           corev1.ServiceTypeClusterIP,
			ClusterIP:      corev1.ClusterIPNone,
			Selector:       selectors,
			Ports: []corev1.ServicePort{
				{
					Name:       SentinelContainerPortName,
					Port:       builder.DefaultValkeySentinelPort,
					TargetPort: intstr.FromInt(builder.DefaultValkeySentinelPort),
					Protocol:   "TCP",
				},
			},
		},
	}

	svc, problems, err := overwrite.Service(svc, sen.Spec.Overwrites, core.OverwriteTargetHeadless)
	if err != nil {
		return nil, err
	}
	overwrite.Report(inst, svc.Name, problems)
	return svc, nil
}

// GeneratePodService returns a new Service for the given ValkeyFailover and index, with the configed service type
func GeneratePodService(inst types.SentinelInstance, index int) (*corev1.Service, error) {
	return GeneratePodNodePortService(inst, index, 0)
}

func GeneratePodNodePortService(inst types.SentinelInstance, index int, nodePort int32) (*corev1.Service, error) {
	var (
		sen             = inst.Definition()
		name            = SentinelPodServiceName(sen.Name, index)
		protocol, ptype = builder.IPFamilySpec(sen.Spec.Access.IPFamilyPrefer)
	)
	labels := GenerateCommonLabels(sen.Name)
	selectors := map[string]string{
		builder.PodNameLabelKey: name,
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       sen.GetNamespace(),
			Labels:          labels,
			Annotations:     sen.Spec.Access.Annotations,
			OwnerReferences: util.BuildOwnerReferences(sen),
		},
		Spec: corev1.ServiceSpec{
			IPFamilies:     protocol,
			IPFamilyPolicy: ptype,
			Type:           sen.Spec.Access.ServiceType,
			Ports: []corev1.ServicePort{
				{
					Port:       builder.DefaultValkeySentinelPort,
					TargetPort: intstr.FromInt(builder.DefaultValkeySentinelPort),
					Protocol:   corev1.ProtocolTCP,
					Name:       SentinelContainerPortName,
					NodePort:   nodePort,
				},
			},
			Selector: selectors,
		},
	}

	svc, problems, err := overwrite.Service(svc, sen.Spec.Overwrites, core.OverwriteTargetPod)
	if err != nil {
		return nil, err
	}
	overwrite.Report(inst, svc.Name, problems)
	return svc, nil
}
