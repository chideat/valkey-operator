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

package monitor

import (
	"context"
	"fmt"
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/testutil"
	"github.com/chideat/valkey-operator/pkg/kubernetes/clientset/mocks"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func newFakeInstance(nodes, masters []types.ValkeyNode) *testutil.FakeFailoverInstance {
	rf := &v1alpha1.Failover{
		TypeMeta:   metav1.TypeMeta{Kind: "Failover", APIVersion: "valkey.buf.red/v1alpha1"},
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}
	return testutil.NewFakeFailoverInstance(rf).WithNodes(nodes...).WithMasters(masters...)
}

func notFoundConfigMap() (*corev1.ConfigMap, error) {
	return nil, kerrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "rf-ha-repl-test")
}

// TestManualMonitorFailover_SuccessReturnsNil is the R3 regression: a successful
// promotion must return nil, not the "No available node to failover" error.
func TestManualMonitorFailover_SuccessReturnsNil(t *testing.T) {
	ctx := context.Background()
	node := testutil.NewFakeValkeyNode("node0", "10.0.0.1", 6379, core.NodeRoleReplica)
	inst := newFakeInstance([]types.ValkeyNode{node}, []types.ValkeyNode{node})

	clientMock := &mocks.ClientSet{}
	// Master() -> NotFound -> ErrNoMaster; Monitor() -> NotFound -> CreateConfigMap
	clientMock.On("GetConfigMap", ctx, "default", "rf-ha-repl-test").Return(notFoundConfigMap())
	clientMock.On("CreateConfigMap", ctx, "default", mock.Anything).Return(nil)

	mon, err := NewManualMonitor(ctx, clientMock, inst)
	require.NoError(t, err)

	err = mon.Failover(ctx)
	assert.NoError(t, err, "successful failover must return nil")
	clientMock.AssertCalled(t, "CreateConfigMap", ctx, "default", mock.Anything)
}

// TestManualMonitorFailover_ReplicaOfError surfaces an error from the promote step.
func TestManualMonitorFailover_ReplicaOfError(t *testing.T) {
	ctx := context.Background()
	node := testutil.NewFakeValkeyNode("node0", "10.0.0.1", 6379, core.NodeRoleReplica).
		WithReplicaOfErr(fmt.Errorf("replicaof failed"))
	inst := newFakeInstance([]types.ValkeyNode{node}, []types.ValkeyNode{node})

	clientMock := &mocks.ClientSet{}
	clientMock.On("GetConfigMap", ctx, "default", "rf-ha-repl-test").Return(notFoundConfigMap())

	mon, err := NewManualMonitor(ctx, clientMock, inst)
	require.NoError(t, err)

	err = mon.Failover(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "replicaof failed")
}

// TestManualMonitorFailover_NoCandidate keeps the correct error when there is no
// node to promote.
func TestManualMonitorFailover_NoCandidate(t *testing.T) {
	ctx := context.Background()
	inst := newFakeInstance(nil, nil)

	clientMock := &mocks.ClientSet{}
	clientMock.On("GetConfigMap", ctx, "default", "rf-ha-repl-test").Return(notFoundConfigMap())

	mon, err := NewManualMonitor(ctx, clientMock, inst)
	require.NoError(t, err)

	err = mon.Failover(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "No available node to failover")
}
