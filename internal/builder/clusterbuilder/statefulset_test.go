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
	"github.com/chideat/valkey-operator/pkg/types/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestVolumeMounts(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *v1alpha1.Cluster
		user     *user.User
		expected []corev1.VolumeMount
	}{
		{
			name: "Basic test",
			cluster: &v1alpha1.Cluster{
				Spec: v1alpha1.ClusterSpec{},
			},
			user: &user.User{
				Password: &user.Password{},
			},
			expected: []corev1.VolumeMount{
				{Name: ConfigVolumeName, MountPath: ConfigVolumeMountPath},
				{Name: StorageVolumeName, MountPath: StorageVolumeMountPath},
				{Name: ValkeyOptVolumeName, MountPath: ValkeyOptVolumeMountPath},
				{Name: ValkeyTempVolumeName, MountPath: ValkeyTempVolumeMountPath},
			},
		},
		{
			name: "With password",
			cluster: &v1alpha1.Cluster{
				Spec: v1alpha1.ClusterSpec{},
			},
			user: &user.User{
				Password: &user.Password{
					SecretName: "secret-name",
				},
			},
			expected: []corev1.VolumeMount{
				{Name: ConfigVolumeName, MountPath: ConfigVolumeMountPath},
				{Name: StorageVolumeName, MountPath: StorageVolumeMountPath},
				{Name: ValkeyOptVolumeName, MountPath: ValkeyOptVolumeMountPath},
				{Name: ValkeyTempVolumeName, MountPath: ValkeyTempVolumeMountPath},
				{Name: ValkeyPasswordVolumeName, MountPath: PasswordVolumeMountPath},
			},
		},
		{
			name: "With TLS",
			cluster: &v1alpha1.Cluster{
				Spec: v1alpha1.ClusterSpec{
					Access: core.InstanceAccess{
						EnableTLS: true,
					},
				},
			},
			user: &user.User{
				Password: &user.Password{},
			},
			expected: []corev1.VolumeMount{
				{Name: ConfigVolumeName, MountPath: ConfigVolumeMountPath},
				{Name: StorageVolumeName, MountPath: StorageVolumeMountPath},
				{Name: ValkeyOptVolumeName, MountPath: ValkeyOptVolumeMountPath},
				{Name: ValkeyTempVolumeName, MountPath: ValkeyTempVolumeMountPath},
				{Name: TLSVolumeName, MountPath: TLSVolumeMountPath},
			},
		},
		{
			name: "With password and TLS",
			cluster: &v1alpha1.Cluster{
				Spec: v1alpha1.ClusterSpec{
					Access: core.InstanceAccess{
						EnableTLS: true,
					},
				},
			},
			user: &user.User{
				Password: &user.Password{
					SecretName: "secret-name",
				},
			},
			expected: []corev1.VolumeMount{
				{Name: ConfigVolumeName, MountPath: ConfigVolumeMountPath},
				{Name: StorageVolumeName, MountPath: StorageVolumeMountPath},
				{Name: ValkeyOptVolumeName, MountPath: ValkeyOptVolumeMountPath},
				{Name: ValkeyTempVolumeName, MountPath: ValkeyTempVolumeMountPath},
				{Name: ValkeyPasswordVolumeName, MountPath: PasswordVolumeMountPath},
				{Name: TLSVolumeName, MountPath: TLSVolumeMountPath},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			volumeMounts := buildVolumeMounts(tt.cluster, tt.user)
			assert.ElementsMatch(t, tt.expected, volumeMounts)
		})
	}
}

// TestBuildPersistentClaimsOwnerReferences is the R10 regression: when
// RetainAfterDeleted is true, the PVC templates must carry OwnerReferences. The
// old `for _, vc := range ret` loop mutated a copy and silently dropped them.
func TestBuildPersistentClaimsOwnerReferences(t *testing.T) {
	capacity := resource.MustParse("1Gi")
	sc := "standard"
	newCluster := func(retain bool) *v1alpha1.Cluster {
		return &v1alpha1.Cluster{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Cluster",
				APIVersion: "valkey.buf.red/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "r10-cluster",
				Namespace: "default",
				UID:       k8stypes.UID("r10-cluster-uid"),
			},
			Spec: v1alpha1.ClusterSpec{
				Storage: &core.Storage{
					StorageClassName:   &sc,
					Capacity:           &capacity,
					RetainAfterDeleted: retain,
				},
			},
		}
	}

	t.Run("retain sets owner references on the PVC template", func(t *testing.T) {
		cluster := newCluster(true)
		pvcs := buildPersistentClaims(cluster, map[string]string{"test": "r10"})
		require.Len(t, pvcs, 1)
		require.Len(t, pvcs[0].OwnerReferences, 1,
			"OwnerReferences must be set on the returned PVC, not on a discarded copy")
		assert.Equal(t, k8stypes.UID("r10-cluster-uid"), pvcs[0].OwnerReferences[0].UID)
		assert.Equal(t, "Cluster", pvcs[0].OwnerReferences[0].Kind)
	})

	t.Run("no owner references when retain is disabled", func(t *testing.T) {
		cluster := newCluster(false)
		pvcs := buildPersistentClaims(cluster, nil)
		require.Len(t, pvcs, 1)
		assert.Empty(t, pvcs[0].OwnerReferences)
	})
}

// The cluster data init container carried no securityContext at all. The helper image
// declares no USER, so the container started as root and a namespace enforcing the Pod
// Security Admission "restricted" profile rejected the whole pod -- no instance of the
// cluster architecture could be created there.
func TestValkeyDataInitContainerHasSecurityContext(t *testing.T) {
	cluster := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
		Spec:       v1alpha1.ClusterSpec{},
	}
	c := buildValkeyDataInitContainer(cluster, &user.User{Password: &user.Password{}}, nil)
	require.NotNil(t, c)
	require.NotNil(t, c.SecurityContext, "init container must carry a securityContext")

	sc := c.SecurityContext
	require.NotNil(t, sc.RunAsUser)
	assert.NotEqual(t, int64(0), *sc.RunAsUser, "init container must not run as root")
	require.NotNil(t, sc.RunAsNonRoot)
	assert.True(t, *sc.RunAsNonRoot)

	// the four fields the "restricted" profile requires
	require.NotNil(t, sc.AllowPrivilegeEscalation)
	assert.False(t, *sc.AllowPrivilegeEscalation)
	require.NotNil(t, sc.Capabilities)
	assert.Equal(t, []corev1.Capability{"ALL"}, sc.Capabilities.Drop)
	require.NotNil(t, sc.Privileged)
	assert.False(t, *sc.Privileged)
	require.NotNil(t, sc.SeccompProfile)
	assert.Equal(t, corev1.SeccompProfileTypeRuntimeDefault, sc.SeccompProfile.Type)
}

// A caller-supplied securityContext must survive. The previous implementation rebuilt a
// fresh struct and copied back only RunAsUser/RunAsGroup/RunAsNonRoot, silently discarding
// everything else the caller had set.
func TestValkeyDataInitContainerHonoursCallerSecurityContext(t *testing.T) {
	cluster := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
		Spec: v1alpha1.ClusterSpec{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:      ptr.To(int64(1234)),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeUnconfined},
			},
		},
	}
	c := buildValkeyDataInitContainer(cluster, &user.User{Password: &user.Password{}}, nil)
	require.NotNil(t, c)
	require.NotNil(t, c.SecurityContext)

	assert.Equal(t, int64(1234), *c.SecurityContext.RunAsUser)
	assert.Equal(t, corev1.SeccompProfileTypeUnconfined, c.SecurityContext.SeccompProfile.Type,
		"a caller-supplied seccomp profile must not be overwritten by the default")
}

// TestGenerateStatefulSetAppliesOverwrites: spec.overwrites lands on the
// generated StatefulSet, and a protected field it touches stays as generated,
// with a Warning event that names it.
func TestGenerateStatefulSetAppliesOverwrites(t *testing.T) {
	newCluster := func(overwrites ...core.Overwrite) *v1alpha1.Cluster {
		return &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "uid"},
			Spec: v1alpha1.ClusterSpec{
				Image:      "valkey/valkey:8.1",
				Replicas:   v1alpha1.ClusterReplicas{Shards: 3, ReplicasOfShard: 2},
				Exporter:   &core.Exporter{Image: "oliver006/redis_exporter:v1.67.0-alpine"},
				Overwrites: overwrites,
			},
		}
	}
	opUser := &user.User{
		Name:     user.DefaultOperatorUserName,
		Role:     user.RoleOperator,
		Password: &user.Password{SecretName: "cluster-acl-demo-operator-secret"},
	}

	plain := testutil.NewFakeClusterInstance(newCluster()).WithUsers(opUser)
	generated, err := GenerateStatefulSet(plain, 0)
	require.NoError(t, err)
	assert.Empty(t, plain.Events())
	assert.NotContains(t, generated.Annotations, overwrite.ChecksumAnnotation)

	inst := testutil.NewFakeClusterInstance(newCluster(core.Overwrite{
		Kind: core.OverwriteKindStatefulSet,
		Patch: apiextensionsv1.JSON{Raw: []byte(`{"spec": {
			"replicas": 5,
			"template": {
				"metadata": {"annotations": {"example.com/scrape": "true"}},
				"spec": {"containers": [{"name": "exporter", "args": ["--include-system-metrics=true"]}]}
			}
		}}`)},
	})).WithUsers(opUser)
	sts, err := GenerateStatefulSet(inst, 0)
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
