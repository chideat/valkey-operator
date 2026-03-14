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
	"github.com/chideat/valkey-operator/internal/actor"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/pkg/kubernetes/clientset/mocks"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/version"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// mockSentinelInstance implements types.SentinelInstance for testing.
type mockSentinelInstance struct {
	*v1alpha1.Sentinel
	nodeCount int
}

// types.Object
func (m *mockSentinelInstance) GetObjectKind() schema.ObjectKind { return m.Sentinel.GetObjectKind() }
func (m *mockSentinelInstance) DeepCopyObject() runtime.Object   { return m.Sentinel.DeepCopy() }
func (m *mockSentinelInstance) NamespacedName() client.ObjectKey {
	return client.ObjectKeyFromObject(m.Sentinel)
}
func (m *mockSentinelInstance) Version() version.ValkeyVersion { return version.ValkeyVersion("7.2") }
func (m *mockSentinelInstance) IsReady() bool                  { return true }
func (m *mockSentinelInstance) Restart(ctx context.Context, annotationKeyVal ...string) error {
	return nil
}
func (m *mockSentinelInstance) Refresh(ctx context.Context) error { return nil }

// types.Instance
func (m *mockSentinelInstance) Arch() core.Arch                     { return core.ValkeySentinel }
func (m *mockSentinelInstance) Issuer() *certmetav1.ObjectReference { return nil }
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

// types.SentinelInstance
func (m *mockSentinelInstance) Definition() *v1alpha1.Sentinel { return m.Sentinel }
func (m *mockSentinelInstance) Replication() types.SentinelReplication {
	return nil
}
func (m *mockSentinelInstance) Nodes() []types.SentinelNode {
	return make([]types.SentinelNode, m.nodeCount)
}
func (m *mockSentinelInstance) RawNodes(ctx context.Context) ([]corev1.Pod, error) {
	return nil, nil
}
func (m *mockSentinelInstance) Clusters(ctx context.Context) ([]string, error) { return nil, nil }
func (m *mockSentinelInstance) GetPassword() (string, error)                   { return "", nil }
func (m *mockSentinelInstance) Selector() map[string]string {
	return map[string]string{"app": "sentinel"}
}

func newTestSentinelInstance(name, ns string) *mockSentinelInstance {
	return &mockSentinelInstance{
		Sentinel: &v1alpha1.Sentinel{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: v1alpha1.SentinelSpec{},
		},
	}
}

// TestSentinelActorEnsureResource_Pause_AllPodsDeleted verifies that when the pause
// annotation is set and no sentinel nodes exist, the actor returns the Pause command.
func TestSentinelActorEnsureResource_Pause_AllPodsDeleted(t *testing.T) {
	ctx := context.Background()
	clientMock := &mocks.ClientSet{}
	inst := newTestSentinelInstance("test-sentinel", "default")

	// Set pause annotation
	inst.Sentinel.Spec.PodAnnotations = map[string]string{
		builder.PauseAnnotationKey: "2026-03-14T00:00:00Z",
	}

	// ensurePauseStatefulSet: STS not found → returns nil
	clientMock.On("GetStatefulSet", ctx, "default", mock.AnythingOfType("string")).
		Return(nil, kerrors.NewNotFound(schema.GroupResource{Resource: "statefulsets"}, "rfs-test-sentinel"))

	// Nodes() returns 0 → actor returns Pause
	inst.nodeCount = 0

	a := NewEnsureResourceActor(clientMock, logr.Discard())
	result := a.Do(ctx, inst)

	assert.NotNil(t, result)
	assert.Equal(t, actor.CommandPaused, result.NextCommand())
	clientMock.AssertExpectations(t)
}

// TestSentinelActorEnsureResource_Pause_PodsExist verifies that when the pause
// annotation is set but sentinel nodes still exist, the actor returns Requeue.
func TestSentinelActorEnsureResource_Pause_PodsExist(t *testing.T) {
	ctx := context.Background()
	clientMock := &mocks.ClientSet{}
	inst := newTestSentinelInstance("test-sentinel", "default")

	// Set pause annotation
	inst.Sentinel.Spec.PodAnnotations = map[string]string{
		builder.PauseAnnotationKey: "2026-03-14T00:00:00Z",
	}

	// ensurePauseStatefulSet: STS found with replicas=0 → no update needed
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rfs-test-sentinel",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: ptr.To(int32(0)),
		},
	}
	clientMock.On("GetStatefulSet", ctx, "default", mock.AnythingOfType("string")).
		Return(sts, nil)

	// Nodes() returns 1 → actor returns Requeue
	inst.nodeCount = 1

	a := NewEnsureResourceActor(clientMock, logr.Discard())
	result := a.Do(ctx, inst)

	assert.NotNil(t, result)
	assert.Equal(t, actor.CommandRequeue, result.NextCommand())
	clientMock.AssertExpectations(t)
}

// TestSentinelActorEnsureResource_Pause_STSScaleDown verifies that when the STS has
// non-zero replicas and pause is requested, the actor scales the STS to zero.
func TestSentinelActorEnsureResource_Pause_STSScaleDown(t *testing.T) {
	ctx := context.Background()
	clientMock := &mocks.ClientSet{}
	inst := newTestSentinelInstance("test-sentinel", "default")

	inst.Sentinel.Spec.PodAnnotations = map[string]string{
		builder.PauseAnnotationKey: "2026-03-14T00:00:00Z",
	}

	// STS with replicas=3 → actor scales to 0
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rfs-test-sentinel",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: ptr.To(int32(3)),
		},
	}
	clientMock.On("GetStatefulSet", ctx, "default", mock.AnythingOfType("string")).
		Return(sts, nil)
	clientMock.On("UpdateStatefulSet", ctx, "default", mock.Anything).Return(nil)

	inst.nodeCount = 0

	a := NewEnsureResourceActor(clientMock, logr.Discard())
	result := a.Do(ctx, inst)

	assert.NotNil(t, result)
	assert.Equal(t, actor.CommandPaused, result.NextCommand())
	clientMock.AssertCalled(t, "UpdateStatefulSet", ctx, "default", mock.Anything)
	clientMock.AssertExpectations(t)
}
