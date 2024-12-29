/*
Copyright 2023 The RedisOperator Authors.

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
	"strconv"

	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	RedisArchRoleRedis = "redis"
	RedisArchRoleSEN   = "sentinel"
	RedisRoleMaster    = "master"
	RedisRoleReplica   = "slave"
	RedisRoleLabel     = "redis.middleware.alauda.io/role"
	RedisSVCPort       = 6379
	RedisSVCPortName   = "redis"
)

func NewRWSvcForCR(rf *v1alpha1.Failover) *corev1.Service {
	selectorLabels := GenerateSelectorLabels(RedisArchRoleRedis, rf.Name)
	selectorLabels[RedisRoleLabel] = RedisRoleMaster
	labels := GetCommonLabels(rf.Name, selectorLabels)
	svcName := GetRedisRWServiceName(rf.Name)
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
					Port:       RedisSVCPort,
					TargetPort: intstr.FromInt(RedisSVCPort),
					Protocol:   corev1.ProtocolTCP,
					Name:       RedisSVCPortName,
				},
			},
			Selector: selectorLabels,
		},
	}
}

func NewReadOnlyForCR(rf *v1alpha1.Failover) *corev1.Service {
	selectorLabels := GenerateSelectorLabels(RedisArchRoleRedis, rf.Name)
	selectorLabels[RedisRoleLabel] = RedisRoleReplica
	labels := GetCommonLabels(rf.Name, selectorLabels)
	svcName := GetRedisROServiceName(rf.Name)
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
					Port:       RedisSVCPort,
					TargetPort: intstr.FromInt(RedisSVCPort),
					Protocol:   corev1.ProtocolTCP,
					Name:       RedisSVCPortName,
				},
			},
			Selector: selectorLabels,
		},
	}
}

func NewExporterServiceForCR(rf *v1alpha1.Failover, selectors map[string]string) *corev1.Service {
	name := GetFailoverStatefulSetName(rf.Name)
	namespace := rf.Namespace
	selectorLabels := GenerateSelectorLabels(RedisArchRoleRedis, rf.Name)
	labels := GetCommonLabels(rf.Name, selectors, selectorLabels)
	labels[builder.LabelRedisArch] = RedisArchRoleSEN
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
			Selector:       selectorLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http-metrics",
					Port:       9121,
					TargetPort: intstr.FromInt(9121),
					Protocol:   "TCP",
				},
			},
		},
	}
}

// NewPodService returns a new Service for the given RedisFailover and index, with the configed service type
func NewPodService(rf *v1alpha1.Failover, index int, selectors map[string]string) *corev1.Service {
	return NewPodNodePortService(rf, index, selectors, 0)
}

func NewPodNodePortService(rf *v1alpha1.Failover, index int, selectors map[string]string, nodePort int32) *corev1.Service {
	namespace := rf.Namespace
	ptype := corev1.IPFamilyPolicySingleStack
	protocol := []corev1.IPFamily{}
	if rf.Spec.Access.IPFamilyPrefer == corev1.IPv6Protocol {
		protocol = append(protocol, corev1.IPv6Protocol)
	} else {
		protocol = append(protocol, corev1.IPv4Protocol)
	}
	labels := lo.Assign(GetCommonLabels(rf.Name), selectors, GenerateSelectorLabels(RedisArchRoleRedis, rf.Name))
	selectorLabels := map[string]string{
		builder.PodNameLabelKey: GetFailoverStatefulSetName(rf.Name) + "-" + strconv.Itoa(index),
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            GetFailoverNodePortServiceName(rf, index),
			Namespace:       namespace,
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
					Port:       6379,
					Protocol:   corev1.ProtocolTCP,
					Name:       "client",
					TargetPort: intstr.FromInt(6379),
					NodePort:   nodePort,
				},
			},
			Selector: selectorLabels,
		},
	}
}
