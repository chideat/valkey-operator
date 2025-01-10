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
	"fmt"
	"slices"
	"strings"

	"github.com/chideat/valkey-operator/internal/config"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/actor"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/types"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
)

type RuleEngine struct {
	client        kubernetes.ClientSet
	eventRecorder record.EventRecorder
	logger        logr.Logger
}

func NewRuleEngine(client kubernetes.ClientSet, eventRecorder record.EventRecorder, logger logr.Logger) (*RuleEngine, error) {
	if client == nil {
		return nil, fmt.Errorf("require client set")
	}
	if eventRecorder == nil {
		return nil, fmt.Errorf("require EventRecorder")
	}

	ctrl := RuleEngine{
		client:        client,
		eventRecorder: eventRecorder,
		logger:        logger,
	}
	return &ctrl, nil
}

func (g *RuleEngine) Inspect(ctx context.Context, val types.Instance) *actor.ActorResult {
	logger := val.Logger()

	sentinel := val.(types.SentinelInstance)
	if sentinel == nil {
		return nil
	}
	logger.V(3).Info("Inspecting Sentinel")

	cr := sentinel.Definition()
	if val := cr.Spec.PodAnnotations[config.PAUSE_ANNOTATION_KEY]; val != "" {
		return actor.NewResult(CommandEnsureResource)
	}

	// NOTE: checked if resource is fullfilled, expecially for pod binded services
	if isFullfilled, _ := sentinel.IsResourceFullfilled(ctx); !isFullfilled {
		return actor.NewResult(CommandEnsureResource)
	}

	if ret := g.isPodHealNeeded(ctx, sentinel, logger); ret != nil {
		return ret
	}

	// check all registered replication and clean:
	// 1. fail nodes
	// 2. duplicate nodes with same id
	// 3. unexcepted sentinel nodes ???
	if ret := g.isResetSentinelNeeded(ctx, sentinel, logger); ret != nil {
		return ret
	}
	return actor.NewResult(CommandEnsureResource)
}

func (g *RuleEngine) isPodHealNeeded(ctx context.Context, inst types.SentinelInstance, logger logr.Logger) *actor.ActorResult {
	if pods, err := inst.RawNodes(ctx); err != nil {
		logger.Error(err, "failed to get pods")
		return actor.RequeueWithError(err)
	} else if len(pods) > int(inst.Definition().Spec.Replicas) {
		return actor.NewResult(CommandEnsureResource)
	}

	// check if svc and pod in consistence
	for i, node := range inst.Nodes() {
		if i != node.Index() {
			return actor.NewResult(CommandHealPod)
		}

		if typ := inst.Definition().Spec.Access.ServiceType; node.IsReady() &&
			(typ == corev1.ServiceTypeNodePort || typ == corev1.ServiceTypeLoadBalancer) {
			announceIP := node.DefaultIP().String()
			announcePort := node.Port()
			svc, err := g.client.GetService(ctx, inst.GetNamespace(), node.GetName())
			if errors.IsNotFound(err) {
				return actor.NewResult(CommandEnsureResource)
			} else if err != nil {
				return actor.RequeueWithError(err)
			}
			if typ == corev1.ServiceTypeNodePort {
				port := util.GetServicePortByName(svc, "sentinel")
				if port != nil {
					if int(port.NodePort) != announcePort {
						return actor.NewResult(CommandHealPod)
					}
				} else {
					logger.Error(fmt.Errorf("service %s not found", node.GetName()), "failed to get service, which should not happen")
				}
			} else if typ == corev1.ServiceTypeLoadBalancer {
				if slices.IndexFunc(svc.Status.LoadBalancer.Ingress, func(i corev1.LoadBalancerIngress) bool {
					return i.IP == announceIP
				}) < 0 {
					return actor.NewResult(CommandHealPod)
				}
			}
		}
	}
	return nil
}

func (g *RuleEngine) isResetSentinelNeeded(ctx context.Context, inst types.SentinelInstance, logger logr.Logger) *actor.ActorResult {
	var clusters []string
	for _, node := range inst.Nodes() {
		if vals, err := node.MonitoringClusters(ctx); err != nil {
			logger.Error(err, "failed to get monitoring clusters")
		} else {
			for _, v := range vals {
				if !slices.Contains(clusters, v) {
					clusters = append(clusters, v)
				}
			}
		}
	}

	for _, name := range clusters {
		for _, node := range inst.Nodes() {
			needReset := NeedResetValkeySentinel(ctx, name, node, logger)
			if needReset {
				return actor.NewResult(CommandHealMonitor)
			}
		}
	}
	return nil
}

func NeedResetValkeySentinel(ctx context.Context, name string, node types.SentinelNode, logger logr.Logger) bool {
	master, replicas, err := node.MonitoringNodes(ctx, name)
	if err != nil {
		logger.Error(err, "failed to get monitoring nodes", "name", name)
		return false
	}
	if master == nil || strings.Contains(master.Flags, "o_down") {
		return true
	}

	ids := map[string]struct{}{master.RunId: {}}
	for _, repl := range replicas {
		if strings.Contains(repl.Flags, "o_down") {
			return true
		}
		if _, ok := ids[repl.RunId]; ok {
			return true
		}
		ids[repl.RunId] = struct{}{}
	}

	brothers, err := node.Brothers(ctx, name)
	if err != nil {
		logger.Error(err, "failed to get brothers", "name", name)
		return false
	}
	ids = map[string]struct{}{master.RunId: {}}
	for _, b := range brothers {
		// sentinel nodes will not set node to o_down, always keep in s_down status
		if strings.Contains(b.Flags, "o_down") {
			return true
		}
		if _, ok := ids[b.RunId]; ok {
			return true
		}
		ids[b.RunId] = struct{}{}
	}
	return false
}
