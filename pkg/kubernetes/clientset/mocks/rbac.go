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

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	v1 "k8s.io/api/rbac/v1"
)

// RBAC is an autogenerated mock type for the RBAC type
type RBAC struct {
	mock.Mock
}

// CreateClusterRole provides a mock function with given fields: ctx, role
func (_m *RBAC) CreateClusterRole(ctx context.Context, role *v1.ClusterRole) error {
	ret := _m.Called(ctx, role)

	if len(ret) == 0 {
		panic("no return value specified for CreateClusterRole")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.ClusterRole) error); ok {
		r0 = rf(ctx, role)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateClusterRoleBinding provides a mock function with given fields: ctx, rb
func (_m *RBAC) CreateClusterRoleBinding(ctx context.Context, rb *v1.ClusterRoleBinding) error {
	ret := _m.Called(ctx, rb)

	if len(ret) == 0 {
		panic("no return value specified for CreateClusterRoleBinding")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.ClusterRoleBinding) error); ok {
		r0 = rf(ctx, rb)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateOrUpdateClusterRole provides a mock function with given fields: ctx, role
func (_m *RBAC) CreateOrUpdateClusterRole(ctx context.Context, role *v1.ClusterRole) error {
	ret := _m.Called(ctx, role)

	if len(ret) == 0 {
		panic("no return value specified for CreateOrUpdateClusterRole")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.ClusterRole) error); ok {
		r0 = rf(ctx, role)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateOrUpdateClusterRoleBinding provides a mock function with given fields: ctx, rb
func (_m *RBAC) CreateOrUpdateClusterRoleBinding(ctx context.Context, rb *v1.ClusterRoleBinding) error {
	ret := _m.Called(ctx, rb)

	if len(ret) == 0 {
		panic("no return value specified for CreateOrUpdateClusterRoleBinding")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.ClusterRoleBinding) error); ok {
		r0 = rf(ctx, rb)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateOrUpdateRole provides a mock function with given fields: ctx, namespace, role
func (_m *RBAC) CreateOrUpdateRole(ctx context.Context, namespace string, role *v1.Role) error {
	ret := _m.Called(ctx, namespace, role)

	if len(ret) == 0 {
		panic("no return value specified for CreateOrUpdateRole")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1.Role) error); ok {
		r0 = rf(ctx, namespace, role)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateOrUpdateRoleBinding provides a mock function with given fields: ctx, namespace, rb
func (_m *RBAC) CreateOrUpdateRoleBinding(ctx context.Context, namespace string, rb *v1.RoleBinding) error {
	ret := _m.Called(ctx, namespace, rb)

	if len(ret) == 0 {
		panic("no return value specified for CreateOrUpdateRoleBinding")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1.RoleBinding) error); ok {
		r0 = rf(ctx, namespace, rb)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateRole provides a mock function with given fields: ctx, namespace, role
func (_m *RBAC) CreateRole(ctx context.Context, namespace string, role *v1.Role) error {
	ret := _m.Called(ctx, namespace, role)

	if len(ret) == 0 {
		panic("no return value specified for CreateRole")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1.Role) error); ok {
		r0 = rf(ctx, namespace, role)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateRoleBinding provides a mock function with given fields: ctx, namespace, rb
func (_m *RBAC) CreateRoleBinding(ctx context.Context, namespace string, rb *v1.RoleBinding) error {
	ret := _m.Called(ctx, namespace, rb)

	if len(ret) == 0 {
		panic("no return value specified for CreateRoleBinding")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1.RoleBinding) error); ok {
		r0 = rf(ctx, namespace, rb)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetClusterRole provides a mock function with given fields: ctx, name
func (_m *RBAC) GetClusterRole(ctx context.Context, name string) (*v1.ClusterRole, error) {
	ret := _m.Called(ctx, name)

	if len(ret) == 0 {
		panic("no return value specified for GetClusterRole")
	}

	var r0 *v1.ClusterRole
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*v1.ClusterRole, error)); ok {
		return rf(ctx, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *v1.ClusterRole); ok {
		r0 = rf(ctx, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.ClusterRole)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetClusterRoleBinding provides a mock function with given fields: ctx, name
func (_m *RBAC) GetClusterRoleBinding(ctx context.Context, name string) (*v1.ClusterRoleBinding, error) {
	ret := _m.Called(ctx, name)

	if len(ret) == 0 {
		panic("no return value specified for GetClusterRoleBinding")
	}

	var r0 *v1.ClusterRoleBinding
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*v1.ClusterRoleBinding, error)); ok {
		return rf(ctx, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *v1.ClusterRoleBinding); ok {
		r0 = rf(ctx, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.ClusterRoleBinding)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetRole provides a mock function with given fields: ctx, namespace, name
func (_m *RBAC) GetRole(ctx context.Context, namespace string, name string) (*v1.Role, error) {
	ret := _m.Called(ctx, namespace, name)

	if len(ret) == 0 {
		panic("no return value specified for GetRole")
	}

	var r0 *v1.Role
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*v1.Role, error)); ok {
		return rf(ctx, namespace, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *v1.Role); ok {
		r0 = rf(ctx, namespace, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.Role)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, namespace, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetRoleBinding provides a mock function with given fields: ctx, namespace, name
func (_m *RBAC) GetRoleBinding(ctx context.Context, namespace string, name string) (*v1.RoleBinding, error) {
	ret := _m.Called(ctx, namespace, name)

	if len(ret) == 0 {
		panic("no return value specified for GetRoleBinding")
	}

	var r0 *v1.RoleBinding
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*v1.RoleBinding, error)); ok {
		return rf(ctx, namespace, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *v1.RoleBinding); ok {
		r0 = rf(ctx, namespace, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.RoleBinding)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, namespace, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateClusterRole provides a mock function with given fields: ctx, role
func (_m *RBAC) UpdateClusterRole(ctx context.Context, role *v1.ClusterRole) error {
	ret := _m.Called(ctx, role)

	if len(ret) == 0 {
		panic("no return value specified for UpdateClusterRole")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.ClusterRole) error); ok {
		r0 = rf(ctx, role)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateClusterRoleBinding provides a mock function with given fields: ctx, role
func (_m *RBAC) UpdateClusterRoleBinding(ctx context.Context, role *v1.ClusterRoleBinding) error {
	ret := _m.Called(ctx, role)

	if len(ret) == 0 {
		panic("no return value specified for UpdateClusterRoleBinding")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.ClusterRoleBinding) error); ok {
		r0 = rf(ctx, role)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateRole provides a mock function with given fields: ctx, namespace, role
func (_m *RBAC) UpdateRole(ctx context.Context, namespace string, role *v1.Role) error {
	ret := _m.Called(ctx, namespace, role)

	if len(ret) == 0 {
		panic("no return value specified for UpdateRole")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1.Role) error); ok {
		r0 = rf(ctx, namespace, role)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewRBAC creates a new instance of RBAC. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewRBAC(t interface {
	mock.TestingT
	Cleanup(func())
}) *RBAC {
	mock := &RBAC{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
