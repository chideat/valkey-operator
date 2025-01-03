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

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
	"github.com/chideat/valkey-operator/internal/util"
	model "github.com/chideat/valkey-operator/internal/valkey"
	clientset "github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/slot"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/version"
	"github.com/go-logr/logr"
	appv1 "k8s.io/api/apps/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ types.ClusterShard = (*RedisClusterShard)(nil)

func LoadRedisClusterShards(ctx context.Context, client clientset.ClientSet, cluster types.ClusterInstance, logger logr.Logger) ([]types.ClusterShard, error) {
	// load pods by label
	labels := clusterbuilder.GetClusterLabels(cluster.GetName(), nil)

	var shards []types.ClusterShard
	if resp, err := client.ListStatefulSetByLabels(ctx, cluster.GetNamespace(), labels); err != nil {
		logger.Error(err, "load statefulset failed")
		return nil, err
	} else {
		for _, sts := range resp.Items {
			sts := sts.DeepCopy()
			if shard, err := NewRedisClusterShard(ctx, client, cluster, sts, logger); err != nil {
				logger.Error(err, fmt.Sprintf("parse shard %s failed", sts.GetName()))
			} else {
				shards = append(shards, shard)
			}
		}
		sort.SliceStable(shards, func(i, j int) bool {
			return shards[i].Index() < shards[j].Index()
		})
	}
	return shards, nil
}

// NewRedisClusterShard
func NewRedisClusterShard(ctx context.Context, client clientset.ClientSet, cluster types.ClusterInstance, sts *appv1.StatefulSet, logger logr.Logger) (types.ClusterShard, error) {
	if client == nil {
		return nil, fmt.Errorf("require clientset")
	}
	if cluster == nil {
		return nil, fmt.Errorf("require cluster instance")
	}
	if sts == nil {
		return nil, fmt.Errorf("require statefulset")
	}

	shard := RedisClusterShard{
		StatefulSet: *sts,
		client:      client,
		cluster:     cluster,
		logger:      logger.WithName("Shard"),
	}

	users := cluster.Users()
	var err error
	if shard.nodes, err = model.LoadNodes(ctx, client, sts, users.GetOpUser(), logger); err != nil {
		logger.Error(err, "load shard nodes failed", "shard", sts.GetName())
		return nil, err
	}
	return &shard, nil
}

// RedisClusterShard
type RedisClusterShard struct {
	appv1.StatefulSet

	client  clientset.ClientSet
	cluster types.ClusterInstance
	nodes   []types.ValkeyNode

	logger logr.Logger
}

func (s *RedisClusterShard) NamespacedName() apitypes.NamespacedName {
	if s.StatefulSet.Namespace == "" || s.StatefulSet.Name == "" {
		return apitypes.NamespacedName{}
	}
	return apitypes.NamespacedName{
		Namespace: s.StatefulSet.Namespace,
		Name:      s.StatefulSet.Name,
	}
}

// Version
func (s *RedisClusterShard) Version() version.ValkeyVersion {
	if s == nil {
		return version.ValkeyVersionUnknown
	}

	container := util.GetContainerByName(&s.Spec.Template.Spec, clusterbuilder.ServerContainerName)
	ver, _ := version.ParseValkeyVersionFromImage(container.Image)
	return ver
}

// Index redis shard index. so the statefulset name must match ^drc-<name>-[0-9]+$ format
func (s *RedisClusterShard) Index() int {
	if s == nil || s.StatefulSet.GetName() == "" {
		return -1
	}

	name := s.StatefulSet.GetName()
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '-' {
			index, _ := strconv.ParseInt(name[i+1:], 10, 32)
			return int(index)
		}
	}
	// this should not happen
	return -1
}

// Nodes returns all the nodes of this slots
func (s *RedisClusterShard) Nodes() []types.ValkeyNode {
	if s == nil {
		return nil
	}
	return s.nodes
}

// Master for nodes not join the cluster, it's role is also master
func (s *RedisClusterShard) Master() types.ValkeyNode {
	if s == nil || len(s.nodes) == 0 {
		return nil
	}

	var emptyMaster types.ValkeyNode
	for _, node := range s.nodes {
		// if the node joined, and is master, then it's the master
		if node.Role() == core.NodeRoleMaster && node.IsJoined() {
			if node.Slots().Count(slot.SlotAssigned) > 0 || node.Slots().Count(slot.SlotImporting) > 0 {
				return node
			}
			if emptyMaster == nil {
				emptyMaster = node
			}
		}
	}
	// the master node may failed, or is a new cluster without slots assigned
	return emptyMaster
}

func (s *RedisClusterShard) Replicas() []types.ValkeyNode {
	if s == nil || len(s.nodes) == 0 {
		return nil
	}
	var replicas []types.ValkeyNode
	for _, node := range s.nodes {
		if node.Role() == core.NodeRoleReplica {
			replicas = append(replicas, node)
		}
	}
	return replicas
}

// Slots
func (s *RedisClusterShard) Slots() *slot.Slots {
	if s == nil {
		return nil
	}
	for _, node := range s.nodes {
		if node.IsJoined() && node.Role() == core.NodeRoleMaster &&
			(node.Slots().Count(slot.SlotAssigned) > 0 || node.Slots().Count(slot.SlotImporting) > 0) {
			return node.Slots()
		}
	}
	return nil
}

func (s *RedisClusterShard) IsReady() bool {
	if s == nil {
		return false
	}
	return s.Status().ReadyReplicas == *s.Spec.Replicas && s.Status().UpdateRevision == s.Status().CurrentRevision
}

// IsImporting
func (s *RedisClusterShard) IsImporting() bool {
	if s == nil {
		return false
	}

	for _, shard := range s.cluster.Status().Shards {
		if shard.Index == int32(s.Index()) {
			for _, slots := range shard.Slots {
				if slots.Status == slot.SlotImporting.String() {
					return true
				}
			}
		}
	}
	return false
}

// IsMigrating
func (s *RedisClusterShard) IsMigrating() bool {
	if s == nil {
		return false
	}

	for _, shard := range s.cluster.Status().Shards {
		if shard.Index == int32(s.Index()) {
			for _, slots := range shard.Slots {
				if slots.Status == slot.SlotMigrating.String() {
					return true
				}
			}
		}
	}
	return false
}

// Restart
func (s *RedisClusterShard) Restart(ctx context.Context, annotationKeyVal ...string) error {
	// update all shards
	logger := s.logger.WithName("Restart")

	kv := map[string]string{
		"kubectl.kubernetes.io/restartedAt": time.Now().Format(time.RFC3339Nano),
	}
	for i := 0; i < len(annotationKeyVal)-1; i += 2 {
		kv[annotationKeyVal[i]] = annotationKeyVal[i+1]
	}

	data, _ := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": kv,
				},
			},
		},
	})

	if err := s.client.Client().Patch(ctx, &s.StatefulSet,
		client.RawPatch(k8stypes.StrategicMergePatchType, data)); err != nil {
		logger.Error(err, "restart statefulset failed", "target", client.ObjectKeyFromObject(&s.StatefulSet))
		return err
	}
	return nil
}

// Refresh
func (s *RedisClusterShard) Refresh(ctx context.Context) error {
	logger := s.logger.WithName("Refresh")

	var err error
	if s.nodes, err = model.LoadNodes(ctx, s.client, &s.StatefulSet, s.cluster.Users().GetOpUser(), logger); err != nil {
		logger.Error(err, "load shard nodes failed", "shard", s.GetName())
		return err
	}
	return nil
}

func (s *RedisClusterShard) Status() *appv1.StatefulSetStatus {
	if s == nil {
		return nil
	}
	return &s.StatefulSet.Status
}

func (s *RedisClusterShard) Definition() *appv1.StatefulSet {
	if s == nil {
		return nil
	}
	return &s.StatefulSet
}
