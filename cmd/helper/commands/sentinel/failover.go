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
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/chideat/valkey-operator/cmd/helper/commands"
	"github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/go-logr/logr"
	"github.com/urfave/cli/v2"
	"k8s.io/client-go/kubernetes"
)

func checkReadyToFailover(ctx context.Context, nodes []string, name string, senAuthInfo *valkey.AuthInfo, logger logr.Logger) ([]string, error) {
	var healthyNodes []string
	for _, node := range nodes {
		if err := func() error {
			senClient := valkey.NewValkeyClient(node, *senAuthInfo)
			defer senClient.Close()

			val, err := senClient.DoWithTimeout(ctx, time.Second*3, "SENTINEL", "MASTER", name)
			if err != nil {
				senClient.Close()
				return err
			}

			fields, _ := valkey.StringMap(val, nil)
			if fields["flags"] == "master" {
				return nil
			}
			return fmt.Errorf("monitoring master not healthy, flags=%s", fields["flags"])
		}(); err == nil {
			healthyNodes = append(healthyNodes, node)
		} else {
			logger.Error(err, "check monitoring master failed", "node", node)
		}
	}
	return healthyNodes, nil
}

var (
	ErrFailoverRetry = errors.New("failover retry")
	ErrFailoverAbort = errors.New("failover abort")
	ErrNoMaster      = errors.New("no master found")
)

func batchDo(ctx context.Context, nodes []string, name string, senAuthInfo *valkey.AuthInfo, minSucceed int, logger logr.Logger,
	callback func(ctx context.Context, client valkey.ValkeyClient) (any, error)) (rets []any, err error) {

	logger.Info("check sentinel status")
	healthySentinelNodeAddrs, err := checkReadyToFailover(ctx, nodes, name, senAuthInfo, logger)
	if err != nil {
		logger.Error(err, "cluster not ready to do failover, retry in 10s")
		time.Sleep(time.Second * 10)
		return nil, err
	}
	if len(healthySentinelNodeAddrs) < len(nodes)/2+1 {
		logger.Error(errors.New("not enough healthy sentinel nodes"), "cluster not ready to do failover, retry in 10s")
		return nil, ErrFailoverRetry
	}

	for _, node := range healthySentinelNodeAddrs {
		if len(rets) >= minSucceed {
			break
		}
		func() {
			senClient := valkey.NewValkeyClient(node, *senAuthInfo)
			defer senClient.Close()

			if ret, err := callback(ctx, senClient); err != nil {
				logger.Error(err, "do command failed", "node", node)
			} else {
				rets = append(rets, ret)
			}
		}()
	}
	return
}

func getCurrentMaster(ctx context.Context, nodes []string, name string, senAuthInfo *valkey.AuthInfo, logger logr.Logger) (string, error) {
	quorum := len(nodes)/2 + 1
	masterInfos, err := batchDo(ctx, nodes, name, senAuthInfo, len(nodes), logger, func(ctx context.Context, client valkey.ValkeyClient) (any, error) {
		if info, err := valkey.StringMap(client.DoWithTimeout(ctx, time.Second*3, "SENTINEL", "MASTER", name)); err != nil {
			return nil, err
		} else if info["flags"] == "master" && info["ip"] != "" && info["port"] != "" {
			return net.JoinHostPort(info["ip"], info["port"]), nil
		}
		return nil, errors.New("master not found")
	})
	if err != nil {
		return "", err
	}

	if len(masterInfos) == 0 {
		return "", ErrFailoverAbort
	}

	masterAddr := ""
	masterGroup := map[string]int{}
	for _, val := range masterInfos {
		if val == nil {
			continue
		}
		addr := val.(string)
		masterGroup[addr]++
		if masterGroup[addr] >= quorum {
			masterAddr = addr
			break
		}
	}
	if masterAddr == "" {
		return "", ErrNoMaster
	}
	return masterAddr, nil
}

func doFailover(ctx context.Context, nodes []string, name string, senAuthInfo *valkey.AuthInfo, escapeAddresses map[string]struct{}, logger logr.Logger) error {
	if masterAddr, err := getCurrentMaster(ctx, nodes, name, senAuthInfo, logger); err != nil {
		return err
	} else {
		fields := strings.Split(masterAddr, ":")
		ok2 := fields[len(fields)-1] != "6379"
		if port := fields[len(fields)-1]; port != "6379" {
			_, ok2 = escapeAddresses[port]
		}
		if _, ok := escapeAddresses[masterAddr]; ok || ok2 {
			logger.Info("current node is master, try failover", "master", masterAddr)
		} else {
			logger.Info("master is not in serve addresses, skip", "master", masterAddr, "escapeAddresses", escapeAddresses)
			return nil
		}
	}

	if _, err := batchDo(ctx, nodes, name, senAuthInfo, 1, logger, func(ctx context.Context, senClient valkey.ValkeyClient) (any, error) {
		return senClient.DoWithTimeout(ctx, time.Second, "SENTINEL", "FAILOVER", name)
	}); err != nil {
		logger.Error(err, "do failover failed")
		return ErrFailoverRetry
	}

	for j := 0; j < 6; j++ {
		if masterAddr, err := getCurrentMaster(ctx, nodes, name, senAuthInfo, logger); err != nil {
			return err
		} else {
			fields := strings.Split(masterAddr, ":")
			ok2 := false
			if port := fields[len(fields)-1]; port != "6379" {
				_, ok2 = escapeAddresses[port]
			}
			if _, ok := escapeAddresses[masterAddr]; !ok && !ok2 {
				logger.Info("failover success", "master", masterAddr)
				return nil
			}
		}
		time.Sleep(time.Second * 5)
	}
	return ErrFailoverRetry
}

func Failover(ctx context.Context, c *cli.Context, client *kubernetes.Clientset, logger logr.Logger) error {
	var (
		sentinelUri      = c.String("monitor-uri")
		name             = c.String("name")
		podIPs           = c.String("pod-ips")
		_escapeAddresses = c.StringSlice("escape")
		escapeAddresses  = make(map[string]struct{})
	)
	if sentinelUri == "" {
		return nil
	}
	for _, addr := range _escapeAddresses {
		if addr != "" && addr != ":" && addr != "[]:" {
			if strings.HasPrefix(addr, "[") {
				if a, err := netip.ParseAddrPort(addr); err == nil {
					addr = net.JoinHostPort(a.Addr().String(), fmt.Sprintf("%d", a.Port()))
				}
			}
			escapeAddresses[addr] = struct{}{}
		}
	}
	if podIPs != "" {
		for _, ip := range strings.Split(podIPs, ",") {
			escapeAddresses[net.JoinHostPort(ip, "6379")] = struct{}{}
		}
	}

	var sentinelNodes []string
	if val, err := url.Parse(sentinelUri); err != nil {
		return cli.Exit(fmt.Sprintf("parse sentinel uri failed, error=%s", err), 1)
	} else {
		sentinelNodes = strings.Split(val.Host, ",")
	}

	senAuthInfo, err := commands.LoadMonitorAuthInfo(c, ctx, client)
	if err != nil {
		logger.Error(err, "load sentinel auth info failed")
		return cli.Exit(fmt.Sprintf("load sentinel auth info failed, error=%s", err), 1)
	}

	var masterAddr string
	for range 10 {
		select {
		case <-ctx.Done():
			return context.DeadlineExceeded
		default:
		}

		err := doFailover(ctx, sentinelNodes, name, senAuthInfo, escapeAddresses, logger)
		if err == ErrFailoverAbort {
			logger.Error(err, "failover aborted")
			return err
		} else if err != nil {
			logger.Error(err, "do failover failed")
			time.Sleep(time.Second * 5)
			continue
		}

		if masterAddr, err = getCurrentMaster(ctx, sentinelNodes, name, senAuthInfo, logger); err != nil {
			if err == ErrFailoverAbort {
				logger.Error(err, "failover aborted")
				return err
			}
			logger.Error(err, "get current master failed")
			continue
		} else {
			fields := strings.Split(masterAddr, ":")
			ok2 := false
			if port := fields[len(fields)-1]; port != "6379" {
				_, ok2 = escapeAddresses[port]
			}
			if _, ok := escapeAddresses[masterAddr]; ok || ok2 {
				logger.Info("current node is master, try failover", "master", masterAddr)
				time.Sleep(time.Second * 5)
			} else {
				logger.Info("master is not in serve addresses, skip", "master", masterAddr, "escapeAddresses", escapeAddresses)
				break
			}
		}
	}
	if masterAddr == "" {
		return ErrNoMaster
	} else if _, ok := escapeAddresses[masterAddr]; ok {
		logger.Info("try failover failed", "old master", masterAddr)
		return ErrFailoverAbort
	} else {
		logger.Info("master is not current node", "master", masterAddr)
		fmt.Println(masterAddr)
	}
	return nil
}
