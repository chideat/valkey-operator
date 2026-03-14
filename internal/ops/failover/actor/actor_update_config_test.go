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

package actor

import (
	"context"
	"crypto/tls"
	"testing"

	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/failoverbuilder"
	ops "github.com/chideat/valkey-operator/internal/ops/failover"
	"github.com/chideat/valkey-operator/pkg/kubernetes/clientset/mocks"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/version"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// mockFailoverInstance implements types.FailoverInstance for testing.
// It embeds *v1alpha1.Failover for metav1.Object methods.
type mockFailoverInstance struct {
	*v1alpha1.Failover
	nodeCount  int
	restartErr error
}

// types.Object
func (m *mockFailoverInstance) GetObjectKind() schema.ObjectKind { return m.Failover.GetObjectKind() }
func (m *mockFailoverInstance) DeepCopyObject() runtime.Object   { return m.Failover.DeepCopy() }
func (m *mockFailoverInstance) NamespacedName() client.ObjectKey {
	return client.ObjectKeyFromObject(m.Failover)
}
func (m *mockFailoverInstance) Version() version.ValkeyVersion { return version.ValkeyVersion("7.2") }
func (m *mockFailoverInstance) IsReady() bool                  { return true }
func (m *mockFailoverInstance) Restart(ctx context.Context, annotationKeyVal ...string) error {
	return m.restartErr
}
func (m *mockFailoverInstance) Refresh(ctx context.Context) error { return nil }

// types.Instance
func (m *mockFailoverInstance) Arch() core.Arch                     { return core.ValkeyFailover }
func (m *mockFailoverInstance) Issuer() *certmetav1.ObjectReference { return nil }
func (m *mockFailoverInstance) Users() types.Users                  { return nil }
func (m *mockFailoverInstance) TLSConfig() *tls.Config              { return nil }
func (m *mockFailoverInstance) IsInService() bool                   { return true }
func (m *mockFailoverInstance) IsACLUserExists() bool               { return false }
func (m *mockFailoverInstance) IsACLAppliedToAll() bool             { return false }
func (m *mockFailoverInstance) IsResourceFullfilled(ctx context.Context) (bool, error) {
	return true, nil
}
func (m *mockFailoverInstance) UpdateStatus(ctx context.Context, st types.InstanceStatus, message string) error {
	return nil
}
func (m *mockFailoverInstance) SendEventf(eventtype, reason, messageFmt string, args ...any) {}
func (m *mockFailoverInstance) Logger() logr.Logger                                          { return logr.Discard() }

// types.FailoverInstance
func (m *mockFailoverInstance) Definition() *v1alpha1.Failover { return m.Failover }
func (m *mockFailoverInstance) Replication() types.Replication { return nil }
func (m *mockFailoverInstance) Masters() []types.ValkeyNode    { return nil }
func (m *mockFailoverInstance) Nodes() []types.ValkeyNode {
	return make([]types.ValkeyNode, m.nodeCount)
}
func (m *mockFailoverInstance) RawNodes(ctx context.Context) ([]corev1.Pod, error) { return nil, nil }
func (m *mockFailoverInstance) Monitor() types.FailoverMonitor                    { return nil }
func (m *mockFailoverInstance) IsBindedSentinel() bool                            { return false }
func (m *mockFailoverInstance) IsStandalone() bool                                { return false }
func (m *mockFailoverInstance) Selector() map[string]string {
	return map[string]string{"app": "valkey"}
}

func newTestFailoverInstance(name, ns string, customConfigs map[string]string) *mockFailoverInstance {
	return &mockFailoverInstance{
		Failover: &v1alpha1.Failover{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: v1alpha1.FailoverSpec{
				CustomConfigs: customConfigs,
			},
		},
	}
}

func TestActorUpdateConfig_ConfigMapNotFound(t *testing.T) {
	ctx := context.Background()
	clientMock := &mocks.ClientSet{}
	inst := newTestFailoverInstance("test-failover", "default", nil)

	clientMock.On("GetConfigMap", ctx, "default", mock.AnythingOfType("string")).
		Return(nil, kerrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "rfr-test-failover"))

	a := NewSentinelUpdateConfig(clientMock, logr.Discard())
	result := a.Do(ctx, inst)

	assert.NotNil(t, result)
	assert.Equal(t, ops.CommandEnsureResource, result.NextCommand())
	assert.Error(t, result.Err())
	clientMock.AssertExpectations(t)
}

func TestActorUpdateConfig_ConfigUnchanged(t *testing.T) {
	ctx := context.Background()
	clientMock := &mocks.ClientSet{}
	inst := newTestFailoverInstance("test-failover", "default", nil)

	newCm, err := failoverbuilder.GenerateConfigMap(inst)
	require.NoError(t, err)

	// LastApplied matches new config → no diff → no intermediate update
	oldCm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      newCm.Name,
			Namespace: newCm.Namespace,
			Annotations: map[string]string{
				builder.LastAppliedConfigAnnotationKey: newCm.Data[builder.ValkeyConfigKey],
			},
		},
		Data: newCm.Data,
	}

	clientMock.On("GetConfigMap", ctx, "default", newCm.Name).Return(oldCm, nil)
	clientMock.On("UpdateConfigMap", ctx, "default", mock.Anything).Return(nil)

	a := NewSentinelUpdateConfig(clientMock, logr.Discard())
	result := a.Do(ctx, inst)

	assert.Nil(t, result)
	clientMock.AssertNumberOfCalls(t, "UpdateConfigMap", 1)
	clientMock.AssertExpectations(t)
}

func TestActorUpdateConfig_HotConfigChanged(t *testing.T) {
	ctx := context.Background()
	clientMock := &mocks.ClientSet{}
	inst := newTestFailoverInstance("test-failover", "default", map[string]string{
		"maxmemory": "200mb",
	})
	// nodeCount=0 → no node.Setup calls needed

	newCm, err := failoverbuilder.GenerateConfigMap(inst)
	require.NoError(t, err)

	// Old last-applied has maxmemory=100mb; new has 200mb → "maxmemory" in changed (hot-config)
	oldCm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      newCm.Name,
			Namespace: newCm.Namespace,
			Annotations: map[string]string{
				builder.LastAppliedConfigAnnotationKey: "maxmemory 100mb\n",
			},
		},
		Data: newCm.Data,
	}

	clientMock.On("GetConfigMap", ctx, "default", newCm.Name).Return(oldCm, nil)
	// First call: update CM with last-applied annotation; second call: final update
	clientMock.On("UpdateConfigMap", ctx, "default", mock.Anything).Return(nil)

	a := NewSentinelUpdateConfig(clientMock, logr.Discard())
	result := a.Do(ctx, inst)

	assert.Nil(t, result)
	clientMock.AssertNumberOfCalls(t, "UpdateConfigMap", 2)
	clientMock.AssertExpectations(t)
}

func TestActorUpdateConfig_RestartRequiredChanged(t *testing.T) {
	ctx := context.Background()
	clientMock := &mocks.ClientSet{}
	// tcp-backlog is in RequireRestart policy
	inst := newTestFailoverInstance("test-failover", "default", map[string]string{
		"tcp-backlog": "512",
	})

	newCm, err := failoverbuilder.GenerateConfigMap(inst)
	require.NoError(t, err)

	// Old last-applied has tcp-backlog=128; new has 512 → triggers restart
	oldCm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      newCm.Name,
			Namespace: newCm.Namespace,
			Annotations: map[string]string{
				builder.LastAppliedConfigAnnotationKey: "tcp-backlog 128\n",
			},
		},
		Data: newCm.Data,
	}

	clientMock.On("GetConfigMap", ctx, "default", newCm.Name).Return(oldCm, nil)
	clientMock.On("UpdateConfigMap", ctx, "default", mock.Anything).Return(nil)

	a := NewSentinelUpdateConfig(clientMock, logr.Discard())
	result := a.Do(ctx, inst)

	// Restart called on inst (restartErr=nil), so result should be nil
	assert.Nil(t, result)
	clientMock.AssertNumberOfCalls(t, "UpdateConfigMap", 2)
	clientMock.AssertExpectations(t)
}

