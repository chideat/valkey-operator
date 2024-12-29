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

package actor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/config"
	ops "github.com/chideat/valkey-operator/internal/ops/failover"
	"github.com/chideat/valkey-operator/internal/redis/failover/monitor"
	"github.com/chideat/valkey-operator/pkg/actor"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/types/redis"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
)

var _ actor.Actor = (*actorHealMaster)(nil)

func init() {
	actor.Register(core.ValkeyFailover, NewHealMasterActor)
}

func NewHealMasterActor(client kubernetes.ClientSet, logger logr.Logger) actor.Actor {
	return &actorHealMaster{
		client: client,
		logger: logger,
	}
}

type actorHealMaster struct {
	client kubernetes.ClientSet
	logger logr.Logger
}

func (a *actorHealMaster) Version() *semver.Version {
	return semver.MustParse("3.18.0")
}

func (a *actorHealMaster) SupportedCommands() []actor.Command {
	return []actor.Command{ops.CommandHealMonitor}
}

func (a *actorHealMaster) Do(ctx context.Context, val types.RedisInstance) *actor.ActorResult {
	logger := val.Logger().WithValues("actor", ops.CommandHealMonitor.String())

	inst := val.(types.RedisFailoverInstance)
	if len(inst.Nodes()) == 0 {
		return actor.NewResult(ops.CommandEnsureResource)
	}

	// check current master
	var (
		err             error
		monitorInited   bool
		instMonitor     = inst.Monitor()
		masterCandidate redis.RedisNode
		monitoringNodes = map[string]struct{}{}
		// used to check if all any node online, if not, we should reset the monitor
		onlineNodeCount int
		// used to indicate whether a node has been registered, if no nodes are registered,
		// it means that the node is occupied or the node registration information is wrong and needs to be re-registered;
		// in addition, there is an intersection between this check of onlineNodeCount
		registeredNodeCount int
	)

	monitorMaster, err := instMonitor.Master(ctx)
	if err != nil {
		if errors.Is(err, monitor.ErrMultipleMaster) {
			// TODO: try fix multiple master
			monitorMaster, _ = instMonitor.Master(ctx, true)
			if monitorMaster == nil {
				logger.Error(err, "multi masters found, sentinel split brain")
				return actor.RequeueWithError(err)
			}

			monitoringNodes[monitorMaster.Address()] = struct{}{}
			if monitor.IsMonitoringNodeOnline(monitorMaster) {
				onlineNodeCount += 1
			}
		} else if !errors.Is(err, monitor.ErrNoMaster) &&
			!errors.Is(err, monitor.ErrAddressConflict) {
			logger.Error(err, "failed to get master node")
			return actor.RequeueWithError(err)
		}
	} else {
		monitoringNodes[monitorMaster.Address()] = struct{}{}
		if monitor.IsMonitoringNodeOnline(monitorMaster) {
			onlineNodeCount += 1
		}
	}

	if monitorInited, err = instMonitor.Inited(ctx); err != nil {
		logger.Error(err, "failed to check monitor inited")
		return actor.RequeueWithError(err)
	}

	if replicaNodes, err := instMonitor.Replicas(ctx); err != nil {
		logger.Error(err, "failed to get replicas")
		return actor.RequeueWithError(err)
	} else {
		for _, node := range replicaNodes {
			monitoringNodes[node.Address()] = struct{}{}
			if monitor.IsMonitoringNodeOnline(node) {
				onlineNodeCount += 1
			}
		}
	}
	for _, node := range inst.Nodes() {
		if !node.IsReady() {
			continue
		}
		addr := net.JoinHostPort(node.DefaultIP().String(), strconv.Itoa(node.Port()))
		addr2 := net.JoinHostPort(node.DefaultInternalIP().String(), strconv.Itoa(node.InternalIPort()))
		_, ok := monitoringNodes[addr]
		_, ok2 := monitoringNodes[addr2]
		if ok || ok2 {
			registeredNodeCount++
		}
		if monitorMaster != nil &&
			(monitorMaster.Address() == addr || monitorMaster.Address() == addr2 ||
				(monitorMaster.Port != "6379" && monitorMaster.Port == strconv.Itoa(node.Port()))) {
			masterCandidate = node
		}
	}

	if monitorMaster == nil || !monitorInited || onlineNodeCount == 0 || registeredNodeCount == 0 {
		nodes := inst.Nodes()
		// cases:
		// 1. new create instance, should select one node as master
		// 2. sentinel is new created, should check who is the master, or how can do as a master
		listeningMasters := map[string]int{}
		if masterCandidate == nil {
			// check if master exists
			for _, node := range nodes {
				if node.Role() == core.NodeRoleMaster && node.Info().ConnectedReplicas > 0 {
					masterCandidate = node
					break
				}
			}
		}

		if masterCandidate == nil {
			// check if exists nodes got most replicas
			for _, node := range nodes {
				if node.Role() == core.NodeRoleReplica && node.IsMasterLinkUp() {
					addr := net.JoinHostPort(node.ConfigedMasterIP(), node.ConfigedMasterPort())
					listeningMasters[addr]++
				}
			}
			masterCandidate = func() redis.RedisNode {
				var (
					listeningMostAddr string
					count             int
				)
				for addr, c := range listeningMasters {
					if listeningMostAddr == "" {
						listeningMostAddr = addr
						count = c
						continue
					} else if c > count {
						listeningMostAddr = addr
						count = c
					}
				}
				if listeningMostAddr != "" {
					for _, node := range nodes {
						if net.JoinHostPort(node.ConfigedMasterIP(), node.ConfigedMasterPort()) == listeningMostAddr {
							return node
						}
					}
				}
				return nil
			}()
		}

		if masterCandidate == nil {
			// find node with most repl offset
			replIds := map[string]struct{}{}
			for _, node := range nodes {
				if node.Info().MasterReplOffset > 0 {
					replIds[node.Info().MasterReplId] = struct{}{}
				}
			}
			if len(replIds) == 1 {
				slices.SortStableFunc(nodes, func(i, j redis.RedisNode) int {
					if i.Info().MasterReplOffset >= j.Info().MasterReplOffset {
						return -1
					}
					return 1
				})
				masterCandidate = nodes[0]
			}
		}

		if masterCandidate == nil {
			// selected uptime longest node as master
			slices.SortStableFunc(nodes, func(i, j redis.RedisNode) int {
				if i.Info().UptimeInSeconds >= j.Info().UptimeInSeconds {
					return -1
				}
				return 1
			})
			masterCandidate = nodes[0]
		}

		if masterCandidate != nil {
			if !masterCandidate.IsReady() {
				logger.Error(fmt.Errorf("candicate master not ready"), "selected master node is not ready", "node", masterCandidate.GetName())
				return actor.Requeue()
			}
			if err := instMonitor.Monitor(ctx, masterCandidate); err != nil {
				logger.Error(err, "failed to init sentinel")
				return actor.RequeueWithError(err)
			}
			addr := net.JoinHostPort(masterCandidate.DefaultIP().String(), strconv.Itoa(masterCandidate.Port()))
			logger.Info("setup master node", "addr", addr)
			inst.SendEventf(corev1.EventTypeWarning, config.EventSetupMaster, "setup sentinels with master %s", addr)
		} else {
			err := fmt.Errorf("cannot find any usable master node")
			logger.Error(err, "failed to setup master node")
			return actor.RequeueWithError(err)
		}
	} else {
		if masterCandidate != nil && masterCandidate.Role() != core.NodeRoleMaster {
			// exists cases all nodes are replicas, and sentinel connected to one of this replicas
			if err := masterCandidate.Setup(ctx, []any{"REPLICAOF", "NO", "ONE"}); err != nil {
				logger.Error(err, "failed to setup replicaof", "node", masterCandidate.GetName())
				return actor.RequeueWithError(err)
			}
			addr := net.JoinHostPort(masterCandidate.DefaultIP().String(), strconv.Itoa(masterCandidate.Port()))
			inst.SendEventf(corev1.EventTypeWarning, config.EventSetupMaster, "reset slave %s node as new master", addr)
			return actor.Requeue()
		}

		if strings.Contains(monitorMaster.Flags, "down") || masterCandidate == nil {
			logger.Info("master node is down, check if it's ok to do MANUAL FAILOVER")
			// when master is down, the node many keep healthy in sentinel for about 30s
			// here manually check the master node connected status
			replicas, err := instMonitor.Replicas(ctx)
			if err != nil {
				logger.Error(err, "failed to get replicas")
				return actor.RequeueWithError(err)
			}
			healthyReplicas := 0
			for _, repl := range replicas {
				if !strings.Contains(repl.Flags, "down") && !strings.Contains(repl.Flags, "disconnected") {
					healthyReplicas++
				}
			}
			if healthyReplicas == 0 {
				// TODO: do init setup
				err := fmt.Errorf("cannot do failover")
				logger.Error(err, "not healthy replicas found for failover")
				return actor.NewResultWithError(ops.CommandHealPod, err)
			}
			logger.Info("master node is down, try FAILOVER MANUALLY")
			if err := instMonitor.Failover(ctx); err != nil {
				logger.Error(err, "failed to do failover")
				return actor.RequeueWithError(err)
			}
			inst.SendEventf(corev1.EventTypeWarning, config.EventFailover, "try failover as no master found")
			return actor.Requeue()
			// TODO: maybe we can manually setup the replica as a new master
		} else {
			masterAddr := monitorMaster.Address()
			// check all other nodes connected to master
			for _, node := range inst.Nodes() {
				if !node.IsReady() {
					logger.Error(fmt.Errorf("node is not ready"), "node cannot join cluster", "node", node.GetName())
					continue
				}
				listeningAddr := net.JoinHostPort(node.DefaultIP().String(), strconv.Itoa(node.Port()))
				listeningInternalAddr := net.JoinHostPort(node.DefaultInternalIP().String(), strconv.Itoa(node.InternalIPort()))
				if masterAddr == listeningAddr || masterAddr == listeningInternalAddr {
					continue
				}

				bindedMasterAddr := net.JoinHostPort(node.ConfigedMasterIP(), node.ConfigedMasterPort())
				if bindedMasterAddr == masterAddr && node.IsMasterLinkUp() ||
					// check if they are connect to the same nodeport
					(node.ConfigedMasterPort() != "" && node.ConfigedMasterPort() != "6379" &&
						node.ConfigedMasterPort() == monitorMaster.Port) {
					continue
				}

				logger.Info("node is not link to master", "node", node.GetName(), "current listening", bindedMasterAddr, "current master", masterAddr)
				if err := node.ReplicaOf(ctx, monitorMaster.IP, monitorMaster.Port); err != nil {
					logger.Error(err, "failed to rebind replica, rejoin in next reconcile")
					continue
				}
			}
		}
	}
	return nil
}
