package types

import (
	"context"

	"github.com/chideat/valkey-operator/api/v1alpha1"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type Replication interface {
	Object

	Definition() *appv1.StatefulSet
	Status() *appv1.StatefulSetStatus

	// Master returns the master node of this shard which has joined the cluster
	// Keep in mind that, this not means the master has been assigned slots
	Master() ValkeyNode
	// Replicas returns nodes whoses role is slave
	Replicas() []ValkeyNode
	Nodes() []ValkeyNode
}

type Sentinel interface {
	Object

	Definition() *appv1.Deployment
	Status() *appv1.DeploymentStatus

	Nodes() []SentinelNode
}

type SentinelReplication interface {
	Object

	Definition() *appv1.StatefulSet
	Status() *appv1.StatefulSetStatus

	Nodes() []SentinelNode
}

type SentinelInstance interface {
	Instance

	Definition() *v1alpha1.Sentinel
	Replication() SentinelReplication
	Nodes() []SentinelNode
	RawNodes(ctx context.Context) ([]corev1.Pod, error)

	// helper methods
	GetPassword() (string, error)

	Selector() map[string]string
}
