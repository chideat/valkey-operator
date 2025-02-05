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

package sentinel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chideat/valkey-operator/internal/builder/sentinelbuilder"
	"github.com/chideat/valkey-operator/internal/util"
	clientset "github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/version"
	"github.com/go-logr/logr"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ types.SentinelReplication = (*ValkeySentinelReplication)(nil)

type ValkeySentinelReplication struct {
	appv1.StatefulSet
	client   clientset.ClientSet
	instance types.SentinelInstance
	nodes    []types.SentinelNode
	logger   logr.Logger
}

func NewValkeySentinelReplication(ctx context.Context, client clientset.ClientSet, inst types.SentinelInstance, logger logr.Logger) (*ValkeySentinelReplication, error) {
	if client == nil {
		return nil, fmt.Errorf("require clientset")
	}
	if inst == nil {
		return nil, fmt.Errorf("require sentinel instance")
	}

	name := sentinelbuilder.SentinelStatefulSetName(inst.GetName())
	sts, err := client.GetStatefulSet(ctx, inst.GetNamespace(), name)
	if errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		logger.Info("load deployment failed", "name", name)
		return nil, err
	}

	node := ValkeySentinelReplication{
		StatefulSet: *sts,
		client:      client,
		instance:    inst,
		logger:      logger.WithName("ValkeySentinelReplication"),
	}
	if node.nodes, err = LoadValkeySentinelNodes(ctx, client, sts, inst.Users().GetOpUser(), logger); err != nil {

		logger.Error(err, "load shard nodes failed", "shard", sts.GetName())
		return nil, err
	}
	return &node, nil
}

func (s *ValkeySentinelReplication) NamespacedName() client.ObjectKey {
	if s == nil {
		return client.ObjectKey{}
	}
	return client.ObjectKey{
		Namespace: s.GetNamespace(),
		Name:      s.GetName(),
	}
}

func (s *ValkeySentinelReplication) Version() version.ValkeyVersion {
	if s == nil {
		return version.ValkeyVersionUnknown
	}
	container := util.GetContainerByName(&s.Spec.Template.Spec, sentinelbuilder.SentinelContainerName)
	ver, _ := version.ParseValkeyVersionFromImage(container.Image)
	return ver
}

func (s *ValkeySentinelReplication) Definition() *appv1.StatefulSet {
	if s == nil {
		return nil
	}
	return &s.StatefulSet
}

func (s *ValkeySentinelReplication) Nodes() []types.SentinelNode {
	if s == nil {
		return nil
	}
	return s.nodes
}

func (s *ValkeySentinelReplication) Restart(ctx context.Context, annotationKeyVal ...string) error {
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
		logger.Error(err, "restart deployment failed", "target", client.ObjectKeyFromObject(&s.StatefulSet))
		return err
	}
	return nil
}

func (s *ValkeySentinelReplication) IsReady() bool {
	if s == nil {
		return false
	}
	return s.Status().ReadyReplicas == *s.Spec.Replicas && s.Status().UpdateRevision == s.Status().CurrentRevision
}

func (s *ValkeySentinelReplication) Refresh(ctx context.Context) error {
	logger := s.logger.WithName("Refresh")

	var err error
	if s.nodes, err = LoadValkeySentinelNodes(ctx, s.client, &s.StatefulSet, s.instance.Users().GetOpUser(), logger); err != nil {
		logger.Error(err, "load shard nodes failed", "shard", s.GetName())
		return err
	}
	return nil
}

func (s *ValkeySentinelReplication) Status() *appv1.StatefulSetStatus {
	if s == nil {
		return nil
	}
	return &s.StatefulSet.Status
}
