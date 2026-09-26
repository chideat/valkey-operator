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
	"fmt"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/overwrite"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ClusterHeadlessSvcName(name string, i int) string {
	return fmt.Sprintf("%s-%d", name, i)
}

func ClusterNodeServiceName(clusterName string, shard, repl int) string {
	return fmt.Sprintf("%s-%s-%d-%d", builder.ResourcePrefix(core.ValkeyCluster), clusterName, shard, repl)
}

// GenerateHeadlessService creates a new headless service for the given Cluster.
func GenerateHeadlessService(inst types.ClusterInstance, index int) (*corev1.Service, error) {
	var (
		cluster   = inst.Definition()
		name      = ClusterHeadlessSvcName(cluster.GetName(), index)
		selectors = GenerateClusterStatefulSetSelectors(cluster.Name, index)
		labels    = GenerateClusterStatefulSetLabels(cluster.Name, index)
	)

	protocol, ptype := builder.IPFamilySpec(cluster.Spec.Access.IPFamilyPrefer)
	clientPort := corev1.ServicePort{Name: "client", Port: 6379}
	gossipPort := corev1.ServicePort{Name: "gossip", Port: 16379}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       cluster.Namespace,
			Labels:          labels,
			OwnerReferences: util.BuildOwnerReferences(cluster),
		},
		Spec: corev1.ServiceSpec{
			IPFamilies:     protocol,
			IPFamilyPolicy: ptype,
			Ports:          []corev1.ServicePort{clientPort, gossipPort},
			Selector:       selectors,
			ClusterIP:      corev1.ClusterIPNone,
		},
	}

	svc, problems, err := overwrite.Service(svc, cluster.Spec.Overwrites, core.OverwriteTargetHeadless)
	if err != nil {
		return nil, err
	}
	overwrite.Report(inst, svc.Name, problems)
	return svc, nil
}

func GenerateInstanceService(inst types.ClusterInstance) (*corev1.Service, error) {
	cluster := inst.Definition()
	selectors := GenerateClusterStatefulSetSelectors(cluster.Name, -1)
	labels := GenerateClusterStatefulSetLabels(cluster.Name, -1)
	// Set arch label, for identifying arch in prometheus, so wo can find metrics data for cluster only.
	labels[builder.ArchLabelKey] = string(core.ValkeyCluster)

	protocol, ptype := builder.IPFamilySpec(cluster.Spec.Access.IPFamilyPrefer)

	var ports []corev1.ServicePort
	clientPort := corev1.ServicePort{Name: "client", Port: 6379}
	ports = append(ports, clientPort)
	if cluster.Spec.Exporter != nil {
		ports = append(ports, corev1.ServicePort{Name: "metrics", Port: builder.ExporterPortNumber})
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          labels,
			Name:            cluster.Name,
			Namespace:       cluster.Namespace,
			OwnerReferences: util.BuildOwnerReferences(cluster),
		},
		Spec: corev1.ServiceSpec{
			IPFamilies:     protocol,
			IPFamilyPolicy: ptype,
			Ports:          ports,
			Selector:       selectors,
		},
	}

	svc, problems, err := overwrite.Service(svc, cluster.Spec.Overwrites, core.OverwriteTargetInstance)
	if err != nil {
		return nil, err
	}
	overwrite.Report(inst, svc.Name, problems)
	return svc, nil
}

func GenerateNodePortService(inst types.ClusterInstance, name string, labels map[string]string, port int32) (*corev1.Service, error) {
	cluster := inst.Definition()
	clientPort := corev1.ServicePort{Name: "client", Port: 6379, NodePort: port}
	selectorLabels := map[string]string{
		builder.PodNameLabelKey: name,
	}
	protocol, ptype := builder.IPFamilySpec(cluster.Spec.Access.IPFamilyPrefer)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          labels,
			Name:            name,
			Namespace:       cluster.Namespace,
			OwnerReferences: util.BuildOwnerReferences(cluster),
		},
		Spec: corev1.ServiceSpec{
			IPFamilies:     protocol,
			IPFamilyPolicy: ptype,
			Ports:          []corev1.ServicePort{clientPort},
			Selector:       selectorLabels,
			Type:           corev1.ServiceTypeNodePort,
		},
	}

	svc, problems, err := overwrite.Service(svc, cluster.Spec.Overwrites, core.OverwriteTargetPod)
	if err != nil {
		return nil, err
	}
	overwrite.Report(inst, svc.Name, problems)
	return svc, nil
}

func GeneratePodService(inst types.ClusterInstance, name string, typ corev1.ServiceType, annotations map[string]string) (*corev1.Service, error) {
	cluster := inst.Definition()
	clientPort := corev1.ServicePort{Name: "client", Port: 6379}
	gossipPort := corev1.ServicePort{Name: "gossip", Port: 16379}
	selectors := map[string]string{
		builder.PodNameLabelKey: name,
	}
	labels := GenerateClusterStatefulSetLabels(cluster.Name, -1)

	protocol, ptype := builder.IPFamilySpec(cluster.Spec.Access.IPFamilyPrefer)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          labels,
			Annotations:     annotations,
			Name:            name,
			Namespace:       cluster.Namespace,
			OwnerReferences: util.BuildOwnerReferences(cluster),
		},
		Spec: corev1.ServiceSpec{
			IPFamilies:     protocol,
			IPFamilyPolicy: ptype,
			Ports:          []corev1.ServicePort{clientPort, gossipPort},
			Selector:       selectors,
			Type:           typ,
		},
	}

	svc, problems, err := overwrite.Service(svc, cluster.Spec.Overwrites, core.OverwriteTargetPod)
	if err != nil {
		return nil, err
	}
	overwrite.Report(inst, svc.Name, problems)
	return svc, nil
}
