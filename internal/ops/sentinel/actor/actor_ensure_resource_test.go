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
	"fmt"
	"testing"
	"time"

	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/actor"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/sentinelbuilder"
	"github.com/chideat/valkey-operator/internal/testutil"
	"github.com/chideat/valkey-operator/pkg/kubernetes/clientset/mocks"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/version"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
func (m *mockSentinelInstance) Version() version.ValkeyVersion     { return version.ValkeyVersion("7.2") }
func (m *mockSentinelInstance) SafeVersion() version.ValkeyVersion { return version.ValkeyVersion("7.2") }
func (m *mockSentinelInstance) IsReady() bool                  { return true }
func (m *mockSentinelInstance) Restart(ctx context.Context, annotationKeyVal ...string) error {
	return nil
}
func (m *mockSentinelInstance) Refresh(ctx context.Context) error { return nil }

// types.Instance
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

// TestEnsurePodDisruptionBudgetNoticesOverwrites: the budget is updated when
// its overwrites are added or removed, although its spec stays the same.
func TestEnsurePodDisruptionBudgetNoticesOverwrites(t *testing.T) {
	labels := core.Overwrite{Kind: core.OverwriteKindPodDisruptionBudget, Patch: apiextensionsv1.JSON{Raw: []byte(`{"metadata":{"labels":{"team":"cache"}}}`)}}
	newInst := func(overwrites ...core.Overwrite) *testutil.FakeSentinelInstance {
		return testutil.NewFakeSentinelInstance(&v1alpha1.Sentinel{
			ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
			Spec:       v1alpha1.SentinelSpec{Replicas: 3, Overwrites: overwrites},
		})
	}
	live := func(overwrites ...core.Overwrite) *policyv1.PodDisruptionBudget {
		pdb, err := sentinelbuilder.GeneratePodDisruptionBudget(newInst(overwrites...))
		require.NoError(t, err)
		return pdb
	}
	for _, tc := range []struct {
		name   string
		live   *policyv1.PodDisruptionBudget
		inst   *testutil.FakeSentinelInstance
		update bool
	}{
		{"no overwrites", live(), newInst(), false},
		{"the same overwrites", live(labels), newInst(labels), false},
		{"overwrites added", live(), newInst(labels), true},
		{"overwrites removed", live(labels), newInst(), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			clientMock := &mocks.ClientSet{}
			clientMock.On("GetPodDisruptionBudget", ctx, "default", tc.live.Name).Return(tc.live, nil)
			clientMock.On("UpdatePodDisruptionBudget", ctx, "default", mock.Anything).Return(nil).Maybe()

			a := &actorEnsureResource{client: clientMock, logger: logr.Discard()}
			assert.Nil(t, a.ensurePodDisruptionBudget(ctx, tc.inst, logr.Discard()))
			if tc.update {
				clientMock.AssertCalled(t, "UpdatePodDisruptionBudget", ctx, "default", mock.Anything)
			} else {
				clientMock.AssertNotCalled(t, "UpdatePodDisruptionBudget", ctx, "default", mock.Anything)
			}
		})
	}
}

// podSentinelNode is a sentinel node backed by the pod the test gives it.
type podSentinelNode struct {
	types.SentinelNode
	pod *corev1.Pod
}

func (n podSentinelNode) Definition() *corev1.Pod { return n.pod }
func (n podSentinelNode) IsTerminating() bool     { return n.pod.DeletionTimestamp != nil }

// readySentinelReplication is a StatefulSet whose status counts all its pods
// ready.
type readySentinelReplication struct {
	types.SentinelReplication
	sts   *appsv1.StatefulSet
	nodes []types.SentinelNode
}

func (r readySentinelReplication) Definition() *appsv1.StatefulSet { return r.sts }
func (r readySentinelReplication) IsReady() bool                   { return true }
func (r readySentinelReplication) Nodes() []types.SentinelNode     { return r.nodes }

type replicatedSentinel struct {
	*testutil.FakeSentinelInstance
	replication types.SentinelReplication
}

func (r replicatedSentinel) Replication() types.SentinelReplication { return r.replication }

// withSentinelPods gives inst a StatefulSet of pods that have been Ready for
// an hour; the one named deleting is shutting down, and still Ready until it
// stops.
func withSentinelPods(inst *testutil.FakeSentinelInstance, deleting string) replicatedSentinel {
	sen := inst.Definition()
	var nodes []types.SentinelNode
	for i := range int(sen.Spec.Replicas) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-%d", sentinelbuilder.SentinelStatefulSetName(sen.Name), i), Namespace: sen.Namespace},
			Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{
				Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour)),
			}}},
		}
		if pod.Name == deleting {
			pod.DeletionTimestamp = ptr.To(metav1.Now())
		}
		nodes = append(nodes, podSentinelNode{pod: pod})
	}
	sts := &appsv1.StatefulSet{
		Spec:   appsv1.StatefulSetSpec{Replicas: ptr.To(sen.Spec.Replicas)},
		Status: appsv1.StatefulSetStatus{ReadyReplicas: sen.Spec.Replicas},
	}
	return replicatedSentinel{FakeSentinelInstance: inst, replication: readySentinelReplication{sts: sts, nodes: nodes}}
}

// TestPodRestartsWaitForTheDeletedPod: after changing a pod Service the actor
// deletes that pod, so the pod reads its new address when it starts, and does
// the next one on a later reconcile. A deleted pod stays Ready while it shuts
// down, so without waiting for it to go the next reconcile would delete
// another sentinel before the first is back.
func TestPodRestartsWaitForTheDeletedPod(t *testing.T) {
	newSentinel := func(access core.InstanceAccess) *v1alpha1.Sentinel {
		return &v1alpha1.Sentinel{
			ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
			Spec: v1alpha1.SentinelSpec{
				Replicas: 3,
				Access:   v1alpha1.SentinelInstanceAccess{InstanceAccess: access},
			},
		}
	}

	t.Run("pod Services", func(t *testing.T) {
		// spec.access.annotations changed, and rfs-demo-2 was restarted for it.
		access := core.InstanceAccess{ServiceType: corev1.ServiceTypeClusterIP, Annotations: map[string]string{"note": "new"}}
		old, updated := newSentinel(core.InstanceAccess{ServiceType: corev1.ServiceTypeClusterIP}), newSentinel(access)
		live := map[string]*corev1.Service{
			"rfs-demo-0": sentinelbuilder.GeneratePodService(old, 0),
			"rfs-demo-1": sentinelbuilder.GeneratePodService(old, 1),
			"rfs-demo-2": sentinelbuilder.GeneratePodService(updated, 2),
		}
		for _, tc := range []struct {
			name     string
			deleting string
			wait     bool
		}{
			{"while rfs-demo-2 shuts down", "rfs-demo-2", true},
			{"once rfs-demo-2 is back", "", false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				ctx := context.Background()
				clientMock := &mocks.ClientSet{}
				for name, svc := range live {
					clientMock.On("GetService", ctx, "default", name).Return(svc.DeepCopy(), nil)
				}
				clientMock.On("UpdateService", ctx, "default", mock.Anything).Return(nil).Maybe()
				clientMock.On("GetPod", ctx, "default", mock.Anything).Return(nil, nil).Maybe()

				a := &actorEnsureResource{client: clientMock, logger: logr.Discard()}
				ret := a.ensureValkeyPodService(ctx, withSentinelPods(testutil.NewFakeSentinelInstance(newSentinel(access)), tc.deleting), logr.Discard())
				if tc.wait {
					assert.Equal(t, actor.Requeue(), ret)
					clientMock.AssertNotCalled(t, "UpdateService", ctx, "default", mock.Anything)
				} else {
					assert.Nil(t, ret)
					clientMock.AssertCalled(t, "UpdateService", ctx, "default", mock.MatchedBy(func(svc *corev1.Service) bool { return svc.Name == "rfs-demo-1" }))
				}
			})
		}
	})

	t.Run("node port Services", func(t *testing.T) {
		// rfs-demo-2 has a node port outside spec.access.ports; rfs-demo-0 was
		// moved to one of them and restarted.
		access := core.InstanceAccess{ServiceType: corev1.ServiceTypeNodePort, Ports: "30001,30002,30003"}
		sen := newSentinel(access)
		live := map[string]*corev1.Service{
			"rfs-demo-0": sentinelbuilder.GeneratePodNodePortService(sen, 0, 30001),
			"rfs-demo-1": sentinelbuilder.GeneratePodNodePortService(sen, 1, 30002),
			"rfs-demo-2": sentinelbuilder.GeneratePodNodePortService(sen, 2, 31000),
		}
		for _, tc := range []struct {
			name     string
			deleting string
			wait     bool
		}{
			{"while rfs-demo-0 shuts down", "rfs-demo-0", true},
			{"once rfs-demo-0 is back", "", false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				ctx := context.Background()
				clientMock := &mocks.ClientSet{}
				var items []corev1.Service
				for _, name := range []string{"rfs-demo-0", "rfs-demo-1", "rfs-demo-2"} {
					items = append(items, *live[name])
					clientMock.On("GetService", ctx, "default", name).Return(live[name].DeepCopy(), nil)
				}
				clientMock.On("GetServiceByLabels", ctx, "default", mock.Anything).Return(&corev1.ServiceList{Items: items}, nil)
				clientMock.On("UpdateService", ctx, "default", mock.Anything).Return(nil).Maybe()
				clientMock.On("GetPod", ctx, "default", mock.Anything).Return(nil, nil).Maybe()

				a := &actorEnsureResource{client: clientMock, logger: logr.Discard()}
				ret := a.ensureValkeySpecifiedNodePortService(ctx, withSentinelPods(testutil.NewFakeSentinelInstance(sen), tc.deleting), logr.Discard())
				if tc.wait {
					assert.Equal(t, actor.Requeue(), ret)
					clientMock.AssertNotCalled(t, "UpdateService", ctx, "default", mock.Anything)
				} else {
					assert.Nil(t, ret)
					clientMock.AssertCalled(t, "UpdateService", ctx, "default", mock.MatchedBy(func(svc *corev1.Service) bool {
						return svc.Name == "rfs-demo-2" && svc.Spec.Ports[0].NodePort == 30003
					}))
				}
			})
		}
	})
}
