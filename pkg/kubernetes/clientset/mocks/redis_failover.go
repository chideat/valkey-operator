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
// Code generated by mockery v2.45.0. DO NOT EDIT.

package mocks

import (
	client "sigs.k8s.io/controller-runtime/pkg/client"

	context "context"

	mock "github.com/stretchr/testify/mock"

	v1 "github.com/chideat/valkey-operator/api/v1alpha1"
)

// Failover is an autogenerated mock type for the Failover type
type Failover struct {
	mock.Mock
}

// GetFailover provides a mock function with given fields: ctx, namespace, name
func (_m *Failover) GetFailover(ctx context.Context, namespace string, name string) (*v1.Failover, error) {
	ret := _m.Called(ctx, namespace, name)

	if len(ret) == 0 {
		panic("no return value specified for GetFailover")
	}

	var r0 *v1.Failover
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*v1.Failover, error)); ok {
		return rf(ctx, namespace, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *v1.Failover); ok {
		r0 = rf(ctx, namespace, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.Failover)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, namespace, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListFailovers provides a mock function with given fields: ctx, namespace, opts
func (_m *Failover) ListFailovers(ctx context.Context, namespace string, opts client.ListOptions) (*v1.FailoverList, error) {
	ret := _m.Called(ctx, namespace, opts)

	if len(ret) == 0 {
		panic("no return value specified for ListFailovers")
	}

	var r0 *v1.FailoverList
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, client.ListOptions) (*v1.FailoverList, error)); ok {
		return rf(ctx, namespace, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, client.ListOptions) *v1.FailoverList); ok {
		r0 = rf(ctx, namespace, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.FailoverList)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, client.ListOptions) error); ok {
		r1 = rf(ctx, namespace, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateFailover provides a mock function with given fields: ctx, inst
func (_m *Failover) UpdateFailover(ctx context.Context, inst *v1.Failover) error {
	ret := _m.Called(ctx, inst)

	if len(ret) == 0 {
		panic("no return value specified for UpdateFailover")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.Failover) error); ok {
		r0 = rf(ctx, inst)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateFailoverStatus provides a mock function with given fields: ctx, inst
func (_m *Failover) UpdateFailoverStatus(ctx context.Context, inst *v1.Failover) error {
	ret := _m.Called(ctx, inst)

	if len(ret) == 0 {
		panic("no return value specified for UpdateFailoverStatus")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.Failover) error); ok {
		r0 = rf(ctx, inst)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewFailover creates a new instance of Failover. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewFailover(t interface {
	mock.TestingT
	Cleanup(func())
}) *Failover {
	mock := &Failover{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
