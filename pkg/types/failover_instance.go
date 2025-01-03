package types

import (
	"context"

	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/pkg/valkey"
	corev1 "k8s.io/api/core/v1"
)

type FailoverMonitor interface {
	Policy() v1alpha1.FailoverPolicy
	Master(ctx context.Context, flags ...bool) (*valkey.SentinelMonitorNode, error)
	Replicas(ctx context.Context) ([]*valkey.SentinelMonitorNode, error)
	Inited(ctx context.Context) (bool, error)
	AllNodeMonitored(ctx context.Context) (bool, error)
	UpdateConfig(ctx context.Context, params map[string]string) error
	Failover(ctx context.Context) error
	Monitor(ctx context.Context, node ValkeyNode) error
}

type FailoverInstance interface {
	Instance

	Definition() *v1alpha1.Failover
	Masters() []ValkeyNode
	Nodes() []ValkeyNode
	RawNodes(ctx context.Context) ([]corev1.Pod, error)
	Monitor() FailoverMonitor

	IsBindedSentinel() bool
	IsStandalone() bool
	Selector() map[string]string
}
