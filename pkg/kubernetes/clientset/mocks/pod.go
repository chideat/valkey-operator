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
	client "sigs.k8s.io/controller-runtime/pkg/client"

	context "context"

	io "io"

	mock "github.com/stretchr/testify/mock"

	v1 "k8s.io/api/core/v1"
)

// Pod is an autogenerated mock type for the Pod type
type Pod struct {
	mock.Mock
}

// CreateOrUpdatePod provides a mock function with given fields: ctx, namespace, pod
func (_m *Pod) CreateOrUpdatePod(ctx context.Context, namespace string, pod *v1.Pod) error {
	ret := _m.Called(ctx, namespace, pod)

	if len(ret) == 0 {
		panic("no return value specified for CreateOrUpdatePod")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1.Pod) error); ok {
		r0 = rf(ctx, namespace, pod)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreatePod provides a mock function with given fields: ctx, namespace, pod
func (_m *Pod) CreatePod(ctx context.Context, namespace string, pod *v1.Pod) error {
	ret := _m.Called(ctx, namespace, pod)

	if len(ret) == 0 {
		panic("no return value specified for CreatePod")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1.Pod) error); ok {
		r0 = rf(ctx, namespace, pod)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeletePod provides a mock function with given fields: ctx, namespace, name, opts
func (_m *Pod) DeletePod(ctx context.Context, namespace string, name string, opts ...client.DeleteOption) error {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, namespace, name)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for DeletePod")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, ...client.DeleteOption) error); ok {
		r0 = rf(ctx, namespace, name, opts...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Exec provides a mock function with given fields: ctx, namespace, name, containerName, cmd
func (_m *Pod) Exec(ctx context.Context, namespace string, name string, containerName string, cmd []string) (io.Reader, io.Reader, error) {
	ret := _m.Called(ctx, namespace, name, containerName, cmd)

	if len(ret) == 0 {
		panic("no return value specified for Exec")
	}

	var r0 io.Reader
	var r1 io.Reader
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, []string) (io.Reader, io.Reader, error)); ok {
		return rf(ctx, namespace, name, containerName, cmd)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, []string) io.Reader); ok {
		r0 = rf(ctx, namespace, name, containerName, cmd)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(io.Reader)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, string, []string) io.Reader); ok {
		r1 = rf(ctx, namespace, name, containerName, cmd)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(io.Reader)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, string, string, string, []string) error); ok {
		r2 = rf(ctx, namespace, name, containerName, cmd)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// GetPod provides a mock function with given fields: ctx, namespace, name
func (_m *Pod) GetPod(ctx context.Context, namespace string, name string) (*v1.Pod, error) {
	ret := _m.Called(ctx, namespace, name)

	if len(ret) == 0 {
		panic("no return value specified for GetPod")
	}

	var r0 *v1.Pod
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*v1.Pod, error)); ok {
		return rf(ctx, namespace, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *v1.Pod); ok {
		r0 = rf(ctx, namespace, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.Pod)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, namespace, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListPodByLabels provides a mock function with given fields: ctx, namespace, label_map
func (_m *Pod) ListPodByLabels(ctx context.Context, namespace string, label_map map[string]string) (*v1.PodList, error) {
	ret := _m.Called(ctx, namespace, label_map)

	if len(ret) == 0 {
		panic("no return value specified for ListPodByLabels")
	}

	var r0 *v1.PodList
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, map[string]string) (*v1.PodList, error)); ok {
		return rf(ctx, namespace, label_map)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, map[string]string) *v1.PodList); ok {
		r0 = rf(ctx, namespace, label_map)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.PodList)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, map[string]string) error); ok {
		r1 = rf(ctx, namespace, label_map)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListPods provides a mock function with given fields: ctx, namespace
func (_m *Pod) ListPods(ctx context.Context, namespace string) (*v1.PodList, error) {
	ret := _m.Called(ctx, namespace)

	if len(ret) == 0 {
		panic("no return value specified for ListPods")
	}

	var r0 *v1.PodList
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*v1.PodList, error)); ok {
		return rf(ctx, namespace)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *v1.PodList); ok {
		r0 = rf(ctx, namespace)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.PodList)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, namespace)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PatchPodLabel provides a mock function with given fields: ctx, pod, labelkey, labelValue
func (_m *Pod) PatchPodLabel(ctx context.Context, pod *v1.Pod, labelkey string, labelValue string) error {
	ret := _m.Called(ctx, pod, labelkey, labelValue)

	if len(ret) == 0 {
		panic("no return value specified for PatchPodLabel")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.Pod, string, string) error); ok {
		r0 = rf(ctx, pod, labelkey, labelValue)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdatePod provides a mock function with given fields: ctx, namespace, pod
func (_m *Pod) UpdatePod(ctx context.Context, namespace string, pod *v1.Pod) error {
	ret := _m.Called(ctx, namespace, pod)

	if len(ret) == 0 {
		panic("no return value specified for UpdatePod")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1.Pod) error); ok {
		r0 = rf(ctx, namespace, pod)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewPod creates a new instance of Pod. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewPod(t interface {
	mock.TestingT
	Cleanup(func())
}) *Pod {
	mock := &Pod{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
