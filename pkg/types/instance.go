/*
Copyright 2023 The RedisOperator Authors.

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

package types

import (
	"context"
	"crypto/tls"

	"github.com/chideat/valkey-operator/api/v1alpha1"
	clusterv1 "github.com/chideat/valkey-operator/api/v1alpha1"
	databasesv1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/pkg/redis"
	"github.com/chideat/valkey-operator/pkg/security/acl"
	"github.com/chideat/valkey-operator/pkg/slot"
	rtypes "github.com/chideat/valkey-operator/pkg/types/redis"
	"github.com/go-logr/logr"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Object interface {
	v1.Object
	GetObjectKind() schema.ObjectKind
	DeepCopyObject() runtime.Object
	NamespacedName() client.ObjectKey
	Version() rtypes.RedisVersion
	IsReady() bool

	Restart(ctx context.Context, annotationKeyVal ...string) error
	Refresh(ctx context.Context) error
}

type InstanceStatus string

const (
	Any    InstanceStatus = ""
	OK     InstanceStatus = "OK"
	Fail   InstanceStatus = "Fail"
	Paused InstanceStatus = "Paused"
)

type RedisInstance interface {
	Object

	Arch() rtypes.RedisArch
	Users() acl.Users
	TLSConfig() *tls.Config
	IsInService() bool
	IsACLUserExists() bool
	IsACLAppliedToAll() bool
	IsResourceFullfilled(ctx context.Context) (bool, error)
	UpdateStatus(ctx context.Context, st InstanceStatus, message string) error
	SendEventf(eventtype, reason, messageFmt string, args ...interface{})
	Logger() logr.Logger
}

type RedisReplication interface {
	Object

	Definition() *appv1.StatefulSet
	Status() *appv1.StatefulSetStatus

	// Master returns the master node of this shard which has joined the cluster
	// Keep in mind that, this not means the master has been assigned slots
	Master() rtypes.RedisNode
	// Replicas returns nodes whoses role is slave
	Replicas() []rtypes.RedisNode
	Nodes() []rtypes.RedisNode
}

type RedisSentinel interface {
	Object

	Definition() *appv1.Deployment
	Status() *appv1.DeploymentStatus

	Nodes() []rtypes.RedisSentinelNode
}

type RedisSentinelReplication interface {
	Object

	Definition() *appv1.StatefulSet
	Status() *appv1.StatefulSetStatus

	Nodes() []rtypes.RedisSentinelNode
}

type RedisSentinelInstance interface {
	RedisInstance

	Definition() *v1alpha1.Sentinel
	Replication() RedisSentinelReplication
	Nodes() []rtypes.RedisSentinelNode
	RawNodes(ctx context.Context) ([]corev1.Pod, error)

	// helper methods
	GetPassword() (string, error)

	Selector() map[string]string
}

type FailoverMonitor interface {
	Policy() databasesv1.FailoverPolicy
	Master(ctx context.Context, flags ...bool) (*redis.SentinelMonitorNode, error)
	Replicas(ctx context.Context) ([]*redis.SentinelMonitorNode, error)
	Inited(ctx context.Context) (bool, error)
	AllNodeMonitored(ctx context.Context) (bool, error)
	UpdateConfig(ctx context.Context, params map[string]string) error
	Failover(ctx context.Context) error
	Monitor(ctx context.Context, node rtypes.RedisNode) error
}

type RedisFailoverInstance interface {
	RedisInstance

	Definition() *v1alpha1.Failover
	Masters() []rtypes.RedisNode
	Nodes() []rtypes.RedisNode
	RawNodes(ctx context.Context) ([]corev1.Pod, error)
	Monitor() FailoverMonitor

	IsBindedSentinel() bool
	IsStandalone() bool
	Selector() map[string]string
}

// RedisClusterShard
type RedisClusterShard interface {
	Object

	Definition() *appv1.StatefulSet
	Status() *appv1.StatefulSetStatus

	Index() int
	Nodes() []rtypes.RedisNode
	// Master returns the master node of this shard which has joined the cluster
	// Keep in mind that, this not means the master has been assigned slots
	Master() rtypes.RedisNode
	// Replicas returns nodes whoses role is slave
	Replicas() []rtypes.RedisNode

	// Slots returns the slots of this shard
	Slots() *slot.Slots
	IsImporting() bool
	IsMigrating() bool
}

// RedisInstance
type RedisClusterInstance interface {
	RedisInstance

	Definition() *v1alpha1.Cluster
	Status() *v1alpha1.ClusterStatus

	Masters() []rtypes.RedisNode
	Nodes() []rtypes.RedisNode
	RawNodes(ctx context.Context) ([]corev1.Pod, error)
	Shards() []RedisClusterShard
	RewriteShards(ctx context.Context, shards []*clusterv1.ClusterShards) error
}
