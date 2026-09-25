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
	"context"
	"crypto/tls"
	"testing"

	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/overwrite"
	"github.com/chideat/valkey-operator/internal/testutil"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/version"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockSentinelInstance struct {
	*v1alpha1.Sentinel
}

func (m *mockSentinelInstance) Definition() *v1alpha1.Sentinel {
	return m.Sentinel
}

// Implement types.Object interface
func (m *mockSentinelInstance) GetObjectKind() schema.ObjectKind { return m.Sentinel.GetObjectKind() }
func (m *mockSentinelInstance) DeepCopyObject() runtime.Object   { return m.Sentinel.DeepCopy() }
func (m *mockSentinelInstance) NamespacedName() client.ObjectKey {
	return client.ObjectKeyFromObject(m.Sentinel)
}
func (m *mockSentinelInstance) Version() version.ValkeyVersion     { return version.ValkeyVersion("7.2") }
func (m *mockSentinelInstance) SafeVersion() version.ValkeyVersion { return version.ValkeyVersion("7.2") }
func (m *mockSentinelInstance) IsReady() bool                  { return true }
func (m *mockSentinelInstance) Restart(ctx context.Context, annotationKeyVal ...string) error {
	return nil
}
func (m *mockSentinelInstance) Refresh(ctx context.Context) error { return nil }

// Implement types.Instance interface
func (m *mockSentinelInstance) Arch() core.Arch                     { return core.ValkeySentinel }
func (m *mockSentinelInstance) Issuer() *certmetav1.IssuerReference { return nil }
func (m *mockSentinelInstance) Users() types.Users                  { return nil }
func (m *mockSentinelInstance) TLSConfig() *tls.Config              { return nil }
func (m *mockSentinelInstance) IsInService() bool                   { return true }
func (m *mockSentinelInstance) IsACLUserExists() bool               { return false }
func (m *mockSentinelInstance) IsACLAppliedToAll() bool             { return false }
func (m *mockSentinelInstance) IsResourceFullfilled(ctx context.Context) (bool, error) {
	return true, nil
}
func (m *mockSentinelInstance) UpdateStatus(ctx context.Context, st types.InstanceStatus, message string) error {
	return nil
}
func (m *mockSentinelInstance) SendEventf(eventtype, reason, messageFmt string, args ...any) {}
func (m *mockSentinelInstance) Logger() logr.Logger                                          { return logr.Discard() }

// Implement types.SentinelInstance interface
func (m *mockSentinelInstance) Replication() types.SentinelReplication             { return nil }
func (m *mockSentinelInstance) Nodes() []types.SentinelNode                        { return nil }
func (m *mockSentinelInstance) RawNodes(ctx context.Context) ([]corev1.Pod, error) { return nil, nil }
func (m *mockSentinelInstance) Clusters(ctx context.Context) ([]string, error)     { return nil, nil }
func (m *mockSentinelInstance) GetPassword() (string, error)                       { return "", nil }
func (m *mockSentinelInstance) Selector() map[string]string                        { return nil }

func TestGenerateSentinelStatefulset(t *testing.T) {
	sentinel := &v1alpha1.Sentinel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sentinel",
			Namespace: "default",
			UID:       "test-uid",
		},
		Spec: v1alpha1.SentinelSpec{
			Image:    "valkey/valkey:7.2",
			Replicas: 3,
			Access: v1alpha1.SentinelInstanceAccess{
				InstanceAccess: core.InstanceAccess{
					IPFamilyPrefer: corev1.IPv4Protocol,
					EnableTLS:      true,
				},
				DefaultPasswordSecret: "test-secret",
			},
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser: func(i int64) *int64 { return &i }(1000),
			},
		},
	}

	inst := &mockSentinelInstance{Sentinel: sentinel}
	ss, err := GenerateSentinelStatefulset(inst)
	assert.NoError(t, err)
	assert.NotNil(t, ss)

	// Verify metadata
	assert.Equal(t, "rfs-test-sentinel", ss.Name)
	assert.Equal(t, "default", ss.Namespace)

	// Verify replicas
	assert.Equal(t, int32(3), *ss.Spec.Replicas)

	// Verify containers
	assert.Len(t, ss.Spec.Template.Spec.InitContainers, 1)
	assert.Len(t, ss.Spec.Template.Spec.Containers, 2)

	// Verify init container
	initCont := ss.Spec.Template.Spec.InitContainers[0]
	assert.Equal(t, "init", initCont.Name)

	// Verify server container
	serverCont := ss.Spec.Template.Spec.Containers[0]
	assert.Equal(t, SentinelContainerName, serverCont.Name)
	assert.Equal(t, "valkey/valkey:7.2", serverCont.Image)

	// Check env vars exist
	envMap := make(map[string]string)
	for _, env := range serverCont.Env {
		envMap[env.Name] = env.Value
	}
	assert.Equal(t, "test-secret", envMap[builder.OperatorSecretName])
	assert.Equal(t, "true", envMap["TLS_ENABLED"])

	// Verify agent container
	agentCont := ss.Spec.Template.Spec.Containers[1]
	assert.Equal(t, "agent", agentCont.Name)

	// Verify volumes
	// 3 base volumes + 1 TLS + 1 Auth = 5
	assert.Len(t, ss.Spec.Template.Spec.Volumes, 5)
}

func TestGenerateSentinelStatefulset_NoTLS(t *testing.T) {
	sentinel := &v1alpha1.Sentinel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sentinel-no-tls",
			Namespace: "default",
		},
		Spec: v1alpha1.SentinelSpec{
			Image:    "valkey/valkey:7.2",
			Replicas: 1,
			Access: v1alpha1.SentinelInstanceAccess{
				InstanceAccess: core.InstanceAccess{
					IPFamilyPrefer: corev1.IPv4Protocol,
				},
				// No password secret
			},
		},
	}

	inst := &mockSentinelInstance{Sentinel: sentinel}
	ss, err := GenerateSentinelStatefulset(inst)
	assert.NoError(t, err)
	assert.NotNil(t, ss)

	// Verify volumes
	// 3 base volumes (Config, Data, Opt)
	assert.Len(t, ss.Spec.Template.Spec.Volumes, 3)
}

// TestGenerateSentinelStatefulsetAppliesOverwrites: spec.overwrites lands on
// the generated StatefulSet, and a protected field it touches stays as
// generated, with a Warning event that names it.
func TestGenerateSentinelStatefulsetAppliesOverwrites(t *testing.T) {
	newSentinel := func(overwrites ...core.Overwrite) *v1alpha1.Sentinel {
		return &v1alpha1.Sentinel{
			ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "uid"},
			Spec: v1alpha1.SentinelSpec{
				Image:      "valkey/valkey:8.1",
				Replicas:   3,
				Overwrites: overwrites,
			},
		}
	}
	patch := func(doc string) core.Overwrite {
		return core.Overwrite{Kind: core.OverwriteKindStatefulSet, Patch: apiextensionsv1.JSON{Raw: []byte(doc)}}
	}

	plain := testutil.NewFakeSentinelInstance(newSentinel())
	generated, err := GenerateSentinelStatefulset(plain)
	require.NoError(t, err)
	assert.Empty(t, plain.Events())
	assert.NotContains(t, generated.Annotations, overwrite.ChecksumAnnotation)

	inst := testutil.NewFakeSentinelInstance(newSentinel(patch(`{"spec": {
		"replicas": 5,
		"template": {
			"metadata": {"annotations": {"example.com/scrape": "true"}},
			"spec": {"containers": [{"name": "agent", "env": [{"name": "LOG_LEVEL", "value": "debug"}]}]}
		}
	}}`)))
	sts, err := GenerateSentinelStatefulset(inst)
	require.NoError(t, err)

	assert.Equal(t, generated.Spec.Replicas, sts.Spec.Replicas, "the operator scales the StatefulSet")
	assert.Equal(t, "true", sts.Spec.Template.Annotations["example.com/scrape"])
	var env []corev1.EnvVar
	for _, c := range sts.Spec.Template.Spec.Containers {
		if c.Name == builder.AgentContainerName {
			env = c.Env
		}
	}
	if assert.NotEmpty(t, env) {
		assert.Equal(t, corev1.EnvVar{Name: "LOG_LEVEL", Value: "debug"}, env[len(env)-1],
			"the user's variable follows the operator's")
	}
	assert.NotEmpty(t, sts.Annotations[overwrite.ChecksumAnnotation])
	assert.Equal(t, []string{"Warning Overwrites overwrites for " + sts.Name + ": spec.replicas restored"}, inst.Events())

	t.Run("template annotations stay writable", func(t *testing.T) {
		// The sentinel actor writes its checksums into the template
		// annotations; an overwrite that leaves them empty must not make the
		// map nil.
		sts, err := GenerateSentinelStatefulset(testutil.NewFakeSentinelInstance(newSentinel(patch(`{"spec": {"minReadySeconds": 5}}`))))
		require.NoError(t, err)
		assert.Equal(t, int32(5), sts.Spec.MinReadySeconds)
		assert.NotNil(t, sts.Spec.Template.Annotations)
	})
}
