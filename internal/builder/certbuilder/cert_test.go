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

package certbuilder

import (
	"context"
	"crypto/tls"
	"testing"
	"time"

	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/version"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// mockInstance implements types.Instance for testing
type mockInstance struct {
	metav1.ObjectMeta
	runtime.Object
	issuer *certmetav1.ObjectReference
	gvk    schema.GroupVersionKind
}

func (m *mockInstance) GetObjectKind() schema.ObjectKind {
	return &mockObjectKind{gvk: m.gvk}
}

// mockObjectKind implements schema.ObjectKind interface
type mockObjectKind struct {
	gvk schema.GroupVersionKind
}

func (m *mockObjectKind) GroupVersionKind() schema.GroupVersionKind {
	return m.gvk
}

func (m *mockObjectKind) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	m.gvk = gvk
}

func (m *mockInstance) DeepCopyObject() runtime.Object {
	return m
}

func (m *mockInstance) NamespacedName() client.ObjectKey {
	return client.ObjectKey{
		Namespace: m.GetNamespace(),
		Name:      m.GetName(),
	}
}

func (m *mockInstance) Version() version.ValkeyVersion {
	return version.DefaultValKeyVersion
}

func (m *mockInstance) IsReady() bool {
	return true
}

func (m *mockInstance) Restart(ctx context.Context, annotationKeyVal ...string) error {
	return nil
}

func (m *mockInstance) Refresh(ctx context.Context) error {
	return nil
}

func (m *mockInstance) Arch() core.Arch {
	return core.ValkeyCluster
}

func (m *mockInstance) Issuer() *certmetav1.ObjectReference {
	return m.issuer
}

func (m *mockInstance) Users() types.Users {
	return types.Users{}
}

func (m *mockInstance) TLSConfig() *tls.Config {
	return nil
}

func (m *mockInstance) IsInService() bool {
	return true
}

func (m *mockInstance) IsACLUserExists() bool {
	return true
}

func (m *mockInstance) IsACLAppliedToAll() bool {
	return true
}

func (m *mockInstance) IsResourceFullfilled(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *mockInstance) UpdateStatus(ctx context.Context, st types.InstanceStatus, message string) error {
	return nil
}

func (m *mockInstance) SendEventf(eventtype, reason, messageFmt string, args ...any) {
}

func (m *mockInstance) Logger() logr.Logger {
	return logr.Discard()
}

func newMockInstance() *mockInstance {
	return &mockInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-instance",
			Namespace: "test-namespace",
			UID:       "test-uid",
		},
		issuer: &certmetav1.ObjectReference{
			Name:  "test-issuer",
			Kind:  "ClusterIssuer",
			Group: "cert-manager.io",
		},
		gvk: schema.GroupVersionKind{
			Group:   "valkey.buf.red",
			Version: "v1alpha1",
			Kind:    "Cluster",
		},
	}
}

func TestGenerateCertName(t *testing.T) {
	name := "test-name"
	expected := "test-name-cert"

	result := GenerateCertName(name)

	assert.Equal(t, expected, result)
}

func TestGenerateSSLSecretName(t *testing.T) {
	name := "test-name"
	expected := "test-name-tls"

	result := GenerateSSLSecretName(name)

	assert.Equal(t, expected, result)
}

func TestGenerateServiceDNSName(t *testing.T) {
	serviceName := "test-service"
	namespace := "test-namespace"
	expected := "test-service.test-namespace"

	result := GenerateServiceDNSName(serviceName, namespace)

	assert.Equal(t, expected, result)
}

func TestGenerateHeadlessDNSName(t *testing.T) {
	podName := "test-pod"
	serviceName := "test-service"
	namespace := "test-namespace"
	expected := "test-pod.test-service.test-namespace"

	result := GenerateHeadlessDNSName(podName, serviceName, namespace)

	assert.Equal(t, expected, result)
}

func TestGenerateValkeyTLSOptions(t *testing.T) {
	expected := "--tls --cert /tls/tls.crt --key /tls/tls.key --cacert /tls/ca.crt"

	result := GenerateValkeyTLSOptions()

	assert.Equal(t, expected, result)
}

func TestNewCertificate(t *testing.T) {
	testCases := []struct {
		name        string
		instance    types.Instance
		dns         []string
		labels      map[string]string
		expectError bool
	}{
		{
			name:        "Valid instance with issuer",
			instance:    newMockInstance(),
			dns:         []string{"test.example.com", "test2.example.com"},
			labels:      map[string]string{"app": "valkey"},
			expectError: false,
		},
		{
			name: "Instance with nil issuer",
			instance: &mockInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-namespace",
				},
				issuer: nil,
			},
			dns:         []string{"test.example.com"},
			labels:      map[string]string{},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cert, err := NewCertificate(tc.instance, tc.dns, tc.labels)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, cert)
			} else {
				require.NoError(t, err)
				require.NotNil(t, cert)

				// Verify certificate properties
				assert.Equal(t, GenerateCertName(tc.instance.GetName()), cert.Name)
				assert.Equal(t, tc.instance.GetNamespace(), cert.Namespace)
				assert.Equal(t, tc.labels, cert.Labels)
				assert.NotEmpty(t, cert.OwnerReferences)

				// Verify certificate spec
				assert.Equal(t, tc.dns, cert.Spec.DNSNames)
				assert.Equal(t, *tc.instance.Issuer(), cert.Spec.IssuerRef)
				assert.Equal(t, GenerateSSLSecretName(tc.instance.GetName()), cert.Spec.SecretName)

				// Verify duration
				expectedDuration := 87600 * time.Hour
				assert.Equal(t, expectedDuration, cert.Spec.Duration.Duration)
			}
		})
	}
}
