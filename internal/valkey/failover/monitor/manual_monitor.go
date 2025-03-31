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
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"time"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/util"
	clientset "github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/types"
	vkcli "github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ types.FailoverMonitor = (*ManualMonitor)(nil)

type ManualMonitor struct {
	client       clientset.ClientSet
	failover     types.FailoverInstance
	resourceName string

	logger logr.Logger
}

func NewManualMonitor(ctx context.Context, client clientset.ClientSet, inst types.FailoverInstance) (*ManualMonitor, error) {
	if client == nil {
		return nil, fmt.Errorf("require clientset")
	}
	if inst == nil {
		return nil, fmt.Errorf("require instance")
	}

	m := &ManualMonitor{
		client:       client,
		failover:     inst,
		resourceName: fmt.Sprintf("rf-ha-repl-%s", inst.GetName()),
		logger:       inst.Logger(),
	}
	return m, nil
}

func (s *ManualMonitor) Policy() v1alpha1.FailoverPolicy {
	return v1alpha1.ManualFailoverPolicy
}

func convertNodeInfoToSentinelNodeInfo(node types.ValkeyNode) *vkcli.SentinelMonitorNode {
	var (
		flag             = "slave"
		masterLinkStatus = lo.If(node.IsMasterLinkUp(), "ok").Else("down")
	)
	if node.Role() == core.NodeRoleMaster {
		flag = "master"
		masterLinkStatus = ""
	}
	mnode := vkcli.SentinelMonitorNode{
		Name:             "mymaster",
		RunId:            node.Info().RunId,
		IP:               node.DefaultIP().String(),
		Port:             fmt.Sprintf("%d", node.Port()),
		MasterHost:       node.Info().MasterHost,
		MasterPort:       node.Info().MasterPort,
		Flags:            flag,
		MasterLinkStatus: masterLinkStatus,
	}
	return &mnode
}

func (s *ManualMonitor) Master(ctx context.Context, flags ...bool) (*vkcli.SentinelMonitorNode, error) {
	cm, err := s.client.GetConfigMap(ctx, s.failover.GetNamespace(), s.resourceName)
	if errors.IsNotFound(err) {
		return nil, ErrNoMaster
	} else if err != nil {
		return nil, err
	}
	if addr := cm.Data["master"]; addr == "" {
		return nil, ErrNoMaster
	} else if ipPort, err := netip.ParseAddrPort(addr); err != nil {
		return nil, err
	} else {
		var mnode *vkcli.SentinelMonitorNode
		for _, node := range s.failover.Nodes() {
			if node.DefaultIP().String() == ipPort.Addr().String() && node.Port() == int(ipPort.Port()) {
				mnode = convertNodeInfoToSentinelNodeInfo(node)
				break
			}
		}
		if mnode == nil {
			mnode = &vkcli.SentinelMonitorNode{
				Name:  "mymaster",
				IP:    ipPort.Addr().String(),
				Port:  fmt.Sprintf("%d", ipPort.Port()),
				Flags: "down,master",
			}
		}
		return mnode, nil
	}
}

func (s *ManualMonitor) Replicas(ctx context.Context) (ret []*vkcli.SentinelMonitorNode, err error) {
	for _, node := range s.failover.Nodes() {
		if node.Role() == core.NodeRoleReplica {
			ret = append(ret, convertNodeInfoToSentinelNodeInfo(node))
		}
	}
	return ret, nil
}

func (s *ManualMonitor) Inited(ctx context.Context) (bool, error) {
	if cm, err := s.client.GetConfigMap(ctx, s.failover.GetNamespace(), s.resourceName); errors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	} else {
		return cm.Data["master"] != "", nil
	}
}

func (s *ManualMonitor) AllNodeMonitored(ctx context.Context) (bool, error) {
	registeredNodes := map[string]struct{}{}
	if masterNode, _ := s.Master(ctx); IsMonitoringNodeOnline(masterNode) {
		registeredNodes[masterNode.Address()] = struct{}{}
	}
	replicas, _ := s.Replicas(ctx)
	for _, replica := range replicas {
		if IsMonitoringNodeOnline(replica) {
			registeredNodes[replica.Address()] = struct{}{}
		}
	}
	for _, node := range s.failover.Nodes() {
		addr := net.JoinHostPort(node.DefaultIP().String(), strconv.Itoa(node.Port()))
		addr2 := net.JoinHostPort(node.DefaultInternalIP().String(), strconv.Itoa(node.InternalPort()))
		_, ok := registeredNodes[addr]
		_, ok2 := registeredNodes[addr2]
		if !ok && !ok2 {
			return false, nil
		}
	}
	return true, nil
}

func (s *ManualMonitor) UpdateConfig(ctx context.Context, params map[string]string) error {
	return nil
}

func (s *ManualMonitor) Failover(ctx context.Context) error {
	currentMaster, err := s.Master(ctx)
	if err != nil && err != ErrNoMaster {
		return err
	}

	// find one replica to promote
	if currentMaster != nil {
		for _, node := range s.failover.Nodes() {
			if node.DefaultIP().String() == currentMaster.IP &&
				strconv.Itoa(node.Port()) == currentMaster.Port &&
				node.Role() == core.NodeRoleMaster {
				// self is master, not failover
				return nil
			}
		}
	}

	// master is down, find a replica to promote
	var (
		masterCandidate types.ValkeyNode
		masterNodes     = s.failover.Masters()
	)
	if len(masterNodes) == 1 {
		masterCandidate = masterNodes[0]
	} else if nodes := s.failover.Nodes(); len(nodes) > 0 {
		slices.SortStableFunc(nodes, func(i, j types.ValkeyNode) int {
			if i.Info().ConnectedReplicas > j.Info().ConnectedReplicas {
				return -1
			}
			return 1
		})
		if nodes[0].Info().ConnectedReplicas > 0 {
			masterCandidate = nodes[0]
		}
		if masterCandidate == nil {
			replIds := map[string]struct{}{}
			for _, node := range nodes {
				if node.Info().MasterReplOffset > 0 {
					replIds[node.Info().MasterReplId] = struct{}{}
				}
			}
			if len(replIds) == 1 {
				slices.SortStableFunc(nodes, func(i, j types.ValkeyNode) int {
					if i.Info().MasterReplOffset > j.Info().MasterReplOffset {
						return -1
					}
					return 1
				})
				masterCandidate = nodes[0]
			}
		}
		if masterCandidate == nil {
			slices.SortStableFunc(nodes, func(i, j types.ValkeyNode) int {
				if i.Info().UptimeInSeconds > j.Info().UptimeInSeconds {
					return -1
				}
				return 1
			})
			masterCandidate = nodes[0]
		}
	}

	if masterCandidate != nil {
		if err := masterCandidate.ReplicaOf(ctx, "NO", "ONE"); err != nil {
			return err
		}
		time.Sleep(time.Second)
		if err := s.Monitor(ctx, masterCandidate); err != nil {
			s.logger.Error(err, "monitor failed", "node", masterCandidate.GetName())
			return err
		}
		masterIP := masterCandidate.DefaultIP().String()
		masterPort := masterCandidate.Port()
		for _, node := range s.failover.Nodes() {
			if node.DefaultIP().String() == masterIP && node.Port() == masterPort {
				continue
			}
			if err := node.ReplicaOf(ctx, masterIP, strconv.Itoa(masterPort)); err != nil {
				s.logger.Error(err, "replicaof failed", "node", node.GetName(), "master", masterCandidate.GetName())
			}
		}
	}
	return fmt.Errorf("No available node to failover")
}

func (s *ManualMonitor) Monitor(ctx context.Context, masterNode types.ValkeyNode) error {
	masterAddr := net.JoinHostPort(masterNode.DefaultIP().String(), fmt.Sprintf("%d", masterNode.Port()))
	if oldCm, err := s.client.GetConfigMap(ctx, s.failover.GetNamespace(), s.resourceName); errors.IsNotFound(err) {
		cm := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            s.resourceName,
				Namespace:       s.failover.GetNamespace(),
				Labels:          s.failover.GetLabels(),
				OwnerReferences: util.BuildOwnerReferences(s.failover.Definition()),
			},
			Data: map[string]string{
				"master": masterAddr,
			},
		}
		if err := s.client.CreateConfigMap(ctx, s.failover.GetNamespace(), &cm); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if oldCm.Data["master"] != masterAddr {
		oldCm.Data["master"] = masterAddr
		if err := s.client.UpdateConfigMap(ctx, s.failover.GetNamespace(), oldCm); err != nil {
			return err
		}
	}
	return nil
}

func (s *ManualMonitor) Reset(ctx context.Context) error {
	return nil
}
