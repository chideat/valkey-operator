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
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	v1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/overwrite"
	"github.com/chideat/valkey-operator/internal/testutil"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

func TestClusterHeadlessSvcName(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		index       int
		expected    string
	}{
		{
			name:        "Basic test",
			clusterName: "test-cluster",
			index:       0,
			expected:    "test-cluster-0",
		},
		{
			name:        "With positive index",
			clusterName: "valkey-cluster",
			index:       3,
			expected:    "valkey-cluster-3",
		},
		{
			name:        "With negative index",
			clusterName: "redis-cluster",
			index:       -1,
			expected:    "redis-cluster--1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClusterHeadlessSvcName(tt.clusterName, tt.index)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClusterNodeServiceName(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		shard       int
		repl        int
		expected    string
	}{
		{
			name:        "Basic test",
			clusterName: "test-cluster",
			shard:       0,
			repl:        0,
			expected:    "drc-test-cluster-0-0",
		},
		{
			name:        "With positive shard and repl",
			clusterName: "valkey-cluster",
			shard:       3,
			repl:        2,
			expected:    "drc-valkey-cluster-3-2",
		},
		{
			name:        "With negative values",
			clusterName: "redis-cluster",
			shard:       -1,
			repl:        -1,
			expected:    "drc-redis-cluster--1--1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClusterNodeServiceName(tt.clusterName, tt.shard, tt.repl)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateHeadlessService(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		namespace   string
		ipFamily    corev1.IPFamily
		index       int
	}{
		{
			name:        "IPv4 Headless Service",
			clusterName: "test-cluster",
			namespace:   "default",
			ipFamily:    corev1.IPv4Protocol,
			index:       0,
		},
		{
			name:        "IPv6 Headless Service",
			clusterName: "test-cluster",
			namespace:   "test-namespace",
			ipFamily:    corev1.IPv6Protocol,
			index:       1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a cluster with the desired IP family preference
			cluster := &v1alpha1.Cluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Cluster",
					APIVersion: "valkey.chideat.com/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.clusterName,
					Namespace: tt.namespace,
					UID:       k8stypes.UID(tt.clusterName + "-uid"),
				},
				Spec: v1alpha1.ClusterSpec{
					Access: core.InstanceAccess{
						IPFamilyPrefer: tt.ipFamily,
					},
				},
			}

			// Generate the headless service
			svc, err := GenerateHeadlessService(testutil.NewFakeClusterInstance(cluster), tt.index)

			// Verify the service
			require.NoError(t, err)
			require.NotNil(t, svc)

			// Check basic properties
			expectedName := ClusterHeadlessSvcName(tt.clusterName, tt.index)
			assert.Equal(t, expectedName, svc.Name)
			assert.Equal(t, tt.namespace, svc.Namespace)

			// Check that the service is headless
			assert.Equal(t, corev1.ClusterIPNone, svc.Spec.ClusterIP)

			// Check IP Family settings
			assert.NotEmpty(t, svc.Spec.IPFamilies)
			assert.Equal(t, tt.ipFamily, svc.Spec.IPFamilies[0])
			assert.Equal(t, corev1.IPFamilyPolicySingleStack, *svc.Spec.IPFamilyPolicy)

			// Check ports
			assert.Len(t, svc.Spec.Ports, 2)
			var clientPort, gossipPort *corev1.ServicePort
			for i := range svc.Spec.Ports {
				port := &svc.Spec.Ports[i]
				if port.Name == "client" {
					clientPort = port
				} else if port.Name == "gossip" {
					gossipPort = port
				}
			}

			require.NotNil(t, clientPort, "Client port should exist")
			require.NotNil(t, gossipPort, "Gossip port should exist")
			assert.Equal(t, int32(6379), clientPort.Port)
			assert.Equal(t, int32(16379), gossipPort.Port)

			// Check selectors
			expectedSelectors := GenerateClusterStatefulSetSelectors(tt.clusterName, tt.index)
			assert.Equal(t, expectedSelectors, svc.Spec.Selector)

			// Check labels
			expectedLabels := GenerateClusterStatefulSetLabels(tt.clusterName, tt.index)
			assert.Equal(t, expectedLabels, svc.Labels)

			// Verify owner references are set
			assert.NotEmpty(t, svc.OwnerReferences)
			assert.Equal(t, tt.clusterName, svc.OwnerReferences[0].Name)
		})
	}
}

func TestGenerateInstanceService(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		namespace   string
		ipFamily    corev1.IPFamily
		hasExporter bool
	}{
		{
			name:        "IPv4 Instance Service without exporter",
			clusterName: "test-cluster",
			namespace:   "default",
			ipFamily:    corev1.IPv4Protocol,
			hasExporter: false,
		},
		{
			name:        "IPv6 Instance Service with exporter",
			clusterName: "test-cluster",
			namespace:   "test-namespace",
			ipFamily:    corev1.IPv6Protocol,
			hasExporter: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a cluster with the desired IP family preference
			cluster := &v1alpha1.Cluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Cluster",
					APIVersion: "valkey.chideat.com/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.clusterName,
					Namespace: tt.namespace,
					UID:       k8stypes.UID(tt.clusterName + "-uid"),
				},
				Spec: v1alpha1.ClusterSpec{
					Access: core.InstanceAccess{
						IPFamilyPrefer: tt.ipFamily,
					},
				},
			}

			// Add exporter if needed
			if tt.hasExporter {
				cluster.Spec.Exporter = &core.Exporter{
					Image: "exporter:latest",
				}
			}

			// Generate the instance service
			svc, err := GenerateInstanceService(testutil.NewFakeClusterInstance(cluster))

			// Verify the service
			require.NoError(t, err)
			require.NotNil(t, svc)

			// Check basic properties
			assert.Equal(t, tt.clusterName, svc.Name)
			assert.Equal(t, tt.namespace, svc.Namespace)

			// Check IP Family settings
			assert.NotEmpty(t, svc.Spec.IPFamilies)
			assert.Equal(t, tt.ipFamily, svc.Spec.IPFamilies[0])
			assert.Equal(t, corev1.IPFamilyPolicySingleStack, *svc.Spec.IPFamilyPolicy)

			// Check ports
			if tt.hasExporter {
				assert.Len(t, svc.Spec.Ports, 2)
				var metricsPort *corev1.ServicePort
				for i := range svc.Spec.Ports {
					if svc.Spec.Ports[i].Name == "metrics" {
						metricsPort = &svc.Spec.Ports[i]
						break
					}
				}
				require.NotNil(t, metricsPort, "Metrics port should exist when exporter is specified")
				assert.Equal(t, builder.ExporterPortNumber, metricsPort.Port)
			} else {
				assert.Len(t, svc.Spec.Ports, 1)
			}

			// Check client port
			var clientPort *corev1.ServicePort
			for i := range svc.Spec.Ports {
				if svc.Spec.Ports[i].Name == "client" {
					clientPort = &svc.Spec.Ports[i]
					break
				}
			}
			require.NotNil(t, clientPort, "Client port should exist")
			assert.Equal(t, int32(6379), clientPort.Port)

			// Check selectors
			expectedSelectors := GenerateClusterStatefulSetSelectors(tt.clusterName, -1)
			assert.Equal(t, expectedSelectors, svc.Spec.Selector)

			// Check labels
			labels := svc.Labels
			assert.Equal(t, tt.clusterName, labels[builder.InstanceNameLabelKey])
			assert.Equal(t, string(core.ValkeyCluster), labels[builder.ArchLabelKey])

			// Verify owner references are set
			assert.NotEmpty(t, svc.OwnerReferences)
			assert.Equal(t, tt.clusterName, svc.OwnerReferences[0].Name)
		})
	}
}

func TestGenerateNodePortService(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		namespace   string
		ipFamily    corev1.IPFamily
		nodeName    string
		nodePort    int32
	}{
		{
			name:        "IPv4 NodePort Service",
			clusterName: "test-cluster",
			namespace:   "default",
			ipFamily:    corev1.IPv4Protocol,
			nodeName:    "valkey-node-0-0",
			nodePort:    30001,
		},
		{
			name:        "IPv6 NodePort Service",
			clusterName: "test-cluster",
			namespace:   "test-namespace",
			ipFamily:    corev1.IPv6Protocol,
			nodeName:    "valkey-node-1-1",
			nodePort:    30002,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a cluster with the desired IP family preference
			cluster := &v1alpha1.Cluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Cluster",
					APIVersion: "valkey.chideat.com/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.clusterName,
					Namespace: tt.namespace,
					UID:       k8stypes.UID(tt.clusterName + "-uid"),
				},
				Spec: v1alpha1.ClusterSpec{
					Access: core.InstanceAccess{
						IPFamilyPrefer: tt.ipFamily,
					},
				},
			}

			// Create labels for the service
			labels := map[string]string{
				"test-label": "test-value",
			}

			// Generate the NodePort service
			svc, err := GenerateNodePortService(testutil.NewFakeClusterInstance(cluster), tt.nodeName, labels, tt.nodePort)
			require.NoError(t, err)

			// Verify the service
			require.NotNil(t, svc)

			// Check basic properties
			assert.Equal(t, tt.nodeName, svc.Name)
			assert.Equal(t, tt.namespace, svc.Namespace)

			// Check service type
			assert.Equal(t, corev1.ServiceTypeNodePort, svc.Spec.Type)

			// Check IP Family settings
			assert.NotEmpty(t, svc.Spec.IPFamilies)
			assert.Equal(t, tt.ipFamily, svc.Spec.IPFamilies[0])
			assert.Equal(t, corev1.IPFamilyPolicySingleStack, *svc.Spec.IPFamilyPolicy)

			// Check ports
			assert.Len(t, svc.Spec.Ports, 1)
			port := svc.Spec.Ports[0]
			assert.Equal(t, "client", port.Name)
			assert.Equal(t, int32(6379), port.Port)
			assert.Equal(t, tt.nodePort, port.NodePort)

			// Check selectors
			assert.Equal(t, tt.nodeName, svc.Spec.Selector[builder.PodNameLabelKey])

			// Check labels
			assert.Equal(t, "test-value", svc.Labels["test-label"])

			// Verify owner references are set
			assert.NotEmpty(t, svc.OwnerReferences)
			assert.Equal(t, tt.clusterName, svc.OwnerReferences[0].Name)
		})
	}
}

func TestGeneratePodService(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		namespace   string
		ipFamily    corev1.IPFamily
		podName     string
		serviceType corev1.ServiceType
		annotations map[string]string
	}{
		{
			name:        "ClusterIP Pod Service without annotations",
			clusterName: "test-cluster",
			namespace:   "default",
			ipFamily:    corev1.IPv4Protocol,
			podName:     "valkey-pod-0",
			serviceType: corev1.ServiceTypeClusterIP,
			annotations: nil,
		},
		{
			name:        "LoadBalancer Pod Service with annotations",
			clusterName: "test-cluster",
			namespace:   "test-namespace",
			ipFamily:    corev1.IPv6Protocol,
			podName:     "valkey-pod-1",
			serviceType: corev1.ServiceTypeLoadBalancer,
			annotations: map[string]string{
				"service.beta.kubernetes.io/aws-load-balancer-type": "nlb",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a cluster with the desired IP family preference
			cluster := &v1alpha1.Cluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Cluster",
					APIVersion: "valkey.chideat.com/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.clusterName,
					Namespace: tt.namespace,
					UID:       k8stypes.UID(tt.clusterName + "-uid"),
				},
				Spec: v1alpha1.ClusterSpec{
					Access: core.InstanceAccess{
						IPFamilyPrefer: tt.ipFamily,
					},
				},
			}

			// Generate the Pod service
			svc, err := GeneratePodService(testutil.NewFakeClusterInstance(cluster), tt.podName, tt.serviceType, tt.annotations)
			require.NoError(t, err)

			// Verify the service
			require.NotNil(t, svc)

			// Check basic properties
			assert.Equal(t, tt.podName, svc.Name)
			assert.Equal(t, tt.namespace, svc.Namespace)

			// Check service type
			assert.Equal(t, tt.serviceType, svc.Spec.Type)

			// Check IP Family settings
			assert.NotEmpty(t, svc.Spec.IPFamilies)
			assert.Equal(t, tt.ipFamily, svc.Spec.IPFamilies[0])
			assert.Equal(t, corev1.IPFamilyPolicySingleStack, *svc.Spec.IPFamilyPolicy)

			// Check ports
			assert.Len(t, svc.Spec.Ports, 2)
			var clientPort, gossipPort *corev1.ServicePort
			for i := range svc.Spec.Ports {
				port := &svc.Spec.Ports[i]
				if port.Name == "client" {
					clientPort = port
				} else if port.Name == "gossip" {
					gossipPort = port
				}
			}

			require.NotNil(t, clientPort, "Client port should exist")
			require.NotNil(t, gossipPort, "Gossip port should exist")
			assert.Equal(t, int32(6379), clientPort.Port)
			assert.Equal(t, int32(16379), gossipPort.Port)

			// Check selectors
			assert.Equal(t, tt.podName, svc.Spec.Selector[builder.PodNameLabelKey])

			// Check annotations
			if tt.annotations != nil {
				for k, v := range tt.annotations {
					assert.Equal(t, v, svc.Annotations[k])
				}
			}

			// Check labels
			labels := svc.Labels
			assert.Equal(t, tt.clusterName, labels[builder.InstanceNameLabelKey])

			// Verify owner references are set
			assert.NotEmpty(t, svc.OwnerReferences)
			assert.Equal(t, tt.clusterName, svc.OwnerReferences[0].Name)
		})
	}
}

// Every generator in this file must render three states, not two. The tables
// above parameterize only IPv4 and IPv6 — the two the old conditional got
// right — which is how an unset preference silently rendered as IPv4 and made
// every Service invalid on a single-stack IPv6 cluster.
func TestServiceIPFamilies(t *testing.T) {
	newCluster := func(family corev1.IPFamily) *v1alpha1.Cluster {
		return &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
				UID:       k8stypes.UID("test-cluster-uid"),
			},
			Spec: v1alpha1.ClusterSpec{
				Access: core.InstanceAccess{IPFamilyPrefer: family},
			},
		}
	}

	generators := map[string]func(types.ClusterInstance) (*corev1.Service, error){
		"GenerateHeadlessService": func(c types.ClusterInstance) (*corev1.Service, error) {
			return GenerateHeadlessService(c, 0)
		},
		"GenerateInstanceService": GenerateInstanceService,
		"GenerateNodePortService": func(c types.ClusterInstance) (*corev1.Service, error) {
			return GenerateNodePortService(c, "test-nodeport", map[string]string{}, 30000)
		},
		"GeneratePodService": func(c types.ClusterInstance) (*corev1.Service, error) {
			return GeneratePodService(c, "test-pod-svc", corev1.ServiceTypeClusterIP, nil)
		},
	}
	for name, generate := range generators {
		t.Run(name, func(t *testing.T) {
			assertIPFamilyStates(t, func(family corev1.IPFamily) *corev1.Service {
				svc, err := generate(testutil.NewFakeClusterInstance(newCluster(family)))
				require.NoError(t, err)
				return svc
			})
		})
	}
}

// assertIPFamilyStates checks the three-state contract shared by every service
// builder: an explicit preference is pinned single-stack, and an unset one
// leaves both fields nil so the API server assigns the cluster's own families.
func assertIPFamilyStates(t *testing.T, generate func(corev1.IPFamily) *corev1.Service) {
	t.Helper()

	t.Run("unset defers to the cluster", func(t *testing.T) {
		svc := generate("")
		require.NotNil(t, svc)
		assert.Nil(t, svc.Spec.IPFamilies, "an unset preference must not pin a family")
		assert.Nil(t, svc.Spec.IPFamilyPolicy, "an unset preference must not pin a policy")
	})

	for _, family := range []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol} {
		t.Run("explicit "+string(family), func(t *testing.T) {
			svc := generate(family)
			require.NotNil(t, svc)
			assert.Equal(t, []corev1.IPFamily{family}, svc.Spec.IPFamilies)
			require.NotNil(t, svc.Spec.IPFamilyPolicy)
			assert.Equal(t, corev1.IPFamilyPolicySingleStack, *svc.Spec.IPFamilyPolicy)
		})
	}
}

// TestServiceAppliesOverwrites: a Service generator merges the overwrites of
// its target, and a protected field they touch stays as generated, with a
// Warning event that names it.
func TestServiceAppliesOverwrites(t *testing.T) {
	newCluster := func(overwrites ...core.Overwrite) *v1alpha1.Cluster {
		return &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "uid"},
			Spec: v1alpha1.ClusterSpec{
				Replicas:   v1alpha1.ClusterReplicas{Shards: 3, ReplicasOfShard: 1},
				Overwrites: overwrites,
			},
		}
	}

	plain := testutil.NewFakeClusterInstance(newCluster())
	generated, err := GenerateInstanceService(plain)
	require.NoError(t, err)
	assert.Empty(t, plain.Events())
	assert.NotContains(t, generated.Annotations, overwrite.ChecksumAnnotation)

	inst := testutil.NewFakeClusterInstance(newCluster(core.Overwrite{
		Kind:   core.OverwriteKindService,
		Target: core.OverwriteTargetInstance,
		Patch:  apiextensionsv1.JSON{Raw: []byte(`{"metadata": {"labels": {"team": "cache"}}, "spec": {"selector": {"app": "other"}}}`)},
	}))
	svc, err := GenerateInstanceService(inst)
	require.NoError(t, err)
	assert.Equal(t, "cache", svc.Labels["team"])
	assert.Equal(t, generated.Spec.Selector, svc.Spec.Selector, "the selector picks the operator's pods")
	assert.NotEmpty(t, svc.Annotations[overwrite.ChecksumAnnotation])
	assert.Equal(t, []string{"Warning Overwrites overwrites for " + svc.Name + ": spec.selector restored"}, inst.Events())

	headless, err := GenerateHeadlessService(inst, 0)
	require.NoError(t, err)
	assert.NotContains(t, headless.Labels, "team", "the overwrites of another target")
}
