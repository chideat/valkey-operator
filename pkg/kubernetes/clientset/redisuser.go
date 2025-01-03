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

	redisv1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RedisUser interface {
	// ListRedisUsers lists the redisusers on a cluster.
	ListRedisUsers(ctx context.Context, namespace string, opts client.ListOptions) (*redisv1.RedisUserList, error)
	// GetRedisUser get the redisuser on a cluster.
	GetRedisUser(ctx context.Context, namespace, name string) (*redisv1.RedisUser, error)
	// UpdateRedisUser update the redisuser on a cluster.
	UpdateRedisUser(ctx context.Context, ru *redisv1.RedisUser) error
	// Create
	CreateRedisUser(ctx context.Context, ru *redisv1.RedisUser) error
	//create if not exites
	CreateIfNotExistsRedisUser(ctx context.Context, ru *redisv1.RedisUser) error

	//create or update
	CreateOrUpdateRedisUser(ctx context.Context, ru *redisv1.RedisUser) error
}

type RedisUserOption struct {
	client client.Client
	logger logr.Logger
}

func NewRedisUserService(client client.Client, logger logr.Logger) *RedisUserOption {
	logger = logger.WithName("k8s.redisuser")
	return &RedisUserOption{
		client: client,
		logger: logger,
	}
}

func (r *RedisUserOption) ListRedisUsers(ctx context.Context, namespace string, opts client.ListOptions) (*redisv1.RedisUserList, error) {
	ret := redisv1.RedisUserList{}
	err := r.client.List(ctx, &ret, &opts)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

func (r *RedisUserOption) GetRedisUser(ctx context.Context, namespace, name string) (*redisv1.RedisUser, error) {
	ret := redisv1.RedisUser{}
	err := r.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &ret)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

func (r *RedisUserOption) UpdateRedisUser(ctx context.Context, ru *redisv1.RedisUser) error {
	o := redisv1.RedisUser{}
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
		r.logger.Error(err, "update redis user failed")
		return err
	}
	if err := r.client.Status().Update(ctx, &o); err != nil {
		r.logger.Error(err, "update redis user status failed")
		return err
	}
	return err
}

func (r *RedisUserOption) DeleteRedisUser(ctx context.Context, namespace string, name string) error {
	ret := redisv1.RedisUser{}
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

func (r *RedisUserOption) CreateRedisUser(ctx context.Context, ru *redisv1.RedisUser) error {
	if err := r.client.Create(ctx, ru); err != nil {
		return err
	}
	return nil
}

func (r *RedisUserOption) CreateIfNotExistsRedisUser(ctx context.Context, ru *redisv1.RedisUser) error {
	if _, err := r.GetRedisUser(ctx, ru.Namespace, ru.Name); err != nil {
		if errors.IsNotFound(err) {
			return r.CreateRedisUser(ctx, ru)
		}
		return err
	}
	return nil
}

func (r *RedisUserOption) CreateOrUpdateRedisUser(ctx context.Context, ru *redisv1.RedisUser) error {
	if oldRu, err := r.GetRedisUser(ctx, ru.Namespace, ru.Name); err != nil {
		if errors.IsNotFound(err) {
			return r.CreateRedisUser(ctx, ru)
		}
		return err
	} else {
		oldRu.Spec = ru.Spec
	}
	return r.UpdateRedisUser(ctx, ru)
}
