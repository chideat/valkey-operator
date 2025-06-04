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

package clusterbuilder

import (
	"context"
	"crypto/tls"
	"strings"
	"testing"

	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/chideat/valkey-operator/api/core"
	v1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/version"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MockClusterInstance implements a simple mock for types.ClusterInstance
type MockClusterInstance struct {
	*v1alpha1.Cluster
	valkeyVer version.ValkeyVersion
	gvk       schema.GroupVersionKind
}

// NewMockClusterInstance creates a new instance of MockClusterInstance
func NewMockClusterInstance(name, namespace string, customConfigs map[string]string,
	resources corev1.ResourceRequirements, valkeyVer version.ValkeyVersion) *MockClusterInstance {

	// Create a mock with gvk information for owner references
	return &MockClusterInstance{
		Cluster: &v1alpha1.Cluster{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Cluster",
				APIVersion: "valkey.chideat.com/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				UID:       k8stypes.UID(name + "-uid"), // Simple mock UID
			},
			Spec: v1alpha1.ClusterSpec{
				CustomConfigs: customConfigs,
				Resources:     resources,
			},
		},
		valkeyVer: valkeyVer,
		gvk: schema.GroupVersionKind{
			Group:   "valkey.chideat.com",
			Version: "v1alpha1",
			Kind:    "Cluster",
		},
	}
}

// GetObjectKind returns the correct GroupVersionKind for owner references
func (m *MockClusterInstance) GetObjectKind() schema.ObjectKind {
	return m
}

// GroupVersionKind implements the ObjectKind interface
func (m *MockClusterInstance) GroupVersionKind() schema.GroupVersionKind {
	return m.gvk
}

// SetGroupVersionKind implements the ObjectKind interface
func (m *MockClusterInstance) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	m.gvk = gvk
}

func (m *MockClusterInstance) Definition() *v1alpha1.Cluster {
	return m.Cluster
}

func (m *MockClusterInstance) Version() version.ValkeyVersion {
	return m.valkeyVer
}

// Arch returns the architecture of the cluster
func (m *MockClusterInstance) Arch() core.Arch {
	return core.ValkeyCluster
}

// Status returns a nil status as it's not needed for the tests
func (m *MockClusterInstance) Status() *v1alpha1.ClusterStatus {
	return nil
}

// The following methods implement the remaining required interface methods
// with minimal functionality needed for the tests
func (m *MockClusterInstance) NamespacedName() client.ObjectKey {
	return client.ObjectKey{Name: m.GetName(), Namespace: m.GetNamespace()}
}

func (m *MockClusterInstance) IsReady() bool {
	return true
}

func (m *MockClusterInstance) Restart(ctx context.Context, annotationKeyVal ...string) error {
	return nil
}

func (m *MockClusterInstance) Refresh(ctx context.Context) error {
	return nil
}

func (m *MockClusterInstance) Issuer() *certmetav1.ObjectReference {
	return nil
}

func (m *MockClusterInstance) Users() types.Users {
	return nil
}

func (m *MockClusterInstance) TLSConfig() *tls.Config {
	return nil
}

func (m *MockClusterInstance) IsInService() bool {
	return true
}

func (m *MockClusterInstance) IsACLUserExists() bool {
	return false
}

func (m *MockClusterInstance) IsACLAppliedToAll() bool {
	return false
}

func (m *MockClusterInstance) IsResourceFullfilled(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *MockClusterInstance) UpdateStatus(ctx context.Context, st types.InstanceStatus, message string) error {
	return nil
}

func (m *MockClusterInstance) SendEventf(eventtype, reason, messageFmt string, args ...any) {
}

func (m *MockClusterInstance) Logger() logr.Logger {
	return logr.Discard()
}

func (m *MockClusterInstance) Masters() []types.ValkeyNode {
	return nil
}

func (m *MockClusterInstance) Nodes() []types.ValkeyNode {
	return nil
}

func (m *MockClusterInstance) RawNodes(ctx context.Context) ([]corev1.Pod, error) {
	return nil, nil
}

func (m *MockClusterInstance) Shards() []types.ClusterShard {
	return nil
}

func (m *MockClusterInstance) RewriteShards(ctx context.Context, shards []*v1alpha1.ClusterShards) error {
	return nil
}

// TestValkeyConfigMapName tests the ValkeyConfigMapName function
func TestValkeyConfigMapName(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		expected    string
	}{
		{
			name:        "Basic test",
			clusterName: "test-cluster",
			expected:    "valkey-cluster-test-cluster",
		},
		{
			name:        "Empty string",
			clusterName: "",
			expected:    "valkey-cluster-",
		},
		{
			name:        "Special characters",
			clusterName: "test-cluster-123!@#",
			expected:    "valkey-cluster-test-cluster-123!@#",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValkeyConfigMapName(tt.clusterName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// testBuildValkeyConfigs is a test helper function that works with simpler inputs
func testBuildValkeyConfigs(t *testing.T, name, namespace string, customConfigs map[string]string,
	resources corev1.ResourceRequirements, valkeyVer version.ValkeyVersion) (string, error) {

	mockInstance := NewMockClusterInstance(name, namespace, customConfigs, resources, valkeyVer)
	return buildValkeyConfigs(mockInstance)
}

// testNewConfigMapForCR is a test helper function that works with simpler inputs
func testNewConfigMapForCR(t *testing.T, name, namespace string, customConfigs map[string]string,
	resources corev1.ResourceRequirements, valkeyVer version.ValkeyVersion) (*corev1.ConfigMap, error) {

	mockInstance := NewMockClusterInstance(name, namespace, customConfigs, resources, valkeyVer)
	return NewConfigMapForCR(mockInstance)
}

// TestBuildValkeyConfigs tests the buildValkeyConfigs function using the test helper
func TestBuildValkeyConfigs(t *testing.T) {
	tests := []struct {
		name           string
		clusterName    string
		namespace      string
		customConfigs  map[string]string
		resources      corev1.ResourceRequirements
		valkeyVer      version.ValkeyVersion
		expectedConfig []string
		notExpected    []string
		wantErr        bool
	}{
		{
			name:          "Basic configs",
			clusterName:   "test-cluster",
			namespace:     "default",
			customConfigs: map[string]string{},
			valkeyVer:     version.ValkeyVersion("8.0"),
			expectedConfig: []string{
				"save 60 10000",
				"save 300 100",
				"save 600 1",
				"ignore-warnings ARM64-COW-BUG",
				"cluster-allow-replica-migration no",
				"cluster-migration-barrier 10",
			},
			wantErr: false,
		},
		{
			name:        "With maxmemory and policy set",
			clusterName: "test-cluster",
			namespace:   "default",
			customConfigs: map[string]string{
				"maxmemory":        "1gb",
				"maxmemory-policy": "noeviction",
			},
			valkeyVer: version.ValkeyVersion("8.0"),
			expectedConfig: []string{
				"maxmemory 1gb",
				"maxmemory-policy noeviction",
			},
			wantErr: false,
		},
		{
			name:        "With resources but no explicit maxmemory",
			clusterName: "test-cluster",
			namespace:   "default",
			customConfigs: map[string]string{
				"maxmemory-policy": "noeviction",
			},
			resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
			},
			valkeyVer: version.ValkeyVersion("8.0"),
			expectedConfig: []string{
				"maxmemory ",
				"maxmemory-policy noeviction",
			},
			wantErr: false,
		},
		{
			name:        "With appendonly enabled",
			clusterName: "test-cluster",
			namespace:   "default",
			customConfigs: map[string]string{
				"appendonly": "yes",
			},
			valkeyVer: version.ValkeyVersion("8.0"),
			expectedConfig: []string{
				"appendonly yes",
			},
			notExpected: []string{
				"save ",
			},
			wantErr: false,
		},
		{
			name:        "With invalid client-output-buffer-limit",
			clusterName: "test-cluster",
			namespace:   "default",
			customConfigs: map[string]string{
				"client-output-buffer-limit": "invalid format",
			},
			valkeyVer: version.ValkeyVersion("8.0"),
			wantErr:   true,
		},
		{
			name:        "With valid client-output-buffer-limit",
			clusterName: "test-cluster",
			namespace:   "default",
			customConfigs: map[string]string{
				"client-output-buffer-limit": "normal 0 0 0 slave 256mb 64mb 60",
			},
			valkeyVer: version.ValkeyVersion("8.0"),
			expectedConfig: []string{
				"client-output-buffer-limit normal 0 0 0",
				"client-output-buffer-limit slave 256mb 64mb 60",
			},
			wantErr: false,
		},
		{
			name:        "With invalid save format",
			clusterName: "test-cluster",
			namespace:   "default",
			customConfigs: map[string]string{
				"save": "60 10000 invalid",
			},
			valkeyVer: version.ValkeyVersion("8.0"),
			wantErr:   true,
		},
		{
			name:        "With valid save format",
			clusterName: "test-cluster",
			namespace:   "default",
			customConfigs: map[string]string{
				"save": "60 10000 300 100",
			},
			valkeyVer: version.ValkeyVersion("8.0"),
			expectedConfig: []string{
				"save 60 10000",
				"save 300 100",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := testBuildValkeyConfigs(t, tt.clusterName, tt.namespace,
				tt.customConfigs, tt.resources, tt.valkeyVer)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, config)

			// Check that expected configs are present
			for _, expectedLine := range tt.expectedConfig {
				assert.True(t, strings.Contains(config, expectedLine),
					"Config should contain %q, but got: %s", expectedLine, config)
			}

			// Check that not expected configs are not present
			for _, notExpectedLine := range tt.notExpected {
				assert.False(t, strings.Contains(config, notExpectedLine),
					"Config should not contain %q, but got: %s", notExpectedLine, config)
			}
		})
	}
}

// TestNewConfigMapForCR tests the NewConfigMapForCR function using the test helper
func TestNewConfigMapForCR(t *testing.T) {
	tests := []struct {
		name          string
		clusterName   string
		namespace     string
		customConfigs map[string]string
		resources     corev1.ResourceRequirements
		valkeyVer     version.ValkeyVersion
		wantErr       bool
	}{
		{
			name:          "Basic cluster",
			clusterName:   "test-cluster",
			namespace:     "default",
			customConfigs: map[string]string{},
			valkeyVer:     version.ValkeyVersion("8.0"),
			wantErr:       false,
		},
		{
			name:        "Cluster with custom configs",
			clusterName: "test-cluster",
			namespace:   "default",
			customConfigs: map[string]string{
				"maxmemory":          "1gb",
				"maxmemory-policy":   "allkeys-lru",
				"appendonly":         "yes",
				"repl-diskless-sync": "no",
			},
			valkeyVer: version.ValkeyVersion("8.0"),
			wantErr:   false,
		},
		{
			name:          "Cluster with resources set",
			clusterName:   "test-cluster",
			namespace:     "default",
			customConfigs: map[string]string{},
			resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
			valkeyVer: version.ValkeyVersion("8.0"),
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configMap, err := testNewConfigMapForCR(t, tt.clusterName, tt.namespace,
				tt.customConfigs, tt.resources, tt.valkeyVer)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, configMap)

			// Check basic properties
			assert.Equal(t, ValkeyConfigMapName(tt.clusterName), configMap.Name)
			assert.Equal(t, tt.namespace, configMap.Namespace)

			// Check that the config content exists
			assert.Contains(t, configMap.Data, builder.ValkeyConfigKey)
			assert.NotEmpty(t, configMap.Data[builder.ValkeyConfigKey])

			// Check that labels are set
			assert.NotNil(t, configMap.Labels)
			assert.Equal(t, string(core.ValkeyCluster), configMap.Labels[builder.InstanceTypeLabelKey])
			assert.Equal(t, tt.clusterName, configMap.Labels[builder.InstanceNameLabelKey])
		})
	}
}
