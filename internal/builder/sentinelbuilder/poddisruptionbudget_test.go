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
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder/overwrite"
	"github.com/chideat/valkey-operator/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	policyv1 "k8s.io/api/policy/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// TestGeneratePodDisruptionBudgetAppliesOverwrites: PodDisruptionBudget
// overwrites land on the generated budget, and a protected field they touch
// stays as generated, with a Warning event that names it.
func TestGeneratePodDisruptionBudgetAppliesOverwrites(t *testing.T) {
	newSentinel := func(overwrites ...core.Overwrite) *v1alpha1.Sentinel {
		return &v1alpha1.Sentinel{
			ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "uid"},
			Spec: v1alpha1.SentinelSpec{
				Replicas:   3,
				Overwrites: overwrites,
			},
		}
	}

	plain := testutil.NewFakeSentinelInstance(newSentinel())
	generated, err := GeneratePodDisruptionBudget(plain)
	require.NoError(t, err)
	assert.Empty(t, plain.Events())
	assert.NotContains(t, generated.Annotations, overwrite.ChecksumAnnotation)

	inst := testutil.NewFakeSentinelInstance(newSentinel(core.Overwrite{
		Kind: core.OverwriteKindPodDisruptionBudget,
		Patch: apiextensionsv1.JSON{Raw: []byte(`{
			"metadata": {"labels": {"team": "cache"}},
			"spec": {"unhealthyPodEvictionPolicy": "AlwaysAllow", "selector": {"matchLabels": {"app": "other"}}}
		}`)},
	}))
	pdb, err := GeneratePodDisruptionBudget(inst)
	require.NoError(t, err)

	assert.Equal(t, "cache", pdb.Labels["team"])
	assert.Equal(t, ptr.To(policyv1.AlwaysAllow), pdb.Spec.UnhealthyPodEvictionPolicy)
	assert.Equal(t, generated.Spec.Selector, pdb.Spec.Selector, "the selector picks the operator's pods")
	assert.NotEmpty(t, pdb.Annotations[overwrite.ChecksumAnnotation])
	assert.Equal(t, []string{"Warning Overwrites overwrites for " + pdb.Name + ": spec.selector restored"}, inst.Events())
}
