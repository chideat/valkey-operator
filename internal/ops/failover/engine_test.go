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

package failover

import (
	"context"
	"crypto/tls"
	"net"
	"testing"

	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/pkg/slot"
	"github.com/chideat/valkey-operator/pkg/types"
	vkcli "github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/chideat/valkey-operator/pkg/version"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// --- mockValkeyNode ---

type mockValkeyNode struct {
	name              string
	index             int
	ip                net.IP
	port              int
	internalIP        net.IP
	internalPort      int
	deletionTimestamp *metav1.Time
	gracePeriod       *int64
}

func (m *mockValkeyNode) GetObjectKind() schema.ObjectKind           { return schema.EmptyObjectKind }
func (m *mockValkeyNode) GetName() string                            { return m.name }
func (m *mockValkeyNode) GetNamespace() string                       { return "default" }
func (m *mockValkeyNode) SetName(name string)                        {}
func (m *mockValkeyNode) SetNamespace(ns string)                     {}
func (m *mockValkeyNode) GetGenerateName() string                    { return "" }
func (m *mockValkeyNode) SetGenerateName(name string)                {}
func (m *mockValkeyNode) GetUID() k8stypes.UID                       { return "" }
func (m *mockValkeyNode) SetUID(uid k8stypes.UID)                    {}
func (m *mockValkeyNode) GetResourceVersion() string                 { return "" }
func (m *mockValkeyNode) SetResourceVersion(version string)          {}
func (m *mockValkeyNode) GetGeneration() int64                       { return 0 }
func (m *mockValkeyNode) SetGeneration(generation int64)             {}
func (m *mockValkeyNode) GetSelfLink() string                        { return "" }
func (m *mockValkeyNode) SetSelfLink(selfLink string)                {}
func (m *mockValkeyNode) GetCreationTimestamp() metav1.Time          { return metav1.Time{} }
func (m *mockValkeyNode) SetCreationTimestamp(timestamp metav1.Time) {}
func (m *mockValkeyNode) GetDeletionTimestamp() *metav1.Time         { return m.deletionTimestamp }
func (m *mockValkeyNode) SetDeletionTimestamp(timestamp *metav1.Time) {
	m.deletionTimestamp = timestamp
}
func (m *mockValkeyNode) GetDeletionGracePeriodSeconds() *int64           { return m.gracePeriod }
func (m *mockValkeyNode) SetDeletionGracePeriodSeconds(i *int64)          { m.gracePeriod = i }
func (m *mockValkeyNode) GetLabels() map[string]string                    { return nil }
func (m *mockValkeyNode) SetLabels(labels map[string]string)              {}
func (m *mockValkeyNode) GetAnnotations() map[string]string               { return nil }
func (m *mockValkeyNode) SetAnnotations(annotations map[string]string)    {}
func (m *mockValkeyNode) GetFinalizers() []string                         { return nil }
func (m *mockValkeyNode) SetFinalizers(finalizers []string)               {}
func (m *mockValkeyNode) GetOwnerReferences() []metav1.OwnerReference     { return nil }
func (m *mockValkeyNode) SetOwnerReferences(refs []metav1.OwnerReference) {}
func (m *mockValkeyNode) GetClusterName() string                          { return "" }
func (m *mockValkeyNode) SetClusterName(clusterName string)               {}
func (m *mockValkeyNode) GetManagedFields() []metav1.ManagedFieldsEntry   { return nil }
func (m *mockValkeyNode) SetManagedFields(mf []metav1.ManagedFieldsEntry) {}

func (m *mockValkeyNode) Definition() *corev1.Pod { return nil }
func (m *mockValkeyNode) ID() string              { return m.name }
func (m *mockValkeyNode) Index() int              { return m.index }
func (m *mockValkeyNode) IsConnected() bool       { return true }
func (m *mockValkeyNode) IsTerminating() bool {
	return m.deletionTimestamp != nil
}
func (m *mockValkeyNode) IsMasterLinkUp() bool                                 { return true }
func (m *mockValkeyNode) IsReady() bool                                        { return true }
func (m *mockValkeyNode) IsJoined() bool                                       { return true }
func (m *mockValkeyNode) MasterID() string                                     { return "" }
func (m *mockValkeyNode) IsMasterFailed() bool                                 { return false }
func (m *mockValkeyNode) CurrentVersion() version.ValkeyVersion                { return version.ValkeyVersion("7.2") }
func (m *mockValkeyNode) IsACLApplied() bool                                   { return false }
func (m *mockValkeyNode) Role() core.NodeRole                                  { return core.NodeRoleReplica }
func (m *mockValkeyNode) Slots() *slot.Slots                                   { return nil }
func (m *mockValkeyNode) Config() map[string]string                            { return nil }
func (m *mockValkeyNode) ConfigedMasterIP() string                             { return "" }
func (m *mockValkeyNode) ConfigedMasterPort() string                           { return "" }
func (m *mockValkeyNode) Setup(ctx context.Context, margs ...[]any) error      { return nil }
func (m *mockValkeyNode) ReplicaOf(ctx context.Context, ip, port string) error { return nil }
func (m *mockValkeyNode) SetACLUser(ctx context.Context, username string, passwords []string, rules string) (any, error) {
	return nil, nil
}
func (m *mockValkeyNode) Query(ctx context.Context, cmd string, args ...any) (any, error) {
	return nil, nil
}
func (m *mockValkeyNode) Info() vkcli.NodeInfo                     { return vkcli.NodeInfo{} }
func (m *mockValkeyNode) ClusterInfo() vkcli.ClusterNodeInfo       { return vkcli.ClusterNodeInfo{} }
func (m *mockValkeyNode) IPort() int                               { return m.port }
func (m *mockValkeyNode) InternalIPort() int                       { return m.internalPort }
func (m *mockValkeyNode) Port() int                                { return m.port }
func (m *mockValkeyNode) InternalPort() int                        { return m.internalPort }
func (m *mockValkeyNode) DefaultIP() net.IP                        { return m.ip }
func (m *mockValkeyNode) DefaultInternalIP() net.IP                { return m.internalIP }
func (m *mockValkeyNode) IPs() []net.IP                            { return []net.IP{m.ip} }
func (m *mockValkeyNode) NodeIP() net.IP                           { return m.ip }
func (m *mockValkeyNode) Status() corev1.PodPhase                  { return corev1.PodRunning }
func (m *mockValkeyNode) ContainerStatus() *corev1.ContainerStatus { return nil }
func (m *mockValkeyNode) Refresh(ctx context.Context) error        { return nil }

// --- mockFailoverMonitor ---

type mockFailoverMonitor struct {
	master        *vkcli.SentinelMonitorNode
	allMonitored  bool
	allMonitorErr error
}

func (m *mockFailoverMonitor) Policy() v1alpha1.FailoverPolicy {
	return v1alpha1.SentinelFailoverPolicy
}
func (m *mockFailoverMonitor) Master(ctx context.Context, flags ...bool) (*vkcli.SentinelMonitorNode, error) {
	return m.master, nil
}
func (m *mockFailoverMonitor) Replicas(ctx context.Context) ([]*vkcli.SentinelMonitorNode, error) {
	return nil, nil
}
func (m *mockFailoverMonitor) Inited(ctx context.Context) (bool, error) { return true, nil }
func (m *mockFailoverMonitor) AllNodeMonitored(ctx context.Context) (bool, error) {
	return m.allMonitored, m.allMonitorErr
}
func (m *mockFailoverMonitor) UpdateConfig(ctx context.Context, params map[string]string) error {
	return nil
}
func (m *mockFailoverMonitor) Failover(ctx context.Context) error { return nil }
func (m *mockFailoverMonitor) Monitor(ctx context.Context, node types.ValkeyNode) error {
	return nil
}
func (m *mockFailoverMonitor) Reset(ctx context.Context) error { return nil }

// --- mockFailoverInstance ---

type testFailoverInstance struct {
	failover *v1alpha1.Failover
	nodes    []types.ValkeyNode
	rawNodes []corev1.Pod
	monitor  types.FailoverMonitor
}

// types.Object / metav1.Object - delegate to embedded Failover
func (m *testFailoverInstance) GetObjectKind() schema.ObjectKind                { return m.failover.GetObjectKind() }
func (m *testFailoverInstance) DeepCopyObject() runtime.Object                  { return m.failover.DeepCopy() }
func (m *testFailoverInstance) GetName() string                                 { return m.failover.GetName() }
func (m *testFailoverInstance) GetNamespace() string                            { return m.failover.GetNamespace() }
func (m *testFailoverInstance) SetName(name string)                             { m.failover.SetName(name) }
func (m *testFailoverInstance) SetNamespace(ns string)                          { m.failover.SetNamespace(ns) }
func (m *testFailoverInstance) GetGenerateName() string                         { return "" }
func (m *testFailoverInstance) SetGenerateName(name string)                     {}
func (m *testFailoverInstance) GetUID() k8stypes.UID                            { return m.failover.GetUID() }
func (m *testFailoverInstance) SetUID(uid k8stypes.UID)                         {}
func (m *testFailoverInstance) GetResourceVersion() string                      { return "" }
func (m *testFailoverInstance) SetResourceVersion(v string)                     {}
func (m *testFailoverInstance) GetGeneration() int64                            { return 0 }
func (m *testFailoverInstance) SetGeneration(g int64)                           {}
func (m *testFailoverInstance) GetSelfLink() string                             { return "" }
func (m *testFailoverInstance) SetSelfLink(s string)                            {}
func (m *testFailoverInstance) GetCreationTimestamp() metav1.Time               { return metav1.Time{} }
func (m *testFailoverInstance) SetCreationTimestamp(t metav1.Time)              {}
func (m *testFailoverInstance) GetDeletionTimestamp() *metav1.Time              { return nil }
func (m *testFailoverInstance) SetDeletionTimestamp(t *metav1.Time)             {}
func (m *testFailoverInstance) GetDeletionGracePeriodSeconds() *int64           { return nil }
func (m *testFailoverInstance) SetDeletionGracePeriodSeconds(i *int64)          {}
func (m *testFailoverInstance) GetLabels() map[string]string                    { return nil }
func (m *testFailoverInstance) SetLabels(labels map[string]string)              {}
func (m *testFailoverInstance) GetAnnotations() map[string]string               { return nil }
func (m *testFailoverInstance) SetAnnotations(a map[string]string)              {}
func (m *testFailoverInstance) GetFinalizers() []string                         { return nil }
func (m *testFailoverInstance) SetFinalizers(f []string)                        {}
func (m *testFailoverInstance) GetOwnerReferences() []metav1.OwnerReference     { return nil }
func (m *testFailoverInstance) SetOwnerReferences(r []metav1.OwnerReference)    {}
func (m *testFailoverInstance) GetClusterName() string                          { return "" }
func (m *testFailoverInstance) SetClusterName(s string)                         {}
func (m *testFailoverInstance) GetManagedFields() []metav1.ManagedFieldsEntry   { return nil }
func (m *testFailoverInstance) SetManagedFields(mf []metav1.ManagedFieldsEntry) {}

func (m *testFailoverInstance) NamespacedName() client.ObjectKey {
	return client.ObjectKeyFromObject(m.failover)
}
func (m *testFailoverInstance) Version() version.ValkeyVersion { return version.ValkeyVersion("7.2") }
func (m *testFailoverInstance) IsReady() bool                  { return true }
func (m *testFailoverInstance) Restart(ctx context.Context, annotationKeyVal ...string) error {
	return nil
}
func (m *testFailoverInstance) Refresh(ctx context.Context) error { return nil }

// types.Instance
func (m *testFailoverInstance) Arch() core.Arch                     { return core.ValkeyFailover }
func (m *testFailoverInstance) Issuer() *certmetav1.ObjectReference { return nil }
func (m *testFailoverInstance) Users() types.Users                  { return nil }
func (m *testFailoverInstance) TLSConfig() *tls.Config              { return nil }
func (m *testFailoverInstance) IsInService() bool                   { return true }
func (m *testFailoverInstance) IsACLUserExists() bool               { return false }
func (m *testFailoverInstance) IsACLAppliedToAll() bool             { return false }
func (m *testFailoverInstance) IsResourceFullfilled(ctx context.Context) (bool, error) {
	return true, nil
}
func (m *testFailoverInstance) UpdateStatus(ctx context.Context, st types.InstanceStatus, message string) error {
	return nil
}
func (m *testFailoverInstance) SendEventf(eventtype, reason, messageFmt string, args ...any) {}
func (m *testFailoverInstance) Logger() logr.Logger                                          { return logr.Discard() }

// types.FailoverInstance
func (m *testFailoverInstance) Definition() *v1alpha1.Failover { return m.failover }
func (m *testFailoverInstance) Replication() types.Replication { return nil }
func (m *testFailoverInstance) Masters() []types.ValkeyNode    { return nil }
func (m *testFailoverInstance) Nodes() []types.ValkeyNode      { return m.nodes }
func (m *testFailoverInstance) RawNodes(ctx context.Context) ([]corev1.Pod, error) {
	return m.rawNodes, nil
}
func (m *testFailoverInstance) Monitor() types.FailoverMonitor { return m.monitor }
func (m *testFailoverInstance) IsBindedSentinel() bool         { return false }
func (m *testFailoverInstance) IsStandalone() bool             { return false }
func (m *testFailoverInstance) Selector() map[string]string {
	return map[string]string{"app": "valkey"}
}

// --- helpers ---

func makePod(name, ip string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			PodIP: ip,
		},
	}
}

func newNode(name string, index int, ip string, port int) *mockValkeyNode {
	parsed := net.ParseIP(ip)
	return &mockValkeyNode{
		name:         name,
		index:        index,
		ip:           parsed,
		port:         port,
		internalIP:   parsed,
		internalPort: port,
	}
}

func newFailoverInstance(nodes []types.ValkeyNode, rawPods []corev1.Pod, mon types.FailoverMonitor) *testFailoverInstance {
	return &testFailoverInstance{
		failover: &v1alpha1.Failover{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ha",
				Namespace: "default",
			},
			Spec: v1alpha1.FailoverSpec{
				Access: core.InstanceAccess{
					ServiceType: corev1.ServiceTypeClusterIP,
				},
			},
		},
		nodes:    nodes,
		rawNodes: rawPods,
		monitor:  mon,
	}
}

// --- tests ---

// TestIsNodesHealthy_IndexMismatch_WhenMasterNodeUnreachable:
// Only ha-1 is in inst.Nodes() (ha-0 was unreachable at LoadNodes time).
// Sentinel reports ha-1 as master (10.0.0.2); masterNode search finds ha-1 in
// inst.Nodes() and the match succeeds. The index-mismatch loop (engine.go
// lines 324-329) then runs: with only ha-1 in the slice, i=0 but ha1.Index()=1
// → currently returns CommandHealPod.
// After the fix (skip check when len(inst.Nodes()) < len(rawPods)) it should
// return nil, allowing isPatchLabelNeeded to run.
func TestIsNodesHealthy_IndexMismatch_WhenMasterNodeUnreachable(t *testing.T) {
	ctx := context.Background()

	// Only ha-1 is connected (index=1)
	ha1 := newNode("ha-1", 1, "10.0.0.2", 6379)

	mon := &mockFailoverMonitor{
		allMonitored: true,
		// Sentinel reports ha-1 as master
		master: &vkcli.SentinelMonitorNode{IP: "10.0.0.2", Port: "6379"},
	}

	inst := newFailoverInstance(
		[]types.ValkeyNode{ha1},
		[]corev1.Pod{
			makePod("ha-0", "10.0.0.1"),
			makePod("ha-1", "10.0.0.2"),
		},
		mon,
	)

	engine := &RuleEngine{logger: logr.Discard()}
	result := engine.isNodesHealthy(ctx, inst, logr.Discard())

	// Currently FAILS: returns CommandHealPod due to i=0 != node.Index()=1
	assert.Nil(t, result, "expected nil (master is reachable, pod just missing from Nodes slice)")
}

// TestIsNodesHealthy_MasterNil_WhenMasterPodUnreachable:
// Sentinel reports ha-0 as master (10.0.0.1). ha-0 is not in inst.Nodes()
// because it was unreachable at LoadNodes time (AllNodeMonitored=true, so the
// monitor check passes). masterNode search returns nil because ha-0 is absent
// from the slice. Current code returns CommandHealMonitor.
// After the fix (check if the unreachable pod exists in RawNodes and return nil
// instead) isPatchLabelNeeded is allowed to run.
func TestIsNodesHealthy_MasterNil_WhenMasterPodUnreachable(t *testing.T) {
	ctx := context.Background()

	// Only ha-1 is connected (index=1)
	ha1 := newNode("ha-1", 1, "10.0.0.2", 6379)

	mon := &mockFailoverMonitor{
		allMonitored: true,
		// Sentinel still reports ha-0 as master (old master, not yet failed over)
		master: &vkcli.SentinelMonitorNode{IP: "10.0.0.1", Port: "6379"},
	}

	inst := newFailoverInstance(
		[]types.ValkeyNode{ha1},
		[]corev1.Pod{
			makePod("ha-0", "10.0.0.1"),
			makePod("ha-1", "10.0.0.2"),
		},
		mon,
	)

	engine := &RuleEngine{logger: logr.Discard()}
	result := engine.isNodesHealthy(ctx, inst, logr.Discard())

	// Currently FAILS: masterNode is nil (ha-0 not in inst.Nodes()) → returns CommandHealMonitor
	assert.Nil(t, result, "expected nil (master pod is just unreachable, not a split-brain)")
}

// TestIsNodesHealthy_IndexMismatch_AllReachable_StillHeals:
// Both ha-0 and ha-1 are reachable but appear in wrong order in Nodes() slice.
// i=0 → node.Index()=1, genuine mismatch → should return CommandHealPod.
// This tests existing (correct) behavior and should PASS before and after the fix.
func TestIsNodesHealthy_IndexMismatch_AllReachable_StillHeals(t *testing.T) {
	ctx := context.Background()

	// Both nodes connected, but slice order is wrong (ha-1 first, then ha-0)
	ha1 := newNode("ha-1", 1, "10.0.0.2", 6379)
	ha0 := newNode("ha-0", 0, "10.0.0.1", 6379)

	mon := &mockFailoverMonitor{
		allMonitored: true,
		master:       &vkcli.SentinelMonitorNode{IP: "10.0.0.1", Port: "6379"},
	}

	inst := newFailoverInstance(
		[]types.ValkeyNode{ha1, ha0}, // wrong order: ha-1 at index 0 in slice
		[]corev1.Pod{
			makePod("ha-0", "10.0.0.1"),
			makePod("ha-1", "10.0.0.2"),
		},
		mon,
	)

	engine := &RuleEngine{logger: logr.Discard()}
	result := engine.isNodesHealthy(ctx, inst, logr.Discard())

	// Should PASS: genuine mismatch (all pods reachable), returns CommandHealPod
	assert.NotNil(t, result, "expected non-nil result for genuine index mismatch")
	assert.Equal(t, CommandHealPod, result.NextCommand())
}

// Compile-time assertion: testFailoverInstance satisfies types.FailoverInstance
var _ types.FailoverInstance = (*testFailoverInstance)(nil)

// Compile-time assertion: mockValkeyNode satisfies types.ValkeyNode
var _ types.ValkeyNode = (*mockValkeyNode)(nil)

// Compile-time assertion: mockFailoverMonitor satisfies types.FailoverMonitor
var _ types.FailoverMonitor = (*mockFailoverMonitor)(nil)
