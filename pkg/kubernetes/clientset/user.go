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

	v1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type User interface {
	// ListUsers lists the users on a cluster.
	ListUsers(ctx context.Context, namespace string, opts client.ListOptions) (*v1alpha1.UserList, error)
	// GetUser get the user on a cluster.
	GetUser(ctx context.Context, namespace, name string) (*v1alpha1.User, error)
	// UpdateUser update the user on a cluster.
	UpdateUser(ctx context.Context, ru *v1alpha1.User) error
	// Create
	CreateUser(ctx context.Context, ru *v1alpha1.User) error
	//create if not exites
	CreateIfNotExistsUser(ctx context.Context, ru *v1alpha1.User) error

	//create or update
	CreateOrUpdateUser(ctx context.Context, ru *v1alpha1.User) error
}

type UserOption struct {
	client client.Client
	logger logr.Logger
}

func NewUserService(client client.Client, logger logr.Logger) *UserOption {
	logger = logger.WithName("k8s.user")
	return &UserOption{
		client: client,
		logger: logger,
	}
}

func (r *UserOption) ListUsers(ctx context.Context, namespace string, opts client.ListOptions) (*v1alpha1.UserList, error) {
	ret := v1alpha1.UserList{}
	err := r.client.List(ctx, &ret, &opts)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

func (r *UserOption) GetUser(ctx context.Context, namespace, name string) (*v1alpha1.User, error) {
	ret := v1alpha1.User{}
	err := r.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &ret)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

func (r *UserOption) UpdateUser(ctx context.Context, ru *v1alpha1.User) error {
	o := v1alpha1.User{}
	err := r.client.Get(ctx, types.NamespacedName{
		Name:      ru.Name,
		Namespace: ru.Namespace,
	}, &o)
	if err != nil {
		return err
	}
	o.Annotations = ru.Annotations
	o.Labels = ru.Labels
	o.Spec = ru.Spec
	o.Status = ru.Status
	if err := r.client.Update(ctx, &o); err != nil {
		r.logger.Error(err, "update user failed")
		return err
	}
	if err := r.client.Status().Update(ctx, &o); err != nil {
		r.logger.Error(err, "update user status failed")
		return err
	}
	return err
}

func (r *UserOption) DeleteUser(ctx context.Context, namespace string, name string) error {
	ret := v1alpha1.User{}
	if err := r.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &ret); err != nil {
		return err
	}
	if err := r.client.Delete(ctx, &ret); err != nil {
		return err
	}
	return nil
}

func (r *UserOption) CreateUser(ctx context.Context, ru *v1alpha1.User) error {
	if err := r.client.Create(ctx, ru); err != nil {
		return err
	}
	return nil
}

func (r *UserOption) CreateIfNotExistsUser(ctx context.Context, ru *v1alpha1.User) error {
	if _, err := r.GetUser(ctx, ru.Namespace, ru.Name); err != nil {
		if errors.IsNotFound(err) {
			return r.CreateUser(ctx, ru)
		}
		return err
	}
	return nil
}

func (r *UserOption) CreateOrUpdateUser(ctx context.Context, ru *v1alpha1.User) error {
	if oldRu, err := r.GetUser(ctx, ru.Namespace, ru.Name); err != nil {
		if errors.IsNotFound(err) {
			return r.CreateUser(ctx, ru)
		}
		return err
	} else {
		oldRu.Spec = ru.Spec
	}
	return r.UpdateUser(ctx, ru)
}
