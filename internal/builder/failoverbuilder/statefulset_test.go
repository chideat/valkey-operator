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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

// TestBuildPersistentClaimsOwnerReferences is the R10 regression: when
// RetainAfterDeleted is true, the PVC templates must carry OwnerReferences. The
// old `for _, vc := range ret` loop mutated a copy and silently dropped them.
func TestBuildPersistentClaimsOwnerReferences(t *testing.T) {
	capacity := resource.MustParse("1Gi")
	sc := "standard"
	newFailover := func(retain bool) *v1alpha1.Failover {
		return &v1alpha1.Failover{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Failover",
				APIVersion: "valkey.buf.red/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "r10-failover",
				Namespace: "default",
				UID:       k8stypes.UID("r10-failover-uid"),
			},
			Spec: v1alpha1.FailoverSpec{
				Storage: &core.Storage{
					StorageClassName:   &sc,
					Capacity:           &capacity,
					RetainAfterDeleted: retain,
				},
			},
		}
	}

	t.Run("retain sets owner references on the PVC template", func(t *testing.T) {
		pvcs := buildPersistentClaims(newFailover(true), map[string]string{"test": "r10"})
		require.Len(t, pvcs, 1)
		require.Len(t, pvcs[0].OwnerReferences, 1,
			"OwnerReferences must be set on the returned PVC, not on a discarded copy")
		assert.Equal(t, k8stypes.UID("r10-failover-uid"), pvcs[0].OwnerReferences[0].UID)
		assert.Equal(t, "Failover", pvcs[0].OwnerReferences[0].Kind)
	})

	t.Run("no owner references when retain is disabled", func(t *testing.T) {
		pvcs := buildPersistentClaims(newFailover(false), nil)
		require.Len(t, pvcs, 1)
		assert.Empty(t, pvcs[0].OwnerReferences)
	})
}
