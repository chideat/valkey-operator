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
*/package types

import (
	"context"
	"net"
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/pkg/slot"
	vkcli "github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/chideat/valkey-operator/pkg/version"
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

// Mock implementations for ValkeyNode and SentinelNode interfaces
type MockValkeyNode struct{}

func (m *MockValkeyNode) GetObjectKind() schema.ObjectKind                     { return nil }
func (m *MockValkeyNode) Definition() *corev1.Pod                              { return nil }
func (m *MockValkeyNode) ID() string                                           { return "mock-id" }
func (m *MockValkeyNode) Index() int                                           { return 0 }
func (m *MockValkeyNode) IsConnected() bool                                    { return true }
func (m *MockValkeyNode) IsTerminating() bool                                  { return false }
func (m *MockValkeyNode) IsMasterLinkUp() bool                                 { return true }
func (m *MockValkeyNode) IsReady() bool                                        { return true }
func (m *MockValkeyNode) IsJoined() bool                                       { return true }
func (m *MockValkeyNode) MasterID() string                                     { return "master-id" }
func (m *MockValkeyNode) IsMasterFailed() bool                                 { return false }
func (m *MockValkeyNode) CurrentVersion() version.ValkeyVersion                { return "6.2.1" }
func (m *MockValkeyNode) IsACLApplied() bool                                   { return true }
func (m *MockValkeyNode) Role() core.NodeRole                                  { return core.NodeRoleMaster }
func (m *MockValkeyNode) Slots() *slot.Slots                                   { return nil }
func (m *MockValkeyNode) Config() map[string]string                            { return nil }
func (m *MockValkeyNode) ConfigedMasterIP() string                             { return "127.0.0.1" }
func (m *MockValkeyNode) ConfigedMasterPort() string                           { return "6379" }
func (m *MockValkeyNode) Setup(ctx context.Context, margs ...[]any) error      { return nil }
func (m *MockValkeyNode) ReplicaOf(ctx context.Context, ip, port string) error { return nil }
func (m *MockValkeyNode) SetACLUser(ctx context.Context, username string, passwords []string, rules string) (interface{}, error) {
	return nil, nil
}
func (m *MockValkeyNode) Query(ctx context.Context, cmd string, args ...any) (any, error) {
	return nil, nil
}
func (m *MockValkeyNode) Info() vkcli.NodeInfo                     { return vkcli.NodeInfo{} }
func (m *MockValkeyNode) ClusterInfo() vkcli.ClusterNodeInfo       { return vkcli.ClusterNodeInfo{} }
func (m *MockValkeyNode) IPort() int                               { return 6379 }
func (m *MockValkeyNode) InternalIPort() int                       { return 6379 }
func (m *MockValkeyNode) Port() int                                { return 6379 }
func (m *MockValkeyNode) InternalPort() int                        { return 6379 }
func (m *MockValkeyNode) DefaultIP() net.IP                        { return net.ParseIP("127.0.0.1") }
func (m *MockValkeyNode) DefaultInternalIP() net.IP                { return net.ParseIP("127.0.0.1") }
func (m *MockValkeyNode) IPs() []net.IP                            { return []net.IP{net.ParseIP("127.0.0.1")} }
func (m *MockValkeyNode) NodeIP() net.IP                           { return net.ParseIP("127.0.0.1") }
func (m *MockValkeyNode) Status() corev1.PodPhase                  { return corev1.PodRunning }
func (m *MockValkeyNode) ContainerStatus() *corev1.ContainerStatus { return nil }
func (m *MockValkeyNode) Refresh(ctx context.Context) error        { return nil }

func TestValkeyNodeMethods(t *testing.T) {
	node := &MockValkeyNode{}

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
