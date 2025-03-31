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

package runner

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/chideat/valkey-operator/pkg/slot"
	"github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
)

const (
	SlaveWaitInterval  = time.Second * 60
	MasterWaitInterval = time.Second * 2

	DefaultValkeyServerPort = "6379"
)

// RebalanceSlots
func RebalanceSlots(ctx context.Context, authInfo valkey.AuthInfo, address string, logger logr.Logger) error {
	// check current nodes info
	client := valkey.NewValkeyClient(address, authInfo)
	defer client.Close()

	// user timer todo wait
	timer := time.NewTimer(time.Millisecond * 100)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}

		logger.V(2).Info("check rebalance")

		// get slots
		nodes, err := client.Nodes(ctx)
		if err != nil {
			logger.Error(err, "get nodes info failed", "user", authInfo.Username)

			timer.Reset(MasterWaitInterval)
			continue
		}

		self := nodes.Self()
		if self.Role != valkey.MasterRole {
			timer.Reset(SlaveWaitInterval)
			continue
		}

		if err := func() error {
			logger.V(2).Info("check cluster ok status for 15 seconds")
			// make sure cluster status ok for at least 15 seconds
			startTime := time.Now().Unix()
			for time.Now().Unix()-startTime < 15 {
				if err := func() error {
					nctx, cancel := context.WithTimeout(ctx, time.Second*3)
					defer cancel()

					if ok, err := isClusterStateOk(nctx, client); err != nil {
						logger.Error(err, "check cluster status failed")
						return err
					} else if !ok {
						logger.Info("cluster status not ok, restart check")
						return fmt.Errorf("cluster status not ok")
					}
					return nil
				}(); err != nil {
					return err
				}
				time.Sleep(time.Second * 3)
			}
			return nil
		}(); err != nil {
			timer.Reset(MasterWaitInterval)
			continue
		}

		slots := self.Slots()
		logger.V(2).Info("slots", "importing slots", slots.Slots(slot.SlotImporting))
		if migCount := slots.Count(slot.SlotImporting); migCount == 0 {
			timer.Reset(MasterWaitInterval)
			continue
		}

		var (
			slotIds      = slots.Slots(slot.SlotImporting)
			lastNodeId   string
			sourceClient valkey.ValkeyClient
			sourceNode   *valkey.ClusterNode
		)
		for i, slotId := range slotIds {
			_, sourceNodeId := slots.MoveingStatus(slotId)
			if sourceNodeId == "" {
				logger.Info("slot importing source not found ", "slot", slotId)
				continue
			}
			if lastNodeId != sourceNodeId {
				if sourceClient != nil {
					sourceClient.Close()
					sourceClient = nil
					sourceNode = nil
				}

				if sourceNode = nodes.Get(sourceNodeId); sourceNode == nil {
					logger.V(2).Error(err, "source node not found", "id", sourceNodeId)
					break
				}
				if sourceAddr, err := func() (string, error) {
					client := client.Clone(ctx, sourceNode.Addr)

					// TRICK: local connect to nodeport not work
					if ret, err := valkey.Strings(client.Do(ctx, "CONFIG", "GET", "bind")); err != nil {
						logger.Error(err, "get node bind address failed", "id", sourceNode.Id)
						return "", err
					} else if len(ret) == 2 {
						bindIPs := strings.Fields(ret[1])
						ip := getLocalAddressByIPFamily(os.Getenv("IP_FAMILY_PREFER"), bindIPs, os.Getenv("POD_IP"))
						return net.JoinHostPort(ip, DefaultValkeyServerPort), nil
					}
					return "", errors.New("no valid bind address found")
				}(); err != nil {
					break
				} else {
					sourceClient = client.Clone(ctx, sourceAddr)
					lastNodeId = sourceNodeId
				}
			}

			if i%100 == 0 {
				if ok, err := isClusterStateOk(ctx, client); err != nil {
					logger.Error(err, "check cluster status failed")
					break
				} else if !ok {
					logger.Info("cluster status not ok, wait for next round")
					break
				}
			}

			if err := importSlots(ctx, nodes, self, sourceNode, client, sourceClient, slotId, authInfo, logger); err != nil {
				logger.Error(err, "import slot failed", "slot", slotId, "source", sourceNode.Addr, "dest", self.Addr)
				break
			}
		}
		if sourceClient != nil {
			sourceClient.Close()
			sourceClient = nil
		}
		timer.Reset(MasterWaitInterval)
	}
}

func getLocalAddressByIPFamily(family string, ips []string, def string) string {
	nips := ips[0:0]
	for _, addr := range ips {
		if addr == "127.0.0.1" || addr == "localhost" || addr == "local.inject" || addr == "ipv6-localhost" {
			continue
		}
		nips = append(nips, addr)
	}
	if len(nips) == 0 {
		return def
	}

	if family == "" {
		return nips[0]
	}
	for _, addr := range ips {
		ip, _ := netip.ParseAddr(addr)
		if ip.Is4() && family == string(v1.IPv4Protocol) ||
			ip.Is6() && family == string(v1.IPv6Protocol) {
			return addr
		}
	}
	return def
}

func importSlots(ctx context.Context, nodes valkey.ClusterNodes, currentNode, sourceNode *valkey.ClusterNode, client, sourceClient valkey.ValkeyClient, slotId int, authInfo valkey.AuthInfo, logger logr.Logger) error {
	broadcastSlotStatus := func() {
		for _, node := range nodes {
			if node.Role == valkey.SlaveRole ||
				node.Id == sourceNode.Id ||
				node.Id == currentNode.Id ||
				node.IsFailed() || !node.IsConnected() {
				continue
			}

			func() {
				tmpClient := client.Clone(ctx, node.Addr)
				defer tmpClient.Close()

				// ignore errors
				if err := RetrySetSlotStatus(ctx, tmpClient, slotId, currentNode.Id, 2); err != nil {
					logger.Error(err, "broadcast slot status failed", "slot", slotId, "node", node.Addr)
				}
			}()
		}
	}

	sourceNodesInfo, err := sourceClient.Nodes(ctx)
	if err != nil {
		logger.Error(err, "load nodes info failed")
		return err
	}
	sourceSelf := sourceNodesInfo.Self()
	sourceSlots := sourceSelf.Slots()
	if status := sourceSlots.Status(slotId); status != slot.SlotMigrating {
		if status == slot.SlotAssigned {
			return fmt.Errorf("the slot migrating flag not set, check in next round")
			// if _, err := sourceClient.Do(ctx, "CLUSTER", "SETSLOT", slotId, "MIGRATING", currentNode.ID); err != nil {
			// 	logger.Error(err, "fix the missing slot migrating flag failed", "slot", slotId)
			// 	return err
			// }
		} else {
			currentSlotNodeId := func() string {
				// TODO: fix failed source migrating
				masterNodes := sourceNodesInfo.Masters()
				for _, node := range masterNodes {
					if node.Slots().Status(slotId) == slot.SlotAssigned {
						return node.Id
					}
				}
				return ""
			}()
			if currentSlotNodeId == currentNode.Id {
				// update slot info
				if err := RetrySetSlotStatus(ctx, client, slotId, currentNode.Id, 5); err != nil {
					logger.Error(err, "clean slot importing flags failed", "slot", slotId)
					return err
				}
				broadcastSlotStatus()

				return nil
			}

			err := fmt.Errorf("slot not owned by node")
			logger.Error(err, "check slot status failed", "slot", slotId, "node", sourceNode.Id, "status", status)
			return err
		}
	}

	localAddr := getLocalAddressByIPFamily(os.Getenv("IP_FAMILY_PREFER"),
		strings.Split(os.Getenv("POD_IPS"), ","), os.Getenv("POD_IP"))

	logger.Info("import slot", "slot", slotId, "source", sourceNode.Id, "desc", currentNode.Id)
	for {
		// get keys of this slot
		keys, err := valkey.Values(sourceClient.Do(ctx, "cluster", "getkeysinslot", slotId, 3))
		if err != nil {
			logger.Error(err, "get keys of slot failed", "slot", slotId)
			return err
		}
		if len(keys) == 0 {
			break
		}

		// references: https://valkey.io/commands/migrate/
		args := []any{localAddr, DefaultValkeyServerPort, "", 0, 3600000, "REPLACE"}
		if authInfo.Password != "" {
			if authInfo.Username != "" && authInfo.Username != "default" {
				args = append(args, "AUTH2", authInfo.Username, authInfo.Password)
			} else {
				args = append(args, "AUTH", authInfo.Password)
			}
		}

		args = append(args, "KEYS")
		args = append(args, keys...)
		if _, err := sourceClient.Do(ctx, "MIGRATE", args...); err != nil {
			logger.V(2).Error(err, "migrate keys failed")
			return err
		}
		time.Sleep(time.Millisecond * 50)
	}

	if err := RetrySetSlotStatus(ctx, client, slotId, currentNode.Id, 5); err != nil {
		logger.Error(err, "clean slot importing flags failed", "slot", slotId)
		return err
	}
	// update slot info
	if err := RetrySetSlotStatus(ctx, sourceClient, slotId, currentNode.Id, 5); err != nil {
		logger.Error(err, "clean slot migrating flags failed", "slot", slotId)
		return err
	}

	broadcastSlotStatus()

	time.Sleep(time.Millisecond * 100)

	return nil
}

func RetrySetSlotStatus(ctx context.Context, client valkey.ValkeyClient, slotId int, nodeId string, retryCount int) (err error) {
	for i := 0; i < retryCount; i++ {
		if _, err = client.Do(ctx, "CLUSTER", "SETSLOT", slotId, "NODE", nodeId); err == nil {
			return nil
		}
		if errors.Is(err, context.DeadlineExceeded) {
			continue
		}
	}
	return
}

func isClusterStateOk(ctx context.Context, cli valkey.ValkeyClient) (bool, error) {
	nctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := valkey.String(cli.Do(nctx, "CLUSTER", "INFO"))
	if err != nil {
		return false, err
	}
	return strings.Contains(resp, "cluster_state:ok") || strings.Contains(resp, "cluster_state: ok"), nil
}
