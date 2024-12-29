/*
Copyright 2023 The RedisOperator Authors.

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

package redis

import (
	"context"
	"net"
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	rediscli "github.com/chideat/valkey-operator/pkg/redis"
	"github.com/chideat/valkey-operator/pkg/slot"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestNewNodeRole(t *testing.T) {
	testCases := []struct {
		input    string
		expected core.NodeRole
	}{
		{"master", core.NodeRoleMaster},
		{"slave", core.NodeRoleReplica},
		{"replica", core.NodeRoleReplica},
		{"sentinel", core.NodeRoleSentinel},
		{"unknown", core.NodeRoleNone},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := NewNodeRole(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// Mock implementations for RedisNode and RedisSentinelNode interfaces
type MockRedisNode struct{}

func (m *MockRedisNode) GetObjectKind() schema.ObjectKind                     { return nil }
func (m *MockRedisNode) Definition() *corev1.Pod                              { return nil }
func (m *MockRedisNode) ID() string                                           { return "mock-id" }
func (m *MockRedisNode) Index() int                                           { return 0 }
func (m *MockRedisNode) IsConnected() bool                                    { return true }
func (m *MockRedisNode) IsTerminating() bool                                  { return false }
func (m *MockRedisNode) IsMasterLinkUp() bool                                 { return true }
func (m *MockRedisNode) IsReady() bool                                        { return true }
func (m *MockRedisNode) IsJoined() bool                                       { return true }
func (m *MockRedisNode) MasterID() string                                     { return "master-id" }
func (m *MockRedisNode) IsMasterFailed() bool                                 { return false }
func (m *MockRedisNode) CurrentVersion() RedisVersion                         { return "6.2.1" }
func (m *MockRedisNode) IsACLApplied() bool                                   { return true }
func (m *MockRedisNode) Role() core.NodeRole                                  { return core.NodeRoleMaster }
func (m *MockRedisNode) Slots() *slot.Slots                                   { return nil }
func (m *MockRedisNode) Config() map[string]string                            { return nil }
func (m *MockRedisNode) ConfigedMasterIP() string                             { return "127.0.0.1" }
func (m *MockRedisNode) ConfigedMasterPort() string                           { return "6379" }
func (m *MockRedisNode) Setup(ctx context.Context, margs ...[]any) error      { return nil }
func (m *MockRedisNode) ReplicaOf(ctx context.Context, ip, port string) error { return nil }
func (m *MockRedisNode) SetACLUser(ctx context.Context, username string, passwords []string, rules string) (interface{}, error) {
	return nil, nil
}
func (m *MockRedisNode) Query(ctx context.Context, cmd string, args ...any) (any, error) {
	return nil, nil
}
func (m *MockRedisNode) Info() rediscli.RedisInfo                 { return rediscli.RedisInfo{} }
func (m *MockRedisNode) ClusterInfo() rediscli.RedisClusterInfo   { return rediscli.RedisClusterInfo{} }
func (m *MockRedisNode) IPort() int                               { return 6379 }
func (m *MockRedisNode) InternalIPort() int                       { return 6379 }
func (m *MockRedisNode) Port() int                                { return 6379 }
func (m *MockRedisNode) InternalPort() int                        { return 6379 }
func (m *MockRedisNode) DefaultIP() net.IP                        { return net.ParseIP("127.0.0.1") }
func (m *MockRedisNode) DefaultInternalIP() net.IP                { return net.ParseIP("127.0.0.1") }
func (m *MockRedisNode) IPs() []net.IP                            { return []net.IP{net.ParseIP("127.0.0.1")} }
func (m *MockRedisNode) NodeIP() net.IP                           { return net.ParseIP("127.0.0.1") }
func (m *MockRedisNode) Status() corev1.PodPhase                  { return corev1.PodRunning }
func (m *MockRedisNode) ContainerStatus() *corev1.ContainerStatus { return nil }
func (m *MockRedisNode) Refresh(ctx context.Context) error        { return nil }

func TestRedisNodeMethods(t *testing.T) {
	node := &MockRedisNode{}

	if node.ID() != "mock-id" {
		t.Errorf("Expected ID 'mock-id', got %s", node.ID())
	}
	if !node.IsConnected() {
		t.Errorf("Expected IsConnected to be true")
	}
	if node.IsTerminating() {
		t.Errorf("Expected IsTerminating to be false")
	}
	if !node.IsMasterLinkUp() {
		t.Errorf("Expected IsMasterLinkUp to be true")
	}
	if !node.IsReady() {
		t.Errorf("Expected IsReady to be true")
	}
	if !node.IsJoined() {
		t.Errorf("Expected IsJoined to be true")
	}
	if node.MasterID() != "master-id" {
		t.Errorf("Expected MasterID 'master-id', got %s", node.MasterID())
	}
	if node.IsMasterFailed() {
		t.Errorf("Expected IsMasterFailed to be false")
	}
	if node.CurrentVersion() != "6.2.1" {
		t.Errorf("Expected CurrentVersion '6.2.1', got %s", node.CurrentVersion())
	}
	if !node.IsACLApplied() {
		t.Errorf("Expected IsACLApplied to be true")
	}
	if node.Role() != core.NodeRoleMaster {
		t.Errorf("Expected Role 'master', got %v", node.Role())
	}
	if node.ConfigedMasterIP() != "127.0.0.1" {
		t.Errorf("Expected ConfigedMasterIP '127.0.0.1', got %s", node.ConfigedMasterIP())
	}
	if node.ConfigedMasterPort() != "6379" {
		t.Errorf("Expected ConfigedMasterPort '6379', got %s", node.ConfigedMasterPort())
	}
	if node.IPort() != 6379 {
		t.Errorf("Expected IPort 6379, got %d", node.IPort())
	}
	if node.InternalIPort() != 6379 {
		t.Errorf("Expected InternalIPort 6379, got %d", node.InternalIPort())
	}
	if node.Port() != 6379 {
		t.Errorf("Expected Port 6379, got %d", node.Port())
	}
	if node.InternalPort() != 6379 {
		t.Errorf("Expected InternalPort 6379, got %d", node.InternalPort())
	}
	if !node.DefaultIP().Equal(net.ParseIP("127.0.0.1")) {
		t.Errorf("Expected DefaultIP '127.0.0.1', got %s", node.DefaultIP())
	}
	if !node.DefaultInternalIP().Equal(net.ParseIP("127.0.0.1")) {
		t.Errorf("Expected DefaultInternalIP '127.0.0.1', got %s", node.DefaultInternalIP())
	}
	if len(node.IPs()) != 1 || !node.IPs()[0].Equal(net.ParseIP("127.0.0.1")) {
		t.Errorf("Expected IPs '[127.0.0.1]', got %v", node.IPs())
	}
	if !node.NodeIP().Equal(net.ParseIP("127.0.0.1")) {
		t.Errorf("Expected NodeIP '127.0.0.1', got %s", node.NodeIP())
	}
	if node.Status() != corev1.PodRunning {
		t.Errorf("Expected Status 'Running', got %v", node.Status())
	}
}
