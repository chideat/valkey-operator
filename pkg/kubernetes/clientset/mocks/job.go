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

	v1 "k8s.io/api/batch/v1"
)

// Job is an autogenerated mock type for the Job type
type Job struct {
	mock.Mock
}

// CreateIfNotExistsJob provides a mock function with given fields: ctx, namespace, job
func (_m *Job) CreateIfNotExistsJob(ctx context.Context, namespace string, job *v1.Job) error {
	ret := _m.Called(ctx, namespace, job)

	if len(ret) == 0 {
		panic("no return value specified for CreateIfNotExistsJob")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1.Job) error); ok {
		r0 = rf(ctx, namespace, job)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateJob provides a mock function with given fields: ctx, namespace, job
func (_m *Job) CreateJob(ctx context.Context, namespace string, job *v1.Job) error {
	ret := _m.Called(ctx, namespace, job)

	if len(ret) == 0 {
		panic("no return value specified for CreateJob")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1.Job) error); ok {
		r0 = rf(ctx, namespace, job)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateOrUpdateJob provides a mock function with given fields: ctx, namespace, job
func (_m *Job) CreateOrUpdateJob(ctx context.Context, namespace string, job *v1.Job) error {
	ret := _m.Called(ctx, namespace, job)

	if len(ret) == 0 {
		panic("no return value specified for CreateOrUpdateJob")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1.Job) error); ok {
		r0 = rf(ctx, namespace, job)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteJob provides a mock function with given fields: ctx, namespace, name
func (_m *Job) DeleteJob(ctx context.Context, namespace string, name string) error {
	ret := _m.Called(ctx, namespace, name)

	if len(ret) == 0 {
		panic("no return value specified for DeleteJob")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) error); ok {
		r0 = rf(ctx, namespace, name)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetJob provides a mock function with given fields: ctx, namespace, name
func (_m *Job) GetJob(ctx context.Context, namespace string, name string) (*v1.Job, error) {
	ret := _m.Called(ctx, namespace, name)

	if len(ret) == 0 {
		panic("no return value specified for GetJob")
	}

	var r0 *v1.Job
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*v1.Job, error)); ok {
		return rf(ctx, namespace, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *v1.Job); ok {
		r0 = rf(ctx, namespace, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.Job)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, namespace, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListJobs provides a mock function with given fields: ctx, namespace, cl
func (_m *Job) ListJobs(ctx context.Context, namespace string, cl client.ListOptions) (*v1.JobList, error) {
	ret := _m.Called(ctx, namespace, cl)

	if len(ret) == 0 {
		panic("no return value specified for ListJobs")
	}

	var r0 *v1.JobList
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, client.ListOptions) (*v1.JobList, error)); ok {
		return rf(ctx, namespace, cl)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, client.ListOptions) *v1.JobList); ok {
		r0 = rf(ctx, namespace, cl)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.JobList)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, client.ListOptions) error); ok {
		r1 = rf(ctx, namespace, cl)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListJobsByLabel provides a mock function with given fields: ctx, namespace, label_map
func (_m *Job) ListJobsByLabel(ctx context.Context, namespace string, label_map map[string]string) (*v1.JobList, error) {
	ret := _m.Called(ctx, namespace, label_map)

	if len(ret) == 0 {
		panic("no return value specified for ListJobsByLabel")
	}

	var r0 *v1.JobList
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, map[string]string) (*v1.JobList, error)); ok {
		return rf(ctx, namespace, label_map)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, map[string]string) *v1.JobList); ok {
		r0 = rf(ctx, namespace, label_map)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.JobList)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, map[string]string) error); ok {
		r1 = rf(ctx, namespace, label_map)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateJob provides a mock function with given fields: ctx, namespace, job
func (_m *Job) UpdateJob(ctx context.Context, namespace string, job *v1.Job) error {
	ret := _m.Called(ctx, namespace, job)

	if len(ret) == 0 {
		panic("no return value specified for UpdateJob")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1.Job) error); ok {
		r0 = rf(ctx, namespace, job)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewJob creates a new instance of Job. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewJob(t interface {
	mock.TestingT
	Cleanup(func())
}) *Job {
	mock := &Job{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
