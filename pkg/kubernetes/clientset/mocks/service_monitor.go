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

	v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	mock "github.com/stretchr/testify/mock"
)

// ServiceMonitor is an autogenerated mock type for the ServiceMonitor type
type ServiceMonitor struct {
	mock.Mock
}

// CreateOrUpdateServiceMonitor provides a mock function with given fields: ctx, namespace, sm
func (_m *ServiceMonitor) CreateOrUpdateServiceMonitor(ctx context.Context, namespace string, sm *v1.ServiceMonitor) error {
	ret := _m.Called(ctx, namespace, sm)

	if len(ret) == 0 {
		panic("no return value specified for CreateOrUpdateServiceMonitor")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1.ServiceMonitor) error); ok {
		r0 = rf(ctx, namespace, sm)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateServiceMonitor provides a mock function with given fields: ctx, namespace, sm
func (_m *ServiceMonitor) CreateServiceMonitor(ctx context.Context, namespace string, sm *v1.ServiceMonitor) error {
	ret := _m.Called(ctx, namespace, sm)

	if len(ret) == 0 {
		panic("no return value specified for CreateServiceMonitor")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1.ServiceMonitor) error); ok {
		r0 = rf(ctx, namespace, sm)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetServiceMonitor provides a mock function with given fields: ctx, namespace, name
func (_m *ServiceMonitor) GetServiceMonitor(ctx context.Context, namespace string, name string) (*v1.ServiceMonitor, error) {
	ret := _m.Called(ctx, namespace, name)

	if len(ret) == 0 {
		panic("no return value specified for GetServiceMonitor")
	}

	var r0 *v1.ServiceMonitor
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*v1.ServiceMonitor, error)); ok {
		return rf(ctx, namespace, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *v1.ServiceMonitor); ok {
		r0 = rf(ctx, namespace, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.ServiceMonitor)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, namespace, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateServiceMonitor provides a mock function with given fields: ctx, namespace, sm
func (_m *ServiceMonitor) UpdateServiceMonitor(ctx context.Context, namespace string, sm *v1.ServiceMonitor) error {
	ret := _m.Called(ctx, namespace, sm)

	if len(ret) == 0 {
		panic("no return value specified for UpdateServiceMonitor")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1.ServiceMonitor) error); ok {
		r0 = rf(ctx, namespace, sm)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewServiceMonitor creates a new instance of ServiceMonitor. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewServiceMonitor(t interface {
	mock.TestingT
	Cleanup(func())
}) *ServiceMonitor {
	mock := &ServiceMonitor{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
