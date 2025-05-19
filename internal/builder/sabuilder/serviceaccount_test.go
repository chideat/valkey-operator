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

package sabuilder

import (
	"testing"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/stretchr/testify/assert"
)

// mockObject implements client.Object for testing
type mockObject struct {
	metav1.ObjectMeta
	runtime.Object
}

func (m *mockObject) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (m *mockObject) DeepCopyObject() runtime.Object {
	return m
}

func TestGenerateServiceAccount(t *testing.T) {
	mockObj := &mockObject{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
		},
	}

	serviceAccount := GenerateServiceAccount(mockObj)

	assert.Equal(t, ValkeyInstanceServiceAccountName, serviceAccount.Name)
	assert.Equal(t, "test-namespace", serviceAccount.Namespace)
}

func TestGenerateRole(t *testing.T) {
	mockObj := &mockObject{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
		},
	}

	role := GenerateRole(mockObj)

	assert.Equal(t, ValkeyInstanceRoleName, role.Name)
	assert.Equal(t, "test-namespace", role.Namespace)
	assert.Len(t, role.Rules, 3)

	// Check first rule for secrets, configmaps, services
	assert.Contains(t, role.Rules[0].APIGroups, corev1.GroupName)
	assert.ElementsMatch(t, role.Rules[0].Resources, []string{"secrets", "configmaps", "services"})
	assert.ElementsMatch(t, role.Rules[0].Verbs, []string{"get", "list", "create", "update", "watch"})

	// Check second rule for pods, pods/exec
	assert.Contains(t, role.Rules[1].APIGroups, corev1.GroupName)
	assert.ElementsMatch(t, role.Rules[1].Resources, []string{"pods", "pods/exec"})
	assert.ElementsMatch(t, role.Rules[1].Verbs, []string{"create", "get", "list", "watch", "patch"})

	// Check third rule for statefulsets
	assert.Contains(t, role.Rules[2].APIGroups, appv1.GroupName)
	assert.ElementsMatch(t, role.Rules[2].Resources, []string{"statefulsets"})
	assert.ElementsMatch(t, role.Rules[2].Verbs, []string{"get"})
}

func TestGenerateRoleBinding(t *testing.T) {
	mockObj := &mockObject{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
		},
	}

	roleBinding := GenerateRoleBinding(mockObj)

	assert.Equal(t, ValkeyInstanceRoleBindingName, roleBinding.Name)
	assert.Equal(t, "test-namespace", roleBinding.Namespace)
	assert.Equal(t, rbacv1.GroupName, roleBinding.RoleRef.APIGroup)
	assert.Equal(t, "Role", roleBinding.RoleRef.Kind)
	assert.Equal(t, ValkeyInstanceRoleName, roleBinding.RoleRef.Name)
	assert.Len(t, roleBinding.Subjects, 1)
	assert.Equal(t, "ServiceAccount", roleBinding.Subjects[0].Kind)
	assert.Equal(t, ValkeyInstanceServiceAccountName, roleBinding.Subjects[0].Name)
	assert.Equal(t, "test-namespace", roleBinding.Subjects[0].Namespace)
}

func TestGenerateClusterRole(t *testing.T) {
	mockObj := &mockObject{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
		},
	}

	clusterRole := GenerateClusterRole(mockObj)

	assert.Equal(t, ValkeyInstanceRoleName, clusterRole.Name)
	assert.Len(t, clusterRole.Rules, 1)
	assert.Contains(t, clusterRole.Rules[0].APIGroups, corev1.GroupName)
	assert.ElementsMatch(t, clusterRole.Rules[0].Resources, []string{"nodes"})
	assert.ElementsMatch(t, clusterRole.Rules[0].Verbs, []string{"get"})
}

func TestGenerateClusterRoleBinding(t *testing.T) {
	mockObj := &mockObject{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
		},
	}

	clusterRoleBinding := GenerateClusterRoleBinding(mockObj)

	expectedName := "test-namespace-" + ValkeyInstanceRoleBindingName
	assert.Equal(t, expectedName, clusterRoleBinding.Name)
	assert.Equal(t, rbacv1.GroupName, clusterRoleBinding.RoleRef.APIGroup)
	assert.Equal(t, "ClusterRole", clusterRoleBinding.RoleRef.Kind)
	assert.Equal(t, ValkeyInstanceRoleName, clusterRoleBinding.RoleRef.Name)
	assert.Len(t, clusterRoleBinding.Subjects, 1)
	assert.Equal(t, "ServiceAccount", clusterRoleBinding.Subjects[0].Kind)
	assert.Equal(t, ValkeyInstanceServiceAccountName, clusterRoleBinding.Subjects[0].Name)
	assert.Equal(t, "test-namespace", clusterRoleBinding.Subjects[0].Namespace)
}
