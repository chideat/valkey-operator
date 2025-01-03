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
	"errors"
	"fmt"
	"strings"
	"time"

	vkcli "github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/chideat/valkey-operator/pkg/version"
)

func NewSentinelNode(ctx context.Context, addr, username, password string, tlsConf *tls.Config) (*SentinelNode, error) {
	client := vkcli.NewValkeyClient(addr, vkcli.AuthConfig{
		Username:  username,
		Password:  password,
		TLSConfig: tlsConf,
	})
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		return nil, err
	}
	info, err := client.Info(ctx, "server")
	if err != nil {
		return nil, err
	}
	version, _ := version.ParseValkeyVersion(info.Version)

	return &SentinelNode{addr: addr, client: client, version: version}, nil
}

type SentinelNode struct {
	addr    string
	client  vkcli.ValkeyClient
	version version.ValkeyVersion
}

func (sn *SentinelNode) Version() version.ValkeyVersion {
	return sn.version
}

func (sn *SentinelNode) MonitoringMaster(ctx context.Context, name string) (*vkcli.SentinelMonitorNode, error) {
	if sn == nil || sn.client == nil {
		return nil, fmt.Errorf("no client")
	}

	val, err := sn.client.Do(ctx, "SENTINEL", "MASTER", name)
	if err != nil {
		if strings.Contains(err.Error(), "No such master with that name") {
			return nil, ErrNoMaster
		}
		return nil, err
	}
	return vkcli.ParseSentinelMonitorNode(val), nil
}

func (sn *SentinelNode) MonitoringReplicas(ctx context.Context, name string) ([]*vkcli.SentinelMonitorNode, error) {
	if sn == nil || sn.client == nil {
		return nil, fmt.Errorf("no client")
	}

	var (
		replicas []*vkcli.SentinelMonitorNode
	)

	if vals, err := vkcli.Values(sn.client.Do(ctx, "SENTINEL", "REPLICAS", name)); err != nil {
		if strings.Contains(err.Error(), "No such master with that name") {
			return nil, ErrNoMaster
		}
		return nil, err
	} else {
		for _, val := range vals {
			replicas = append(replicas, vkcli.ParseSentinelMonitorNode(val))
		}
	}
	return replicas, nil
}

func (sn *SentinelNode) UpdateConfig(ctx context.Context, name string, params map[string]string) error {
	if sn == nil || sn.client == nil {
		return fmt.Errorf("no client")
	}

	cmds := [][]any{}
	for k, v := range params {
		cmds = append(cmds, []any{"SENTINEL", "SET", name, k, v})
	}
	cmds = append(cmds, []any{"SENTINEL", "RESET", name})
	rets, err := sn.client.Pipeline(ctx, cmds)
	if err != nil {
		return err
	}
	for _, ret := range rets {
		if ret.Error != nil {
			if strings.Contains(err.Error(), "No such master with that name") {
				err = errors.Join(err, ErrNoMaster)
			} else {
				err = errors.Join(err, ret.Error)
			}
		}
	}
	return err
}

func (sn *SentinelNode) Monitor(ctx context.Context, name, ip, port string, quorum int, params map[string]string) error {
	if sn == nil || sn.client == nil {
		return fmt.Errorf("no client")
	}

	if _, err := sn.client.Do(ctx, "SENTINEL", "REMOVE", name); err != nil {
		// ignore error if no such master
		if !strings.Contains(err.Error(), "No such master with that name") {
			return err
		}
	}

	cmds := [][]any{{"SENTINEL", "MONITOR", name, ip, port, quorum}}
	for k, v := range params {
		cmds = append(cmds, []any{"SENTINEL", "SET", name, k, v})
	}
	cmds = append(cmds, []any{"SENTINEL", "RESET", name})

	rets, err := sn.client.Pipeline(ctx, cmds)
	if err != nil {
		return err
	}
	for _, ret := range rets {
		if ret.Error != nil {
			err = errors.Join(err, ret.Error)
		}
	}
	return err
}

func (sn *SentinelNode) Failover(ctx context.Context, name string) error {
	if sn == nil || sn.client == nil {
		return fmt.Errorf("no client")
	}

	masterNode, err := sn.MonitoringMaster(ctx, name)
	if err != nil {
		return err
	}
	var (
		masterAddr        = masterNode.Address()
		currentMasterAddr string
	)
	if _, err := sn.client.Do(ctx, "SENTINEL", "FAILOVER", name); err != nil {
		return err
	}
	for j := 0; j < 10; j++ {
		time.Sleep(3 * time.Second)
		if currentNode, err := sn.MonitoringMaster(ctx, name); err != nil {
			return err
		} else {
			currentMasterAddr = currentNode.Address()
			if currentMasterAddr != masterAddr {
				return nil
			}
		}
	}
	return fmt.Errorf("failover timeout, old master addr %s, current master addr %s", masterAddr, currentMasterAddr)
}

func (sn *SentinelNode) IsReady() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := sn.client.Ping(ctx); err != nil {
		return false
	}
	return true
}
