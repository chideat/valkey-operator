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

package actor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/actor"
	"github.com/chideat/valkey-operator/internal/config"
	ops "github.com/chideat/valkey-operator/internal/ops/failover"
	"github.com/chideat/valkey-operator/internal/valkey/failover/monitor"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/version"
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
	return semver.MustParse("0.1.0")
}

func (a *actorHealMaster) SupportedCommands() []actor.Command {
	return []actor.Command{ops.CommandHealMonitor}
}

func (a *actorHealMaster) Do(ctx context.Context, val types.Instance) *actor.ActorResult {
	logger := val.Logger().WithValues("actor", ops.CommandHealMonitor.String())

	inst := val.(types.FailoverInstance)
	if len(inst.Nodes()) == 0 {
		return actor.NewResult(ops.CommandEnsureResource)
	}

	// check current master
	var (
		err             error
		monitorInited   bool
		instMonitor     = inst.Monitor()
		masterCandidate types.ValkeyNode
		monitoringNodes = map[string]struct{}{}
		idAddrMap       = map[string][]string{}
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
			} else {
				monitoringNodes[monitorMaster.Address()] = struct{}{}
				if monitor.IsMonitoringNodeOnline(monitorMaster) {
					onlineNodeCount += 1
				}
			}
		} else if errors.Is(err, monitor.ErrAddressConflict) {
			// do failover to force sentinel update node's announce info
			if err := instMonitor.Failover(ctx); err != nil {
				logger.Error(err, "do manual failover failed")
				// continue with master setup
			} else {
				return actor.RequeueAfter(time.Second * 10)
			}
		} else if !errors.Is(err, monitor.ErrNoMaster) {
			logger.Error(err, "failed to get master node")
			return actor.RequeueWithError(err)
		}
	} else {
		monitoringNodes[monitorMaster.Address()] = struct{}{}
		if monitorMaster.Port != "6379" {
			monitoringNodes[monitorMaster.Port] = struct{}{}
		}
		if monitor.IsMonitoringNodeOnline(monitorMaster) {
			onlineNodeCount += 1
			if monitorMaster.RunId != "" && !slices.Contains(idAddrMap[monitorMaster.RunId], monitorMaster.Address()) {
				idAddrMap[monitorMaster.RunId] = append(idAddrMap[monitorMaster.RunId], monitorMaster.Address())
			}
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
			if node.Port != "6379" {
				monitoringNodes[node.Port] = struct{}{}
			}
			if monitor.IsMonitoringNodeOnline(node) {
				onlineNodeCount += 1
				if node.RunId != "" && !slices.Contains(idAddrMap[node.RunId], node.Address()) {
					idAddrMap[node.RunId] = append(idAddrMap[node.RunId], node.Address())
				}
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

		// NOTE: only check replica nodes
		if node.Role() == core.NodeRoleReplica {
			registeredAddrs := idAddrMap[node.Info().RunId]
			if (len(registeredAddrs) == 1 && registeredAddrs[0] != addr && registeredAddrs[0] != addr2) ||
				len(registeredAddrs) > 1 {
				// found a node registered with different address
				logger.Info("node registered with different address", "node", node.GetName(), "registered", registeredAddrs, "current", addr)
				// replicaof no one
				if err := node.ReplicaOf(ctx, "NO", "ONE"); err != nil {
					logger.Error(err, "failed to reset replicaof", "node", node.GetName())
					return actor.RequeueWithError(err)
				}
				inst.SendEventf(corev1.EventTypeWarning, config.EventResetReplica, "reset replica %s", node.GetName())

				// reset all sentinels
				_ = inst.Monitor().Reset(ctx)

				return actor.RequeueAfter(time.Second * 5)
			}
		}
	}

	if monitorMaster == nil || !monitorInited || onlineNodeCount == 0 || registeredNodeCount == 0 {
		nodes := inst.Nodes()

		masterCandidate = a.candidatePicker(ctx, nodes)
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
		if masterCandidate != nil && masterCandidate.Role() != core.NodeRoleMaster && masterCandidate.Config()["replica-priority"] != "0" {
			// exists cases all nodes are replicas, and sentinel connected to one of this replicas
			if err := masterCandidate.ReplicaOf(ctx, "NO", "ONE"); err != nil {
				logger.Error(err, "failed to setup replicaof", "node", masterCandidate.GetName())
				return actor.RequeueWithError(err)
			}
			addr := net.JoinHostPort(masterCandidate.DefaultIP().String(), strconv.Itoa(masterCandidate.Port()))
			inst.SendEventf(corev1.EventTypeWarning, config.EventSetupMaster, "reset replica %s node as new master", addr)
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
		} else if masterCandidate.Config()["replica-priority"] != "0" {
			masterAddr := monitorMaster.Address()
			// check all other nodes connected to master
			for _, node := range inst.Nodes() {
				if !node.IsReady() {
					logger.Error(fmt.Errorf("node is not ready"), "node cannot join cluster", "node", node.GetName())
					continue
				}
				listeningAddr := net.JoinHostPort(node.DefaultIP().String(), strconv.Itoa(node.Port()))
				listeningInternalAddr := net.JoinHostPort(node.DefaultInternalIP().String(), strconv.Itoa(node.InternalIPort()))
				if masterAddr == listeningAddr || masterAddr == listeningInternalAddr ||
					(monitorMaster.Port != "6379" && monitorMaster.Port == strconv.Itoa(node.Port())) {
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
		} else {
			logger.Info("node is flaged not to failover", "node", masterCandidate.GetName())
			return actor.Requeue()
		}
	}
	return nil
}

func (a *actorHealMaster) candidatePicker(ctx context.Context, nodes []types.ValkeyNode) types.ValkeyNode {
	// cases:
	// 1. new create instance, should select one node as master
	// 2. sentinel is new created, should check who is the master, or how can do as a master

	tmpNodes := nodes
	nodes = nodes[0:0]
	for _, node := range tmpNodes {
		if err := node.Refresh(ctx); err != nil {
			a.logger.Error(err, "failed to refresh node", "node", node.GetName())
			continue
		}
		if !node.IsReady() || node.Config()["replica-priority"] == "0" {
			continue
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil
	}

	var (
		readyNodes []types.ValkeyNode
		maxVersion version.ValkeyVersion
		dbsize     int64
		onlineTime int64
	)
	// check if master exists
	for _, node := range nodes {
		if node.Role() == core.NodeRoleMaster && node.Info().Dbsize > 0 {
			readyNodes = append(readyNodes, node)
			if node.CurrentVersion().Compare(maxVersion) > 0 {
				maxVersion = node.CurrentVersion()
			}
			if node.Info().Dbsize > dbsize {
				dbsize = node.Info().Dbsize
			}
			if node.Info().UptimeInSeconds > onlineTime {
				onlineTime = node.Info().UptimeInSeconds
			}
		}
	}

	if len(readyNodes) == 1 {
		return readyNodes[0]
	} else if len(readyNodes) > 1 {
		// check nodes with most data
		slices.SortStableFunc(readyNodes, func(i, j types.ValkeyNode) int {
			if i.Info().Dbsize >= j.Info().Dbsize {
				return 1
			}
			return -1
		})

		for _, node := range readyNodes {
			if node.CurrentVersion() == maxVersion {
				return node
			}
		}
		return readyNodes[0]
	}

	// check if replicas exists
	maxVersion = version.ValkeyVersionUnknown
	dbsize = 0
	onlineTime = 0
	readyNodes = readyNodes[0:0]
	for _, node := range nodes {
		if node.Info().Dbsize > 0 {
			readyNodes = append(readyNodes, node)
			if node.CurrentVersion().Compare(maxVersion) > 0 {
				maxVersion = node.CurrentVersion()
			}
			if node.Info().Dbsize > dbsize {
				dbsize = node.Info().Dbsize
			}
		}
		if node.Info().UptimeInSeconds > onlineTime {
			onlineTime = node.Info().UptimeInSeconds
		}
	}

	if len(readyNodes) == 1 {
		return readyNodes[0]
	} else if len(readyNodes) > 1 {
		// check nodes with most data
		slices.SortStableFunc(readyNodes, func(i, j types.ValkeyNode) int {
			if i.Info().Dbsize >= j.Info().Dbsize {
				return 1
			}
			return -1
		})

		for _, node := range readyNodes {
			if node.CurrentVersion() == maxVersion {
				return node
			}
		}
		return readyNodes[0]
	}

	// selected uptime longest node as master
	slices.SortStableFunc(nodes, func(i, j types.ValkeyNode) int {
		if i.Info().UptimeInSeconds >= j.Info().UptimeInSeconds {
			return 1
		}
		return -1
	})
	return nodes[0]
}
