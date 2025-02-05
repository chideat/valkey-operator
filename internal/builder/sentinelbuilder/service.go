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
*/package sentinelbuilder

import (
	"fmt"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func SentinelHeadlessServiceName(sentinelName string) string {
	return fmt.Sprintf("%s-%s", builder.ResourcePrefix(core.ValkeySentinel), sentinelName)
}

func SentinelPodServiceName(sentinelName string, i int) string {
	return fmt.Sprintf("%s-%d", SentinelStatefulSetName(sentinelName), i)
}

func GenerateSentinelHeadlessService(inst *v1alpha1.Sentinel) *corev1.Service {
	name := SentinelHeadlessServiceName(inst.Name)
	namespace := inst.Namespace

	selectors := GenerateSelectorLabels(inst.Name)
	labels := GenerateCommonLabels(inst.Name)
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
			Annotations:     inst.Spec.Access.Annotations,
			OwnerReferences: util.BuildOwnerReferences(inst),
		},
		Spec: corev1.ServiceSpec{
			IPFamilies:     protocol,
			IPFamilyPolicy: &ptype,
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
}

// GeneratePodService returns a new Service for the given ValkeyFailover and index, with the configed service type
func GeneratePodService(sen *v1alpha1.Sentinel, index int) *corev1.Service {
	return GeneratePodNodePortService(sen, index, 0)
}

func GeneratePodNodePortService(sen *v1alpha1.Sentinel, index int, nodePort int32) *corev1.Service {
	var (
		name     = SentinelPodServiceName(sen.Name, index)
		protocol = []corev1.IPFamily{}
	)
	if sen.Spec.Access.IPFamilyPrefer == corev1.IPv6Protocol {
		protocol = append(protocol, corev1.IPv6Protocol)
	} else {
		protocol = append(protocol, corev1.IPv4Protocol)
	}
	labels := GenerateCommonLabels(sen.Name)
	selectors := map[string]string{
		builder.PodNameLabelKey: name,
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       sen.GetNamespace(),
			Labels:          labels,
			Annotations:     sen.Spec.Access.Annotations,
			OwnerReferences: util.BuildOwnerReferences(sen),
		},
		Spec: corev1.ServiceSpec{
			IPFamilies:     protocol,
			IPFamilyPolicy: ptr.To(corev1.IPFamilyPolicyPreferDualStack),
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
}
