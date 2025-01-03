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
package clientset

import (
	"context"

	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RBAC interface {
	GetRoleBinding(ctx context.Context, namespace string, name string) (*rbacv1.RoleBinding, error)
	CreateRoleBinding(ctx context.Context, namespace string, rb *rbacv1.RoleBinding) error
	CreateOrUpdateRoleBinding(ctx context.Context, namespace string, rb *rbacv1.RoleBinding) error
	GetRole(ctx context.Context, namespace string, name string) (*rbacv1.Role, error)
	CreateRole(ctx context.Context, namespace string, role *rbacv1.Role) error
	UpdateRole(ctx context.Context, namespace string, role *rbacv1.Role) error
	CreateOrUpdateRole(ctx context.Context, namespace string, role *rbacv1.Role) error
	GetClusterRole(ctx context.Context, name string) (*rbacv1.ClusterRole, error)
	CreateClusterRole(ctx context.Context, role *rbacv1.ClusterRole) error
	CreateOrUpdateClusterRole(ctx context.Context, role *rbacv1.ClusterRole) error
	UpdateClusterRole(ctx context.Context, role *rbacv1.ClusterRole) error
	CreateClusterRoleBinding(ctx context.Context, rb *rbacv1.ClusterRoleBinding) error
	GetClusterRoleBinding(ctx context.Context, name string) (*rbacv1.ClusterRoleBinding, error)
	UpdateClusterRoleBinding(ctx context.Context, role *rbacv1.ClusterRoleBinding) error
	CreateOrUpdateClusterRoleBinding(ctx context.Context, rb *rbacv1.ClusterRoleBinding) error
}

type RBACOption struct {
	client client.Client
	logger logr.Logger
}

func NewRBAC(kubeClient client.Client, logger logr.Logger) RBAC {
	logger = logger.WithValues("service", "k8s.rbac")
	return &RBACOption{
		client: kubeClient,
		logger: logger,
	}
}

func (s *RBACOption) GetClusterRole(ctx context.Context, name string) (*rbacv1.ClusterRole, error) {
	role := &rbacv1.ClusterRole{}
	err := s.client.Get(ctx, types.NamespacedName{
		Name: name,
	}, role)

	if err != nil {
		return nil, err
	}
	return role, err
}

func (s *RBACOption) UpdateClusterRole(ctx context.Context, role *rbacv1.ClusterRole) error {
	err := s.client.Update(ctx, role)
	if err != nil {
		return err
	}
	s.logger.WithValues("ClusterRoleName", role.Name).V(3).Info("clusterRole updated")
	return nil
}

func (s *RBACOption) CreateClusterRole(ctx context.Context, role *rbacv1.ClusterRole) error {
	err := s.client.Create(ctx, role)
	if err != nil {
		return err
	}
	s.logger.WithValues("ClusterRoleName", role.Name).V(3).Info("clusterRole created")
	return nil
}

func (s *RBACOption) CreateOrUpdateClusterRole(ctx context.Context, role *rbacv1.ClusterRole) error {
	if _, err := s.GetClusterRole(ctx, role.GetName()); errors.IsNotFound(err) {
		return s.CreateClusterRole(ctx, role)
	} else if err != nil {
		return err
	} else {
		return s.UpdateClusterRole(ctx, role)
	}
}

func (s *RBACOption) CreateOrUpdateClusterRoleBinding(ctx context.Context, rb *rbacv1.ClusterRoleBinding) error {

	if _, err := s.GetClusterRoleBinding(ctx, rb.GetName()); errors.IsNotFound(err) {
		return s.CreateClusterRoleBinding(ctx, rb)
	} else if err != nil {
		return err
	} else {
		return s.client.Update(ctx, rb)
	}
}

func (s *RBACOption) GetClusterRoleBinding(ctx context.Context, name string) (*rbacv1.ClusterRoleBinding, error) {
	rb := &rbacv1.ClusterRoleBinding{}
	err := s.client.Get(ctx, types.NamespacedName{
		Name: name,
	}, rb)

	if err != nil {
		return nil, err
	}
	return rb, err
}

func (s *RBACOption) CreateClusterRoleBinding(ctx context.Context, rb *rbacv1.ClusterRoleBinding) error {
	err := s.client.Create(ctx, rb)
	if err != nil {
		return err
	}
	s.logger.WithValues("ClusterRoleBindingName", rb.Name).V(3).Info("ClusterRoleBinding created")
	return nil
}

func (s *RBACOption) UpdateClusterRoleBinding(ctx context.Context, role *rbacv1.ClusterRoleBinding) error {
	err := s.client.Update(ctx, role)
	if err != nil {
		return err
	}
	s.logger.WithValues("ClusterRoleBindingName", role.Name).V(3).Info("clusterRoleBinding updated")
	return nil
}

func (s *RBACOption) GetRoleBinding(ctx context.Context, namespace string, name string) (*rbacv1.RoleBinding, error) {
	rb := &rbacv1.RoleBinding{}
	err := s.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, rb)

	if err != nil {
		return nil, err
	}
	return rb, err
}

func (s *RBACOption) CreateRoleBinding(ctx context.Context, namespace string, rb *rbacv1.RoleBinding) error {
	rb.Namespace = namespace
	if err := s.client.Create(ctx, rb); err != nil {
		return err
	}
	return nil
}

func (s *RBACOption) CreateOrUpdateRoleBinding(ctx context.Context, namespace string, rb *rbacv1.RoleBinding) error {
	rb.Namespace = namespace

	if _, err := s.GetRoleBinding(ctx, namespace, rb.GetName()); errors.IsNotFound(err) {
		return s.CreateRoleBinding(ctx, namespace, rb)
	} else if err != nil {
		return err
	} else {
		return s.client.Update(ctx, rb)
	}
}

func (s *RBACOption) GetRole(ctx context.Context, namespace string, name string) (*rbacv1.Role, error) {
	role := &rbacv1.Role{}
	err := s.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, role)

	if err != nil {
		return nil, err
	}
	return role, err
}

func (s *RBACOption) CreateRole(ctx context.Context, namespace string, role *rbacv1.Role) error {
	err := s.client.Create(ctx, role)
	if err != nil {
		return err
	}
	s.logger.WithValues("namespace", namespace, "roleName", role.Name).V(3).Info("role created")
	return nil
}

func (s *RBACOption) UpdateRole(ctx context.Context, namespace string, role *rbacv1.Role) error {
	err := s.client.Update(ctx, role)
	if err != nil {
		return err
	}
	s.logger.WithValues("namespace", namespace, "roleName", role.Name).V(3).Info("role updated")
	return nil
}

func (s *RBACOption) CreateOrUpdateRole(ctx context.Context, namespace string, role *rbacv1.Role) error {
	if _, err := s.GetRole(ctx, namespace, role.GetName()); errors.IsNotFound(err) {
		return s.CreateRole(ctx, namespace, role)
	} else if err != nil {
		return err
	} else {
		return s.UpdateRole(ctx, namespace, role)
	}
}
