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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
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

	generators := map[string]func(*v1alpha1.Failover) *corev1.Service{
		"GenerateReadWriteService": GenerateReadWriteService,
		"GenerateReadonlyService":  GenerateReadonlyService,
		"GenerateExporterService":  GenerateExporterService,
		"GeneratePodNodePortService": func(rf *v1alpha1.Failover) *corev1.Service {
			return GeneratePodNodePortService(rf, 0, 0)
		},
	}
	for name, generate := range generators {
		t.Run(name, func(t *testing.T) {
			assertIPFamilyStates(t, func(family corev1.IPFamily) *corev1.Service {
				return generate(newFailover(family))
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
