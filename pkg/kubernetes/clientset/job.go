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

package clientset

import (
	"context"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Job interface {
	GetJob(ctx context.Context, namespace, name string) (*v1.Job, error)
	ListJobs(ctx context.Context, namespace string, cl client.ListOptions) (*v1.JobList, error)
	DeleteJob(ctx context.Context, namespace, name string) error
	CreateJob(ctx context.Context, namespace string, job *v1.Job) error
	UpdateJob(ctx context.Context, namespace string, job *v1.Job) error
	CreateOrUpdateJob(ctx context.Context, namespace string, job *v1.Job) error
	CreateIfNotExistsJob(ctx context.Context, namespace string, job *v1.Job) error
	ListJobsByLabel(ctx context.Context, namespace string, label_map map[string]string) (*v1.JobList, error)
}

type JobOption struct {
	client client.Client
	logger logr.Logger
}

func NewJob(kubeClient client.Client, logger logr.Logger) Job {
	logger = logger.WithValues("service", "k8s.Job")
	return &JobOption{
		client: kubeClient,
		logger: logger,
	}
}

func (c *JobOption) GetJob(ctx context.Context, namespace string, name string) (*v1.Job, error) {
	job := &v1.Job{}
	err := c.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, job)
	if err != nil {
		return nil, err
	}
	return job, err
}

func (c *JobOption) ListJobs(ctx context.Context, namespace string, cl client.ListOptions) (*v1.JobList, error) {
	cs := &v1.JobList{}
	listOps := &cl
	err := c.client.List(ctx, cs, listOps)
	return cs, err
}

func (c *JobOption) DeleteJob(ctx context.Context, namespace string, name string) error {
	job := &v1.Job{}
	if err := c.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, job); err != nil {
		return err
	}
	return c.client.Delete(ctx, job)
}

func (c *JobOption) CreateJob(ctx context.Context, namespace string, job *v1.Job) error {
	err := c.client.Create(ctx, job)
	if err != nil {
		return err
	}
	c.logger.WithValues("namespace", namespace, "Job", job.ObjectMeta.Name).V(3).Info("Job created")
	return err
}

func (c *JobOption) UpdateJob(ctx context.Context, namespace string, job *v1.Job) error {
	err := c.client.Update(ctx, job)
	if err != nil {
		return err
	}
	c.logger.WithValues("namespace", namespace, "Job", job.ObjectMeta.Name).V(3).Info("Job updated")
	return nil
}

func (c *JobOption) CreateOrUpdateJob(ctx context.Context, namespace string, job *v1.Job) error {
	storedJob, err := c.GetJob(ctx, namespace, job.Name)
	if err != nil {
		// If no resource we need to create.
		if errors.IsNotFound(err) {
			return c.CreateJob(ctx, namespace, job)
		}
		return err
	}
	job.ResourceVersion = storedJob.ResourceVersion
	return c.UpdateJob(ctx, namespace, job)
}

func (c *JobOption) CreateIfNotExistsJob(ctx context.Context, namespace string, job *v1.Job) error {
	_, err := c.GetJob(ctx, namespace, job.Name)
	if err != nil {
		// If no resource we need to create.
		if errors.IsNotFound(err) {
			return c.CreateJob(ctx, namespace, job)
		}
		return err
	}
	return err
}
func (c *JobOption) ListJobsByLabel(ctx context.Context, namespace string, label_map map[string]string) (*v1.JobList, error) {
	listOps := &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labels.SelectorFromSet(label_map),
	}
	joblist := &v1.JobList{}
	err := c.client.List(ctx, joblist, listOps)
	return joblist, err
}
