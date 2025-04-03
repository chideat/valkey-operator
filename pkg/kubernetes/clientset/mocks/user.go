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
	client "sigs.k8s.io/controller-runtime/pkg/client"

	context "context"

	mock "github.com/stretchr/testify/mock"

	v1 "github.com/chideat/valkey-operator/api/v1alpha1"
)

// User is an autogenerated mock type for the User type
type User struct {
	mock.Mock
}

// CreateIfNotExistsUser provides a mock function with given fields: ctx, ru
func (_m *User) CreateIfNotExistsUser(ctx context.Context, ru *v1.User) error {
	ret := _m.Called(ctx, ru)

	if len(ret) == 0 {
		panic("no return value specified for CreateIfNotExistsUser")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.User) error); ok {
		r0 = rf(ctx, ru)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateOrUpdateUser provides a mock function with given fields: ctx, ru
func (_m *User) CreateOrUpdateUser(ctx context.Context, ru *v1.User) error {
	ret := _m.Called(ctx, ru)

	if len(ret) == 0 {
		panic("no return value specified for CreateOrUpdateUser")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.User) error); ok {
		r0 = rf(ctx, ru)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateUser provides a mock function with given fields: ctx, ru
func (_m *User) CreateUser(ctx context.Context, ru *v1.User) error {
	ret := _m.Called(ctx, ru)

	if len(ret) == 0 {
		panic("no return value specified for CreateUser")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.User) error); ok {
		r0 = rf(ctx, ru)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetUser provides a mock function with given fields: ctx, namespace, name
func (_m *User) GetUser(ctx context.Context, namespace string, name string) (*v1.User, error) {
	ret := _m.Called(ctx, namespace, name)

	if len(ret) == 0 {
		panic("no return value specified for GetUser")
	}

	var r0 *v1.User
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*v1.User, error)); ok {
		return rf(ctx, namespace, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *v1.User); ok {
		r0 = rf(ctx, namespace, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.User)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, namespace, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListUsers provides a mock function with given fields: ctx, namespace, opts
func (_m *User) ListUsers(ctx context.Context, namespace string, opts client.ListOptions) (*v1.UserList, error) {
	ret := _m.Called(ctx, namespace, opts)

	if len(ret) == 0 {
		panic("no return value specified for ListUsers")
	}

	var r0 *v1.UserList
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, client.ListOptions) (*v1.UserList, error)); ok {
		return rf(ctx, namespace, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, client.ListOptions) *v1.UserList); ok {
		r0 = rf(ctx, namespace, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.UserList)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, client.ListOptions) error); ok {
		r1 = rf(ctx, namespace, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateUser provides a mock function with given fields: ctx, ru
func (_m *User) UpdateUser(ctx context.Context, ru *v1.User) error {
	ret := _m.Called(ctx, ru)

	if len(ret) == 0 {
		panic("no return value specified for UpdateUser")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.User) error); ok {
		r0 = rf(ctx, ru)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewUser creates a new instance of User. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewUser(t interface {
	mock.TestingT
	Cleanup(func())
}) *User {
	mock := &User{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
