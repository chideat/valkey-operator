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
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/overwrite"
	"github.com/chideat/valkey-operator/internal/testutil"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/types/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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

// sentinelMonitor is a failover monitor that reports the sentinel policy.
type sentinelMonitor struct{ types.FailoverMonitor }

func (sentinelMonitor) Policy() v1alpha1.FailoverPolicy { return v1alpha1.SentinelFailoverPolicy }

// TestGenerateStatefulSetAppliesOverwrites: spec.overwrites lands on the
// generated StatefulSet, and a protected field it touches stays as generated,
// with a Warning event that names it.
func TestGenerateStatefulSetAppliesOverwrites(t *testing.T) {
	newFailover := func(overwrites ...core.Overwrite) *v1alpha1.Failover {
		return &v1alpha1.Failover{
			ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "uid"},
			Spec: v1alpha1.FailoverSpec{
				Image:      "valkey/valkey:8.1",
				Replicas:   2,
				Exporter:   &core.Exporter{Image: "oliver006/redis_exporter:v1.67.0-alpine"},
				Overwrites: overwrites,
			},
		}
	}
	opUser := &user.User{
		Name:     user.DefaultOperatorUserName,
		Role:     user.RoleOperator,
		Password: &user.Password{SecretName: "failover-acl-demo-operator-secret"},
	}

	plain := testutil.NewFakeFailoverInstance(newFailover()).WithMonitor(sentinelMonitor{}).WithUsers(opUser)
	generated, err := GenerateStatefulSet(plain)
	require.NoError(t, err)
	assert.Empty(t, plain.Events())
	assert.NotContains(t, generated.Annotations, overwrite.ChecksumAnnotation)

	inst := testutil.NewFakeFailoverInstance(newFailover(core.Overwrite{
		Kind: core.OverwriteKindStatefulSet,
		Patch: apiextensionsv1.JSON{Raw: []byte(`{"spec": {
			"replicas": 5,
			"template": {
				"metadata": {"annotations": {"example.com/scrape": "true"}},
				"spec": {"containers": [{"name": "exporter", "args": ["--include-system-metrics=true"]}]}
			}
		}}`)},
	})).WithMonitor(sentinelMonitor{}).WithUsers(opUser)
	sts, err := GenerateStatefulSet(inst)
	require.NoError(t, err)

	assert.Equal(t, generated.Spec.Replicas, sts.Spec.Replicas, "the operator scales the StatefulSet")
	assert.Equal(t, "true", sts.Spec.Template.Annotations["example.com/scrape"])
	var args []string
	for _, c := range sts.Spec.Template.Spec.Containers {
		if c.Name == builder.ExporterContainerName {
			args = c.Args
		}
	}
	assert.Equal(t, []string{"--include-system-metrics=true"}, args)
	assert.NotEmpty(t, sts.Annotations[overwrite.ChecksumAnnotation])
	assert.Equal(t, []string{"Warning Overwrites overwrites for " + sts.Name + ": spec.replicas restored"}, inst.Events())
}
