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
	"fmt"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ValkeyInstanceServiceAccountName = "valkey-instance-account"
	ValkeyInstanceRoleName           = "valkey-instance-role"
	ValkeyInstanceRoleBindingName    = "valkey-instance-rolebinding"
)

// NewServiceAccount
func NewServiceAccount(obj client.Object) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ValkeyInstanceServiceAccountName,
			Namespace: obj.GetNamespace(),
		},
	}
}

// NewRole
func NewRole(obj client.Object) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ValkeyInstanceRoleName,
			Namespace: obj.GetNamespace(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{"secrets", "configmaps", "services"},
				Verbs:     []string{"get", "list", "create", "update", "watch"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{"pods", "pods/exec"},
				Verbs:     []string{"create", "get", "list", "watch", "patch"},
			},
			{
				APIGroups: []string{appv1.GroupName},
				Resources: []string{"statefulsets"},
				Verbs:     []string{"get"},
			},
		},
	}

}

// NewRoleBinding
func NewRoleBinding(obj client.Object) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ValkeyInstanceRoleBindingName,
			Namespace: obj.GetNamespace(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     ValkeyInstanceRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      ValkeyInstanceServiceAccountName,
				Namespace: obj.GetNamespace(),
			},
		},
	}
}

func NewClusterRole(obj client.Object) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: ValkeyInstanceRoleName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{"nodes"},
				Verbs:     []string{"get"},
			},
		},
	}
}

func NewClusterRoleBinding(obj client.Object) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", obj.GetNamespace(), ValkeyInstanceRoleBindingName),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     ValkeyInstanceRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      ValkeyInstanceServiceAccountName,
				Namespace: obj.GetNamespace(),
			},
		},
	}
}
