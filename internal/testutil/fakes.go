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

// Package testutil provides configurable fakes for the pkg/types interfaces,
// shared across unit tests instead of being re-implemented per package.
package testutil

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/pkg/slot"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/types/user"
	vkcli "github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/chideat/valkey-operator/pkg/version"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ---------------------------------------------------------------------------
// FakeValkeyNode
// ---------------------------------------------------------------------------

// FakeValkeyNode is a configurable types.ValkeyNode for tests. Only the
// fields exercised by current tests are configurable; the rest return inert
// defaults.
type FakeValkeyNode struct {
	metav1.ObjectMeta

	ip           string
	port         int
	role         core.NodeRole
	replicaOfErr error
}

var _ types.ValkeyNode = (*FakeValkeyNode)(nil)

// NewFakeValkeyNode builds a node with the given name/ip/port/role.
func NewFakeValkeyNode(name, ip string, port int, role core.NodeRole) *FakeValkeyNode {
	return &FakeValkeyNode{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		ip:         ip,
		port:       port,
		role:       role,
	}
}

// WithReplicaOfErr makes ReplicaOf return the given error.
func (m *FakeValkeyNode) WithReplicaOfErr(err error) *FakeValkeyNode {
	m.replicaOfErr = err
	return m
}

func (m *FakeValkeyNode) GetObjectKind() schema.ObjectKind                { return nil }
func (m *FakeValkeyNode) Definition() *corev1.Pod                         { return nil }
func (m *FakeValkeyNode) ID() string                                      { return m.GetName() }
func (m *FakeValkeyNode) Index() int                                      { return 0 }
func (m *FakeValkeyNode) IsConnected() bool                               { return true }
func (m *FakeValkeyNode) IsTerminating() bool                             { return false }
func (m *FakeValkeyNode) IsMasterLinkUp() bool                            { return true }
func (m *FakeValkeyNode) IsReady() bool                                   { return true }
func (m *FakeValkeyNode) IsJoined() bool                                  { return true }
func (m *FakeValkeyNode) MasterID() string                                { return "" }
func (m *FakeValkeyNode) IsMasterFailed() bool                            { return false }
func (m *FakeValkeyNode) CurrentVersion() version.ValkeyVersion           { return version.ValkeyVersion("8.0") }
func (m *FakeValkeyNode) IsACLApplied() bool                              { return true }
func (m *FakeValkeyNode) Role() core.NodeRole                             { return m.role }
func (m *FakeValkeyNode) Slots() *slot.Slots                              { return nil }
func (m *FakeValkeyNode) Config() map[string]string                       { return nil }
func (m *FakeValkeyNode) ConfigedMasterIP() string                        { return "" }
func (m *FakeValkeyNode) ConfigedMasterPort() string                      { return "" }
func (m *FakeValkeyNode) Setup(ctx context.Context, margs ...[]any) error { return nil }
func (m *FakeValkeyNode) ReplicaOf(ctx context.Context, ip, port string) error {
	return m.replicaOfErr
}
func (m *FakeValkeyNode) SetACLUser(ctx context.Context, username string, passwords []string, rules string) (any, error) {
	return nil, nil
}
func (m *FakeValkeyNode) Query(ctx context.Context, cmd string, args ...any) (any, error) {
	return nil, nil
}
func (m *FakeValkeyNode) Info() vkcli.NodeInfo                     { return vkcli.NodeInfo{} }
func (m *FakeValkeyNode) ClusterInfo() vkcli.ClusterNodeInfo       { return vkcli.ClusterNodeInfo{} }
func (m *FakeValkeyNode) IPort() int                               { return m.port }
func (m *FakeValkeyNode) InternalIPort() int                       { return m.port }
func (m *FakeValkeyNode) Port() int                                { return m.port }
func (m *FakeValkeyNode) InternalPort() int                        { return m.port }
func (m *FakeValkeyNode) DefaultIP() net.IP                        { return net.ParseIP(m.ip) }
func (m *FakeValkeyNode) DefaultInternalIP() net.IP                { return net.ParseIP(m.ip) }
func (m *FakeValkeyNode) IPs() []net.IP                            { return []net.IP{net.ParseIP(m.ip)} }
func (m *FakeValkeyNode) NodeIP() net.IP                           { return net.ParseIP(m.ip) }
func (m *FakeValkeyNode) Status() corev1.PodPhase                  { return corev1.PodRunning }
func (m *FakeValkeyNode) ContainerStatus() *corev1.ContainerStatus { return nil }
func (m *FakeValkeyNode) Refresh(ctx context.Context) error        { return nil }

// ---------------------------------------------------------------------------
// FakeFailoverInstance
// ---------------------------------------------------------------------------

// FakeFailoverInstance is a configurable types.FailoverInstance for tests. It
// embeds *v1alpha1.Failover for the metav1.Object / runtime.Object surface; the
// remaining behaviour is set through the With* chainable setters.
type FakeFailoverInstance struct {
	*v1alpha1.Failover

	version    version.ValkeyVersion
	nodes      []types.ValkeyNode
	masters    []types.ValkeyNode
	monitor    types.FailoverMonitor
	restartErr error
	selector   map[string]string
	users      types.Users
	events     []string
}

var _ types.FailoverInstance = (*FakeFailoverInstance)(nil)

// NewFakeFailoverInstance wraps the given Failover definition; version defaults
// to 8.0.
func NewFakeFailoverInstance(failover *v1alpha1.Failover) *FakeFailoverInstance {
	return &FakeFailoverInstance{
		Failover: failover,
		version:  version.ValkeyVersion("8.0"),
	}
}

func (m *FakeFailoverInstance) WithVersion(v version.ValkeyVersion) *FakeFailoverInstance {
	m.version = v
	return m
}
func (m *FakeFailoverInstance) WithNodes(nodes ...types.ValkeyNode) *FakeFailoverInstance {
	m.nodes = nodes
	return m
}
func (m *FakeFailoverInstance) WithMasters(masters ...types.ValkeyNode) *FakeFailoverInstance {
	m.masters = masters
	return m
}
func (m *FakeFailoverInstance) WithMonitor(mon types.FailoverMonitor) *FakeFailoverInstance {
	m.monitor = mon
	return m
}
func (m *FakeFailoverInstance) WithRestartErr(err error) *FakeFailoverInstance {
	m.restartErr = err
	return m
}
func (m *FakeFailoverInstance) WithSelector(sel map[string]string) *FakeFailoverInstance {
	m.selector = sel
	return m
}
func (m *FakeFailoverInstance) WithUsers(users ...*user.User) *FakeFailoverInstance {
	m.users = users
	return m
}

// Events returns what SendEventf received, as "type reason message".
func (m *FakeFailoverInstance) Events() []string { return m.events }

func (m *FakeFailoverInstance) NamespacedName() client.ObjectKey {
	return client.ObjectKeyFromObject(m.Failover)
}
func (m *FakeFailoverInstance) Version() version.ValkeyVersion     { return m.version }
func (m *FakeFailoverInstance) SafeVersion() version.ValkeyVersion { return m.version }
func (m *FakeFailoverInstance) IsReady() bool                      { return true }
func (m *FakeFailoverInstance) Restart(ctx context.Context, annotationKeyVal ...string) error {
	return m.restartErr
}
func (m *FakeFailoverInstance) Refresh(ctx context.Context) error   { return nil }
func (m *FakeFailoverInstance) Arch() core.Arch                     { return core.ValkeyFailover }
func (m *FakeFailoverInstance) Issuer() *certmetav1.IssuerReference { return nil }
func (m *FakeFailoverInstance) Users() types.Users                  { return m.users }
func (m *FakeFailoverInstance) TLSConfig() *tls.Config              { return nil }
func (m *FakeFailoverInstance) IsInService() bool                   { return true }
func (m *FakeFailoverInstance) IsACLUserExists() bool               { return false }
func (m *FakeFailoverInstance) IsACLAppliedToAll() bool             { return false }
func (m *FakeFailoverInstance) IsResourceFullfilled(ctx context.Context) (bool, error) {
	return true, nil
}
func (m *FakeFailoverInstance) UpdateStatus(ctx context.Context, st types.InstanceStatus, message string) error {
	return nil
}
func (m *FakeFailoverInstance) SendEventf(eventtype, reason, messageFmt string, args ...any) {
	m.events = append(m.events, eventtype+" "+reason+" "+fmt.Sprintf(messageFmt, args...))
}
func (m *FakeFailoverInstance) Logger() logr.Logger            { return logr.Discard() }
func (m *FakeFailoverInstance) Definition() *v1alpha1.Failover { return m.Failover }
func (m *FakeFailoverInstance) Replication() types.Replication { return nil }
func (m *FakeFailoverInstance) Masters() []types.ValkeyNode    { return m.masters }
func (m *FakeFailoverInstance) Nodes() []types.ValkeyNode      { return m.nodes }
func (m *FakeFailoverInstance) RawNodes(ctx context.Context) ([]corev1.Pod, error) {
	return nil, nil
}
func (m *FakeFailoverInstance) Monitor() types.FailoverMonitor { return m.monitor }
func (m *FakeFailoverInstance) IsBindedSentinel() bool         { return false }
func (m *FakeFailoverInstance) IsStandalone() bool             { return false }
func (m *FakeFailoverInstance) Selector() map[string]string    { return m.selector }

// ---------------------------------------------------------------------------
// FakeClusterInstance
// ---------------------------------------------------------------------------

// FakeClusterInstance is a configurable types.ClusterInstance for tests. It
// embeds *v1alpha1.Cluster for the metav1.Object / runtime.Object surface; the
// remaining behaviour is set through the With* chainable setters.
type FakeClusterInstance struct {
	*v1alpha1.Cluster

	version version.ValkeyVersion
	users   types.Users
	events  []string
}

var _ types.ClusterInstance = (*FakeClusterInstance)(nil)

// NewFakeClusterInstance wraps the given Cluster definition; version defaults
// to 8.0.
func NewFakeClusterInstance(cluster *v1alpha1.Cluster) *FakeClusterInstance {
	return &FakeClusterInstance{
		Cluster: cluster,
		version: version.ValkeyVersion("8.0"),
	}
}

func (m *FakeClusterInstance) WithUsers(users ...*user.User) *FakeClusterInstance {
	m.users = users
	return m
}

// Events returns what SendEventf received, as "type reason message".
func (m *FakeClusterInstance) Events() []string { return m.events }

func (m *FakeClusterInstance) NamespacedName() client.ObjectKey {
	return client.ObjectKeyFromObject(m.Cluster)
}
func (m *FakeClusterInstance) Version() version.ValkeyVersion     { return m.version }
func (m *FakeClusterInstance) SafeVersion() version.ValkeyVersion { return m.version }
func (m *FakeClusterInstance) IsReady() bool                      { return true }
func (m *FakeClusterInstance) Restart(ctx context.Context, annotationKeyVal ...string) error {
	return nil
}
func (m *FakeClusterInstance) Refresh(ctx context.Context) error   { return nil }
func (m *FakeClusterInstance) Arch() core.Arch                     { return core.ValkeyCluster }
func (m *FakeClusterInstance) Issuer() *certmetav1.IssuerReference { return nil }
func (m *FakeClusterInstance) Users() types.Users                  { return m.users }
func (m *FakeClusterInstance) TLSConfig() *tls.Config              { return nil }
func (m *FakeClusterInstance) IsInService() bool                   { return true }
func (m *FakeClusterInstance) IsACLUserExists() bool               { return false }
func (m *FakeClusterInstance) IsACLAppliedToAll() bool             { return false }
func (m *FakeClusterInstance) IsResourceFullfilled(ctx context.Context) (bool, error) {
	return true, nil
}
func (m *FakeClusterInstance) UpdateStatus(ctx context.Context, st types.InstanceStatus, message string) error {
	return nil
}
func (m *FakeClusterInstance) SendEventf(eventtype, reason, messageFmt string, args ...any) {
	m.events = append(m.events, eventtype+" "+reason+" "+fmt.Sprintf(messageFmt, args...))
}
func (m *FakeClusterInstance) Logger() logr.Logger             { return logr.Discard() }
func (m *FakeClusterInstance) Definition() *v1alpha1.Cluster   { return m.Cluster }
func (m *FakeClusterInstance) Status() *v1alpha1.ClusterStatus { return &m.Cluster.Status }
func (m *FakeClusterInstance) Masters() []types.ValkeyNode     { return nil }
func (m *FakeClusterInstance) Nodes() []types.ValkeyNode       { return nil }
func (m *FakeClusterInstance) RawNodes(ctx context.Context) ([]corev1.Pod, error) {
	return nil, nil
}
func (m *FakeClusterInstance) Shards() []types.ClusterShard       { return nil }
func (m *FakeClusterInstance) Shard(index int) types.ClusterShard { return nil }
func (m *FakeClusterInstance) RewriteShards(ctx context.Context, shards []*v1alpha1.ClusterShards) error {
	return nil
}

// ---------------------------------------------------------------------------
// FakeSentinelInstance
// ---------------------------------------------------------------------------

// FakeSentinelInstance is a configurable types.SentinelInstance for tests. It
// embeds *v1alpha1.Sentinel for the metav1.Object / runtime.Object surface; the
// remaining behaviour is set through the With* chainable setters.
type FakeSentinelInstance struct {
	*v1alpha1.Sentinel

	version version.ValkeyVersion
	users   types.Users
	events  []string
}

var _ types.SentinelInstance = (*FakeSentinelInstance)(nil)

// NewFakeSentinelInstance wraps the given Sentinel definition; version
// defaults to 8.0.
func NewFakeSentinelInstance(sentinel *v1alpha1.Sentinel) *FakeSentinelInstance {
	return &FakeSentinelInstance{
		Sentinel: sentinel,
		version:  version.ValkeyVersion("8.0"),
	}
}

func (m *FakeSentinelInstance) WithUsers(users ...*user.User) *FakeSentinelInstance {
	m.users = users
	return m
}

// Events returns what SendEventf received, as "type reason message".
func (m *FakeSentinelInstance) Events() []string { return m.events }

func (m *FakeSentinelInstance) NamespacedName() client.ObjectKey {
	return client.ObjectKeyFromObject(m.Sentinel)
}
func (m *FakeSentinelInstance) Version() version.ValkeyVersion     { return m.version }
func (m *FakeSentinelInstance) SafeVersion() version.ValkeyVersion { return m.version }
func (m *FakeSentinelInstance) IsReady() bool                      { return true }
func (m *FakeSentinelInstance) Restart(ctx context.Context, annotationKeyVal ...string) error {
	return nil
}
func (m *FakeSentinelInstance) Refresh(ctx context.Context) error   { return nil }
func (m *FakeSentinelInstance) Arch() core.Arch                     { return core.ValkeySentinel }
func (m *FakeSentinelInstance) Issuer() *certmetav1.IssuerReference { return nil }
func (m *FakeSentinelInstance) Users() types.Users                  { return m.users }
func (m *FakeSentinelInstance) TLSConfig() *tls.Config              { return nil }
func (m *FakeSentinelInstance) IsInService() bool                   { return true }
func (m *FakeSentinelInstance) IsACLUserExists() bool               { return false }
func (m *FakeSentinelInstance) IsACLAppliedToAll() bool             { return false }
func (m *FakeSentinelInstance) IsResourceFullfilled(ctx context.Context) (bool, error) {
	return true, nil
}
func (m *FakeSentinelInstance) UpdateStatus(ctx context.Context, st types.InstanceStatus, message string) error {
	return nil
}
func (m *FakeSentinelInstance) SendEventf(eventtype, reason, messageFmt string, args ...any) {
	m.events = append(m.events, eventtype+" "+reason+" "+fmt.Sprintf(messageFmt, args...))
}
func (m *FakeSentinelInstance) Logger() logr.Logger                    { return logr.Discard() }
func (m *FakeSentinelInstance) Definition() *v1alpha1.Sentinel         { return m.Sentinel }
func (m *FakeSentinelInstance) Replication() types.SentinelReplication { return nil }
func (m *FakeSentinelInstance) Nodes() []types.SentinelNode            { return nil }
func (m *FakeSentinelInstance) RawNodes(ctx context.Context) ([]corev1.Pod, error) {
	return nil, nil
}
func (m *FakeSentinelInstance) Clusters(ctx context.Context) ([]string, error) { return nil, nil }
func (m *FakeSentinelInstance) GetPassword() (string, error)                   { return "", nil }
func (m *FakeSentinelInstance) Selector() map[string]string                    { return nil }
