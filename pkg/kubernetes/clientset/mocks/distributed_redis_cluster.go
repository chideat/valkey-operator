/*
Copyright 2023 The RedisOperator Authors.

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
	context "context"

	v1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
	mock "github.com/stretchr/testify/mock"
)

// Cluster is an autogenerated mock type for the Cluster type
type Cluster struct {
	mock.Mock
}

// GetCluster provides a mock function with given fields: ctx, namespace, name
func (_m *Cluster) GetCluster(ctx context.Context, namespace string, name string) (*v1alpha1.Cluster, error) {
	ret := _m.Called(ctx, namespace, name)

	if len(ret) == 0 {
		panic("no return value specified for GetCluster")
	}

	var r0 *v1alpha1.Cluster
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*v1alpha1.Cluster, error)); ok {
		return rf(ctx, namespace, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *v1alpha1.Cluster); ok {
		r0 = rf(ctx, namespace, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.Cluster)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, namespace, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateCluster provides a mock function with given fields: ctx, inst
func (_m *Cluster) UpdateCluster(ctx context.Context, inst *v1alpha1.Cluster) error {
	ret := _m.Called(ctx, inst)

	if len(ret) == 0 {
		panic("no return value specified for UpdateCluster")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.Cluster) error); ok {
		r0 = rf(ctx, inst)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateClusterStatus provides a mock function with given fields: ctx, inst
func (_m *Cluster) UpdateClusterStatus(ctx context.Context, inst *v1alpha1.Cluster) error {
	ret := _m.Called(ctx, inst)

	if len(ret) == 0 {
		panic("no return value specified for UpdateClusterStatus")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.Cluster) error); ok {
		r0 = rf(ctx, inst)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewCluster creates a new instance of Cluster. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewCluster(t interface {
	mock.TestingT
	Cleanup(func())
}) *Cluster {
	mock := &Cluster{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
