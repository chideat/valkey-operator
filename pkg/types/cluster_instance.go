package types

import (
	"context"

	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/pkg/slot"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// ClusterShard
type ClusterShard interface {
	Object

	Definition() *appv1.StatefulSet
	Status() *appv1.StatefulSetStatus

	Index() int
	Nodes() []ValkeyNode
	// Master returns the master node of this shard which has joined the cluster
	// Keep in mind that, this not means the master has been assigned slots
	Master() ValkeyNode
	// Replicas returns nodes whoses role is slave
	Replicas() []ValkeyNode

	// Slots returns the slots of this shard
	Slots() *slot.Slots
	IsImporting() bool
	IsMigrating() bool
}

// Instance
type ClusterInstance interface {
	Instance

	Definition() *v1alpha1.Cluster
	Status() *v1alpha1.ClusterStatus

	Masters() []ValkeyNode
	Nodes() []ValkeyNode
	RawNodes(ctx context.Context) ([]corev1.Pod, error)
	Shards() []ClusterShard
	RewriteShards(ctx context.Context, shards []*v1alpha1.ClusterShards) error
}
