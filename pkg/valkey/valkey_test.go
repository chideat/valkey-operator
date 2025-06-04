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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gomodule/redigo/redis"
	"github.com/rafaeljusto/redigomock/v3"
)

func newValkeyMockClient(conn *redigomock.Conn) ValkeyClient {
	client := valkeyClient{}
	client.pool = &redis.Pool{
		DialContext: func(ctx context.Context) (redis.Conn, error) {
			return conn, nil
		},
	}
	return &client
}

func TestParseAddress(t *testing.T) {
	testCases := []struct {
		input          string
		expectedIP     string
		expectedPort   int
		exportedString string
		expectError    bool
	}{
		{"192.168.1.1:8080", "192.168.1.1", 8080, "192.168.1.1:8080", false},
		{"1335::172:168:200:5d1:32428", "1335::172:168:200:5d1", 32428, "[1335::172:168:200:5d1]:32428", false},
		{"::1:6379", "::1", 6379, "[::1]:6379", false},
		{":6379", "", 6379, ":6379", false},
		{"localhost:6379", "localhost", 6379, "localhost:6379", false},
		{"invalid-ip:port", "", 0, "", true},
		{"invalid-ip", "", 0, "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			addr := Address(tc.input)
			if _, _, err := addr.parse(); err != nil {
				if !tc.expectError {
					t.Errorf("Expected an error, but got nil")
				}
			} else {
				if addr.Host() != tc.expectedIP {
					t.Errorf("Expected IP: %s, got: %s", tc.expectedIP, addr.Host())
				}
				if addr.Port() != tc.expectedPort {
					t.Errorf("Expected port: %d, got: %d", tc.expectedPort, addr.Port())
				}
				if addr.String() != tc.exportedString {
					t.Errorf("Expected string: %s, got: %s", tc.exportedString, addr.String())
				}
			}
		})
	}
}

func TestSentinelMonitorNode_Address(t *testing.T) {
	type args struct {
		Node *SentinelMonitorNode
	}

	testCases := []struct {
		name      string
		args      args
		wantError bool
		want      string
	}{
		{
			name:      "empty",
			args:      args{},
			wantError: false,
			want:      "",
		},
		{
			name: "ipv4",
			args: args{
				Node: &SentinelMonitorNode{
					IP:   "192.168.1.1",
					Port: "6379",
				},
			},
			wantError: false,
			want:      "192.168.1.1:6379",
		},
		{
			name: "ipv6",
			args: args{
				Node: &SentinelMonitorNode{
					IP:   "::1",
					Port: "6379",
				},
			},
			wantError: false,
			want:      "[::1]:6379",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			addr := tc.args.Node.Address()
			if addr != tc.want {
				t.Errorf("Expected: %s, got: %s", tc.want, addr)
			}
		})
	}
}

func TestSentinelMonitorNode_IsMaster(t *testing.T) {
	type args struct {
		Node *SentinelMonitorNode
	}

	testCases := []struct {
		name      string
		args      args
		wantError bool
		want      bool
	}{
		{
			name:      "empty",
			args:      args{},
			wantError: false,
		},
		{
			name: "is-master",
			args: args{
				Node: &SentinelMonitorNode{
					Flags: "master",
				},
			},
			wantError: false,
			want:      true,
		},
		{
			name: "master down",
			args: args{
				Node: &SentinelMonitorNode{
					Flags: "s_down,master",
				},
			},
			wantError: false,
			want:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.args.Node.IsMaster() != tc.want {
				t.Errorf("Expected: %t, got: %t", tc.want, tc.args.Node.IsMaster())
			}
		})
	}
}

func TestSentinelMonitorNode_IsFailovering(t *testing.T) {
	type args struct {
		Node *SentinelMonitorNode
	}

	testCases := []struct {
		name      string
		args      args
		wantError bool
		want      bool
	}{
		{
			name:      "empty",
			args:      args{},
			wantError: false,
		},
		{
			name: "is-failovering",
			args: args{
				Node: &SentinelMonitorNode{
					Flags: "master,failover_in_progress",
				},
			},
			wantError: false,
			want:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.args.Node.IsFailovering() != tc.want {
				t.Errorf("Expected: %t, got: %t", tc.want, tc.args.Node.IsFailovering())
			}
		})
	}
}

func TestNewValkeyClient(t *testing.T) {
	testCases := []struct {
		name     string
		addr     string
		authInfo AuthConfig
	}{
		{
			name: "No Auth",
			addr: "localhost:6379",
			authInfo: AuthConfig{
				Username: "",
				Password: "",
			},
		},
		{
			name: "With Password",
			addr: "localhost:6379",
			authInfo: AuthConfig{
				Username: "",
				Password: "password",
			},
		},
		{
			name: "With Username and Password",
			addr: "localhost:6379",
			authInfo: AuthConfig{
				Username: "user",
				Password: "password",
			},
		},
		{
			name: "With TLS",
			addr: "localhost:6379",
			authInfo: AuthConfig{
				Username:  "",
				Password:  "",
				TLSConfig: &tls.Config{},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewValkeyClient(tc.addr, tc.authInfo)
			if client == nil {
				t.Fatalf("Expected non-nil client")
			}
			defer client.Close()

			valkeyClient, ok := client.(*valkeyClient)
			if !ok {
				t.Fatalf("Expected *valkeyClient type")
			}

			if valkeyClient.addr != tc.addr {
				t.Errorf("Expected addr: %s, got: %s", tc.addr, valkeyClient.addr)
			}

			if valkeyClient.authInfo.Username != tc.authInfo.Username {
				t.Errorf("Expected Username: %s, got: %s", tc.authInfo.Username, valkeyClient.authInfo.Username)
			}

			if valkeyClient.authInfo.Password != tc.authInfo.Password {
				t.Errorf("Expected Password: %s, got: %s", tc.authInfo.Password, valkeyClient.authInfo.Password)
			}

			if (valkeyClient.authInfo.TLSConfig == nil) != (tc.authInfo.TLSConfig == nil) {
				t.Errorf("Expected TLSConfig: %v, got: %v", tc.authInfo.TLSConfig, valkeyClient.authInfo.TLSConfig)
			}
		})
	}
}

func TestValkeyClient_Do(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("An error occurred while starting miniredis: %v", err)
	}
	defer s.Close()

	client := NewValkeyClient(s.Addr(), AuthConfig{})
	defer client.Close()

	_, err = client.Do(context.Background(), "SET", "key", "value")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	val, err := redis.String(client.Do(context.Background(), "GET", "key"))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if val != "value" {
		t.Errorf("Expected value: %s, got: %s", "value", val)
	}

	var nilClient ValkeyClient = (*valkeyClient)(nil)
	if _, err = nilClient.Do(context.Background(), "SET", "key", "value"); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestValkeyClient_DoWithTimeout(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("An error occurred while starting miniredis: %v", err)
	}
	defer s.Close()

	client := NewValkeyClient(s.Addr(), AuthConfig{})
	defer client.Close()

	_, err = client.DoWithTimeout(context.Background(), time.Second, "SET", "key", "value")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	val, err := redis.String(client.DoWithTimeout(context.Background(), time.Second, "GET", "key"))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if val != "value" {
		t.Errorf("Expected value: %s, got: %s", "value", val)
	}

	var nilClient ValkeyClient = (*valkeyClient)(nil)
	if _, err = nilClient.DoWithTimeout(context.Background(), time.Second, "SET", "key", "value"); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestValkeyClient_Tx(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("An error occurred while starting miniredis: %v", err)
	}
	defer s.Close()

	client := NewValkeyClient(s.Addr(), AuthConfig{})
	defer client.Close()

	if _, err := client.Tx(context.Background(), []string{"SET", "SET", "GET"}, [][]any{
		{"key1", "value1"},
		{"key2", "value2"},
		{"key1"},
	}); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if _, err := client.Tx(context.Background(), []string{"SET", "SET", "GET"}, [][]any{
		{"key1", "value1", "value2"},
		{"key2", "value2"},
		{"key1"},
	}); err.Error() != "ERR syntax error" {
		t.Fatalf("Expected syntax error, got not error")
	}

	var nilClient ValkeyClient = (*valkeyClient)(nil)
	defer nilClient.Close()
	if _, err = nilClient.Tx(context.Background(), []string{}, [][]any{}); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestValkeyClient_Ping(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("An error occurred while starting miniredis: %v", err)
	}
	defer s.Close()
	s.RequireUserAuth("user", "password")

	client := NewValkeyClient(s.Addr(), AuthConfig{
		Username: "user",
		Password: "password",
	})
	defer client.Close()

	err = client.Ping(context.Background())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func generateSelfSignedCert() (certFile, keyFile string, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return "", "", err
	}

	certOut, err := os.CreateTemp("", "cert.pem")
	if err != nil {
		return "", "", err
	}
	defer certOut.Close()

	keyOut, err := os.CreateTemp("", "key.pem")
	if err != nil {
		return "", "", err
	}
	defer keyOut.Close()

	_ = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return "", "", err
	}
	_ = pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	return certOut.Name(), keyOut.Name(), nil
}

func TestValkeyClient_TLS(t *testing.T) {
	certFile, keyFile, err := generateSelfSignedCert()
	if err != nil {
		t.Fatalf("Failed to generate self-signed certificate: %v", err)
	}
	defer os.Remove(certFile)
	defer os.Remove(keyFile)

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("Failed to load X509 key pair: %v", err)
	}
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}

	s, err := miniredis.RunTLS(tlsConfig)
	if err != nil {
		t.Fatalf("An error occurred while starting miniredis: %v", err)
	}
	defer s.Close()

	s.RequireAuth("password")

	client := NewValkeyClient(s.Addr(), AuthConfig{
		Password:  "password",
		TLSConfig: tlsConfig,
	})
	defer client.Close()

	info, err := client.Info(context.Background())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if info == nil {
		t.Fatalf("Expected non-nil Info")
	}
}

func TestValkeyClient_Pipeline(t *testing.T) {
	tests := []struct {
		name    string
		args    [][]any
		want    []PipelineResult
		wantErr bool
	}{
		{
			name: "single command",
			args: [][]any{
				{"SET", "key1", "value1"},
			},
			want: []PipelineResult{
				{Value: "OK", Error: nil},
			},
			wantErr: false,
		},
		{
			name: "multiple commands",
			args: [][]any{
				{"SET", "key1", "value1"},
				{"GET", "key1"},
				{"DEL", "key1"},
			},
			want: []PipelineResult{
				{Value: "OK", Error: nil},
				{Value: "value1", Error: nil},
				{Value: int64(1), Error: nil},
			},
			wantErr: false,
		},
		{
			name: "invalid command",
			args: [][]any{
				{123, "key1", "value1"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := miniredis.Run()
			if err != nil {
				t.Fatalf("Failed to start miniredis: %v", err)
			}
			defer s.Close()

			client := NewValkeyClient(s.Addr(), AuthConfig{})
			defer client.Close()
			got, err := client.Pipeline(context.Background(), tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("Pipeline() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("Pipeline() got %v results, want %v", len(got), len(tt.want))
				}
			}
		})
	}

	var nilClient ValkeyClient = (*valkeyClient)(nil)
	if _, err := nilClient.Pipeline(context.Background(), [][]any{}); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestValkeyClient_Clone(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("An error occurred while starting miniredis: %v", err)
	}
	defer s.Close()
	s.RequireAuth("password")

	s2, err := miniredis.Run()
	if err != nil {
		t.Fatalf("An error occurred while starting miniredis: %v", err)
	}
	defer s2.Close()
	s2.RequireAuth("password")

	client := NewValkeyClient(s.Addr(), AuthConfig{
		Password: "password",
	})
	defer client.Close()

	client2 := client.Clone(context.Background(), s2.Addr())
	defer client2.Close()

	if _, err := client2.Do(context.Background(), "SET", "key", "value"); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var nilClient ValkeyClient = (*valkeyClient)(nil)
	if nilClient.Clone(context.Background(), "") != nil {
		t.Fatalf("Expected nil client")
	}
}

func TestValkeyClient_Nodes(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer s.Close()

	client := NewValkeyClient(s.Addr(), AuthConfig{})
	defer client.Close()

	nodes, err := client.Nodes(context.Background())
	if err != nil {
		t.Errorf("Nodes() error = %v", err)
		return
	}
	t.Logf("Nodes: %v", nodes)

	var nilClient ValkeyClient = (*valkeyClient)(nil)
	if _, err := nilClient.Nodes(context.Background()); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestValkeyClient_Info(t *testing.T) {
	conn := redigomock.NewConn()
	client := newValkeyMockClient(conn)
	defer client.Close()

	type args struct {
		info string
	}

	testCases := []struct {
		name      string
		args      args
		wantError bool
		want      *NodeInfo
	}{
		{
			name: "cluster info",
			args: args{
				info: `# Server
redis_version:7.2.7
redis_git_sha1:00000000
redis_git_dirty:0
redis_build_id:d602a27dd72c7efa
redis_mode:cluster
os:Linux 5.15.0-56-generic x86_64
arch_bits:64
monotonic_clock:POSIX clock_gettime
multiplexing_api:epoll
atomicvar_api:c11-builtin
gcc_version:13.2.1
process_id:52
process_supervised:no
run_id:1f6aa99e4aabffabe5ef77d22d107315956060cf
tcp_port:6379
server_time_usec:1743664740671985
uptime_in_seconds:16381
uptime_in_days:0
hz:10
configured_hz:10
lru_clock:15611492
executable:/data/redis-server
config_file:/tmp/redis.conf
io_threads_active:0
listener0:name=tcp,bind=10.3.0.172,bind=127.0.0.1,port=6379

# Clients
connected_clients:2
cluster_connections:10
maxclients:10000
client_recent_max_input_buffer:20480
client_recent_max_output_buffer:20504
blocked_clients:0
tracking_clients:0
clients_in_timeout_table:0
total_blocking_keys:0
total_blocking_keys_on_nokey:0

# Memory
used_memory:2306136
used_memory_human:2.20M
used_memory_rss:9572352
used_memory_rss_human:9.13M
used_memory_peak:2324848
used_memory_peak_human:2.22M
used_memory_peak_perc:99.20%
used_memory_overhead:1681732
used_memory_startup:1584104
used_memory_dataset:624404
used_memory_dataset_perc:86.48%
allocator_allocated:2515088
allocator_active:2887680
allocator_resident:6574080
total_system_memory:16777797632
total_system_memory_human:15.63G
used_memory_lua:31744
used_memory_vm_eval:31744
used_memory_lua_human:31.00K
used_memory_scripts_eval:0
number_of_cached_scripts:0
number_of_functions:0
number_of_libraries:0
used_memory_vm_functions:32768
used_memory_vm_total:64512
used_memory_vm_total_human:63.00K
used_memory_functions:184
used_memory_scripts:184
used_memory_scripts_human:184B
maxmemory:167772160
maxmemory_human:160.00M
maxmemory_policy:noeviction
allocator_frag_ratio:1.15
allocator_frag_bytes:372592
allocator_rss_ratio:2.28
allocator_rss_bytes:3686400
rss_overhead_ratio:1.46
rss_overhead_bytes:2998272
mem_fragmentation_ratio:4.22
mem_fragmentation_bytes:7306264
mem_not_counted_for_evict:0
mem_replication_backlog:41012
mem_total_replication_buffers:41008
mem_clients_slaves:0
mem_clients_normal:22400
mem_cluster_links:10720
mem_aof_buffer:0
mem_allocator:jemalloc-5.3.0
active_defrag_running:0
lazyfree_pending_objects:0
lazyfreed_objects:0

# Persistence
loading:0
async_loading:0
current_cow_peak:0
current_cow_size:0
current_cow_size_age:0
current_fork_perc:0.00
current_save_keys_processed:0
current_save_keys_total:0
rdb_changes_since_last_save:373
rdb_bgsave_in_progress:0
rdb_last_save_time:1743664683
rdb_last_bgsave_status:ok
rdb_last_bgsave_time_sec:0
rdb_current_bgsave_time_sec:-1
rdb_saves:2
rdb_last_cow_size:450560
rdb_last_load_keys_expired:0
rdb_last_load_keys_loaded:0
aof_enabled:0
aof_rewrite_in_progress:0
aof_rewrite_scheduled:0
aof_last_rewrite_time_sec:-1
aof_current_rewrite_time_sec:-1
aof_last_bgrewrite_status:ok
aof_rewrites:0
aof_rewrites_consecutive_failures:0
aof_last_write_status:ok
aof_last_cow_size:0
module_fork_in_progress:0
module_fork_last_cow_size:0

# Stats
total_connections_received:6131
total_commands_processed:51453
instantaneous_ops_per_sec:1
total_net_input_bytes:1728823
total_net_output_bytes:28900088
total_net_repl_input_bytes:0
total_net_repl_output_bytes:35746
instantaneous_input_kbps:0.05
instantaneous_output_kbps:0.33
instantaneous_input_repl_kbps:0.00
instantaneous_output_repl_kbps:0.00
rejected_connections:0
sync_full:1
sync_partial_ok:0
sync_partial_err:1
expired_keys:0
expired_stale_perc:0.00
expired_time_cap_reached_count:0
expire_cycle_cpu_milliseconds:160
evicted_keys:0
evicted_clients:0
total_eviction_exceeded_time:0
current_eviction_exceeded_time:0
keyspace_hits:0
keyspace_misses:0
pubsub_channels:0
pubsub_patterns:0
pubsubshard_channels:0
latest_fork_usec:1166
total_forks:2
migrate_cached_sockets:0
slave_expires_tracked_keys:0
active_defrag_hits:0
active_defrag_misses:0
active_defrag_key_hits:0
active_defrag_key_misses:0
total_active_defrag_time:0
current_active_defrag_time:0
tracking_total_keys:0
tracking_total_items:0
tracking_total_prefixes:0
unexpected_error_replies:0
total_error_replies:713
dump_payload_sanitizations:0
total_reads_processed:58301
total_writes_processed:37978
io_threaded_reads_processed:0
io_threaded_writes_processed:0
reply_buffer_shrinks:62
reply_buffer_expands:2
eventloop_cycles:292781
eventloop_duration_sum:63530292
eventloop_duration_cmd_sum:3083152
instantaneous_eventloop_cycles_per_sec:14
instantaneous_eventloop_duration_usec:198
acl_access_denied_auth:0
acl_access_denied_cmd:0
acl_access_denied_key:0
acl_access_denied_channel:0

# Replication
role:master
connected_slaves:1
slave0:ip=100.64.0.2,port=6379,state=online,offset=35569,lag=1
master_failover_state:no-failover
master_replid:2b4495ceaa48926b7d4b7f66e528954e1893484b
master_replid2:0000000000000000000000000000000000000000
master_repl_offset:35569
second_repl_offset:-1
repl_backlog_active:1
repl_backlog_size:2097152
repl_backlog_first_byte_offset:1
repl_backlog_histlen:35569

# CPU
used_cpu_sys:22.074637
used_cpu_user:35.061374
used_cpu_sys_children:0.003467
used_cpu_user_children:0.002980
used_cpu_sys_main_thread:22.052857
used_cpu_user_main_thread:35.041799

# Modules

# Errorstats
errorstat_MOVED:count=712
errorstat_NOAUTH:count=1

# Cluster
cluster_enabled:1

# Keyspace
db0:keys=343,expires=0,avg_ttl=0`,
			},
			wantError: false,
			want: &NodeInfo{
				Version:           "7.2.7",
				ServerMode:        "cluster",
				RunId:             "1f6aa99e4aabffabe5ef77d22d107315956060cf",
				UptimeInSeconds:   16381,
				AOFEnabled:        "0",
				Role:              "master",
				ConnectedReplicas: 1,
				MasterHost:        "",
				MasterPort:        "",
				ClusterEnabled:    "1",
				MasterLinkStatus:  "",
				MasterReplId:      "2b4495ceaa48926b7d4b7f66e528954e1893484b",
				MasterReplOffset:  35569,
				MasterReplId2:     "0000000000000000000000000000000000000000",
				SecondReplOffset:  -1,
				UsedMemory:        2306136,
				UsedMemoryDataset: 624404,
				Dbsize:            343,
			},
		},
		{
			name: "sentinel info",
			args: args{
				info: `# Server
redis_version:7.2.7
redis_git_sha1:00000000
redis_git_dirty:0
redis_build_id:d602a27dd72c7efa
redis_mode:sentinel
os:Linux 5.15.0-56-generic x86_64
arch_bits:64
monotonic_clock:POSIX clock_gettime
multiplexing_api:epoll
atomicvar_api:c11-builtin
gcc_version:13.2.1
process_id:29
process_supervised:no
run_id:f815cccd1188fc17ab4dbde798b0f8c7fa1abd2f
tcp_port:26379
server_time_usec:1743667639890319
uptime_in_seconds:81974
uptime_in_days:0
hz:10
configured_hz:10
lru_clock:15614391
executable:/data/redis-server
config_file:/data/sentinel.conf
io_threads_active:0
listener0:name=tcp,bind=10.3.0.160,bind=127.0.0.1,port=26379

# Clients
connected_clients:4
cluster_connections:0
maxclients:10000
client_recent_max_input_buffer:20480
client_recent_max_output_buffer:0
blocked_clients:0
tracking_clients:0
clients_in_timeout_table:0
total_blocking_keys:0
total_blocking_keys_on_nokey:0

# Stats
total_connections_received:110827
total_commands_processed:770293
instantaneous_ops_per_sec:9
total_net_input_bytes:29353833
total_net_output_bytes:142460972
total_net_repl_input_bytes:0
total_net_repl_output_bytes:0
instantaneous_input_kbps:0.33
instantaneous_output_kbps:3.01
instantaneous_input_repl_kbps:0.00
instantaneous_output_repl_kbps:0.00
rejected_connections:0
sync_full:0
sync_partial_ok:0
sync_partial_err:0
expired_keys:0
expired_stale_perc:0.00
expired_time_cap_reached_count:0
expire_cycle_cpu_milliseconds:2446
evicted_keys:0
evicted_clients:0
total_eviction_exceeded_time:0
current_eviction_exceeded_time:0
keyspace_hits:0
keyspace_misses:0
pubsub_channels:0
pubsub_patterns:0
pubsubshard_channels:0
latest_fork_usec:0
total_forks:0
migrate_cached_sockets:0
slave_expires_tracked_keys:0
active_defrag_hits:0
active_defrag_misses:0
active_defrag_key_hits:0
active_defrag_key_misses:0
total_active_defrag_time:0
current_active_defrag_time:0
tracking_total_keys:0
tracking_total_items:0
tracking_total_prefixes:0
unexpected_error_replies:0
total_error_replies:32
dump_payload_sanitizations:0
total_reads_processed:875345
total_writes_processed:764521
io_threaded_reads_processed:0
io_threaded_writes_processed:0
reply_buffer_shrinks:5160
reply_buffer_expands:0
eventloop_cycles:2946883
eventloop_duration_sum:350738114
eventloop_duration_cmd_sum:8903044
instantaneous_eventloop_cycles_per_sec:37
instantaneous_eventloop_duration_usec:182
acl_access_denied_auth:0
acl_access_denied_cmd:0
acl_access_denied_key:0
acl_access_denied_channel:0

# CPU
used_cpu_sys:206.130316
used_cpu_user:208.367429
used_cpu_sys_children:0.000000
used_cpu_user_children:0.000000
used_cpu_sys_main_thread:206.047602
used_cpu_user_main_thread:208.280289

# Sentinel
sentinel_masters:1
sentinel_tilt:0
sentinel_tilt_since_seconds:-1
sentinel_running_scripts:0
sentinel_scripts_queue_length:0
sentinel_simulate_failure_flags:0
master0:name=mymaster,status=ok,address=192.168.130.253:30634,slaves=1,sentinels=3`,
			},
			wantError: false,
			want: &NodeInfo{
				Version:               "7.2.7",
				ServerMode:            "sentinel",
				RunId:                 "f815cccd1188fc17ab4dbde798b0f8c7fa1abd2f",
				UptimeInSeconds:       81974,
				Role:                  "sentinel",
				SentinelMasters:       1,
				SentinelTiLt:          0,
				SentinelRunningScript: 0,
				SentinelMaster0: SentinelMaster{
					Name:      "mymaster",
					Status:    "ok",
					Address:   "192.168.130.253:30634",
					Replicas:  1,
					Sentinels: 3,
				},
			},
		},
		{
			name: "replica info",
			args: args{
				info: `# Server
redis_version:7.2.7
redis_git_sha1:00000000
redis_git_dirty:0
redis_build_id:d602a27dd72c7efa
redis_mode:standalone
os:Linux 5.15.0-56-generic x86_64
arch_bits:64
monotonic_clock:POSIX clock_gettime
multiplexing_api:epoll
atomicvar_api:c11-builtin
gcc_version:13.2.1
process_id:69
process_supervised:no
run_id:a7b47cb3db33279ef84841217bfac4c74f2036b8
tcp_port:6379
server_time_usec:1743670029978136
uptime_in_seconds:84006
uptime_in_days:0
hz:10
configured_hz:10
lru_clock:15616781
executable:/data/redis-server
config_file:/tmp/redis.conf
io_threads_active:0
listener0:name=tcp,bind=10.3.0.139,bind=127.0.0.1,port=6379

# Clients
connected_clients:8
cluster_connections:0
maxclients:10000
client_recent_max_input_buffer:20480
client_recent_max_output_buffer:0
blocked_clients:0
tracking_clients:0
clients_in_timeout_table:0
total_blocking_keys:0
total_blocking_keys_on_nokey:0

# Memory
used_memory:3601480
used_memory_human:3.43M
used_memory_rss:11206656
used_memory_rss_human:10.69M
used_memory_peak:3667200
used_memory_peak_human:3.50M
used_memory_peak_perc:98.21%
used_memory_overhead:3137080
used_memory_startup:867816
used_memory_dataset:464400
used_memory_dataset_perc:16.99%
allocator_allocated:3743784
allocator_active:4063232
allocator_resident:7802880
total_system_memory:16777797632
total_system_memory_human:15.63G
used_memory_lua:31744
used_memory_vm_eval:31744
used_memory_lua_human:31.00K
used_memory_scripts_eval:0
number_of_cached_scripts:0
number_of_functions:0
number_of_libraries:0
used_memory_vm_functions:32768
used_memory_vm_total:64512
used_memory_vm_total_human:63.00K
used_memory_functions:184
used_memory_scripts:184
used_memory_scripts_human:184B
maxmemory:167772160
maxmemory_human:160.00M
maxmemory_policy:noeviction
allocator_frag_ratio:1.09
allocator_frag_bytes:319448
allocator_rss_ratio:1.92
allocator_rss_bytes:3739648
rss_overhead_ratio:1.44
rss_overhead_bytes:3403776
mem_fragmentation_ratio:3.15
mem_fragmentation_bytes:7645256
mem_not_counted_for_evict:27008
mem_replication_backlog:2132432
mem_total_replication_buffers:2132416
mem_clients_slaves:0
mem_clients_normal:136648
mem_cluster_links:0
mem_aof_buffer:0
mem_allocator:jemalloc-5.3.0
active_defrag_running:0
lazyfree_pending_objects:0
lazyfreed_objects:0

# Persistence
loading:0
async_loading:0
current_cow_peak:0
current_cow_size:0
current_cow_size_age:0
current_fork_perc:0.00
current_save_keys_processed:0
current_save_keys_total:0
rdb_changes_since_last_save:0
rdb_bgsave_in_progress:0
rdb_last_save_time:1743586023
rdb_last_bgsave_status:ok
rdb_last_bgsave_time_sec:-1
rdb_current_bgsave_time_sec:-1
rdb_saves:0
rdb_last_cow_size:0
rdb_last_load_keys_expired:0
rdb_last_load_keys_loaded:0
aof_enabled:0
aof_rewrite_in_progress:0
aof_rewrite_scheduled:0
aof_last_rewrite_time_sec:-1
aof_current_rewrite_time_sec:-1
aof_last_bgrewrite_status:ok
aof_rewrites:0
aof_rewrites_consecutive_failures:0
aof_last_write_status:ok
aof_last_cow_size:0
module_fork_in_progress:0
module_fork_last_cow_size:0

# Stats
total_connections_received:23011
total_commands_processed:568078
instantaneous_ops_per_sec:4
total_net_input_bytes:41141333
total_net_output_bytes:340417234
total_net_repl_input_bytes:17956338
total_net_repl_output_bytes:0
instantaneous_input_kbps:0.38
instantaneous_output_kbps:1.09
instantaneous_input_repl_kbps:0.17
instantaneous_output_repl_kbps:0.00
rejected_connections:0
sync_full:0
sync_partial_ok:0
sync_partial_err:0
expired_keys:0
expired_stale_perc:0.00
expired_time_cap_reached_count:0
expire_cycle_cpu_milliseconds:0
evicted_keys:0
evicted_clients:0
total_eviction_exceeded_time:0
current_eviction_exceeded_time:0
keyspace_hits:0
keyspace_misses:0
pubsub_channels:1
pubsub_patterns:0
pubsubshard_channels:0
latest_fork_usec:0
total_forks:0
migrate_cached_sockets:0
slave_expires_tracked_keys:0
active_defrag_hits:0
active_defrag_misses:0
active_defrag_key_hits:0
active_defrag_key_misses:0
total_active_defrag_time:0
current_active_defrag_time:0
tracking_total_keys:0
tracking_total_items:0
tracking_total_prefixes:0
unexpected_error_replies:0
total_error_replies:1
dump_payload_sanitizations:0
total_reads_processed:579345
total_writes_processed:1213632
io_threaded_reads_processed:0
io_threaded_writes_processed:0
reply_buffer_shrinks:25101
reply_buffer_expands:25094
eventloop_cycles:1409981
eventloop_duration_sum:305498197
eventloop_duration_cmd_sum:12656856
instantaneous_eventloop_cycles_per_sec:13
instantaneous_eventloop_duration_usec:202
acl_access_denied_auth:0
acl_access_denied_cmd:0
acl_access_denied_key:0
acl_access_denied_channel:0

# Replication
role:slave
master_host:192.168.142.59
master_port:31794
master_link_status:up
master_last_io_seconds_ago:0
master_sync_in_progress:0
slave_read_repl_offset:17956161
slave_repl_offset:17956161
slave_priority:100
slave_read_only:1
replica_announced:1
connected_slaves:0
master_failover_state:no-failover
master_replid:d34a3192f680e9246df5058a8c5b8eec7e2f1910
master_replid2:0000000000000000000000000000000000000000
master_repl_offset:17956161
second_repl_offset:-1
repl_backlog_active:1
repl_backlog_size:2097152
repl_backlog_first_byte_offset:15841001
repl_backlog_histlen:2115161

# CPU
used_cpu_sys:176.490920
used_cpu_user:167.622959
used_cpu_sys_children:0.000000
used_cpu_user_children:0.000000
used_cpu_sys_main_thread:176.049835
used_cpu_user_main_thread:167.232796

# Modules

# Errorstats
errorstat_ERR:count=1

# Cluster
cluster_enabled:0

# Keyspace`,
			},
			wantError: false,
			want: &NodeInfo{
				Version:           "7.2.7",
				ServerMode:        "standalone",
				RunId:             "a7b47cb3db33279ef84841217bfac4c74f2036b8",
				UptimeInSeconds:   84006,
				AOFEnabled:        "0",
				Role:              "slave",
				ConnectedReplicas: 0,
				MasterHost:        "192.168.142.59",
				MasterPort:        "31794",
				ClusterEnabled:    "0",
				MasterLinkStatus:  "up",
				MasterReplId:      "d34a3192f680e9246df5058a8c5b8eec7e2f1910",
				MasterReplOffset:  17956161,
				MasterReplId2:     "0000000000000000000000000000000000000000",
				SecondReplOffset:  -1,
				UsedMemory:        3601480,
				UsedMemoryDataset: 464400,
				Dbsize:            0,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			conn.Command("INFO").Expect(tc.args.info)

			info, err := client.Info(context.Background())
			if err != nil {
				t.Errorf("Info() error = %v", err)
				return
			}
			if !reflect.DeepEqual(info, tc.want) {
				t.Errorf("Info() = %v, want %v", info, tc.want)
			}
		})
	}

	var nilClient ValkeyClient = (*valkeyClient)(nil)
	if _, err := nilClient.Info(context.Background()); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestValkeyClient_ClusterInfo(t *testing.T) {
	conn := redigomock.NewConn()
	client := newValkeyMockClient(conn)
	defer client.Close()

	conn.Command("CLUSTER", "INFO").Expect(`cluster_state:ok
cluster_slots_assigned:16384
cluster_slots_ok:16384
cluster_slots_pfail:0
cluster_slots_fail:0
cluster_known_nodes:6
cluster_size:3
cluster_current_epoch:5
cluster_my_epoch:1
cluster_stats_messages_ping_sent:11941
cluster_stats_messages_pong_sent:12518
cluster_stats_messages_meet_sent:7
cluster_stats_messages_sent:24466
cluster_stats_messages_ping_received:12518
cluster_stats_messages_pong_received:11948
cluster_stats_messages_received:24466
total_cluster_links_buffer_limit_exceeded:0`)

	info, err := client.ClusterInfo(context.Background())
	if err != nil {
		t.Errorf("ClusterInfo() error = %v", err)
		return
	}
	if info == nil {
		t.Fatalf("Expected non-nil info")
	}
	if info.ClusterState != "ok" {
		t.Errorf("Expected ClusterState: %s, got: %s", "ok", info.ClusterState)
	}
	if info.ClusterSlotsAssigned != 16384 {
		t.Errorf("Expected ClusterSlotsAssigned: %d, got: %d", 16384, info.ClusterSlotsAssigned)
	}
	if info.ClusterKnownNodes != 6 {
		t.Errorf("Expected ClusterKnownNodes: %d, got: %d", 6, info.ClusterKnownNodes)
	}
	if info.ClusterSize != 3 {
		t.Errorf("Expected ClusterSize: %d, got: %d", 3, info.ClusterSize)
	}
	if info.ClusterCurrentEpoch != 5 {
		t.Errorf("Expected ClusterCurrentEpoch: %d, got: %d", 5, info.ClusterCurrentEpoch)
	}

	var nilClient ValkeyClient = (*valkeyClient)(nil)
	if _, err := nilClient.ClusterInfo(context.Background()); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestParseSentinelMonitorNode(t *testing.T) {
	testCases := []struct {
		name     string
		input    []any
		expected *SentinelMonitorNode
		validate func(*testing.T, *SentinelMonitorNode)
	}{
		{
			name: "master node",
			input: []any{
				[]byte("name"),
				[]byte("mymaster"),
				[]byte("ip"),
				[]byte("192.168.130.253"),
				[]byte("port"),
				[]byte("30634"),
				[]byte("runid"),
				[]byte("8cbc991b66315e9128ed51a8cfdc9d8ac4eeea93"),
				[]byte("flags"),
				[]byte("master"),
				[]byte("link-pending-commands"),
				[]byte("0"),
				[]byte("link-refcount"),
				[]byte("1"),
				[]byte("last-ping-sent"),
				[]byte("0"),
				[]byte("last-ok-ping-reply"),
				[]byte("204"),
				[]byte("last-ping-reply"),
				[]byte("204"),
				[]byte("down-after-milliseconds"),
				[]byte("30000"),
				[]byte("info-refresh"),
				[]byte("3180"),
				[]byte("role-reported"),
				[]byte("master"),
				[]byte("role-reported-time"),
				[]byte("75569380"),
				[]byte("config-epoch"),
				[]byte("0"),
				[]byte("num-slaves"),
				[]byte("1"),
				[]byte("num-other-sentinels"),
				[]byte("2"),
				[]byte("quorum"),
				[]byte("2"),
				[]byte("failover-timeout"),
				[]byte("180000"),
				[]byte("parallel-syncs"),
				[]byte("1"),
			},
			validate: func(t *testing.T, node *SentinelMonitorNode) {
				if node.Name != "mymaster" {
					t.Errorf("Expected Name: %s, got: %s", "mymaster", node.Name)
				}
				if node.IP != "192.168.130.253" {
					t.Errorf("Expected IP: %s, got: %s", "192.168.130.253", node.IP)
				}
				if node.Port != "30634" {
					t.Errorf("Expected Port: %s, got: %s", "30634", node.Port)
				}
				if node.RunId != "8cbc991b66315e9128ed51a8cfdc9d8ac4eeea93" {
					t.Errorf("Expected RunId: %s, got: %s", "8cbc991b66315e9128ed51a8cfdc9d8ac4eeea93", node.RunId)
				}
				if node.Flags != "master" {
					t.Errorf("Expected Flags: %s, got: %s", "master", node.Flags)
				}
				if node.LinkPendingCommands != 0 {
					t.Errorf("Expected LinkPendingCommands: %d, got: %d", 0, node.LinkPendingCommands)
				}
				if node.LinkRefcount != 1 {
					t.Errorf("Expected LinkRefcount: %d, got: %d", 1, node.LinkRefcount)
				}
				if node.LastPingSent != 0 {
					t.Errorf("Expected LastPingSent: %d, got: %d", 0, node.LastPingSent)
				}
				if node.LastOkPingReply != 204 {
					t.Errorf("Expected LastOkPingReply: %d, got: %d", 204, node.LastOkPingReply)
				}
				if node.LastPingReply != 204 {
					t.Errorf("Expected LastPingReply: %d, got: %d", 204, node.LastPingReply)
				}
				if node.DownAfterMilliseconds != 30000 {
					t.Errorf("Expected DownAfterMilliseconds: %d, got: %d", 30000, node.DownAfterMilliseconds)
				}
				if node.InfoRefresh != 3180 {
					t.Errorf("Expected InfoRefresh: %d, got: %d", 3180, node.InfoRefresh)
				}
				if node.RoleReported != "master" {
					t.Errorf("Expected RoleReported: %s, got: %s", "master", node.RoleReported)
				}
				if node.RoleReportedTime != 75569380 {
					t.Errorf("Expected RoleReportedTime: %d, got: %d", 75569380, node.RoleReportedTime)
				}
				if node.ConfigEpoch != 0 {
					t.Errorf("Expected ConfigEpoch: %d, got: %d", 0, node.ConfigEpoch)
				}
				if node.NumSlaves != 1 {
					t.Errorf("Expected NumSlaves: %d, got: %d", 1, node.NumSlaves)
				}
				if node.NumOtherSentinels != 2 {
					t.Errorf("Expected NumOtherSentinels: %d, got: %d", 2, node.NumOtherSentinels)
				}
				if node.Quorum != 2 {
					t.Errorf("Expected Quorum: %d, got: %d", 2, node.Quorum)
				}
				if node.FailoverTimeout != 180000 {
					t.Errorf("Expected FailoverTimeout: %d, got: %d", 180000, node.FailoverTimeout)
				}
				if node.ParallelSyncs != 1 {
					t.Errorf("Expected ParallelSyncs: %d, got: %d", 1, node.ParallelSyncs)
				}
				// Test IsMaster method
				if !node.IsMaster() {
					t.Errorf("Expected IsMaster() to return true")
				}
				// Test Address method
				expectedAddr := "192.168.130.253:30634"
				if node.Address() != expectedAddr {
					t.Errorf("Expected Address(): %s, got: %s", expectedAddr, node.Address())
				}
			},
		},
		{
			name: "replica node",
			input: []any{
				[]byte("name"),
				[]byte("192.168.142.59:31408"),
				[]byte("ip"),
				[]byte("192.168.142.59"),
				[]byte("port"),
				[]byte("31408"),
				[]byte("runid"),
				[]byte("a38cace0f1774c9c552aeeff5bdc08a48ea6ca66"),
				[]byte("flags"),
				[]byte("slave"),
				[]byte("link-pending-commands"),
				[]byte("-2"),
				[]byte("link-refcount"),
				[]byte("1"),
				[]byte("last-ping-sent"),
				[]byte("0"),
				[]byte("last-ok-ping-reply"),
				[]byte("1000"),
				[]byte("last-ping-reply"),
				[]byte("1000"),
				[]byte("down-after-milliseconds"),
				[]byte("30000"),
				[]byte("info-refresh"),
				[]byte("688"),
				[]byte("role-reported"),
				[]byte("slave"),
				[]byte("role-reported-time"),
				[]byte("76199129"),
				[]byte("master-link-down-time"),
				[]byte("0"),
				[]byte("master-link-status"),
				[]byte("ok"),
				[]byte("master-host"),
				[]byte("192.168.130.253"),
				[]byte("master-port"),
				[]byte("30634"),
				[]byte("slave-priority"),
				[]byte("100"),
				[]byte("slave-repl-offset"),
				[]byte("16361822"),
				[]byte("replica-announced"),
				[]byte("1"),
			},
			validate: func(t *testing.T, node *SentinelMonitorNode) {
				if node.Name != "192.168.142.59:31408" {
					t.Errorf("Expected Name: %s, got: %s", "192.168.142.59:31408", node.Name)
				}
				if node.IP != "192.168.142.59" {
					t.Errorf("Expected IP: %s, got: %s", "192.168.142.59", node.IP)
				}
				if node.Port != "31408" {
					t.Errorf("Expected Port: %s, got: %s", "31408", node.Port)
				}
				if node.Flags != "slave" {
					t.Errorf("Expected Flags: %s, got: %s", "slave", node.Flags)
				}
				if node.MasterLinkDownTime != 0 {
					t.Errorf("Expected MasterLinkDownTime: %d, got: %d", 0, node.MasterLinkDownTime)
				}
				if node.MasterLinkStatus != "ok" {
					t.Errorf("Expected MasterLinkStatus: %s, got: %s", "ok", node.MasterLinkStatus)
				}
				if node.MasterHost != "192.168.130.253" {
					t.Errorf("Expected MasterHost: %s, got: %s", "192.168.130.253", node.MasterHost)
				}
				if node.MasterPort != "30634" {
					t.Errorf("Expected MasterPort: %s, got: %s", "30634", node.MasterPort)
				}
				if node.SlavePriority != 100 {
					t.Errorf("Expected SlavePriority: %d, got: %d", 100, node.SlavePriority)
				}
				if node.SlaveReplOffset != 16361822 {
					t.Errorf("Expected SlaveReplOffset: %d, got: %d", 16361822, node.SlaveReplOffset)
				}
				// Test IsMaster method
				if node.IsMaster() {
					t.Errorf("Expected IsMaster() to return false")
				}
			},
		},
		{
			name: "sentinel node",
			input: []any{
				[]byte("name"),
				[]byte("bad4030f06de6422d9fbf4a927d9b5293f1536c8"),
				[]byte("ip"),
				[]byte("192.168.142.59"),
				[]byte("port"),
				[]byte("32570"),
				[]byte("runid"),
				[]byte("bad4030f06de6422d9fbf4a927d9b5293f1536c8"),
				[]byte("flags"),
				[]byte("sentinel"),
				[]byte("link-pending-commands"),
				[]byte("0"),
				[]byte("link-refcount"),
				[]byte("1"),
				[]byte("last-ping-sent"),
				[]byte("0"),
				[]byte("last-ok-ping-reply"),
				[]byte("108"),
				[]byte("last-ping-reply"),
				[]byte("108"),
				[]byte("down-after-milliseconds"),
				[]byte("30000"),
				[]byte("last-hello-message"),
				[]byte("110"),
				[]byte("voted-leader"),
				[]byte("?"),
				[]byte("voted-leader-epoch"),
				[]byte("0"),
			},
			validate: func(t *testing.T, smn *SentinelMonitorNode) {
				if smn.Name != "bad4030f06de6422d9fbf4a927d9b5293f1536c8" {
					t.Errorf("Expected Name: %s, got: %s", "bad4030f06de6422d9fbf4a927d9b5293f1536c8", smn.Name)
				}
				if smn.LastHelloMessage != "110" {
					t.Errorf("Expected LastHelloMessage: %s, got: %s", "110", smn.LastHelloMessage)
				}
				if smn.VotedLeader != "?" {
					t.Errorf("Expected VotedLeader: %s, got: %s", "?", smn.VotedLeader)
				}
				if smn.VotedLeaderEpoch != 0 {
					t.Errorf("Expected VotedLeaderEpoch: %d, got: %d", 0, smn.VotedLeaderEpoch)
				}
			},
		},
		{
			name:  "nil input",
			input: nil,
			validate: func(t *testing.T, node *SentinelMonitorNode) {
				if node == nil {
					t.Errorf("Expected non-nil result for nil input")
				}
			},
		},
		{
			name:  "empty input",
			input: []any{},
			validate: func(t *testing.T, node *SentinelMonitorNode) {
				if node == nil {
					t.Errorf("Expected non-nil result for empty input")
				}
			},
		},
		{
			name: "failovering node",
			input: []any{
				[]byte("name"),
				[]byte("master"),
				[]byte("ip"),
				[]byte("192.168.130.253"),
				[]byte("port"),
				[]byte("30634"),
				[]byte("flags"),
				[]byte("master,failover_in_progress"),
				[]byte("failover_state"),
				[]byte("waiting_for_sync"),
			},
			validate: func(t *testing.T, node *SentinelMonitorNode) {
				if node.Name != "master" {
					t.Errorf("Expected Name: %s, got: %s", "master", node.Name)
				}
				if node.Flags != "master,failover_in_progress" {
					t.Errorf("Expected Flags: %s, got: %s", "master,failover_in_progress", node.Flags)
				}
				if node.FailoverState != "waiting_for_sync" {
					t.Errorf("Expected FailoverState: %s, got: %s", "waiting_for_sync", node.FailoverState)
				}
				// Test IsFailovering method
				if !node.IsFailovering() {
					t.Errorf("Expected IsFailovering() to return true")
				}
			},
		},
		{
			name: "subject down master node",
			input: []any{
				[]byte("name"),
				[]byte("master"),
				[]byte("ip"),
				[]byte("192.168.130.253"),
				[]byte("port"),
				[]byte("30634"),
				[]byte("flags"),
				[]byte("s_down,master"),
				[]byte("s-down-time"),
				[]byte("1234567"),
			},
			validate: func(t *testing.T, node *SentinelMonitorNode) {
				if node.Name != "master" {
					t.Errorf("Expected Name: %s, got: %s", "master", node.Name)
				}
				if node.Flags != "s_down,master" {
					t.Errorf("Expected Flags: %s, got: %s", "s_down,master", node.Flags)
				}
				if node.SDownTime != 1234567 {
					t.Errorf("Expected SDownTime: %d, got: %d", 1234567, node.SDownTime)
				}
				// Test IsMaster method for down master
				if node.IsMaster() {
					t.Errorf("Expected IsMaster() to return false for down master")
				}
			},
		},
		{
			name: "object down master node",
			input: []any{
				[]byte("name"),
				[]byte("master"),
				[]byte("ip"),
				[]byte("192.168.130.253"),
				[]byte("port"),
				[]byte("30634"),
				[]byte("flags"),
				[]byte("o_down,master"),
				[]byte("o-down-time"),
				[]byte("1234567"),
			},
			validate: func(t *testing.T, node *SentinelMonitorNode) {
				if node.Name != "master" {
					t.Errorf("Expected Name: %s, got: %s", "master", node.Name)
				}
				if node.Flags != "o_down,master" {
					t.Errorf("Expected Flags: %s, got: %s", "o_down,master", node.Flags)
				}
				if node.ODownTime != 1234567 {
					t.Errorf("Expected ODownTime: %d, got: %d", 1234567, node.SDownTime)
				}
				// Test IsMaster method for down master
				if node.IsMaster() {
					t.Errorf("Expected IsMaster() to return false for down master")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			node := ParseSentinelMonitorNode(tc.input)
			tc.validate(t, node)
		})
	}
}
