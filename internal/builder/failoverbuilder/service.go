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
*/package failoverbuilder

import (
	"fmt"
	"strconv"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func NodePortServiceName(rf *v1alpha1.Failover, index int) string {
	name := FailoverStatefulSetName(rf.Name)
	return fmt.Sprintf("%s-%d", name, index)
}

func RWServiceName(failoverName string) string {
	return fmt.Sprintf("rfr-%s-read-write", failoverName)
}

func ROServiceName(failoverName string) string {
	return fmt.Sprintf("rfr-%s-read-only", failoverName)
}

func GenerateReadWriteService(rf *v1alpha1.Failover) *corev1.Service {
	selectors := GenerateSelectorLabels(rf.Name)
	selectors[builder.RoleLabelKey] = string(core.NodeRoleMaster)
	labels := GenerateCommonLabels(rf.Name)

	svcName := RWServiceName(rf.Name)
	ptype := corev1.IPFamilyPolicySingleStack
	protocol := []corev1.IPFamily{}
	if rf.Spec.Access.IPFamilyPrefer == corev1.IPv6Protocol {
		protocol = append(protocol, corev1.IPv6Protocol)
	} else {
		protocol = append(protocol, corev1.IPv4Protocol)
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            svcName,
			Namespace:       rf.Namespace,
			Labels:          labels,
			OwnerReferences: util.BuildOwnerReferences(rf),
			Annotations:     rf.Spec.Access.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:           rf.Spec.Access.ServiceType,
			IPFamilies:     protocol,
			IPFamilyPolicy: &ptype,
			Ports: []corev1.ServicePort{
				{
					Port:       builder.DefaultValkeyServerPort,
					TargetPort: intstr.FromInt(builder.DefaultValkeyServerPort),
					Protocol:   corev1.ProtocolTCP,
					Name:       "server",
				},
			},
			Selector: selectors,
		},
	}
}

func GenerateReadonlyService(rf *v1alpha1.Failover) *corev1.Service {
	selectors := GenerateSelectorLabels(rf.Name)
	selectors[builder.RoleLabelKey] = string(core.NodeRoleReplica)
	labels := GenerateCommonLabels(rf.Name, selectors)

	ptype := corev1.IPFamilyPolicySingleStack
	protocol := []corev1.IPFamily{}
	if rf.Spec.Access.IPFamilyPrefer == corev1.IPv6Protocol {
		protocol = append(protocol, corev1.IPv6Protocol)
	} else {
		protocol = append(protocol, corev1.IPv4Protocol)
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            ROServiceName(rf.Name),
			Namespace:       rf.Namespace,
			Labels:          labels,
			OwnerReferences: util.BuildOwnerReferences(rf),
			Annotations:     rf.Spec.Access.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:           rf.Spec.Access.ServiceType,
			IPFamilies:     protocol,
			IPFamilyPolicy: &ptype,
			Ports: []corev1.ServicePort{
				{
					Port:       builder.DefaultValkeyServerPort,
					TargetPort: intstr.FromInt(builder.DefaultValkeyServerPort),
					Protocol:   corev1.ProtocolTCP,
					Name:       "server",
				},
			},
			Selector: selectors,
		},
	}
}

func GenerateExporterService(rf *v1alpha1.Failover) *corev1.Service {
	name := FailoverStatefulSetName(rf.Name)
	namespace := rf.Namespace
	selectors := GenerateSelectorLabels(rf.Name)
	labels := GenerateCommonLabels(rf.Name)
	labels[builder.ArchLabelKey] = string(core.ValkeyFailover)

	defaultAnnotations := map[string]string{
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   "http",
		"prometheus.io/path":   "/metrics",
	}
	annotations := lo.Assign(defaultAnnotations, rf.Spec.Access.Annotations)
	ptype := corev1.IPFamilyPolicySingleStack
	protocol := []corev1.IPFamily{}
	if rf.Spec.Access.IPFamilyPrefer == corev1.IPv6Protocol {
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
			OwnerReferences: util.BuildOwnerReferences(rf),
		},
		Spec: corev1.ServiceSpec{
			IPFamilies:     protocol,
			IPFamilyPolicy: &ptype,
			Type:           corev1.ServiceTypeClusterIP,
			ClusterIP:      corev1.ClusterIPNone,
			Selector:       selectors,
			Ports: []corev1.ServicePort{
				{
					Name:       "metrics",
					Port:       builder.ExporterPortNumber,
					TargetPort: intstr.FromInt(builder.ExporterPortNumber),
					Protocol:   "TCP",
				},
			},
		},
	}
}

// GeneratePodService returns a new Service for the given ValkeyFailover and index, with the configed service type
func GeneratePodService(rf *v1alpha1.Failover, index int) *corev1.Service {
	return GeneratePodNodePortService(rf, index, 0)
}

func GeneratePodNodePortService(rf *v1alpha1.Failover, index int, nodePort int32) *corev1.Service {
	ptype := corev1.IPFamilyPolicySingleStack
	protocol := []corev1.IPFamily{}
	if rf.Spec.Access.IPFamilyPrefer == corev1.IPv6Protocol {
		protocol = append(protocol, corev1.IPv6Protocol)
	} else {
		protocol = append(protocol, corev1.IPv4Protocol)
	}
	selectors := GenerateSelectorLabels(rf.Name)
	labels := lo.Assign(GenerateCommonLabels(rf.Name))
	selectors[builder.PodNameLabelKey] = FailoverStatefulSetName(rf.Name) + "-" + strconv.Itoa(index)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            NodePortServiceName(rf, index),
			Namespace:       rf.GetNamespace(),
			Labels:          labels,
			Annotations:     rf.Spec.Access.Annotations,
			OwnerReferences: util.BuildOwnerReferences(rf),
		},
		Spec: corev1.ServiceSpec{
			Type:           rf.Spec.Access.ServiceType,
			IPFamilies:     protocol,
			IPFamilyPolicy: &ptype,
			Ports: []corev1.ServicePort{
				{
					Port:       builder.DefaultValkeyServerPort,
					Protocol:   corev1.ProtocolTCP,
					Name:       "client",
					TargetPort: intstr.FromInt(6379),
					NodePort:   nodePort,
				},
			},
			Selector: selectors,
		},
	}
}
