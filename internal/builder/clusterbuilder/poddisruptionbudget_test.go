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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
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
			pdb := GeneratePodDisruptionBudget(mockCluster, tt.index)

			// Verify the PDB
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
