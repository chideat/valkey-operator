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

package failover

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/chideat/valkey-operator/cmd/helper/commands"
	"github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/go-logr/logr"
	"github.com/urfave/cli/v2"
	"k8s.io/client-go/kubernetes"
)

func getValkeyInfo(ctx context.Context, valkeyClient valkey.ValkeyClient, logger logr.Logger) (*valkey.NodeInfo, error) {
	var (
		err  error
		info *valkey.NodeInfo
	)
	for i := 0; i < 5; i++ {
		if info, err = valkeyClient.Info(ctx); err != nil {
			logger.Error(err, "get info failed, retry...")
			time.Sleep(time.Second)
			continue
		}
		break
	}
	return info, err
}

func getValkeyConfig(ctx context.Context, valkeyClient valkey.ValkeyClient, logger logr.Logger) (map[string]string, error) {
	var (
		err error
		cfg map[string]string
	)
	for i := 0; i < 5; i++ {
		cfg, err = valkeyClient.ConfigGet(ctx, "*")
		if err != nil {
			logger.Error(err, "get config failed")
			time.Sleep(time.Second)
			continue
		}
		break
	}
	return cfg, err
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
			logger.Error(err, "check monitoring master failed")
		}
	}
	return healthyNodes, nil
}

var (
	ErrFailoverRetry = errors.New("failover retry")
	ErrFailoverAbort = errors.New("failover abort")
)

func doFailover(ctx context.Context, nodes []string, name string, senAuthInfo *valkey.AuthInfo, serveAddresses map[string]struct{}, logger logr.Logger) error {
	var (
		senClient                  valkey.ValkeyClient
		noSentinelUsableCheckCount int
	)
	for i := 0; noSentinelUsableCheckCount < 2; i++ {
		logger.Info("check sentinel status")
		healthySentinelNodeAddrs, err := checkReadyToFailover(ctx, nodes, name, senAuthInfo, logger)
		if err != nil {
			logger.Error(err, "cluster not ready to do failover, retry in 10s")
			time.Sleep(time.Second * 10)
			continue
		} else if len(healthySentinelNodeAddrs) < len(nodes)/2+1 {
			if len(healthySentinelNodeAddrs) == 0 {
				noSentinelUsableCheckCount += 1
				time.Sleep(time.Second * 15)
				continue
			}
			logger.Error(errors.New("not enough healthy sentinel nodes"), "cluster not ready to do failover, retry in 10s")
			time.Sleep(time.Second * 10)
			continue
		}
		noSentinelUsableCheckCount = 0

		// connect to the first healthy sentinel node
		for _, node := range healthySentinelNodeAddrs {
			if senClient, err = func() (valkey.ValkeyClient, error) {
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
		if senClient != nil {
			break
		}
		time.Sleep(time.Second * 10)
	}
	if senClient == nil {
		logger.Error(errors.New("no sentinel node usable"), "failover abort")
		return ErrFailoverAbort
	}
	defer senClient.Close()

	masterInfo, err := valkey.StringMap(senClient.DoWithTimeout(ctx, time.Second, "SENTINEL", "MASTER", name))
	if err != nil {
		logger.Error(err, "get master info failed")
		return ErrFailoverRetry
	}
	var addr string
	if ip, port := masterInfo["ip"], masterInfo["port"]; ip != "" && port != "" {
		addr = net.JoinHostPort(ip, port)
	}

	logger.Info("current node is master, try failover", "master", addr)
	if _, err = senClient.DoWithTimeout(ctx, time.Second, "SENTINEL", "FAILOVER", name); err != nil {
		logger.Error(err, "do failover failed")
		return ErrFailoverRetry
	}

	for j := 0; j < 6; j++ {
		if masterInfo, err := valkey.StringMap(senClient.DoWithTimeout(ctx, time.Second, "SENTINEL", "MASTER", name)); err != nil {
			logger.Error(err, "get info failed")
			return ErrFailoverRetry
		} else {
			var addr string
			if ip, port := masterInfo["ip"], masterInfo["port"]; ip != "" && port != "" {
				addr = net.JoinHostPort(ip, port)
			}
			if addr != "" {
				if _, ok := serveAddresses[addr]; !ok {
					logger.Info("failover success", "master", addr)
					return nil
				}
			}
		}
		time.Sleep(time.Second * 5)
	}
	return ErrFailoverRetry
}

// Shutdown 在退出时做 failover
//
// NOTE: 在4.0, 5.0中，在更新密码时，会重启实例。但是由于密码在重启前已经热更新，导致其他脚本无法连接到实例，包括shutdown脚本
// 为了解决这个问题，针对4,5 版本，会在重启前，先做failover，将master failover 到-0 节点。
// 由于重启是逆序的，最后一个pod启动成功之后，会使用新密码连接到 master，从而确保服务一直可用,切数据不会丢失
func Shutdown(ctx context.Context, c *cli.Context, client *kubernetes.Clientset, logger logr.Logger) error {
	var (
		podIPs  = c.String("pod-ips")
		name    = c.String("name")
		monitor = strings.ToLower(c.String("monitor-policy"))
		timeout = time.Duration(c.Int("timeout")) * time.Second
	)
	if timeout == 0 {
		timeout = time.Second * 300
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	authInfo, err := commands.LoadAuthInfo(c, ctx)
	if err != nil {
		logger.Error(err, "load valkey operator user info failed")
		return err
	}
	senAuthInfo, err := commands.LoadMonitorAuthInfo(c, ctx, client)
	if err != nil {
		logger.Error(err, "load valkey operator user info failed")
		return err
	}

	addr := net.JoinHostPort("local.inject", "6379")
	valkeyClient := valkey.NewValkeyClient(addr, *authInfo)
	defer valkeyClient.Close()

	info, err := getValkeyInfo(ctx, valkeyClient, logger)
	if err != nil {
		logger.Error(err, "check node role failed, abort auto failover")
		return err
	}

	if info.Role == "master" && monitor == "sentinel" {
		err = func() error {
			serveAddresses := map[string]struct{}{}
			if podIPs != "" {
				for _, ip := range strings.Split(podIPs, ",") {
					serveAddresses[net.JoinHostPort(ip, "6379")] = struct{}{}
				}
			}
			if config, err := getValkeyConfig(ctx, valkeyClient, logger); err != nil {
				logger.Error(err, "get config failed ")
			} else {
				ip, port := config["replica-announce-ip"], config["replica-announce-port"]
				if ip != "" && port != "" {
					serveAddresses[net.JoinHostPort(ip, port)] = struct{}{}
				}
			}

			var sentinelNodes []string
			if val := c.String("monitor-uri"); val == "" {
				logger.Error(err, "require monitor uri")
				return errors.New("require monitor uri")
			} else if u, err := url.Parse(val); err != nil {
				logger.Error(err, "parse monitor uri failed")
				return err
			} else {
				sentinelNodes = strings.Split(u.Host, ",")
			}

			for {
				if err := doFailover(ctx, sentinelNodes, name, senAuthInfo, serveAddresses, logger); err != nil {
					if err == ErrFailoverRetry {
						time.Sleep(time.Second * 10)
						continue
					}
					logger.Error(err, "failover aborted")
					return err
				}
				// wait 30 for data sync
				time.Sleep(time.Second * 30)
				return nil
			}
		}()
	}

	logger.Info("do shutdown node")
	// NOTE: here set timeout to 300s, which will try best to do a shutdown snapshot
	// if the data is too large, this snapshot may not be completed
	if _, err := valkeyClient.DoWithTimeout(ctx, time.Second*300, "SHUTDOWN"); err != nil && !errors.Is(err, io.EOF) {
		logger.Error(err, "graceful shutdown failed")
	}
	return err
}
