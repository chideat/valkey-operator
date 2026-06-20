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
	"crypto/tls"
	"fmt"
	"net"
	"testing"

	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/pkg/kubernetes/clientset/mocks"
	"github.com/chideat/valkey-operator/pkg/slot"
	"github.com/chideat/valkey-operator/pkg/types"
	vkcli "github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/chideat/valkey-operator/pkg/version"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ---- mockNode implements types.ValkeyNode (only the fields Failover() uses are configurable) ----
type mockNode struct {
	metav1.ObjectMeta
	ip           string
	port         int
	role         core.NodeRole
	replicaOfErr error
}

func (m *mockNode) GetObjectKind() schema.ObjectKind { return nil }
func (m *mockNode) Definition() *corev1.Pod          { return nil }
func (m *mockNode) ID() string                       { return m.GetName() }
func (m *mockNode) Index() int                       { return 0 }
func (m *mockNode) IsConnected() bool                { return true }
func (m *mockNode) IsTerminating() bool              { return false }
func (m *mockNode) IsMasterLinkUp() bool             { return true }
func (m *mockNode) IsReady() bool                    { return true }
func (m *mockNode) IsJoined() bool                   { return true }
func (m *mockNode) MasterID() string                 { return "" }
func (m *mockNode) IsMasterFailed() bool             { return false }
func (m *mockNode) CurrentVersion() version.ValkeyVersion { return version.ValkeyVersion("8.0") }
func (m *mockNode) IsACLApplied() bool               { return true }
func (m *mockNode) Role() core.NodeRole              { return m.role }
func (m *mockNode) Slots() *slot.Slots               { return nil }
func (m *mockNode) Config() map[string]string        { return nil }
func (m *mockNode) ConfigedMasterIP() string         { return "" }
func (m *mockNode) ConfigedMasterPort() string       { return "" }
func (m *mockNode) Setup(ctx context.Context, margs ...[]any) error { return nil }
func (m *mockNode) ReplicaOf(ctx context.Context, ip, port string) error { return m.replicaOfErr }
func (m *mockNode) SetACLUser(ctx context.Context, username string, passwords []string, rules string) (any, error) {
	return nil, nil
}
func (m *mockNode) Query(ctx context.Context, cmd string, args ...any) (any, error) {
	return nil, nil
}
func (m *mockNode) Info() vkcli.NodeInfo                  { return vkcli.NodeInfo{} }
func (m *mockNode) ClusterInfo() vkcli.ClusterNodeInfo    { return vkcli.ClusterNodeInfo{} }
func (m *mockNode) IPort() int                            { return m.port }
func (m *mockNode) InternalIPort() int                    { return m.port }
func (m *mockNode) Port() int                             { return m.port }
func (m *mockNode) InternalPort() int                     { return m.port }
func (m *mockNode) DefaultIP() net.IP                     { return net.ParseIP(m.ip) }
func (m *mockNode) DefaultInternalIP() net.IP             { return net.ParseIP(m.ip) }
func (m *mockNode) IPs() []net.IP                         { return []net.IP{net.ParseIP(m.ip)} }
func (m *mockNode) NodeIP() net.IP                        { return net.ParseIP(m.ip) }
func (m *mockNode) Status() corev1.PodPhase               { return corev1.PodRunning }
func (m *mockNode) ContainerStatus() *corev1.ContainerStatus { return nil }
func (m *mockNode) Refresh(ctx context.Context) error     { return nil }

var _ types.ValkeyNode = (*mockNode)(nil)

// ---- mockFailoverInstance implements types.FailoverInstance ----
type mockFailoverInstance struct {
	*v1alpha1.Failover
	nodes   []types.ValkeyNode
	masters []types.ValkeyNode
}

func (m *mockFailoverInstance) NamespacedName() client.ObjectKey {
	return client.ObjectKey{Name: m.GetName(), Namespace: m.GetNamespace()}
}

func (m *mockFailoverInstance) Version() version.ValkeyVersion     { return version.ValkeyVersion("8.0") }
func (m *mockFailoverInstance) SafeVersion() version.ValkeyVersion { return version.ValkeyVersion("8.0") }
func (m *mockFailoverInstance) IsReady() bool                  { return true }
func (m *mockFailoverInstance) Restart(ctx context.Context, annotationKeyVal ...string) error {
	return nil
}
func (m *mockFailoverInstance) Refresh(ctx context.Context) error             { return nil }
func (m *mockFailoverInstance) Arch() core.Arch                               { return core.ValkeyFailover }
func (m *mockFailoverInstance) Issuer() *certmetav1.ObjectReference           { return nil }
func (m *mockFailoverInstance) Users() types.Users                           { return nil }
func (m *mockFailoverInstance) TLSConfig() *tls.Config                        { return nil }
func (m *mockFailoverInstance) IsInService() bool                            { return true }
func (m *mockFailoverInstance) IsACLUserExists() bool                        { return false }
func (m *mockFailoverInstance) IsACLAppliedToAll() bool                      { return false }
func (m *mockFailoverInstance) IsResourceFullfilled(ctx context.Context) (bool, error) {
	return true, nil
}
func (m *mockFailoverInstance) UpdateStatus(ctx context.Context, st types.InstanceStatus, message string) error {
	return nil
}
func (m *mockFailoverInstance) SendEventf(eventtype, reason, messageFmt string, args ...any) {}
func (m *mockFailoverInstance) Logger() logr.Logger                          { return logr.Discard() }
func (m *mockFailoverInstance) Definition() *v1alpha1.Failover               { return m.Failover }
func (m *mockFailoverInstance) Replication() types.Replication               { return nil }
func (m *mockFailoverInstance) Masters() []types.ValkeyNode                  { return m.masters }
func (m *mockFailoverInstance) Nodes() []types.ValkeyNode                    { return m.nodes }
func (m *mockFailoverInstance) RawNodes(ctx context.Context) ([]corev1.Pod, error) {
	return nil, nil
}
func (m *mockFailoverInstance) Monitor() types.FailoverMonitor { return nil }
func (m *mockFailoverInstance) IsBindedSentinel() bool         { return false }
func (m *mockFailoverInstance) IsStandalone() bool             { return false }
func (m *mockFailoverInstance) Selector() map[string]string    { return nil }

func newMockFailoverInstance(nodes, masters []types.ValkeyNode) *mockFailoverInstance {
	return &mockFailoverInstance{
		Failover: &v1alpha1.Failover{
			TypeMeta: metav1.TypeMeta{Kind: "Failover", APIVersion: "valkey.buf.red/v1alpha1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
			},
		},
		nodes:   nodes,
		masters: masters,
	}
}

func notFoundConfigMap() (*corev1.ConfigMap, error) {
	return nil, kerrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "rf-ha-repl-test")
}

// TestManualMonitorFailover_SuccessReturnsNil is the R3 regression: a successful
// promotion must return nil, not the "No available node to failover" error.
func TestManualMonitorFailover_SuccessReturnsNil(t *testing.T) {
	ctx := context.Background()
	node := &mockNode{
		ObjectMeta: metav1.ObjectMeta{Name: "node0"},
		ip:         "10.0.0.1",
		port:       6379,
		role:       core.NodeRoleReplica,
	}
	inst := newMockFailoverInstance([]types.ValkeyNode{node}, []types.ValkeyNode{node})

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
	node := &mockNode{
		ObjectMeta:   metav1.ObjectMeta{Name: "node0"},
		ip:           "10.0.0.1",
		port:         6379,
		role:         core.NodeRoleReplica,
		replicaOfErr: fmt.Errorf("replicaof failed"),
	}
	inst := newMockFailoverInstance([]types.ValkeyNode{node}, []types.ValkeyNode{node})

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
	inst := newMockFailoverInstance(nil, nil)

	clientMock := &mocks.ClientSet{}
	clientMock.On("GetConfigMap", ctx, "default", "rf-ha-repl-test").Return(notFoundConfigMap())

	mon, err := NewManualMonitor(ctx, clientMock, inst)
	require.NoError(t, err)

	err = mon.Failover(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "No available node to failover")
}
