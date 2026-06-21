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

package failoverbuilder

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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MockFailoverInstance is a minimal mock of types.FailoverInstance. Only
// Definition() and SafeVersion() carry real behaviour; the rest are stubs since
// GenerateConfigMap only consumes those two.
type MockFailoverInstance struct {
	*v1alpha1.Failover
	valkeyVer version.ValkeyVersion
}

func newMockFailoverInstance(rf *v1alpha1.Failover, ver version.ValkeyVersion) *MockFailoverInstance {
	return &MockFailoverInstance{Failover: rf, valkeyVer: ver}
}

func (m *MockFailoverInstance) Definition() *v1alpha1.Failover     { return m.Failover }
func (m *MockFailoverInstance) Version() version.ValkeyVersion     { return m.valkeyVer }
func (m *MockFailoverInstance) SafeVersion() version.ValkeyVersion { return m.valkeyVer }
func (m *MockFailoverInstance) Arch() core.Arch                    { return core.ValkeyFailover }
func (m *MockFailoverInstance) NamespacedName() client.ObjectKey {
	return client.ObjectKey{Name: m.GetName(), Namespace: m.GetNamespace()}
}
func (m *MockFailoverInstance) IsReady() bool { return true }
func (m *MockFailoverInstance) Restart(ctx context.Context, annotationKeyVal ...string) error {
	return nil
}
func (m *MockFailoverInstance) Refresh(ctx context.Context) error   { return nil }
func (m *MockFailoverInstance) Issuer() *certmetav1.ObjectReference { return nil }
func (m *MockFailoverInstance) Users() types.Users                  { return nil }
func (m *MockFailoverInstance) TLSConfig() *tls.Config              { return nil }
func (m *MockFailoverInstance) IsInService() bool                   { return true }
func (m *MockFailoverInstance) IsACLUserExists() bool               { return false }
func (m *MockFailoverInstance) IsACLAppliedToAll() bool             { return false }
func (m *MockFailoverInstance) IsResourceFullfilled(ctx context.Context) (bool, error) {
	return true, nil
}
func (m *MockFailoverInstance) UpdateStatus(ctx context.Context, st types.InstanceStatus, message string) error {
	return nil
}
func (m *MockFailoverInstance) SendEventf(eventtype, reason, messageFmt string, args ...any) {}
func (m *MockFailoverInstance) Logger() logr.Logger                                          { return logr.Discard() }
func (m *MockFailoverInstance) Replication() types.Replication                               { return nil }
func (m *MockFailoverInstance) Masters() []types.ValkeyNode                                  { return nil }
func (m *MockFailoverInstance) Nodes() []types.ValkeyNode                                    { return nil }
func (m *MockFailoverInstance) RawNodes(ctx context.Context) ([]corev1.Pod, error) {
	return nil, nil
}
func (m *MockFailoverInstance) Monitor() types.FailoverMonitor { return nil }
func (m *MockFailoverInstance) IsBindedSentinel() bool         { return false }
func (m *MockFailoverInstance) IsStandalone() bool             { return false }
func (m *MockFailoverInstance) Selector() map[string]string    { return nil }

var _ types.FailoverInstance = (*MockFailoverInstance)(nil)

func newFailoverWithResources(resources corev1.ResourceRequirements) *v1alpha1.Failover {
	return &v1alpha1.Failover{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Failover",
			APIVersion: "valkey.buf.red/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rf-test",
			Namespace: "default",
			UID:       "rf-test-uid",
		},
		Spec: v1alpha1.FailoverSpec{
			Resources: resources,
		},
	}
}

func TestGenerateConfigMapMemorySizing(t *testing.T) {
	tests := []struct {
		name           string
		resources      corev1.ResourceRequirements
		expectedConfig []string
		notExpected    []string
	}{
		{
			// R5 regression: Limits has no memory, so sizing must fall back to
			// Requests.Memory(). The old `&&` guard never skipped the zero
			// Limits.Memory() and dropped maxmemory entirely.
			name: "Requests-only memory triggers maxmemory sizing",
			resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("500m"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
			expectedConfig: []string{"maxmemory ", "repl-backlog-size "},
		},
		{
			name: "Limits memory drives sizing",
			resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
			expectedConfig: []string{"maxmemory ", "repl-backlog-size "},
		},
		{
			name:        "No memory configured renders no sizing",
			resources:   corev1.ResourceRequirements{},
			notExpected: []string{"maxmemory ", "repl-backlog-size "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := newMockFailoverInstance(newFailoverWithResources(tt.resources), version.ValkeyVersion("8.0"))
			cm, err := GenerateConfigMap(inst)
			require.NoError(t, err)
			data := cm.Data[builder.ValkeyConfigKey]
			require.NotEmpty(t, data)

			for _, expected := range tt.expectedConfig {
				assert.True(t, strings.Contains(data, expected),
					"config should contain %q, got: %s", expected, data)
			}
			for _, notExpected := range tt.notExpected {
				assert.False(t, strings.Contains(data, notExpected),
					"config should not contain %q, got: %s", notExpected, data)
			}
		})
	}
}
