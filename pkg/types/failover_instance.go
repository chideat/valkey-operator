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
