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
*/ // Code generated by mockery v2.45.0. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
)

// NameSpaces is an autogenerated mock type for the NameSpaces type
type NameSpaces struct {
	mock.Mock
}

// GetNameSpace provides a mock function with given fields: ctx, namespace
func (_m *NameSpaces) GetNameSpace(ctx context.Context, namespace string) (*v1.Namespace, error) {
	ret := _m.Called(ctx, namespace)

	if len(ret) == 0 {
		panic("no return value specified for GetNameSpace")
	}

	var r0 *v1.Namespace
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*v1.Namespace, error)); ok {
		return rf(ctx, namespace)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *v1.Namespace); ok {
		r0 = rf(ctx, namespace)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.Namespace)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, namespace)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewNameSpaces creates a new instance of NameSpaces. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewNameSpaces(t interface {
	mock.TestingT
	Cleanup(func())
}) *NameSpaces {
	mock := &NameSpaces{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
