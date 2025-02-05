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
*/package types

import (
	"context"
	"net"
	"strings"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/pkg/slot"
	vkcli "github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/chideat/valkey-operator/pkg/version"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func NewNodeRole(v string) core.NodeRole {
	switch strings.ToLower(v) {
	case "master":
		return core.NodeRoleMaster
	case "slave", "replica":
		return core.NodeRoleReplica
	case "sentinel":
		return core.NodeRoleSentinel
	}
	return core.NodeRoleNone
}

// ValkeyNode
type ValkeyNode interface {
	GetObjectKind() schema.ObjectKind
	metav1.Object

	Definition() *corev1.Pod

	// ID returns cluster node id
	ID() string
	// Index the index of statefulset
	Index() int
	// IsConnected indicate where this node is accessable
	IsConnected() bool
	// IsTerminating indicate whether is pod is deleted
	IsTerminating() bool
	// IsMasterLinkUp indicate whether the master link is up
	IsMasterLinkUp() bool
	// IsReady indicate whether is main container is ready
	IsReady() bool
	// IsJoined will indicate whether this node has joined with other nodes.
	// be sure that, this can't indicate that all pods has joined
	IsJoined() bool
	// MasterID if this node is a slave, return master id it replica to
	MasterID() string
	// IsMasterFailed returns where the master is failed. if itself is master, this func will always return false
	IsMasterFailed() bool
	// CurrentVersion return current valkey server version
	// this value maybe differ with cr def when do version upgrade
	CurrentVersion() version.ValkeyVersion

	// IsACLApplied returns true when the main container got ACL_CONFIGMAP_NAME env
	IsACLApplied() bool

	// Role returns the role of current node
	// be sure that for the new start valkey server, the role is master when in cluster mode
	Role() core.NodeRole
	// Slots if this node is master, returns the slots this nodes assigned
	// else returns nil
	Slots() *slot.Slots

	Config() map[string]string
	ConfigedMasterIP() string
	ConfigedMasterPort() string
	// Setup
	Setup(ctx context.Context, margs ...[]any) error
	ReplicaOf(ctx context.Context, ip, port string) error
	SetACLUser(ctx context.Context, username string, passwords []string, rules string) (interface{}, error)
	Query(ctx context.Context, cmd string, args ...any) (any, error)
	Info() vkcli.NodeInfo
	ClusterInfo() vkcli.ClusterNodeInfo

	IPort() int
	InternalIPort() int
	Port() int
	InternalPort() int
	DefaultIP() net.IP
	DefaultInternalIP() net.IP
	IPs() []net.IP
	NodeIP() net.IP

	Status() corev1.PodPhase
	ContainerStatus() *corev1.ContainerStatus

	Refresh(ctx context.Context) error
}

// SentinelNode
type SentinelNode interface {
	metav1.Object
	SentinelNodeOperation
	GetObjectKind() schema.ObjectKind

	Definition() *corev1.Pod

	// Index the index of statefulset
	Index() int
	// IsTerminating indicate whether is pod is deleted
	IsTerminating() bool
	// IsReady indicate whether is main container is ready
	IsReady() bool
	// IsACLApplied returns true when the main container got ACL_CONFIGMAP_NAME env
	IsACLApplied() bool

	Port() int
	InternalPort() int
	DefaultIP() net.IP
	DefaultInternalIP() net.IP
	IPs() []net.IP
	NodeIP() net.IP

	Status() corev1.PodPhase
	ContainerStatus() *corev1.ContainerStatus
}

type SentinelNodeOperation interface {
	// CurrentVersion return current valkey server version
	// this value maybe differ with cr def when do version upgrade
	CurrentVersion() version.ValkeyVersion

	Refresh(ctx context.Context) error

	Config() map[string]string

	// Setup
	Setup(ctx context.Context, margs ...[]any) error
	SetMonitor(ctx context.Context, name, ip, port, user, password, quorum string) error
	Query(ctx context.Context, cmd string, args ...any) (any, error)
	Info() vkcli.NodeInfo
	// sentinel inspect
	Brothers(ctx context.Context, name string) ([]*vkcli.SentinelMonitorNode, error)
	MonitoringClusters(ctx context.Context) (clusters []string, err error)
	MonitoringNodes(ctx context.Context, name string) (master *vkcli.SentinelMonitorNode, replicas []*vkcli.SentinelMonitorNode, err error)
}
