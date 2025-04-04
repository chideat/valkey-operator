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

	corev1 "k8s.io/api/core/v1"

	mock "github.com/stretchr/testify/mock"

	v1 "k8s.io/api/apps/v1"
)

// StatefulSet is an autogenerated mock type for the StatefulSet type
type StatefulSet struct {
	mock.Mock
}

// CreateIfNotExistsStatefulSet provides a mock function with given fields: ctx, namespace, statefulSet
func (_m *StatefulSet) CreateIfNotExistsStatefulSet(ctx context.Context, namespace string, statefulSet *v1.StatefulSet) error {
	ret := _m.Called(ctx, namespace, statefulSet)

	if len(ret) == 0 {
		panic("no return value specified for CreateIfNotExistsStatefulSet")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1.StatefulSet) error); ok {
		r0 = rf(ctx, namespace, statefulSet)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateOrUpdateStatefulSet provides a mock function with given fields: ctx, namespace, StatefulSet
func (_m *StatefulSet) CreateOrUpdateStatefulSet(ctx context.Context, namespace string, StatefulSet *v1.StatefulSet) error {
	ret := _m.Called(ctx, namespace, StatefulSet)

	if len(ret) == 0 {
		panic("no return value specified for CreateOrUpdateStatefulSet")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1.StatefulSet) error); ok {
		r0 = rf(ctx, namespace, StatefulSet)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateStatefulSet provides a mock function with given fields: ctx, namespace, statefulSet
func (_m *StatefulSet) CreateStatefulSet(ctx context.Context, namespace string, statefulSet *v1.StatefulSet) error {
	ret := _m.Called(ctx, namespace, statefulSet)

	if len(ret) == 0 {
		panic("no return value specified for CreateStatefulSet")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1.StatefulSet) error); ok {
		r0 = rf(ctx, namespace, statefulSet)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteStatefulSet provides a mock function with given fields: ctx, namespace, name, opts
func (_m *StatefulSet) DeleteStatefulSet(ctx context.Context, namespace string, name string, opts ...client.DeleteOption) error {
	_va := make([]any, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []any
	_ca = append(_ca, ctx, namespace, name)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for DeleteStatefulSet")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, ...client.DeleteOption) error); ok {
		r0 = rf(ctx, namespace, name, opts...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetStatefulSet provides a mock function with given fields: ctx, namespace, name
func (_m *StatefulSet) GetStatefulSet(ctx context.Context, namespace string, name string) (*v1.StatefulSet, error) {
	ret := _m.Called(ctx, namespace, name)

	if len(ret) == 0 {
		panic("no return value specified for GetStatefulSet")
	}

	var r0 *v1.StatefulSet
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*v1.StatefulSet, error)); ok {
		return rf(ctx, namespace, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *v1.StatefulSet); ok {
		r0 = rf(ctx, namespace, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.StatefulSet)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, namespace, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetStatefulSetPods provides a mock function with given fields: ctx, namespace, name
func (_m *StatefulSet) GetStatefulSetPods(ctx context.Context, namespace string, name string) (*corev1.PodList, error) {
	ret := _m.Called(ctx, namespace, name)

	if len(ret) == 0 {
		panic("no return value specified for GetStatefulSetPods")
	}

	var r0 *corev1.PodList
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*corev1.PodList, error)); ok {
		return rf(ctx, namespace, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *corev1.PodList); ok {
		r0 = rf(ctx, namespace, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*corev1.PodList)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, namespace, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetStatefulSetPodsByLabels provides a mock function with given fields: ctx, namespace, labels
func (_m *StatefulSet) GetStatefulSetPodsByLabels(ctx context.Context, namespace string, labels map[string]string) (*corev1.PodList, error) {
	ret := _m.Called(ctx, namespace, labels)

	if len(ret) == 0 {
		panic("no return value specified for GetStatefulSetPodsByLabels")
	}

	var r0 *corev1.PodList
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, map[string]string) (*corev1.PodList, error)); ok {
		return rf(ctx, namespace, labels)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, map[string]string) *corev1.PodList); ok {
		r0 = rf(ctx, namespace, labels)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*corev1.PodList)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, map[string]string) error); ok {
		r1 = rf(ctx, namespace, labels)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListStatefulSetByLabels provides a mock function with given fields: ctx, namespace, labels
func (_m *StatefulSet) ListStatefulSetByLabels(ctx context.Context, namespace string, labels map[string]string) (*v1.StatefulSetList, error) {
	ret := _m.Called(ctx, namespace, labels)

	if len(ret) == 0 {
		panic("no return value specified for ListStatefulSetByLabels")
	}

	var r0 *v1.StatefulSetList
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, map[string]string) (*v1.StatefulSetList, error)); ok {
		return rf(ctx, namespace, labels)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, map[string]string) *v1.StatefulSetList); ok {
		r0 = rf(ctx, namespace, labels)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.StatefulSetList)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, map[string]string) error); ok {
		r1 = rf(ctx, namespace, labels)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListStatefulSets provides a mock function with given fields: ctx, namespace
func (_m *StatefulSet) ListStatefulSets(ctx context.Context, namespace string) (*v1.StatefulSetList, error) {
	ret := _m.Called(ctx, namespace)

	if len(ret) == 0 {
		panic("no return value specified for ListStatefulSets")
	}

	var r0 *v1.StatefulSetList
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*v1.StatefulSetList, error)); ok {
		return rf(ctx, namespace)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *v1.StatefulSetList); ok {
		r0 = rf(ctx, namespace)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.StatefulSetList)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, namespace)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateStatefulSet provides a mock function with given fields: ctx, namespace, statefulSet
func (_m *StatefulSet) UpdateStatefulSet(ctx context.Context, namespace string, statefulSet *v1.StatefulSet) error {
	ret := _m.Called(ctx, namespace, statefulSet)

	if len(ret) == 0 {
		panic("no return value specified for UpdateStatefulSet")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1.StatefulSet) error); ok {
		r0 = rf(ctx, namespace, statefulSet)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewStatefulSet creates a new instance of StatefulSet. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewStatefulSet(t interface {
	mock.TestingT
	Cleanup(func())
}) *StatefulSet {
	mock := &StatefulSet{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
