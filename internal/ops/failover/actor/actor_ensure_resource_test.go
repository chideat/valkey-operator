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
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/actor"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/failoverbuilder"
	"github.com/chideat/valkey-operator/internal/testutil"
	"github.com/chideat/valkey-operator/pkg/kubernetes/clientset/mocks"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	policyv1 "k8s.io/api/policy/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
)

// TestActorEnsureResource_Pause_AllPodsDeleted verifies that when the pause annotation
// is set and no pods exist, the actor returns the Pause command.
func TestActorEnsureResource_Pause_AllPodsDeleted(t *testing.T) {
	ctx := context.Background()
	clientMock := &mocks.ClientSet{}
	inst := newTestFailoverInstance("test-failover", "default", nil)

	// Set pause annotation
	inst.Failover.Spec.PodAnnotations = map[string]string{
		builder.PauseAnnotationKey: "2026-03-14T00:00:00Z",
	}

	// pauseStatefulSet: STS not found → returns nil
	clientMock.On("GetStatefulSet", ctx, "default", mock.AnythingOfType("string")).
		Return(nil, kerrors.NewNotFound(schema.GroupResource{Resource: "statefulsets"}, "rfr-test-failover"))

	// No sentinel configured → pauseSentinel returns nil immediately
	// Nodes() returns 0 → actor returns Pause
	a := NewEnsureResourceActor(clientMock, logr.Discard())
	result := a.Do(ctx, inst)

	assert.NotNil(t, result)
	assert.Equal(t, actor.CommandPaused, result.NextCommand())
	clientMock.AssertExpectations(t)
}

// TestActorEnsureResource_Pause_PodsExist verifies that when the pause annotation
// is set but pods still exist, the actor returns Requeue.
func TestActorEnsureResource_Pause_PodsExist(t *testing.T) {
	ctx := context.Background()
	clientMock := &mocks.ClientSet{}
	inst := newTestFailoverInstance("test-failover", "default", nil)

	// Set pause annotation
	inst.Failover.Spec.PodAnnotations = map[string]string{
		builder.PauseAnnotationKey: "2026-03-14T00:00:00Z",
	}

	// pauseStatefulSet: STS exists with replicas=0 → no update needed, returns nil
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rfr-test-failover",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: ptr.To(int32(0)),
		},
	}
	clientMock.On("GetStatefulSet", ctx, "default", mock.AnythingOfType("string")).
		Return(sts, nil)

	// No sentinel configured → pauseSentinel returns nil immediately
	// Nodes() returns 1 → actor returns Requeue
	inst.WithNodes(testutil.NewFakeValkeyNode("node-0", "10.0.0.1", 6379, core.NodeRoleReplica))

	a := NewEnsureResourceActor(clientMock, logr.Discard())
	result := a.Do(ctx, inst)

	assert.NotNil(t, result)
	assert.Equal(t, actor.CommandRequeue, result.NextCommand())
	clientMock.AssertExpectations(t)
}

// TestActorEnsureResource_Pause_STSScaleDown verifies that when the STS has non-zero
// replicas and pause is requested, the actor scales it down.
func TestActorEnsureResource_Pause_STSScaleDown(t *testing.T) {
	ctx := context.Background()
	clientMock := &mocks.ClientSet{}
	inst := newTestFailoverInstance("test-failover", "default", nil)

	inst.Failover.Spec.PodAnnotations = map[string]string{
		builder.PauseAnnotationKey: "2026-03-14T00:00:00Z",
	}

	// STS with replicas=3 → actor scales to 0
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rfr-test-failover",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: ptr.To(int32(3)),
		},
	}
	clientMock.On("GetStatefulSet", ctx, "default", mock.AnythingOfType("string")).
		Return(sts, nil)
	clientMock.On("UpdateStatefulSet", ctx, "default", mock.Anything).Return(nil)

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
	newInst := func(overwrites ...core.Overwrite) *testutil.FakeFailoverInstance {
		return testutil.NewFakeFailoverInstance(&v1alpha1.Failover{
			ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
			Spec:       v1alpha1.FailoverSpec{Replicas: 2, Overwrites: overwrites},
		})
	}
	live := func(overwrites ...core.Overwrite) *policyv1.PodDisruptionBudget {
		pdb, err := failoverbuilder.GeneratePodDisruptionBudget(newInst(overwrites...))
		require.NoError(t, err)
		return pdb
	}
	for _, tc := range []struct {
		name   string
		live   *policyv1.PodDisruptionBudget
		inst   *testutil.FakeFailoverInstance
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
