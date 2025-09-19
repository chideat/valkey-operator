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
	"os"
	"strings"
	"time"

	"github.com/chideat/valkey-operator/cmd/helper/commands"
	"github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/go-logr/logr"
	"github.com/urfave/cli/v2"
	"k8s.io/client-go/kubernetes"
)

func loadAnnounceAddress(filepath string, logger logr.Logger) string {
	if filepath == "" {
		filepath = "/data/announce.conf"
	}
	data, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		logger.Error(err, "read announce file failed", "path", filepath)
		return ""
	}
	lines := strings.Split(string(data), "\n")

	var (
		ip   string
		port string
	)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if fields[0] == "replica-announce-ip" {
			ip = fields[1]
		}
		if fields[0] == "replica-announce-port" {
			port = fields[1]
		}
	}
	if ip != "" && port != "" {
		return net.JoinHostPort(ip, port)
	}
	return ""
}

func getValkeyInfo(ctx context.Context, valkeyClient valkey.ValkeyClient, logger logr.Logger) (*valkey.NodeInfo, error) {
	var (
		err  error
		info *valkey.NodeInfo
	)
	for range 5 {
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
	for range 5 {
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
	ErrFailoverRetry    = errors.New("failover retry")
	ErrFailoverAbort    = errors.New("failover abort")
	ErrNoSentinelUsable = errors.New("no sentinel node usable")
)

func doFailover(ctx context.Context, nodes []string, name string, senAuthInfo *valkey.AuthInfo,
	serveAddresses map[string]struct{}, logger logr.Logger) error {

	batchDo := func(ctx context.Context, minSucceed int, callback func(ctx context.Context, client valkey.ValkeyClient) (any, error)) (rets []any, err error) {
		logger.Info("check sentinel status")
		healthySentinelNodeAddrs, err := checkReadyToFailover(ctx, nodes, name, senAuthInfo, logger)
		if err != nil {
			logger.Error(err, "cluster not ready to do failover, retry in 10s")
			time.Sleep(time.Second * 10)
			return nil, err
		}
		if len(healthySentinelNodeAddrs) == 0 {
			return nil, ErrNoSentinelUsable
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

	getCurrentMaster := func(ctx context.Context, nodes []string) (string, error) {
		quorum := len(nodes)/2 + 1
		masterInfos, err := batchDo(ctx, len(nodes), func(ctx context.Context, client valkey.ValkeyClient) (any, error) {
			if info, err := valkey.StringMap(client.DoWithTimeout(ctx, time.Second*3, "SENTINEL", "MASTER", name)); err != nil {
				return nil, err
			} else if info["flags"] == "master" && info["ip"] != "" && info["port"] != "" {
				return net.JoinHostPort(info["ip"], info["port"]), nil
			}
			return nil, errors.New("master not found")
		})
		if err != nil {
			logger.Error(err, "get master from sentinel failed")
			return "", err
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
			return "", ErrFailoverRetry
		}
		return masterAddr, nil
	}

	if masterAddr, err := getCurrentMaster(ctx, nodes); err != nil {
		logger.Error(err, "get current master failed")
		return err
	} else {
		fields := strings.Split(masterAddr, ":")
		ok2 := false
		if port := fields[len(fields)-1]; port != "6379" {
			_, ok2 = serveAddresses[port]
		}
		if _, ok := serveAddresses[masterAddr]; !ok && !ok2 {
			logger.Info("master is not in serve addresses, skip", "master", masterAddr, "serveAddresses", serveAddresses)
			return nil
		} else {
			logger.Info("current node is master, try failover", "master", masterAddr)
		}
	}

	if _, err := batchDo(ctx, 1, func(ctx context.Context, senClient valkey.ValkeyClient) (any, error) {
		return senClient.DoWithTimeout(ctx, time.Second, "SENTINEL", "FAILOVER", name)
	}); err != nil {
		logger.Error(err, "do failover failed")
		return err
	}

	for range 6 {
		if masterAddr, err := getCurrentMaster(ctx, nodes); err != nil {
			logger.Error(err, "get current master failed")
			return err
		} else {
			fields := strings.Split(masterAddr, ":")
			ok2 := false
			if port := fields[len(fields)-1]; port != "6379" {
				_, ok2 = serveAddresses[port]
			}
			if _, ok := serveAddresses[masterAddr]; !ok && !ok2 {
				logger.Info("failover success", "master", masterAddr)
				return nil
			}
		}
		time.Sleep(time.Second * 5)
	}
	return ErrFailoverRetry
}

// Shutdown shutdown the node and do failover before complete shutdown.
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
		// set node priority to 0 to prevent it from being elected as master again.
		if _, err := valkeyClient.DoWithTimeout(ctx, time.Second*3, "CONFIG", "SET", "replica-priority", "0"); err != nil {
			logger.Error(err, "set replica priority failed, ignored and continue")
		} else {
			time.Sleep(time.Second * 5)
		}

		err = func() error {
			serveAddresses := map[string]struct{}{}
			if podIPs != "" {
				for _, ip := range strings.Split(podIPs, ",") {
					serveAddresses[net.JoinHostPort(ip, "6379")] = struct{}{}
				}
			}
			if config, err := getValkeyConfig(ctx, valkeyClient, logger); err != nil {
				logger.Error(err, "get config failed")
			} else {
				ip, port := config["replica-announce-ip"], config["replica-announce-port"]
				if ip != "" && port != "" {
					serveAddresses[net.JoinHostPort(ip, port)] = struct{}{}
				}
				if port != "6379" {
					serveAddresses[port] = struct{}{}
				}
			}
			if addr := loadAnnounceAddress("", logger); addr != "" {
				fields := strings.Split(addr, ":")
				if fields[len(fields)-1] != "6379" {
					serveAddresses[fields[len(fields)-1]] = struct{}{}
				}
				serveAddresses[addr] = struct{}{}
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

			logger.Info("do failover", "sentinelNodes", sentinelNodes, "serveAddresses", serveAddresses)
			noSentinelRetry := 0
			for {
				if err := doFailover(ctx, sentinelNodes, name, senAuthInfo, serveAddresses, logger); err != nil {
					if err == ErrFailoverAbort {
						logger.Error(err, "failover aborted")
						return err
					} else if err == ErrNoSentinelUsable {
						noSentinelRetry += 1
						if noSentinelRetry >= 6 {
							return err
						}
						logger.Error(err, "no sentinel node usable, retry in 10s")
					} else {
						noSentinelRetry = 0
					}
					time.Sleep(time.Second * 10)
					continue
				}
				// wait 30 for data sync
				time.Sleep(time.Second * 30)
				return nil
			}
		}()
	}

	// sleep some time for other new start pod sync info
	time.Sleep(time.Second * 30)

	logger.Info("do shutdown node")
	// NOTE: here set timeout to 300s, which will try best to do a shutdown snapshot
	// if the data is too large, this snapshot may not be completed
	if _, err := valkeyClient.DoWithTimeout(ctx, time.Second*300, "SHUTDOWN"); err != nil && !errors.Is(err, io.EOF) {
		logger.Error(err, "graceful shutdown failed")
	}
	return err
}
