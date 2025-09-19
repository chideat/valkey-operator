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

package monitor

import (
	"context"
	"crypto/tls"
	"fmt"
	"maps"
	"net"
	"slices"
	"strconv"
	"strings"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/util"
	clientset "github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/types"
	vkcli "github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/chideat/valkey-operator/pkg/version"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrNoUseableNode   = fmt.Errorf("no usable sentinel node")
	ErrNoMaster        = fmt.Errorf("no master")
	ErrDoFailover      = fmt.Errorf("sentinel doing failover")
	ErrMultipleMaster  = fmt.Errorf("multiple master without majority agreement")
	ErrAddressConflict = fmt.Errorf("master address conflict")
	ErrNotEnoughNodes  = fmt.Errorf("not enough sentinel nodes")
)

var _ types.FailoverMonitor = (*SentinelMonitor)(nil)

func IsMonitoringNodeOnline(node *vkcli.SentinelMonitorNode) bool {
	if node == nil {
		return false
	}
	return !strings.Contains(node.Flags, "down") && !strings.Contains(node.Flags, "disconnected")
}

type SentinelMonitor struct {
	client    clientset.ClientSet
	failover  types.FailoverInstance
	groupName string
	nodes     []*SentinelNode

	logger logr.Logger
}

func NewSentinelMonitor(ctx context.Context, k8scli clientset.ClientSet, inst types.FailoverInstance) (*SentinelMonitor, error) {
	if k8scli == nil {
		return nil, fmt.Errorf("require clientset")
	}
	if inst == nil {
		return nil, fmt.Errorf("require instance")
	}

	monitor := &SentinelMonitor{
		client:    k8scli,
		failover:  inst,
		groupName: "mymaster",
		logger:    inst.Logger(),
	}
	var (
		username      = inst.Definition().Status.Monitor.Username
		passwords     []string
		tlsSecret     = inst.Definition().Status.Monitor.TLSSecret
		tlsConfig     *tls.Config
		monitorStatus = inst.Definition().Status.Monitor
	)
	for _, passwordSecret := range []string{monitorStatus.PasswordSecret, monitorStatus.OldPasswordSecret} {
		if passwordSecret == "" {
			passwords = append(passwords, "")
			continue
		}
		if secret, err := k8scli.GetSecret(ctx, inst.GetNamespace(), passwordSecret); err != nil {
			obj := client.ObjectKey{Namespace: inst.GetNamespace(), Name: passwordSecret}
			monitor.logger.Error(err, "get password secret failed", "target", obj)
			return nil, err
		} else {
			passwords = append(passwords, string(secret.Data["password"]))
		}
	}

	if tlsSecret != "" {
		if secret, err := k8scli.GetSecret(ctx, inst.GetNamespace(), tlsSecret); err != nil {
			obj := client.ObjectKey{Namespace: inst.GetNamespace(), Name: tlsSecret}
			monitor.logger.Error(err, "get tls secret failed", "target", obj)
			return nil, err
		} else if tlsConfig, err = util.LoadCertConfigFromSecret(secret); err != nil {
			monitor.logger.Error(err, "load cert config failed")
			return nil, err
		}
	}
	for _, node := range inst.Definition().Status.Monitor.Nodes {
		var (
			err   error
			snode *SentinelNode
		)
		for _, password := range passwords {
			addr := net.JoinHostPort(node.IP, fmt.Sprintf("%d", node.Port))
			if snode, err = NewSentinelNode(ctx, addr, username, password, tlsConfig); err != nil {
				if strings.Contains(err.Error(), "NOAUTH Authentication required") ||
					strings.Contains(err.Error(), "invalid password") ||
					strings.Contains(err.Error(), "Client sent AUTH, but no password is set") ||
					strings.Contains(err.Error(), "invalid username-password pair") {

					monitor.logger.Error(err, "sentinel node auth failed, try old password", "addr", addr)
					continue
				}
				monitor.logger.Error(err, "create sentinel node failed", "addr", addr)
			}
			break
		}
		if snode != nil {
			monitor.nodes = append(monitor.nodes, snode)
		}
	}
	return monitor, nil
}

func (s *SentinelMonitor) Policy() v1alpha1.FailoverPolicy {
	return v1alpha1.SentinelFailoverPolicy
}

func (s *SentinelMonitor) Master(ctx context.Context, flags ...bool) (*vkcli.SentinelMonitorNode, error) {
	if s == nil {
		return nil, nil
	}
	type Stat struct {
		Node  *vkcli.SentinelMonitorNode
		Count int
	}
	var (
		masterStat      []*Stat
		masterIds       = map[string]int{}
		idAddrMap       = map[string][]string{}
		registeredNodes int
	)
	for _, node := range s.nodes {
		n, err := node.MonitoringMaster(ctx, s.groupName)
		if err != nil {
			if err == ErrNoMaster || strings.Contains(err.Error(), "no such host") {
				s.logger.Error(err, "master not registered", "addr", node.addr)
				continue
			}
			// NOTE: here ignored any error, for the node may be offline forever
			s.logger.Error(err, "check monitoring master status of sentinel failed", "addr", node.addr)
			return nil, err
		} else if n.IsFailovering() {
			s.logger.Error(ErrDoFailover, "valkey sentinel is doing failover", "node", n.Address())
			return nil, ErrDoFailover
		} else if !IsMonitoringNodeOnline(n) {
			s.logger.Error(fmt.Errorf("master node offline"), "master node offline", "node", n.Address(), "flags", n.Flags)
			continue
		}

		registeredNodes += 1
		if i := slices.IndexFunc(masterStat, func(s *Stat) bool {
			// NOTE: here cannot use runid to identify the node,
			// for the same node may have different ip and port after quick restarted with a different addr
			if s.Node.IP == n.IP && s.Node.Port == n.Port {
				s.Count++
				return true
			}
			return false
		}); i < 0 {
			masterStat = append(masterStat, &Stat{Node: n, Count: 1})
		}
		if n.RunId != "" {
			masterIds[n.RunId] += 1
			if !slices.Contains(idAddrMap[n.RunId], n.Address()) {
				idAddrMap[n.RunId] = append(idAddrMap[n.RunId], n.Address())
			}
		}
	}
	if len(masterStat) == 0 {
		return nil, ErrNoMaster
	}
	slices.SortStableFunc(masterStat, func(i, j *Stat) int {
		if i.Count >= j.Count {
			return -1
		}
		return 1
	})

	if len(masterStat) > 1 {
		if len(masterIds) == 1 {
			// sentinel not update node info in time, which caused one node has multiple id in sentinel
			return nil, ErrAddressConflict
		}
	}

	for _, node := range s.failover.Nodes() {
		if !node.IsReady() {
			s.logger.Info("node not ready, ignored", "node", node.GetName())
			continue
		}
		registeredAddrs := idAddrMap[node.Info().RunId]
		addr := net.JoinHostPort(node.DefaultIP().String(), strconv.Itoa(node.Port()))
		addr2 := net.JoinHostPort(node.DefaultInternalIP().String(), strconv.Itoa(node.InternalPort()))
		// same runid registered with different addr
		// TODO: limit service InternalTrafficPolicy to Local
		if (len(registeredAddrs) == 1 && registeredAddrs[0] != addr && registeredAddrs[0] != addr2) ||
			len(registeredAddrs) > 1 {
			return nil, ErrAddressConflict
		}
	}

	// masterStat[0].Count == registeredNodes used to check if all nodes are consistent no matter how many sentinel nodes
	if masterStat[0].Count >= 1+len(s.nodes)/2 || masterStat[0].Count == registeredNodes {
		return masterStat[0].Node, nil
	}

	if len(flags) > 0 && flags[0] {
		var nodes []*vkcli.SentinelMonitorNode
		for _, ms := range masterStat {
			nodes = append(nodes, ms.Node)
		}
		// select proper master node
		if mn := s.masterPicking(ctx, s.failover, nodes); mn != nil {
			return mn, nil
		}
	}
	return nil, ErrMultipleMaster
}

func (s *SentinelMonitor) masterPicking(ctx context.Context, inst types.FailoverInstance,
	mnodes []*vkcli.SentinelMonitorNode) *vkcli.SentinelMonitorNode {
	var (
		maxVersionNode *vkcli.SentinelMonitorNode
		maxVersion     version.ValkeyVersion
		versions       int
		mostDataNode   *vkcli.SentinelMonitorNode
		mostDataSize   int64
		onlineTime     int64
	)
	for _, node := range inst.Nodes() {
		if err := node.Refresh(ctx); err != nil {
			s.logger.Error(err, "refresh node failed", "node", node.GetName())
			continue
		}
		if !node.IsReady() || node.Role() != core.NodeRoleMaster || node.Config()["replica-priority"] == "0" {
			continue
		}

		mnode := func(node types.ValkeyNode) *vkcli.SentinelMonitorNode {
			addr := net.JoinHostPort(node.DefaultIP().String(), strconv.Itoa(node.Port()))
			addr2 := net.JoinHostPort(node.DefaultInternalIP().String(), strconv.Itoa(node.InternalIPort()))
			for _, mnode := range mnodes {
				if mnode.Address() == addr || mnode.Address() == addr2 {
					return mnode
				}
			}
			return nil
		}(node)
		if mnode == nil {
			continue
		}

		if maxVersion == "" {
			maxVersion = node.CurrentVersion()
			maxVersionNode = mnode
			versions = 1
		} else if comp := node.CurrentVersion().Compare(maxVersion); comp != 0 {
			versions++
			if comp > 0 {
				maxVersion = node.CurrentVersion()
				maxVersionNode = mnode
			}
		}

		if node.Info().Dbsize > mostDataSize {
			mostDataSize = node.Info().Dbsize
			mostDataNode = mnode
			onlineTime = node.Info().UptimeInSeconds
		} else if mostDataSize != 0 && node.Info().Dbsize == mostDataSize {
			// TODO: this many not be the best way to select the most data node
			if node.Info().UptimeInSeconds > onlineTime {
				mostDataNode = mnode
				onlineTime = node.Info().UptimeInSeconds
			}
		}
	}
	if versions > 0 {
		return maxVersionNode
	}
	return mostDataNode
}

func (s *SentinelMonitor) Replicas(ctx context.Context) ([]*vkcli.SentinelMonitorNode, error) {
	if s == nil {
		return nil, nil
	}

	var nodes []*vkcli.SentinelMonitorNode
	for _, node := range s.nodes {
		ns, err := node.MonitoringReplicas(ctx, s.groupName)
		if err == ErrNoMaster {
			continue
		} else if err != nil {
			s.logger.Error(err, "check monitoring replica status of sentinel failed", "addr", node.addr)
			continue
		}
		for _, n := range ns {
			if i := slices.IndexFunc(nodes, func(smn *vkcli.SentinelMonitorNode) bool {
				// NOTE: here cannot use runid to identify the node,
				// for the same node may have different ip and port after quick restarted with a different addr
				return smn.IP == n.IP && smn.Port == n.Port
			}); i != -1 {
				nodes[i] = n
			} else {
				nodes = append(nodes, n)
			}
		}
	}
	return nodes, nil
}

func (s *SentinelMonitor) Inited(ctx context.Context) (bool, error) {
	if s == nil || len(s.nodes) == 0 {
		return false, ErrNoUseableNode
	}

	for _, node := range s.nodes {
		if masterNode, err := node.MonitoringMaster(ctx, s.groupName); err == ErrNoMaster {
			return false, nil
		} else if err != nil {
			return false, err
		} else if !IsMonitoringNodeOnline(masterNode) {
			return false, nil
		}
	}
	return true, nil
}

// AllNodeMonitored checks if all sentinel nodes are monitoring all the master and replicas
func (s *SentinelMonitor) AllNodeMonitored(ctx context.Context) (bool, error) {
	if s == nil || len(s.nodes) == 0 {
		return false, ErrNoUseableNode
	}

	var (
		registeredNodes = map[string]struct{}{}
		masters         = map[string]int{}
		idAddrMap       = map[string][]string{}
		mastersOffline  []string
	)
	for _, node := range s.nodes {
		if master, err := node.MonitoringMaster(ctx, s.groupName); err != nil {
			if err == ErrNoMaster {
				return false, nil
			}
		} else if master.IsFailovering() {
			return false, ErrDoFailover
		} else if IsMonitoringNodeOnline(master) {
			registeredNodes[master.Address()] = struct{}{}
			masters[master.Address()] += 1
			if master.RunId != "" && !slices.Contains(idAddrMap[master.RunId], master.Address()) {
				idAddrMap[master.RunId] = append(idAddrMap[master.RunId], master.Address())
			}
		} else {
			mastersOffline = append(mastersOffline, master.Address())
			continue
		}

		if replicas, err := node.MonitoringReplicas(ctx, s.groupName); err != nil {
			if err == ErrNoMaster {
				return false, nil
			}
		} else {
			for _, replica := range replicas {
				if IsMonitoringNodeOnline(replica) {
					registeredNodes[replica.Address()] = struct{}{}
				}
				if replica.RunId != "" && !slices.Contains(idAddrMap[replica.RunId], replica.Address()) {
					idAddrMap[replica.RunId] = append(idAddrMap[replica.RunId], replica.Address())
				}
			}
		}
	}
	if len(mastersOffline) > 0 {
		s.logger.Error(fmt.Errorf("not all nodes monitored"), "master nodes offline", "nodes", mastersOffline)
		return false, nil
	}

	for _, node := range s.failover.Nodes() {
		if !node.IsReady() {
			s.logger.Info("node not ready, ignored", "node", node.GetName())
			continue
		}
		registeredAddrs := idAddrMap[node.Info().RunId]
		addr := net.JoinHostPort(node.DefaultIP().String(), strconv.Itoa(node.Port()))
		addr2 := net.JoinHostPort(node.DefaultInternalIP().String(), strconv.Itoa(node.InternalPort()))
		// same runid registered with different addr
		// TODO: limit service InternalTrafficPolicy to Local
		if (len(registeredAddrs) == 1 && registeredAddrs[0] != addr && registeredAddrs[0] != addr2) ||
			len(registeredAddrs) > 1 {
			return false, ErrAddressConflict
		}
		_, ok := registeredNodes[addr]
		_, ok2 := registeredNodes[addr2]
		if !ok && !ok2 {
			return false, nil
		}
	}

	if len(masters) > 1 {
		return false, ErrMultipleMaster
	}
	return true, nil
}

func (s *SentinelMonitor) UpdateConfig(ctx context.Context, params map[string]string) error {
	if s == nil || len(s.nodes) == 0 {
		return ErrNoUseableNode
	}
	logger := s.logger.WithName("UpdateConfig")

	for _, node := range s.nodes {
		masterNode, err := node.MonitoringMaster(ctx, s.groupName)
		if err != nil {
			if err == ErrNoMaster || strings.HasSuffix(err.Error(), "no such host") {
				continue
			}
			logger.Error(err, "check monitoring master failed")
			return err
		}

		needUpdatedConfigs := map[string]string{}
		for k, v := range params {
			switch k {
			case "down-after-milliseconds":
				if v != fmt.Sprintf("%d", masterNode.DownAfterMilliseconds) {
					needUpdatedConfigs[k] = v
				}
			case "failover-timeout":
				if v != fmt.Sprintf("%d", masterNode.FailoverTimeout) {
					needUpdatedConfigs[k] = v
				}
			case "parallel-syncs":
				if v != fmt.Sprintf("%d", masterNode.ParallelSyncs) {
					needUpdatedConfigs[k] = v
				}
			case "auth-pass", "auth-user":
				needUpdatedConfigs[k] = v
			}
		}
		if len(needUpdatedConfigs) > 0 {
			logger.Info("update configs", "node", node.addr, "configs", lo.Keys(params))
			if err := node.UpdateConfig(ctx, s.groupName, needUpdatedConfigs); err != nil {
				logger.Error(err, "update sentinel monitor configs failed", "node", node.addr)
				return err
			}
		}
	}
	return nil
}

func (s *SentinelMonitor) Failover(ctx context.Context) error {
	if s == nil || len(s.nodes) == 0 {
		return ErrNoUseableNode
	}
	logger := s.logger.WithName("failover")

	// check most sentinel nodes available
	var availableNodes []*SentinelNode
	for _, node := range s.nodes {
		if node.IsReady() {
			availableNodes = append(availableNodes, node)
		}
	}
	if len(availableNodes) < 1+len(s.nodes)/2 {
		logger.Error(ErrNotEnoughNodes, "failover failed")
		return ErrNotEnoughNodes
	}
	if err := availableNodes[0].Failover(ctx, s.groupName); err != nil {
		logger.Error(err, "failover failed on node", "addr", availableNodes[0].addr)
		return err
	}
	return nil
}

// Monitor monitors the valkey master node on the sentinel nodes
func (s *SentinelMonitor) Monitor(ctx context.Context, masterNode types.ValkeyNode) error {
	if s == nil || len(s.nodes) == 0 {
		return fmt.Errorf("no monitor")
	}
	logger := s.logger.WithName("monitor")

	quorum := 1 + len(s.nodes)/2
	if s.failover.Definition().Spec.Sentinel.Quorum != nil {
		quorum = int(*s.failover.Definition().Spec.Sentinel.Quorum)
	}

	configs := map[string]string{
		"down-after-milliseconds": "30000",
		"failover-timeout":        "180000",
		"parallel-syncs":          "1",
	}
	maps.Copy(configs, s.failover.Definition().Spec.Sentinel.MonitorConfig)

	opUser := s.failover.Users().GetOpUser()
	configs["auth-pass"] = opUser.Password.String()
	configs["auth-user"] = opUser.Name

	masterIP, masterPort := masterNode.DefaultIP().String(), strconv.Itoa(masterNode.Port())
	for _, node := range s.nodes {
		if master, err := node.MonitoringMaster(ctx, s.groupName); err == ErrNoMaster ||
			(master != nil && (master.IP != masterIP || master.Port != masterPort || master.Quorum != int32(quorum))) ||
			!IsMonitoringNodeOnline(master) {

			if err := node.Monitor(ctx, s.groupName, masterIP, masterPort, quorum, configs); err != nil {
				logger.Error(err, "monitor failed on node", "addr", net.JoinHostPort(masterIP, masterPort))
			}
		} else if master != nil && master.IP == masterIP && master.Port == masterPort {
			needUpdate := false
		_NEED_UPDATE_:
			for k, v := range configs {
				switch k {
				case "down-after-milliseconds":
					needUpdate = v != fmt.Sprintf("%d", master.DownAfterMilliseconds)
					break _NEED_UPDATE_
				case "failover-timeout":
					needUpdate = v != fmt.Sprintf("%d", master.FailoverTimeout)
					break _NEED_UPDATE_
				case "parallel-syncs":
					needUpdate = v != fmt.Sprintf("%d", master.ParallelSyncs)
					break _NEED_UPDATE_
				}
			}
			if needUpdate {
				if err := node.UpdateConfig(ctx, s.groupName, configs); err != nil {
					logger.Error(err, "update config failed on node", "addr", net.JoinHostPort(masterIP, masterPort))
				}
			}
		}
	}
	return nil
}

func (s *SentinelMonitor) Reset(ctx context.Context) error {
	for _, node := range s.nodes {
		if err := node.Reset(ctx, s.groupName); err != nil {
			s.logger.Error(err, "reset sentinel monitor failed", "addr", node)
		}
	}
	return nil
}
