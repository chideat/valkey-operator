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

package failover

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/builder/failoverbuilder"
	model "github.com/chideat/valkey-operator/internal/redis"
	"github.com/chideat/valkey-operator/internal/util"
	clientset "github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/types/redis"
	"github.com/go-logr/logr"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ types.RedisReplication = (*RedisReplication)(nil)

type RedisReplication struct {
	appv1.StatefulSet
	client   clientset.ClientSet
	failover types.RedisFailoverInstance
	nodes    []redis.RedisNode

	logger logr.Logger
}

func LoadRedisReplication(ctx context.Context, client clientset.ClientSet, inst types.RedisFailoverInstance, logger logr.Logger) (types.RedisReplication, error) {
	name := failoverbuilder.GetFailoverStatefulSetName(inst.GetName())
	sts, err := client.GetStatefulSet(ctx, inst.GetNamespace(), name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		logger.Error(err, "load statefulset failed", "name", name)
		return nil, err
	}
	repl, err := NewRedisReplication(ctx, client, inst, sts, logger)
	if err != nil {
		logger.Error(err, "parse shard failed")
	}
	return repl, nil
}

func NewRedisReplication(ctx context.Context, client clientset.ClientSet, inst types.RedisFailoverInstance, sts *appv1.StatefulSet, logger logr.Logger) (types.RedisReplication, error) {
	if client == nil {
		return nil, fmt.Errorf("require clientset")
	}
	if inst == nil {
		return nil, fmt.Errorf("require instance")
	}
	if sts == nil {
		return nil, fmt.Errorf("require statefulset")
	}

	repl := &RedisReplication{
		StatefulSet: *sts,
		client:      client,
		failover:    inst,
		logger:      logger,
	}

	users := inst.Users()
	var err error
	if repl.nodes, err = model.LoadRedisNodes(ctx, client, sts, users.GetOpUser(), logger); err != nil {
		logger.Error(err, "load shard nodes failed", "shard", sts.GetName())
		return nil, err
	}
	return repl, nil
}

func (s *RedisReplication) NamespacedName() client.ObjectKey {
	if s == nil {
		return k8stypes.NamespacedName{}
	}
	return client.ObjectKey{
		Namespace: s.Namespace,
		Name:      s.Name,
	}
}

func (s *RedisReplication) Version() redis.RedisVersion {
	if s == nil {
		return redis.RedisVersionUnknown
	}

	container := util.GetContainerByName(&s.Spec.Template.Spec, failoverbuilder.ServerContainerName)
	ver, _ := redis.ParseRedisVersionFromImage(container.Image)
	return ver
}

func (s *RedisReplication) Nodes() []redis.RedisNode {
	if s == nil {
		return nil
	}
	return s.nodes
}

func (s *RedisReplication) Replicas() []redis.RedisNode {
	if s == nil || len(s.nodes) == 0 {
		return nil
	}
	var replicas []redis.RedisNode
	for _, node := range s.nodes {
		if node.Role() == core.NodeRoleReplica {
			replicas = append(replicas, node)
		}
	}
	return replicas
}

func (s *RedisReplication) Master() redis.RedisNode {
	if s == nil || len(s.nodes) == 0 {
		return nil
	}

	var master redis.RedisNode
	for _, node := range s.nodes {
		// if the node joined, and is master, then it's the master
		if node.Role() == core.NodeRoleMaster {
			master = node
		}
	}
	// the master node may failed, or is a new cluster without slots assigned
	return master
}

func (s *RedisReplication) IsReady() bool {
	if s == nil {
		return false
	}
	return s.Status().ReadyReplicas == *s.Spec.Replicas && s.Status().UpdateRevision == s.Status().CurrentRevision
}

func (s *RedisReplication) Restart(ctx context.Context, annotationKeyVal ...string) error {
	if s == nil {
		return nil
	}
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

func (s *RedisReplication) Refresh(ctx context.Context) error {
	if s == nil {
		return nil
	}
	logger := s.logger.WithName("Refresh")

	var err error
	if s.nodes, err = model.LoadRedisNodes(ctx, s.client, &s.StatefulSet, s.failover.Users().GetOpUser(), logger); err != nil {
		logger.Error(err, "load shard nodes failed", "shard", s.GetName())
		return err
	}
	return nil
}

func (s *RedisReplication) Status() *appv1.StatefulSetStatus {
	if s == nil {
		return nil
	}
	return &s.StatefulSet.Status
}

func (s *RedisReplication) Definition() *appv1.StatefulSet {
	if s == nil {
		return nil
	}
	return &s.StatefulSet
}
