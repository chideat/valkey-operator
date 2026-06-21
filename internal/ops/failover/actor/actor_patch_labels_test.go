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
	"fmt"
	"testing"

	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/actor"
	"github.com/chideat/valkey-operator/pkg/kubernetes/clientset/mocks"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockFailoverMonitor implements types.FailoverMonitor; only Master is controllable.
type mockFailoverMonitor struct {
	masterNode *valkey.SentinelMonitorNode
	masterErr  error
}

func (m *mockFailoverMonitor) Policy() v1alpha1.FailoverPolicy { return v1alpha1.ManualFailoverPolicy }
func (m *mockFailoverMonitor) Master(ctx context.Context, flags ...bool) (*valkey.SentinelMonitorNode, error) {
	return m.masterNode, m.masterErr
}
func (m *mockFailoverMonitor) Replicas(ctx context.Context) ([]*valkey.SentinelMonitorNode, error) {
	return nil, nil
}
func (m *mockFailoverMonitor) Inited(ctx context.Context) (bool, error)           { return true, nil }
func (m *mockFailoverMonitor) AllNodeMonitored(ctx context.Context) (bool, error) { return true, nil }
func (m *mockFailoverMonitor) UpdateConfig(ctx context.Context, params map[string]string) error {
	return nil
}
func (m *mockFailoverMonitor) Failover(ctx context.Context) error                       { return nil }
func (m *mockFailoverMonitor) Monitor(ctx context.Context, node types.ValkeyNode) error { return nil }
func (m *mockFailoverMonitor) Reset(ctx context.Context) error                          { return nil }

// TestActorPatchLabels_MasterError is the R2 regression: when Monitor().Master()
// errors, Do must return a Requeue result and must NOT fall through to
// net.JoinHostPort(masterNode.IP, ...) which would panic on the nil master.
func TestActorPatchLabels_MasterError(t *testing.T) {
	ctx := context.Background()
	clientMock := &mocks.ClientSet{}
	inst := newTestFailoverInstance("test-failover", "default", nil)
	inst.WithMonitor(&mockFailoverMonitor{masterErr: fmt.Errorf("sentinel down")})

	a := NewPatchLabelsActor(clientMock, logr.Discard())

	// Before the fix this panics with a nil-pointer dereference.
	result := a.Do(ctx, inst)

	require.NotNil(t, result)
	assert.Equal(t, actor.CommandRequeue, result.NextCommand())
	require.Error(t, result.Err())
	assert.Contains(t, result.Err().Error(), "sentinel down")
	clientMock.AssertNotCalled(t, "PatchPodLabel")
}

// TestActorPatchLabels_NoPods verifies the success path: a valid master with no
// pods to patch returns nil without touching the client.
func TestActorPatchLabels_NoPods(t *testing.T) {
	ctx := context.Background()
	clientMock := &mocks.ClientSet{}
	inst := newTestFailoverInstance("test-failover", "default", nil)
	inst.WithMonitor(&mockFailoverMonitor{
		masterNode: &valkey.SentinelMonitorNode{IP: "10.0.0.1", Port: "6379"},
	})

	a := NewPatchLabelsActor(clientMock, logr.Discard())
	result := a.Do(ctx, inst)

	assert.Nil(t, result)
	clientMock.AssertNotCalled(t, "PatchPodLabel")
}
