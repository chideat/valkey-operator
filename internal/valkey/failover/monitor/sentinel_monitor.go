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

package monitor

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"

	v1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/util"
	clientset "github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/types"
	vkcli "github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrNoMaster        = fmt.Errorf("no master")
	ErrDoFailover      = fmt.Errorf("redis sentinel doing failover")
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
			s.logger.Error(err, "check monitor status of sentinel failed", "addr", node.addr)
			s.logger.Error(err, "check monitoring master status of sentinel failed", "addr", node.addr)
			return nil, err
		} else if n.IsFailovering() {
			s.logger.Error(ErrDoFailover, "redis sentinel is doing failover", "node", n.Address())
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
			return nil, ErrAddressConflict
		}
	}
	if masterStat[0].Count >= 1+len(s.nodes)/2 || masterStat[0].Count == registeredNodes {
		return masterStat[0].Node, nil
	}

	if len(flags) > 0 && flags[0] {
		// NOTE: force to return the master node with the oldest role reported time
		stat := masterStat[0]
		for _, ms := range masterStat {
			if ms.Node.RoleReportedTime > stat.Node.RoleReportedTime {
				stat = ms
			}
		}
		if stat.Count >= len(s.nodes)/2 {
			return stat.Node, nil
		}
	}
	return nil, ErrMultipleMaster
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
			s.logger.Error(err, "check monitor status of sentinel failed", "addr", node.addr)
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
		return false, fmt.Errorf("no sentinel nodes")
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
		return false, fmt.Errorf("no sentinel nodes")
	}

	var (
		registeredNodes = map[string]struct{}{}
		masters         = map[string]int{}
		masterIds       = map[string]int{}
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
			if master.RunId != "" {
				masterIds[master.RunId] += 1
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
		addr := net.JoinHostPort(node.DefaultIP().String(), strconv.Itoa(node.Port()))
		addr2 := net.JoinHostPort(node.DefaultInternalIP().String(), strconv.Itoa(node.InternalPort()))
		_, ok := registeredNodes[addr]
		_, ok2 := registeredNodes[addr2]
		if !ok && !ok2 {
			return false, nil
		}
	}

	if len(masters) > 1 {
		if len(masterIds) == 1 {
			return false, ErrAddressConflict
		}
		return false, ErrMultipleMaster
	}
	return true, nil
}

func (s *SentinelMonitor) UpdateConfig(ctx context.Context, params map[string]string) error {
	if s == nil || len(s.nodes) == 0 {
		return fmt.Errorf("no sentinel nodes")
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
		return fmt.Errorf("no sentinel nodes")
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

// Monitor monitors the redis master node on the sentinel nodes
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
	for k, v := range s.failover.Definition().Spec.Sentinel.MonitorConfig {
		configs[k] = v
	}

	isAllAclSupported := func() bool {
		for _, node := range s.nodes {
			if !node.Version().IsACLSupported() {
				return false
			}
		}
		return true
	}()

	if s.failover.Version().IsACLSupported() && s.failover.IsACLAppliedToAll() && isAllAclSupported {
		opUser := s.failover.Users().GetOpUser()
		configs["auth-pass"] = opUser.Password.String()
		configs["auth-user"] = opUser.Name
	} else {
		user := s.failover.Users().GetDefaultUser()
		configs["auth-pass"] = user.Password.String()
	}

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
