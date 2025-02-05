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
*/package failover

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"slices"
	"strconv"
	"time"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	v1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/aclbuilder"
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
	"github.com/chideat/valkey-operator/internal/builder/failoverbuilder"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/internal/valkey/failover/monitor"
	"github.com/chideat/valkey-operator/pkg/actor"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/security/acl"
	"github.com/chideat/valkey-operator/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

	logger.V(3).Info("Inspecting failover instance")
	inst := val.(types.FailoverInstance)
	if inst == nil {
		return nil
	}
	cr := inst.Definition()

	if (cr.Spec.PodAnnotations != nil) && cr.Spec.PodAnnotations[config.PAUSE_ANNOTATION_KEY] != "" {
		return actor.NewResult(CommandEnsureResource)
	}

	// NOTE: checked if resource is fullfilled
	if isFullfilled, _ := inst.IsResourceFullfilled(ctx); !isFullfilled {
		return actor.NewResult(CommandEnsureResource)
	}

	// check password
	if ret := g.isPasswordChanged(ctx, inst, logger); ret != nil {
		logger.V(3).Info("checked password", "result", ret)
		return ret
	}

	// check configmap
	if ret := g.isConfigChanged(ctx, inst, logger); ret != nil {
		logger.V(3).Info("checked config", "result", ret)
		return ret
	}

	// check master
	if ret := g.isNodesHealthy(ctx, inst, logger); ret != nil {
		logger.V(3).Info("checked nodes healthy", "result", ret)
		return ret
	}

	// ensure rw service
	if ret := g.isPatchLabelNeeded(ctx, inst, logger); ret != nil {
		logger.V(3).Info("checked labels", "result", ret)
		return ret
	}

	// do clean check
	if ret := g.isResourceCleanNeeded(ctx, inst, logger); ret != nil {
		logger.V(3).Info("clean useless resources", "result", ret)
		return ret
	}
	return actor.NewResult(CommandEnsureResource)
}

func (g *RuleEngine) isPatchLabelNeeded(ctx context.Context, inst types.FailoverInstance, logger logr.Logger) *actor.ActorResult {
	if len(inst.Masters()) != 1 {
		return nil
	}

	masterNode, err := inst.Monitor().Master(ctx)
	if err != nil {
		logger.Error(err, "failed to get master node")
		return actor.RequeueWithError(err)
	}
	masterAddr := net.JoinHostPort(masterNode.IP, masterNode.Port)

	pods, err := inst.RawNodes(ctx)
	if err != nil {
		return actor.RequeueWithError(err)
	}

	for _, pod := range pods {
		var node types.ValkeyNode
		slices.IndexFunc(inst.Nodes(), func(i types.ValkeyNode) bool {
			if i.GetName() == pod.GetName() {
				node = i
				return true
			}
			return false
		})

		labels := pod.GetLabels()
		labelVal := labels[builder.RoleLabelKey]
		if node == nil {
			if labelVal != "" {
				logger.V(3).Info("node not accessable", "name", pod.GetName(), "labels", labels)
				return actor.NewResult(CommandPatchLabels)
			}
			continue
		}

		nodeAddr := net.JoinHostPort(node.DefaultIP().String(), strconv.Itoa(node.Port()))
		switch {
		case nodeAddr == masterAddr && labelVal != string(core.NodeRoleMaster):
			fallthrough
		case labelVal == string(core.NodeRoleMaster) && nodeAddr != masterAddr:
			fallthrough
		case node.Role() == core.NodeRoleReplica && labelVal != string(core.NodeRoleReplica):
			logger.V(3).Info("master labels not match", "node", node.GetName(), "labels", labels)
			return actor.NewResult(CommandPatchLabels)
		}
	}
	return nil
}

func (g *RuleEngine) isPasswordChanged(ctx context.Context, inst types.FailoverInstance, logger logr.Logger) *actor.ActorResult {
	logger.V(3).Info("checkPassword")

	name := aclbuilder.GenerateACLConfigMapName(inst.Arch(), inst.GetName())
	if cm, err := g.client.GetConfigMap(ctx, inst.GetNamespace(), name); errors.IsNotFound(err) {
		return actor.NewResult(CommandUpdateAccount)
	} else if err != nil {
		logger.Error(err, "failed to get configmap", "configmap", name)
		return actor.NewResult(CommandRequeue)
	} else if users, err := acl.LoadACLUsers(ctx, g.client, cm); err != nil {
		return actor.NewResult(CommandUpdateAccount)
	} else if users.GetOpUser() == nil {
		return actor.NewResult(CommandUpdateAccount)
	} else if !reflect.DeepEqual(users.Encode(true), users.Encode(false)) {
		return actor.NewResult(CommandUpdateAccount)
	}
	return nil
}

func (g *RuleEngine) isConfigChanged(ctx context.Context, inst types.FailoverInstance, logger logr.Logger) *actor.ActorResult {
	newCm, err := failoverbuilder.GenerateConfigMap(inst)
	if err != nil {
		return actor.RequeueWithError(err)
	}
	oldCm, err := g.client.GetConfigMap(ctx, newCm.GetNamespace(), newCm.GetName())
	if errors.IsNotFound(err) || oldCm.Data[builder.ValkeyConfigKey] == "" {
		err := fmt.Errorf("configmap %s not found", newCm.GetName())
		return actor.NewResultWithError(CommandEnsureResource, err)
	} else if err != nil {
		return actor.RequeueWithError(err)
	}
	newConf, _ := clusterbuilder.LoadValkeyConfig(newCm.Data[builder.ValkeyConfigKey])
	oldConf, _ := clusterbuilder.LoadValkeyConfig(oldCm.Data[builder.ValkeyConfigKey])
	added, changed, deleted := oldConf.Diff(newConf)
	if len(added)+len(changed)+len(deleted) != 0 {
		return actor.NewResult(CommandUpdateConfig)
	}

	if inst.Monitor().Policy() == v1.SentinelFailoverPolicy {
		// HACK: check and update sentinel monitor config
		// here check and updated sentinel monitor config directly
		if err := inst.Monitor().UpdateConfig(ctx, inst.Definition().Spec.Sentinel.MonitorConfig); err != nil {
			logger.Error(err, "failed to update sentinel monitor config")
			return actor.RequeueWithError(err)
		}
	}
	return nil
}

// 检查是否有节点未加入集群
func (g *RuleEngine) isNodesHealthy(ctx context.Context, inst types.FailoverInstance, logger logr.Logger) *actor.ActorResult {
	// check if svc and pod in consistence
	for _, node := range inst.Nodes() {
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
				port := util.GetServicePortByName(svc, "client")
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

	// AllNodeMonitored can check if master is online.
	// If master is down on any node, trigger CommandHealMonitor
	allMonitored, err := inst.Monitor().AllNodeMonitored(ctx)
	if err != nil {
		if err == monitor.ErrMultipleMaster {
			logger.Error(err, "multi master found")
			return actor.NewResult(CommandHealMonitor)
		} else if err == monitor.ErrAddressConflict {
			logger.Error(err, "sentinel not update node announce address")
			return actor.NewResult(CommandHealMonitor)
		}
		logger.Error(err, "failed to check all nodes monitored")
		return actor.RequeueWithError(err)
	}
	if !allMonitored {
		logger.Info("not all nodes monitored")
		return actor.NewResult(CommandHealMonitor)
	}

	monitorMaster, err := inst.Monitor().Master(ctx)
	if err == monitor.ErrMultipleMaster || err == monitor.ErrNoMaster || err == monitor.ErrAddressConflict {
		logger.Error(err, "no usable master nodes found")
		return actor.NewResult(CommandHealMonitor)
	} else if err != nil {
		logger.Error(err, "failed to get master")
		return actor.RequeueWithError(err)
	}

	var masterNode types.ValkeyNode
	for _, node := range inst.Nodes() {
		addr := net.JoinHostPort(node.DefaultIP().String(), strconv.Itoa(node.Port()))
		addr2 := net.JoinHostPort(node.DefaultInternalIP().String(), strconv.Itoa(node.InternalPort()))
		if addr == monitorMaster.Address() || addr2 == monitorMaster.Address() {
			masterNode = node
			break
		}
	}
	// TODO: here need more check of node connection
	if masterNode == nil {
		logger.Info("master not found on any nodes, maybe master is down")
		return actor.NewResult(CommandHealMonitor)
	}

	now := time.Now()
	for _, node := range inst.Nodes() {
		if node.IsTerminating() &&
			now.After(node.GetDeletionTimestamp().
				Add(time.Duration(*node.GetDeletionGracePeriodSeconds())*time.Second)) {
			logger.Info("node terminted", "node", node.GetName())
			return actor.NewResult(CommandHealPod)
		}
	}

	for i, node := range inst.Nodes() {
		if i != node.Index() {
			logger.Info("node index not match", "node", node.GetName(), "index", node.Index())
			return actor.NewResult(CommandHealPod)
		}
	}
	return nil
}

func (g *RuleEngine) isResourceCleanNeeded(ctx context.Context, inst types.FailoverInstance, logger logr.Logger) *actor.ActorResult {
	if inst.IsReady() {
		// delete sentinel after standalone is ready for old pod to gracefully shutdown
		if !inst.IsBindedSentinel() {
			var sen v1alpha1.Sentinel
			if err := g.client.Client().Get(ctx, client.ObjectKey{
				Namespace: inst.GetNamespace(),
				Name:      inst.GetName(),
			}, &sen); err != nil && !errors.IsNotFound(err) {
				logger.Error(err, "failed to get sentinel statefulset", "sentinel", inst.GetName())
				return actor.RequeueWithError(err)
			} else if err == nil {
				return actor.NewResult(CommandCleanResource)
			}
		}
		// TODO: clean standalone ha configmap when switch arch from standalone to failover
	}
	return nil
}
