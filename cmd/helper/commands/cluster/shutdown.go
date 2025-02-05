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

package cluster

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/chideat/valkey-operator/cmd/helper/commands"
	"github.com/chideat/valkey-operator/cmd/helper/commands/runner"
	"github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/go-logr/logr"
	"github.com/urfave/cli/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// Shutdown 在退出时做 failover
//
// NOTE: 在4.0, 5.0中，在更新密码时，会重启实例。但是由于密码在重启前已经热更新，导致其他脚本无法连接到实例，包括shutdown脚本
// 为了解决这个问题，针对4,5 版本，会在重启前，先做failover，将master failover 到-0 节点。
// 由于重启是逆序的，最后一个pod启动成功之后，会使用新密码连接到 master，从而确保服务一直可用,切数据不会丢失
func Shutdown(ctx context.Context, c *cli.Context, client *kubernetes.Clientset, logger logr.Logger) error {
	var (
		podName = c.String("pod-name")
		timeout = time.Duration(c.Int("timeout")) * time.Second
	)
	if timeout == 0 {
		timeout = time.Second * 300
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	logger.Info("check local nodes.conf")
	authInfo, err := commands.LoadAuthInfo(c, ctx)
	if err != nil {
		logger.Error(err, "load valkey operator user info failed")
		return err
	}

	addr := net.JoinHostPort("local.inject", "6379")
	valkeyClient := valkey.NewValkeyClient(addr, *authInfo)
	defer valkeyClient.Close()

	logger.Info("rewrite nodes.conf")
	if _, err := valkeyClient.Do(ctx, "CLUSTER", "SAVECONFIG"); err != nil {
		logger.Error(err, "rewrite nodes.conf failed, ignored")
	}

	// sync current nodes.conf to configmap
	logger.Info("persistent nodes.conf to configmap")
	if err := runner.SyncFromLocalToEtcd(c, ctx, "", false, logger); err != nil {
		logger.Error(err, "persistent nodes.conf to configmap failed")
	}

	// get all nodes
	data, err := valkey.Bytes(valkeyClient.Do(ctx, "CLUSTER", "NODES"))
	if err != nil {
		logger.Error(err, "get cluster nodes failed")
		return nil
	}

	nodes, err := valkey.ParseNodes(string(data))
	if err != nil {
		logger.Error(err, "parse cluster nodes failed")
		return nil
	}
	if nodes == nil {
		logger.Info("no nodes found")
		return nil
	}

	self := nodes.Self()
	if !self.IsJoined() {
		logger.Info("node not joined")
		return nil
	}

	// NOTE: disable auto failover for terminating pods
	// TODO: update this config to cluster-replica-no-failover
	configName := "cluster-slave-no-failover"
	if _, err := valkeyClient.Do(ctx, "CONFIG", "SET", configName, "yes"); err != nil {
		logger.Error(err, "disable slave failover failed")
	}

	if self.Role == valkey.MasterRole {

		getCandidatePod := func() (*v1.Pod, error) {
			// find pod which is a replica of me
			pods, err := getPodsOfShard(ctx, c, client, logger)
			if err != nil {
				logger.Error(err, "list pods failed")
				return nil, err
			}
			if len(pods) == 1 {
				return nil, nil
			}
			for _, pod := range pods {
				if pod.GetName() == podName {
					continue
				}
				if pod.GetDeletionTimestamp() != nil {
					continue
				}

				if !func() bool {
					for _, cont := range pod.Status.ContainerStatuses {
						if cont.Name == "valkey" && cont.Ready {
							return true
						}
					}
					return false
				}() {
					continue
				}

				addr := getPodAccessAddr(pod.DeepCopy())
				logger.Info("check node", "pod", pod.GetName(), "addr", addr)
				valkeyClient := valkey.NewValkeyClient(addr, *authInfo)
				defer valkeyClient.Close()

				nodes, err := valkeyClient.Nodes(ctx)
				if err != nil {
					logger.Error(err, "load cluster nodes failed")
					return nil, err
				}
				currentNode := nodes.Self()
				if currentNode.MasterId == self.Id {
					return pod.DeepCopy(), nil
				}
			}
			return nil, fmt.Errorf("not candidate pod found")
		}

		for i := 0; i < 20; i++ {
			logger.Info(fmt.Sprintf("try %d failover", i))
			if err := func() error {
				canPod, err := getCandidatePod()
				if err != nil {
					logger.Error(err, "get candidate pod failed")
					return err
				} else if canPod == nil {
					return nil
				}

				randInt := rand.Intn(50) + 1 // #nosec: ignore
				duration := time.Duration(randInt) * time.Second
				logger.Info(fmt.Sprintf("Wait for %s to escape failover conflict", duration))
				time.Sleep(duration)

				addr := getPodAccessAddr(canPod)
				valkeyClient := valkey.NewValkeyClient(addr, *authInfo)
				defer valkeyClient.Close()

				nodes, err := valkeyClient.Nodes(ctx)
				if err != nil {
					logger.Error(err, "load cluster nodes failed")
					return err
				}
				mastrNode := nodes.Get(self.Id)
				action := NoFailoverAction
				if mastrNode.IsFailed() {
					action = ForceFailoverAction
				}
				if err := doValkeyFailover(ctx, valkeyClient, action, logger); err != nil {
					logger.Error(err, "do failed failed")
					return err
				}
				return nil
			}(); err == nil {
				break
			}
			time.Sleep(time.Second * 5)
		}
	}

	// wait for some time for nodes to sync info
	time.Sleep(time.Second * 10)

	logger.Info("do shutdown node")
	if _, err = valkeyClient.Do(ctx, "SHUTDOWN"); err != nil && !errors.Is(err, io.EOF) {
		logger.Error(err, "graceful shutdown failed")
	}
	return nil
}

func getPodAccessAddr(pod *v1.Pod) string {
	addr := net.JoinHostPort(pod.Status.PodIP, "6379")
	ipFamilyPrefer := os.Getenv("IP_FAMILY_PREFER")
	if ipFamilyPrefer != "" {
		for _, podIp := range pod.Status.PodIPs {
			ip, _ := netip.ParseAddr(podIp.IP)
			if ip.Is6() && ipFamilyPrefer == string(v1.IPv6Protocol) {
				addr = net.JoinHostPort(podIp.IP, "6379")
				break
			} else if ip.Is4() && ipFamilyPrefer == string(v1.IPv4Protocol) {
				addr = net.JoinHostPort(podIp.IP, "6379")
				break
			}
		}
	}
	return addr
}
