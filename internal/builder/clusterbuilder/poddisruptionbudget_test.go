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
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder/overwrite"
	"github.com/chideat/valkey-operator/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func TestGeneratePodDisruptionBudget(t *testing.T) {
	tests := []struct {
		name          string
		clusterName   string
		namespace     string
		index         int
		expectName    string
		expectLabels  bool // 当为负索引时，应该检查是否没有 statefulset 标签
		expectMissing bool // 当为负索引时，可能没有某些预期的属性
	}{
		{
			name:         "Valid PDB for index 0",
			clusterName:  "test-cluster",
			namespace:    "default",
			index:        0,
			expectName:   "drc-test-cluster-0",
			expectLabels: true,
		},
		{
			name:         "Valid PDB for index 1",
			clusterName:  "test-cluster",
			namespace:    "custom-namespace",
			index:        1,
			expectName:   "drc-test-cluster-1",
			expectLabels: true,
		},
		{
			name:          "PDB with negative index",
			clusterName:   "another-cluster",
			namespace:     "default",
			index:         -1,
			expectName:    "drc-another-cluster--1", // 实际可能生成的名称
			expectLabels:  false,                    // 不期望有 statefulset 标签
			expectMissing: true,                     // 负索引可能导致一些行为不同
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock cluster instance
			mockCluster := NewMockClusterInstance(tt.clusterName, tt.namespace, nil, corev1.ResourceRequirements{}, "8.0")

			// Generate the PDB
			pdb, err := GeneratePodDisruptionBudget(mockCluster, tt.index)

			// Verify the PDB
			require.NoError(t, err)
			require.NotNil(t, pdb)

			// Check basic properties
			if !tt.expectMissing {
				expectedName := ClusterStatefulSetName(tt.clusterName, tt.index)
				assert.Equal(t, expectedName, pdb.Name)
				assert.Equal(t, tt.namespace, pdb.Namespace)

				// Check that the labels are set correctly
				assert.NotNil(t, pdb.Labels)
				if tt.expectLabels {
					assert.Equal(t, expectedName, pdb.Labels["statefulset"])
				} else {
					assert.Empty(t, pdb.Labels["statefulset"])
				}

				// Check that the selectors are set correctly
				require.NotNil(t, pdb.Spec.Selector)
				assert.Equal(t, GenerateClusterStatefulSetSelectors(tt.clusterName, tt.index), pdb.Spec.Selector.MatchLabels)

				// Check that MaxUnavailable is set to 1
				require.NotNil(t, pdb.Spec.MaxUnavailable)
				expectedMaxUnavailable := ptr.To(intstr.FromInt(1))
				assert.Equal(t, expectedMaxUnavailable.IntVal, pdb.Spec.MaxUnavailable.IntVal)

				// Verify owner references are set
				assert.NotEmpty(t, pdb.OwnerReferences)
				assert.Equal(t, tt.clusterName, pdb.OwnerReferences[0].Name)
			} else {
				// 对于负索引，我们只检查基本的属性
				assert.Equal(t, tt.namespace, pdb.Namespace)

				// 验证标签不包含 statefulset 键
				assert.NotNil(t, pdb.Labels)
				_, hasStatefulSetLabel := pdb.Labels["statefulset"]
				assert.False(t, hasStatefulSetLabel, "Should not have statefulset label for negative index")

				// 验证有正确的 OwnerReferences
				assert.NotEmpty(t, pdb.OwnerReferences)
				assert.Equal(t, tt.clusterName, pdb.OwnerReferences[0].Name)
			}
		})
	}
}

// TestGeneratePodDisruptionBudgetAppliesOverwrites: PodDisruptionBudget
// overwrites land on the generated budget, and a protected field they touch
// stays as generated, with a Warning event that names it.
func TestGeneratePodDisruptionBudgetAppliesOverwrites(t *testing.T) {
	newCluster := func(overwrites ...core.Overwrite) *v1alpha1.Cluster {
		return &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "uid"},
			Spec: v1alpha1.ClusterSpec{
				Replicas:   v1alpha1.ClusterReplicas{Shards: 3, ReplicasOfShard: 2},
				Overwrites: overwrites,
			},
		}
	}

	plain := testutil.NewFakeClusterInstance(newCluster())
	generated, err := GeneratePodDisruptionBudget(plain, 0)
	require.NoError(t, err)
	assert.Empty(t, plain.Events())
	assert.NotContains(t, generated.Annotations, overwrite.ChecksumAnnotation)

	inst := testutil.NewFakeClusterInstance(newCluster(core.Overwrite{
		Kind: core.OverwriteKindPodDisruptionBudget,
		Patch: apiextensionsv1.JSON{Raw: []byte(`{
			"metadata": {"labels": {"team": "cache"}},
			"spec": {"unhealthyPodEvictionPolicy": "AlwaysAllow", "selector": {"matchLabels": {"app": "other"}}}
		}`)},
	}))
	pdb, err := GeneratePodDisruptionBudget(inst, 0)
	require.NoError(t, err)

	assert.Equal(t, "cache", pdb.Labels["team"])
	assert.Equal(t, ptr.To(policyv1.AlwaysAllow), pdb.Spec.UnhealthyPodEvictionPolicy)
	assert.Equal(t, generated.Spec.Selector, pdb.Spec.Selector, "the selector picks the operator's pods")
	assert.NotEmpty(t, pdb.Annotations[overwrite.ChecksumAnnotation])
	assert.Equal(t, []string{"Warning Overwrites overwrites for " + pdb.Name + ": spec.selector restored"}, inst.Events())
}
