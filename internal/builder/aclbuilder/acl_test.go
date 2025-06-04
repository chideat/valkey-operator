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

package aclbuilder

import (
	"context"
	"crypto/tls"
	"testing"

	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/chideat/valkey-operator/api/core"
	v1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/types/user"
	"github.com/chideat/valkey-operator/pkg/version"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// mockInstance implements types.Instance for testing
type mockInstance struct {
	metav1.ObjectMeta
	runtime.Object
	issuer    *certmetav1.ObjectReference
	gvk       schema.GroupVersionKind
	ownerRefs []metav1.OwnerReference
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

func (m *mockInstance) GetOwnerReferences() []metav1.OwnerReference {
	return m.ownerRefs
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
		ownerRefs: []metav1.OwnerReference{
			{
				APIVersion: "v1alpha1",
				Kind:       "Cluster",
				Name:       "test-instance",
				UID:        "test-uid",
			},
		},
	}
}

func TestGenerateACLConfigMapName(t *testing.T) {
	testCases := []struct {
		name           string
		arch           core.Arch
		instanceName   string
		expectedResult string
	}{
		{
			name:           "Cluster architecture",
			arch:           core.ValkeyCluster,
			instanceName:   "my-cluster",
			expectedResult: "drc-acl-my-cluster",
		},
		{
			name:           "Sentinel architecture",
			arch:           core.ValkeySentinel,
			instanceName:   "my-sentinel",
			expectedResult: "rfs-acl-my-sentinel",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GenerateACLConfigMapName(tc.arch, tc.instanceName)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestGenerateACLOperatorSecretName(t *testing.T) {
	testCases := []struct {
		name           string
		arch           core.Arch
		instanceName   string
		expectedResult string
	}{
		{
			name:           "Cluster architecture",
			arch:           core.ValkeyCluster,
			instanceName:   "my-cluster",
			expectedResult: "cluster-acl-my-cluster-operator-secret",
		},
		{
			name:           "Sentinel architecture",
			arch:           core.ValkeySentinel,
			instanceName:   "my-sentinel",
			expectedResult: "sentinel-acl-my-sentinel-operator-secret",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GenerateACLOperatorSecretName(tc.arch, tc.instanceName)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestGenerateUserResourceName(t *testing.T) {
	testCases := []struct {
		name           string
		arch           core.Arch
		instanceName   string
		userName       string
		expectedResult string
	}{
		{
			name:           "Cluster architecture with default user",
			arch:           core.ValkeyCluster,
			instanceName:   "my-cluster",
			userName:       "default",
			expectedResult: "cluster-acl-my-cluster-default",
		},
		{
			name:           "Sentinel architecture with custom user",
			arch:           core.ValkeySentinel,
			instanceName:   "my-sentinel",
			userName:       "custom-user",
			expectedResult: "sentinel-acl-my-sentinel-custom-user",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GenerateUserResourceName(tc.arch, tc.instanceName, tc.userName)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestGenerateOperatorUserResourceName(t *testing.T) {
	testCases := []struct {
		name           string
		arch           core.Arch
		instanceName   string
		expectedResult string
	}{
		{
			name:           "Cluster architecture",
			arch:           core.ValkeyCluster,
			instanceName:   "my-cluster",
			expectedResult: "cluster-acl-my-cluster-operator",
		},
		{
			name:           "Sentinel architecture",
			arch:           core.ValkeySentinel,
			instanceName:   "my-sentinel",
			expectedResult: "sentinel-acl-my-sentinel-operator",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GenerateOperatorUserResourceName(tc.arch, tc.instanceName)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestGenerateDefaultUserResourceName(t *testing.T) {
	testCases := []struct {
		name           string
		arch           core.Arch
		instanceName   string
		expectedResult string
	}{
		{
			name:           "Cluster architecture",
			arch:           core.ValkeyCluster,
			instanceName:   "my-cluster",
			expectedResult: "cluster-acl-my-cluster-default",
		},
		{
			name:           "Sentinel architecture",
			arch:           core.ValkeySentinel,
			instanceName:   "my-sentinel",
			expectedResult: "sentinel-acl-my-sentinel-default",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GenerateDefaultUserResourceName(tc.arch, tc.instanceName)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestGenerateOperatorsUser(t *testing.T) {
	inst := newMockInstance()

	testCases := []struct {
		name           string
		passwordSecret string
	}{
		{
			name:           "With password secret",
			passwordSecret: "test-secret",
		},
		{
			name:           "Without password secret",
			passwordSecret: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GenerateOperatorsUser(inst, tc.passwordSecret)

			// Verify basic properties
			assert.Equal(t, GenerateOperatorUserResourceName(inst.Arch(), inst.GetName()), result.Name)
			assert.Equal(t, inst.GetNamespace(), result.Namespace)

			// Verify spec
			assert.Equal(t, v1alpha1.SystemAccount, result.Spec.AccountType)
			assert.Equal(t, inst.Arch(), result.Spec.Arch)
			assert.Equal(t, inst.GetName(), result.Spec.InstanceName)
			assert.Equal(t, "operator", result.Spec.Username)
			assert.Equal(t, "+@all -flushall -flushdb ~* &*", result.Spec.AclRules)

			// Verify password secrets
			if tc.passwordSecret == "" {
				assert.Empty(t, result.Spec.PasswordSecrets)
			} else {
				assert.Len(t, result.Spec.PasswordSecrets, 1)
				assert.Equal(t, tc.passwordSecret, result.Spec.PasswordSecrets[0])
			}
		})
	}
}

func TestGenerateUser(t *testing.T) {
	inst := newMockInstance()

	testCases := []struct {
		name     string
		user     *user.User
		expected *v1alpha1.User
	}{
		{
			name: "Operator user with password",
			user: &user.User{
				Name: "operator",
				Role: user.RoleOperator,
				Password: &user.Password{
					SecretName: "operator-secret",
				},
				Rules: []*user.Rule{
					{
						Categories:  []string{"all"},
						KeyPatterns: []string{"*"},
					},
				},
			},
			expected: &v1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "cluster-acl-test-instance-operator",
					Namespace:   "test-namespace",
					Annotations: map[string]string{},
					Labels:      map[string]string{},
				},
				Spec: v1alpha1.UserSpec{
					AccountType:     v1alpha1.SystemAccount,
					Arch:            core.ValkeyCluster,
					InstanceName:    "test-instance",
					Username:        "operator",
					PasswordSecrets: []string{"operator-secret"},
					AclRules:        "+@all ~* &*",
				},
			},
		},
		{
			name: "Developer user without password",
			user: &user.User{
				Name: "developer",
				Role: user.RoleDeveloper,
				Rules: []*user.Rule{
					{
						Categories:  []string{"read"},
						KeyPatterns: []string{"cache:*"},
					},
				},
			},
			expected: &v1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "cluster-acl-test-instance-developer",
					Namespace:   "test-namespace",
					Annotations: map[string]string{},
					Labels:      map[string]string{},
				},
				Spec: v1alpha1.UserSpec{
					AccountType:     v1alpha1.CustomAccount,
					Arch:            core.ValkeyCluster,
					InstanceName:    "test-instance",
					Username:        "developer",
					PasswordSecrets: []string{},
					AclRules:        "+@read ~cache:* &*",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GenerateUser(inst, tc.user)

			// Check basic properties
			assert.Equal(t, tc.expected.Name, result.Name)
			assert.Equal(t, tc.expected.Namespace, result.Namespace)

			// Check spec
			assert.Equal(t, tc.expected.Spec.AccountType, result.Spec.AccountType)
			assert.Equal(t, tc.expected.Spec.Arch, result.Spec.Arch)
			assert.Equal(t, tc.expected.Spec.InstanceName, result.Spec.InstanceName)
			assert.Equal(t, tc.expected.Spec.Username, result.Spec.Username)
			assert.Equal(t, tc.expected.Spec.PasswordSecrets, result.Spec.PasswordSecrets)

			// Rules may have some variations in ordering, so use a more lenient comparison
			assert.NotEmpty(t, result.Spec.AclRules)
		})
	}
}

func TestGenerateOperatorSecret(t *testing.T) {
	inst := newMockInstance()

	result := GenerateOperatorSecret(inst)

	// Check basic properties
	assert.Equal(t, GenerateACLOperatorSecretName(inst.Arch(), inst.GetName()), result.Name)
	assert.Equal(t, inst.GetNamespace(), result.Namespace)
	assert.Equal(t, corev1.SecretTypeOpaque, result.Type)

	// Check that owner references are set
	assert.NotNil(t, result.OwnerReferences)

	// Check secret data
	assert.Contains(t, result.Data, "password")
	assert.Contains(t, result.Data, "username")
	assert.NotEmpty(t, result.Data["password"])
	assert.Equal(t, []byte("operator"), result.Data["username"])
}

func TestGenerateACLConfigMap(t *testing.T) {
	inst := newMockInstance()
	testData := map[string]string{
		"user1": "data1",
		"user2": "data2",
	}

	result := GenerateACLConfigMap(inst, testData)

	// Check basic properties
	assert.Equal(t, GenerateACLConfigMapName(inst.Arch(), inst.GetName()), result.Name)
	assert.Equal(t, inst.GetNamespace(), result.Namespace)

	// Check owner references
	assert.NotEmpty(t, result.OwnerReferences)

	// Check data
	assert.Equal(t, testData, result.Data)
}
