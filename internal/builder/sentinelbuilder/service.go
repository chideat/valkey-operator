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
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	ValkeyArchRoleSEN     = "sentinel"
	ValkeySentinelSVCPort = 26379
)

func NewSentinelServiceForCR(inst *v1alpha1.Sentinel, selectors map[string]string) *corev1.Service {
	var (
		namespace = inst.Namespace
		name      = GetSentinelServiceName(inst.Name)
		ptype     = corev1.IPFamilyPolicySingleStack
		protocol  = []corev1.IPFamily{}
	)
	if inst.Spec.Access.IPFamilyPrefer == corev1.IPv6Protocol {
		protocol = append(protocol, corev1.IPv6Protocol)
	} else {
		protocol = append(protocol, corev1.IPv4Protocol)
	}

	selectorLabels := lo.Assign(GenerateSelectorLabels(ValkeyArchRoleSEN, inst.Name), GetCommonLabels(inst.Name))
	if len(selectors) > 0 {
		selectorLabels = lo.Assign(selectors, GenerateSelectorLabels(ValkeyArchRoleSEN, inst.Name))
	}
	// NOTE: remove this label for compatibility for old instances
	// TODO: remove this in 3.22
	delete(selectorLabels, "valkeysentinels.databases.spotahome.com/name")
	labels := GetCommonLabels(inst.Name, GenerateSelectorLabels(ValkeyArchRoleSEN, inst.Name), selectorLabels)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          labels,
			Annotations:     inst.Spec.Access.Annotations,
			OwnerReferences: util.BuildOwnerReferences(inst),
		},
		Spec: corev1.ServiceSpec{
			Type:           inst.Spec.Access.ServiceType,
			IPFamilyPolicy: &ptype,
			IPFamilies:     protocol,
			Selector:       selectorLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       SentinelContainerPortName,
					Port:       ValkeySentinelSVCPort,
					TargetPort: intstr.FromInt(ValkeySentinelSVCPort),
					Protocol:   "TCP",
				},
			},
		},
	}
}

func NewSentinelHeadlessServiceForCR(inst *v1alpha1.Sentinel, selectors map[string]string) *corev1.Service {
	name := GetSentinelHeadlessServiceName(inst.Name)
	namespace := inst.Namespace
	selectorLabels := GenerateSelectorLabels(ValkeyArchRoleSEN, inst.Name)
	labels := GetCommonLabels(inst.Name, selectors, selectorLabels)
	labels[builder.LabelValkeyArch] = ValkeyArchRoleSEN
	annotations := inst.Spec.Access.Annotations
	ptype := corev1.IPFamilyPolicySingleStack
	protocol := []corev1.IPFamily{}
	if inst.Spec.Access.IPFamilyPrefer == corev1.IPv6Protocol {
		protocol = append(protocol, corev1.IPv6Protocol)
	} else {
		protocol = append(protocol, corev1.IPv4Protocol)
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          labels,
			Annotations:     annotations,
			OwnerReferences: util.BuildOwnerReferences(inst),
		},
		Spec: corev1.ServiceSpec{
			IPFamilies:     protocol,
			IPFamilyPolicy: &ptype,
			Type:           corev1.ServiceTypeClusterIP,
			ClusterIP:      corev1.ClusterIPNone,
			Selector:       selectorLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       SentinelContainerPortName,
					Port:       ValkeySentinelSVCPort,
					TargetPort: intstr.FromInt(ValkeySentinelSVCPort),
					Protocol:   "TCP",
				},
			},
		},
	}
}

// NewPodService returns a new Service for the given ValkeyFailover and index, with the configed service type
func NewPodService(sen *v1alpha1.Sentinel, index int, selectors map[string]string) *corev1.Service {
	return NewPodNodePortService(sen, index, selectors, 0)
}

func NewPodNodePortService(sen *v1alpha1.Sentinel, index int, selectors map[string]string, nodePort int32) *corev1.Service {
	var (
		namespace = sen.Namespace
		name      = GetSentinelNodeServiceName(sen.Name, index)
		ptype     = corev1.IPFamilyPolicySingleStack
		protocol  = []corev1.IPFamily{}
	)
	if sen.Spec.Access.IPFamilyPrefer == corev1.IPv6Protocol {
		protocol = append(protocol, corev1.IPv6Protocol)
	} else {
		protocol = append(protocol, corev1.IPv4Protocol)
	}
	labels := lo.Assign(GetCommonLabels(sen.Name), selectors, GenerateSelectorLabels(ValkeyArchRoleSEN, sen.Name))
	selectorLabels := map[string]string{
		builder.PodNameLabelKey: name,
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          labels,
			Annotations:     sen.Spec.Access.Annotations,
			OwnerReferences: util.BuildOwnerReferences(sen),
		},
		Spec: corev1.ServiceSpec{
			IPFamilies:     protocol,
			IPFamilyPolicy: &ptype,
			Type:           sen.Spec.Access.ServiceType,
			Ports: []corev1.ServicePort{
				{
					Port:       ValkeySentinelSVCPort,
					TargetPort: intstr.FromInt(ValkeySentinelSVCPort),
					Protocol:   corev1.ProtocolTCP,
					Name:       SentinelContainerPortName,
					NodePort:   nodePort,
				},
			},
			Selector: selectorLabels,
		},
	}
}
