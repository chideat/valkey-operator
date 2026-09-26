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

package failoverbuilder

import (
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	v1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
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

// The failover services had no test at all, which is why four of the ten sites
// that pinned ipFamilies to IPv4 for an unset preference went unnoticed until a
// single-stack IPv6 cluster rejected every one of them.
func TestServiceIPFamilies(t *testing.T) {
	newFailover := func(family corev1.IPFamily) *v1alpha1.Failover {
		return &v1alpha1.Failover{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-failover",
				Namespace: "default",
				UID:       k8stypes.UID("test-failover-uid"),
			},
			Spec: v1alpha1.FailoverSpec{
				Access: core.InstanceAccess{IPFamilyPrefer: family},
			},
		}
	}

	generators := map[string]func(types.FailoverInstance) (*corev1.Service, error){
		"GenerateReadWriteService": GenerateReadWriteService,
		"GenerateReadonlyService":  GenerateReadonlyService,
		"GenerateExporterService":  GenerateExporterService,
		"GeneratePodNodePortService": func(inst types.FailoverInstance) (*corev1.Service, error) {
			return GeneratePodNodePortService(inst, 0, 0)
		},
	}
	for name, generate := range generators {
		t.Run(name, func(t *testing.T) {
			assertIPFamilyStates(t, func(family corev1.IPFamily) *corev1.Service {
				svc, err := generate(testutil.NewFakeFailoverInstance(newFailover(family)))
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
	newFailover := func(overwrites ...core.Overwrite) *v1alpha1.Failover {
		return &v1alpha1.Failover{
			ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "uid"},
			Spec:       v1alpha1.FailoverSpec{Replicas: 2, Overwrites: overwrites},
		}
	}

	plain := testutil.NewFakeFailoverInstance(newFailover())
	generated, err := GenerateReadWriteService(plain)
	require.NoError(t, err)
	assert.Empty(t, plain.Events())
	assert.NotContains(t, generated.Annotations, overwrite.ChecksumAnnotation)

	inst := testutil.NewFakeFailoverInstance(newFailover(core.Overwrite{
		Kind:   core.OverwriteKindService,
		Target: core.OverwriteTargetReadWrite,
		Patch:  apiextensionsv1.JSON{Raw: []byte(`{"metadata": {"labels": {"team": "cache"}}, "spec": {"selector": {"app": "other"}}}`)},
	}))
	svc, err := GenerateReadWriteService(inst)
	require.NoError(t, err)
	assert.Equal(t, "cache", svc.Labels["team"])
	assert.Equal(t, generated.Spec.Selector, svc.Spec.Selector, "the selector picks the primary")
	assert.NotEmpty(t, svc.Annotations[overwrite.ChecksumAnnotation])
	assert.Equal(t, []string{"Warning Overwrites overwrites for " + svc.Name + ": spec.selector restored"}, inst.Events())

	readonly, err := GenerateReadonlyService(inst)
	require.NoError(t, err)
	assert.NotContains(t, readonly.Labels, "team", "the overwrites of another target")
}
