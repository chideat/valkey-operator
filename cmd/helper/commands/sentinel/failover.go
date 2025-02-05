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
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/chideat/valkey-operator/cmd/helper/commands"
	"github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/go-logr/logr"
	"github.com/urfave/cli/v2"
	"k8s.io/client-go/kubernetes"
)

func isNeededToFailover(ctx context.Context, valkeyCli valkey.ValkeyClient, name string, escapeAddresses []string, logger logr.Logger) (bool, string, error) {
	logger.Info("check if failover is required")
	masterInfo, err := valkey.StringMap(valkeyCli.DoWithTimeout(ctx, time.Second, "SENTINEL", "MASTER", name))
	if err != nil {
		logger.Error(err, "get sentinel info failed")
		return false, "", err
	}
	var (
		masterAddr string
		flags      = masterInfo["flags"]
	)
	if masterInfo["ip"] != "" && masterInfo["port"] != "" {
		masterAddr = net.JoinHostPort(masterInfo["ip"], masterInfo["port"])
	}
	if masterAddr == "" {
		logger.Info("master not registered yet, abort failover")
		return false, "", nil
	}
	if slices.Contains(escapeAddresses, masterAddr) || strings.Contains(flags, "down") || strings.Contains(flags, "disconnected") {
		if numSlaves := masterInfo["num-slaves"]; numSlaves == "0" || numSlaves == "" {
			logger.Info("no suitable replica to promote, abort failover")
			return false, "", nil
		}
		return true, masterAddr, nil
	}
	return false, "", nil
}

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
)

func doFailover(ctx context.Context, nodes []string, name string, senAuthInfo *valkey.AuthInfo, escapeAddresses []string, logger logr.Logger) error {
	var (
		valkeyCli                  valkey.ValkeyClient
		noSentinelUsableCheckCount int
	)
	for i := 0; noSentinelUsableCheckCount < 2 && i < 6; i++ {
		healthySentinelNodeAddrs, err := checkReadyToFailover(ctx, nodes, name, senAuthInfo, logger)
		if err != nil {
			logger.Error(err, "check monitoring master failed, retry later")
			time.Sleep(time.Second * 5)
			continue
		} else if len(healthySentinelNodeAddrs) < len(nodes)/2+1 {
			if len(healthySentinelNodeAddrs) == 0 {
				noSentinelUsableCheckCount += 1
				time.Sleep(time.Second * 10)
				continue
			}
			logger.Error(errors.New("not enough healthy sentinel nodes"), "cluster not ready to do failover, retry in 5s")
			time.Sleep(time.Second * 5)
			continue
		}

		// connect to the first healthy sentinel node
		for _, node := range healthySentinelNodeAddrs {
			if valkeyCli, err = func() (valkey.ValkeyClient, error) {
				senClient := valkey.NewValkeyClient(node, *senAuthInfo)
				if _, err := senClient.DoWithTimeout(ctx, time.Second, "PING"); err != nil {
					senClient.Close()
					return nil, err
				}
				return senClient, nil
			}(); err != nil {
				logger.Error(err, "connect to sentinel node failed")
				continue
			}
			break
		}
	}
	if valkeyCli == nil {
		logger.Error(errors.New("no sentinel node usable"), "failover abort")
		return ErrFailoverAbort
	}
	defer valkeyCli.Close()

	nf, masterAddr, err := isNeededToFailover(ctx, valkeyCli, name, escapeAddresses, logger)
	if err != nil {
		logger.Error(err, "check failover failed")
		return ErrFailoverRetry
	} else if !nf {
		logger.Info("no need to failover, abort")
		return ErrFailoverAbort
	}

	logger.Info("current node need failover, try failover", "master", masterAddr)
	if _, err := valkeyCli.DoWithTimeout(ctx, time.Second, "SENTINEL", "FAILOVER", name); err != nil {
		logger.Error(err, "do failover failed")
		return ErrFailoverRetry
	}
	return ErrFailoverRetry
}

func Failover(ctx context.Context, c *cli.Context, client *kubernetes.Clientset, logger logr.Logger) error {
	var (
		sentinelUri       = c.String("monitor-uri")
		name              = c.String("name")
		podIPs            = c.String("pod-ips")
		failoverAddresses = c.StringSlice("escape")
	)
	if sentinelUri == "" {
		return nil
	}
	if podIPs != "" {
		for _, ip := range strings.Split(podIPs, ",") {
			failoverAddresses = append(failoverAddresses, net.JoinHostPort(ip, "6379"))
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

	for i := 0; i < 10; i++ {
		err := doFailover(ctx, sentinelNodes, name, senAuthInfo, failoverAddresses, logger)
		if err == ErrFailoverAbort {
			break
		} else if err == ErrFailoverRetry {
			time.Sleep(time.Second * 5)
			continue
		} else if err != nil {
			logger.Error(err, "failover failed")
			time.Sleep(time.Second * 5)
			continue
		}
		break
	}
	return nil
}
