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
package clientset

import (
	"context"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CronJob interface {
	GetCronJob(ctx context.Context, namespace, name string) (*v1.CronJob, error)
	ListCronJobs(ctx context.Context, namespace string, cl client.ListOptions) (*v1.CronJobList, error)
	DeleteCronJob(ctx context.Context, namespace, name string) error
	CreateCronJob(ctx context.Context, namespace string, cronjob *v1.CronJob) error
	UpdateCronJob(ctx context.Context, namespace string, job *v1.CronJob) error
	CreateOrUpdateCronJob(ctx context.Context, namespace string, cronjob *v1.CronJob) error
}

type CronJobOption struct {
	client client.Client
	logger logr.Logger
}

func NewCronJob(kubeClient client.Client, logger logr.Logger) CronJob {
	logger = logger.WithValues("service", "k8s.CronJob")
	return &CronJobOption{
		client: kubeClient,
		logger: logger,
	}
}

func (c *CronJobOption) GetCronJob(ctx context.Context, namespace, name string) (*v1.CronJob, error) {
	cronjob := &v1.CronJob{}
	err := c.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, cronjob)
	if err != nil {
		return nil, err
	}
	return cronjob, err
}

func (c *CronJobOption) ListCronJobs(ctx context.Context, namespace string, cl client.ListOptions) (*v1.CronJobList, error) {
	cs := &v1.CronJobList{}
	listOps := &cl
	err := c.client.List(ctx, cs, listOps)
	return cs, err
}

func (c *CronJobOption) DeleteCronJob(ctx context.Context, namespace, name string) error {
	cronjob := &v1.CronJob{}
	if err := c.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, cronjob); err != nil {
		return err
	}
	return c.client.Delete(ctx, cronjob)
}

func (c *CronJobOption) CreateCronJob(ctx context.Context, namespace string, cronjob *v1.CronJob) error {
	err := c.client.Create(ctx, cronjob)
	if err != nil {
		return err
	}
	c.logger.WithValues("namespace", namespace, "cronjob", cronjob.ObjectMeta.Name).V(3).Info("cronjob created")
	return err
}

func (c *CronJobOption) UpdateCronJob(ctx context.Context, namespace string, cronjob *v1.CronJob) error {
	err := c.client.Update(ctx, cronjob)
	if err != nil {
		return err
	}
	c.logger.WithValues("namespace", namespace, "cronjob", cronjob.ObjectMeta.Name).V(3).Info("cronjob updated")
	return nil
}

func (c *CronJobOption) CreateOrUpdateCronJob(ctx context.Context, namespace string, cronjob *v1.CronJob) error {
	old, err := c.GetCronJob(ctx, namespace, cronjob.Name)
	if errors.IsNotFound(err) {
		err := c.CreateCronJob(ctx, namespace, cronjob)
		return err
	} else if err != nil {
		return err
	}
	cronjob.ResourceVersion = old.ResourceVersion
	err = c.UpdateCronJob(ctx, namespace, cronjob)
	return err
}
