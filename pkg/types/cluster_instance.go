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
