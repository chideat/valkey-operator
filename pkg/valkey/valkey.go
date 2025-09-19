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

package valkey

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
)

type SentinelMaster struct {
	Name            string                `json:"name"`
	Status          string                `json:"status"`
	Address         Address               `json:"address"`
	Replicas        int                   `json:"slaves"`
	Sentinels       int                   `json:"sentinels"`
	MonitorReplicas []SentinelMonitorNode `json:"monitor_replicas"`
}

// NodeInfo
type NodeInfo struct {
	Version               string         `json:"valkey_version"`
	ServerMode            string         `json:"server_mode"`
	RunId                 string         `json:"run_id"`
	UptimeInSeconds       int64          `json:"uptime_in_seconds"`
	AOFEnabled            string         `json:"aof_enabled"`
	Role                  string         `json:"role"`
	ConnectedReplicas     int64          `json:"connected_slaves"`
	MasterHost            string         `json:"master_host"`
	MasterPort            string         `json:"master_port"`
	ClusterEnabled        string         `json:"cluster_enabled"`
	MasterLinkStatus      string         `json:"master_link_status"`
	MasterReplId          string         `json:"master_replid"`
	MasterReplOffset      int64          `json:"master_repl_offset"`
	MasterReplId2         string         `json:"master_replid2"`
	SecondReplOffset      int64          `json:"second_repl_offset"`
	UsedMemory            int64          `json:"used_memory"`
	UsedMemoryDataset     int64          `json:"used_memory_dataset"`
	SentinelMasters       int64          `json:"sentinel_masters"`
	SentinelTiLt          int64          `json:"sentinel_tilt"`
	SentinelRunningScript int64          `json:"sentinel_running_scripts"`
	Dbsize                int64          `json:"dbsize"`
	SentinelMaster0       SentinelMaster `json:"master0"`
}

type SentinelMonitorNode struct {
	Name                  string `json:"name"`
	IP                    string `json:"ip"`
	Port                  string `json:"port"`
	RunId                 string `json:"run_id"`
	Flags                 string `json:"flags"`
	LinkPendingCommands   int32  `json:"link_pending_commands"`
	LinkRefcount          int32  `json:"link_refcount"`
	FailoverState         string `json:"failover_state"`
	LastPingSent          int64  `json:"last_ping_sent"`
	LastOkPingReply       int64  `json:"last_ok_ping_reply"`
	LastPingReply         int64  `json:"last_ping_reply"`
	SDownTime             int64  `json:"s_down_time"`
	ODownTime             int64  `json:"o_down_time"`
	DownAfterMilliseconds int64  `json:"down_after_milliseconds"`
	InfoRefresh           int64  `json:"info_refresh"`
	RoleReported          string `json:"role_reported"`
	RoleReportedTime      int64  `json:"role_reported_time"`
	ConfigEpoch           int64  `json:"config_epoch"`
	NumSlaves             int32  `json:"num_slaves"`
	NumOtherSentinels     int32  `json:"num_other_sentinels"`
	Quorum                int32  `json:"quorum"`
	FailoverTimeout       int64  `json:"failover_timeout"`
	ParallelSyncs         int32  `json:"parallel_syncs"`

	// replica fields
	MasterLinkDownTime int64  `json:"master_link_down_time"`
	MasterLinkStatus   string `json:"master_link_status"`
	MasterHost         string `json:"master_host"`
	MasterPort         string `json:"master_port"`
	SlavePriority      int32  `json:"slave_priority"`
	SlaveReplOffset    int64  `json:"slave_repl_offset"`

	// sentinel node specific fields
	LastHelloMessage string `json:"last_hello_message"`
	VotedLeader      string `json:"voted_leader"`
	VotedLeaderEpoch int64  `json:"voted_leader_epoch"`
}

func (s *SentinelMonitorNode) Address() string {
	if s == nil || s.IP == "" || s.Port == "" {
		return ""
	}
	return net.JoinHostPort(s.IP, s.Port)
}

func (s *SentinelMonitorNode) IsMaster() bool {
	if s == nil {
		return false
	}
	return strings.Contains(s.Flags, "master") && !strings.Contains(s.Flags, "down")
}

func (s *SentinelMonitorNode) IsFailovering() bool {
	if s == nil {
		return false
	}
	return slices.Contains(strings.Split(s.Flags, ","), "failover_in_progress")
}

const (
	ClusterStateOk   string = "ok"
	ClusterStateFail string = "fail"
)

type ClusterNodeInfo struct {
	ClusterState         string `json:"cluster_state"`
	ClusterSlotsAssigned int    `json:"cluster_slots_assigned"`
	ClusterSlotsOk       int    `json:"cluster_slots_ok"`
	ClusterSlotsPfail    int    `json:"cluster_slots_pfail"`
	ClusterSlotsFail     int    `json:"cluster_slots_fail"`
	ClusterKnownNodes    int    `json:"cluster_known_nodes"`
	ClusterSize          int    `json:"cluster_size"`
	ClusterCurrentEpoch  int    `json:"cluster_current_epoch"`
	ClusterMyEpoch       int    `json:"cluster_my_epoch"`
}

// ValkeyClient
type ValkeyClient interface {
	Do(ctx context.Context, cmd string, args ...any) (any, error)
	DoWithTimeout(ctx context.Context, timeout time.Duration, cmd string, args ...any) (any, error)
	Tx(ctx context.Context, cmds []string, args [][]any) (any, error)
	Pipeline(ctx context.Context, args [][]any) ([]PipelineResult, error)
	Close() error
	Clone(ctx context.Context, addr string) ValkeyClient

	Ping(ctx context.Context) error
	Info(ctx context.Context, sections ...any) (*NodeInfo, error)
	ClusterInfo(ctx context.Context) (*ClusterNodeInfo, error)
	ConfigGet(ctx context.Context, cate string) (map[string]string, error)
	ConfigSet(ctx context.Context, params map[string]string) error
	Nodes(ctx context.Context) (ClusterNodes, error)
}

type AuthConfig struct {
	Username  string
	Password  string
	TLSConfig *tls.Config
}

type AuthInfo = AuthConfig

type valkeyClient struct {
	authInfo *AuthConfig
	addr     string

	pool *redis.Pool
}

// NewValkeyClient
func NewValkeyClient(addr string, authInfo AuthConfig) ValkeyClient {
	client := valkeyClient{authInfo: &authInfo, addr: addr}
	client.pool = &redis.Pool{
		DialContext: func(ctx context.Context) (redis.Conn, error) {
			var opts []redis.DialOption
			if authInfo.Password != "" {
				if authInfo.Username != "" && authInfo.Username != "default" {
					opts = append(opts, redis.DialUsername(authInfo.Username))
				}
				opts = append(opts, redis.DialPassword(authInfo.Password))
			}
			if authInfo.TLSConfig != nil {
				opts = append(opts,
					redis.DialUseTLS(true),
					redis.DialTLSConfig(authInfo.TLSConfig),
					redis.DialTLSSkipVerify(true),
				)
			}
			opts = append(opts, redis.DialClientName("valkey-operator"))

			ctx, cancel := context.WithTimeout(ctx, time.Second*5)
			defer cancel()

			conn, err := redis.DialContext(ctx, "tcp", addr, opts...)
			if err != nil {
				return nil, err
			}
			return conn, nil
		},
		MaxIdle:   1,
		MaxActive: 1,
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()

			if _, err := redis.DoContext(c, ctx, "PING"); err != nil {
				return err
			}
			return nil
		},
	}
	return &client
}

// Do
func (c *valkeyClient) Do(ctx context.Context, cmd string, args ...any) (any, error) {
	if c == nil || c.pool == nil {
		return nil, nil
	}

	conn := c.pool.Get()
	defer conn.Close()

	return redis.DoContext(conn, ctx, cmd, args...)
}

func (c *valkeyClient) DoWithTimeout(ctx context.Context, timeout time.Duration, cmd string, args ...any) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return c.Do(ctx, cmd, args...)
}

// Tx
func (c *valkeyClient) Tx(ctx context.Context, cmds []string, args [][]any) (any, error) {
	if c == nil || c.pool == nil {
		return nil, nil
	}

	conn := c.pool.Get()
	defer conn.Close()

	if err := conn.Send("MULTI"); err != nil {
		return nil, err
	}
	for i, cmd := range cmds {
		if err := conn.Send(cmd, args[i]...); err != nil {
			return nil, err
		}
	}

	result, err := redis.Values(conn.Do("EXEC"))
	if err != nil {
		return result, err
	}
	for _, v := range result {
		switch rv := v.(type) {
		case redis.Error:
			return result, fmt.Errorf("error: %s", rv.Error())
		}
	}
	return result, nil
}

type PipelineResult struct {
	Value any
	Error error
}

// Pipeline
func (c *valkeyClient) Pipeline(ctx context.Context, args [][]any) ([]PipelineResult, error) {
	if c == nil || c.pool == nil {
		return nil, nil
	}

	conn := c.pool.Get()
	defer conn.Close()

	for _, arg := range args {
		cmd, ok := arg[0].(string)
		if !ok {
			return nil, fmt.Errorf("invalid command")
		}
		if err := conn.Send(cmd, arg[1:]...); err != nil {
			return nil, err
		}
	}
	if err := conn.Flush(); err != nil {
		return nil, err
	}

	var rets []PipelineResult
	for range len(args) {
		ret := PipelineResult{}
		ret.Value, ret.Error = conn.Receive()
		rets = append(rets, ret)
	}
	return rets, nil
}

// Close
func (c *valkeyClient) Close() error {
	if c == nil || c.pool == nil {
		return nil
	}
	return c.pool.Close()
}

func (c *valkeyClient) Clone(ctx context.Context, addr string) ValkeyClient {
	if c == nil || c.authInfo == nil {
		return nil
	}

	return NewValkeyClient(addr, *c.authInfo)
}

// Ping
func (c *valkeyClient) Ping(ctx context.Context) error {
	if c == nil || c.pool == nil {
		return nil
	}

	conn := c.pool.Get()
	defer conn.Close()

	if _, err := redis.DoContext(conn, ctx, "PING"); err != nil {
		return err
	}
	return nil
}

var (
	ErrNil     = redis.ErrNil
	Int        = redis.Int
	Ints       = redis.Ints
	IntMap     = redis.IntMap
	Int64      = redis.Int64
	Int64s     = redis.Int64s
	Int64Map   = redis.Int64Map
	Uint64     = redis.Uint64
	Uint64s    = redis.Uint64s
	Uint64Map  = redis.Uint64Map
	Float64    = redis.Float64
	Float64s   = redis.Float64s
	Float64Map = redis.Float64Map
	String     = redis.String
	Strings    = redis.Strings
	StringMap  = redis.StringMap
	Bytes      = redis.Bytes
	ByteSlices = redis.ByteSlices
	Bool       = redis.Bool
	Values     = redis.Values
	Positions  = redis.Positions
)

// Config
func (c *valkeyClient) ConfigGet(ctx context.Context, cate string) (map[string]string, error) {
	if c == nil || c.pool == nil {
		return nil, nil
	}

	if cate == "" {
		cate = "*"
	}

	conn := c.pool.Get()
	defer conn.Close()

	return redis.StringMap(redis.DoContext(conn, ctx, "CONFIG", "GET", cate))
}

// ConfigSet
func (c *valkeyClient) ConfigSet(ctx context.Context, params map[string]string) error {
	if c == nil || c.pool == nil {
		return nil
	}

	conn := c.pool.Get()
	defer conn.Close()

	for key, val := range params {
		if val == `""` {
			val = ""
		}
		if _, err := redis.DoContext(conn, ctx, "CONFIG", "SET", key, val); err != nil {
			return fmt.Errorf("update config %s failed, error=%s", key, err)
		}
	}
	return nil
}

// Info
func (c *valkeyClient) Info(ctx context.Context, sections ...any) (*NodeInfo, error) {
	if c == nil || c.pool == nil {
		return nil, nil
	}

	conn := c.pool.Get()
	defer conn.Close()

	data, err := redis.String(redis.DoContext(conn, ctx, "INFO", sections...))
	if err != nil {
		return nil, err
	}

	parseInfo := func(data string) *NodeInfo {
		info := NodeInfo{}
		lines := strings.Split(data, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fields := strings.SplitN(line, ":", 2)
			if len(fields) != 2 {
				continue
			}
			switch fields[0] {
			case "valkey_version", "redis_version":
				info.Version = fields[1]
			case "server_mode", "redis_mode":
				info.ServerMode = fields[1]
			case "run_id":
				info.RunId = fields[1]
			case "uptime_in_seconds":
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				info.UptimeInSeconds = val
			case "aof_enabled":
				info.AOFEnabled = fields[1]
			case "role":
				info.Role = fields[1]
			case "master_host":
				info.MasterHost = fields[1]
			case "master_port":
				info.MasterPort = fields[1]
			case "cluster_enabled":
				info.ClusterEnabled = fields[1]
			case "master_link_status":
				info.MasterLinkStatus = fields[1]
			case "master_replid":
				info.MasterReplId = fields[1]
			case "master_repl_offset":
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				info.MasterReplOffset = val
			case "master_replid2":
				info.MasterReplId2 = fields[1]
			case "second_repl_offset":
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				info.SecondReplOffset = val
			case "used_memory":
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				info.UsedMemory = val
			case "used_memory_dataset":
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				info.UsedMemoryDataset = val
			case "sentinel_masters":
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				info.SentinelMasters = val
			case "sentinel_tilt":
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				info.SentinelTiLt = val
			case "connected_slaves":
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				info.ConnectedReplicas = val
			case "sentinel_running_scripts":
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				info.SentinelRunningScript = val
			case "master0":
				info.Role = "sentinel"
				fields := strings.Split(fields[1], ",")
				if len(fields) != 5 {
					continue
				}
				for _, kv := range fields {
					kv := strings.Split(kv, "=")
					if len(kv) != 2 {
						continue
					}
					key, value := kv[0], kv[1]
					switch key {
					case "name":
						info.SentinelMaster0.Name = value
					case "status":
						info.SentinelMaster0.Status = value
					case "address":
						info.SentinelMaster0.Address = Address(value)
					case "slaves":
						info.SentinelMaster0.Replicas, _ = strconv.Atoi(value)
					case "sentinels":
						info.SentinelMaster0.Sentinels, _ = strconv.Atoi(value)
					}
				}
			}
			if strings.HasPrefix(fields[0], "db") {
				vals := strings.SplitN(fields[1], ",", 2)
				if strings.HasPrefix(vals[0], "keys=") {
					val, _ := strconv.ParseInt(vals[0][5:], 10, 64)
					info.Dbsize += val
				}
			}
		}
		return &info
	}
	return parseInfo(data), nil
}

func ParseSentinelMonitorNode(val any) *SentinelMonitorNode {
	kvs, _ := redis.StringMap(val, nil)
	node := SentinelMonitorNode{}
	for k, v := range kvs {
		switch k {
		case "name":
			node.Name = v
		case "ip":
			node.IP = v
		case "port":
			node.Port = v
		case "runid":
			node.RunId = v
		case "flags":
			node.Flags = v
		case "link-pending-commands":
			iv, _ := strconv.ParseInt(v, 10, 32)
			node.LinkPendingCommands = int32(iv)
		case "link-refcount":
			iv, _ := strconv.ParseInt(v, 10, 32)
			node.LinkRefcount = int32(iv)
		case "failover_state":
			node.FailoverState = v
		case "last-ping-sent":
			iv, _ := strconv.ParseInt(v, 10, 64)
			node.LastPingSent = iv
		case "last-ok-ping-reply":
			iv, _ := strconv.ParseInt(v, 10, 64)
			node.LastOkPingReply = iv
		case "last-ping-reply":
			iv, _ := strconv.ParseInt(v, 10, 64)
			node.LastPingReply = iv
		case "s-down-time":
			iv, _ := strconv.ParseInt(v, 10, 64)
			node.SDownTime = iv
		case "o-down-time":
			iv, _ := strconv.ParseInt(v, 10, 64)
			node.ODownTime = iv
		case "down-after-milliseconds":
			iv, _ := strconv.ParseInt(v, 10, 64)
			node.DownAfterMilliseconds = iv
		case "info-refresh":
			iv, _ := strconv.ParseInt(v, 10, 64)
			node.InfoRefresh = iv
		case "role-reported":
			node.RoleReported = v
		case "role-reported-time":
			iv, _ := strconv.ParseInt(v, 10, 64)
			node.RoleReportedTime = iv
		case "config-epoch":
			iv, _ := strconv.ParseInt(v, 10, 64)
			node.ConfigEpoch = iv
		case "num-slaves":
			iv, _ := strconv.ParseInt(v, 10, 32)
			node.NumSlaves = int32(iv)
		case "num-other-sentinels":
			iv, _ := strconv.ParseInt(v, 10, 32)
			node.NumOtherSentinels = int32(iv)
		case "quorum":
			iv, _ := strconv.ParseInt(v, 10, 32)
			node.Quorum = int32(iv)
		case "failover-timeout":
			iv, _ := strconv.ParseInt(v, 10, 64)
			node.FailoverTimeout = iv
		case "parallel-syncs":
			iv, _ := strconv.ParseInt(v, 10, 32)
			node.ParallelSyncs = int32(iv)
		case "master-link-down-time":
			iv, _ := strconv.ParseInt(v, 10, 64)
			node.MasterLinkDownTime = iv
		case "master-link-status":
			node.MasterLinkStatus = v
		case "master-host":
			node.MasterHost = v
		case "master-port":
			node.MasterPort = v
		case "slave-priority":
			iv, _ := strconv.ParseInt(v, 10, 32)
			node.SlavePriority = int32(iv)
		case "slave-repl-offset":
			iv, _ := strconv.ParseInt(v, 10, 64)
			node.SlaveReplOffset = iv
		case "last-hello-message":
			node.LastHelloMessage = v
		case "voted-leader":
			node.VotedLeader = v
		case "voted-leader-epoch":
			iv, _ := strconv.ParseInt(v, 10, 64)
			node.VotedLeaderEpoch = iv
		}
	}
	return &node
}

func (c *valkeyClient) ClusterInfo(ctx context.Context) (*ClusterNodeInfo, error) {
	if c == nil || c.pool == nil {
		return nil, nil
	}

	conn := c.pool.Get()
	defer conn.Close()

	data, err := redis.String(redis.DoContext(conn, ctx, "CLUSTER", "INFO"))
	if err != nil {
		return nil, err
	}

	parseInfo := func(data string) *ClusterNodeInfo {
		info := ClusterNodeInfo{}
		for line := range strings.SplitSeq(data, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fields := strings.SplitN(line, ":", 2)
			if len(fields) != 2 {
				continue
			}
			switch fields[0] {
			case "cluster_state":
				info.ClusterState = fields[1]
			case "cluster_slots_assigned":
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				info.ClusterSlotsAssigned = int(val)
			case "cluster_slots_ok":
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				info.ClusterSlotsOk = int(val)
			case "cluster_slots_pfail":
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				info.ClusterSlotsPfail = int(val)
			case "cluster_slots_fail":
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				info.ClusterSlotsFail = int(val)
			case "cluster_known_nodes":
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				info.ClusterKnownNodes = int(val)
			case "cluster_size":
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				info.ClusterSize = int(val)
			case "cluster_current_epoch":
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				info.ClusterCurrentEpoch = int(val)
			case "cluster_my_epoch":
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				info.ClusterMyEpoch = int(val)
			}
		}
		return &info
	}
	return parseInfo(data), nil
}

// Nodes
func (c *valkeyClient) Nodes(ctx context.Context) (ClusterNodes, error) {
	if c == nil || c.pool == nil {
		return nil, nil
	}
	conn := c.pool.Get()
	defer conn.Close()

	data, err := redis.String(redis.DoContext(conn, ctx, "CLUSTER", "NODES"))
	if err != nil {
		return nil, err
	}

	var nodes ClusterNodes
	for line := range strings.SplitSeq(strings.TrimSpace(data), "\n") {
		line = strings.TrimSpace(line)
		if node, err := ParseNodeFromClusterNode(line); err != nil {
			return nil, err
		} else {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}
