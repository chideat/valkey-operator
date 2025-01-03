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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Secret interface {
	GetSecret(ctx context.Context, namespace string, name string) (*v1.Secret, error)
	CreateSecret(ctx context.Context, namespace string, secret *v1.Secret) error
	UpdateSecret(ctx context.Context, namespace string, secret *v1.Secret) error
	CreateOrUpdateSecret(ctx context.Context, namespace string, secret *v1.Secret) error
	DeleteSecret(ctx context.Context, namespace string, name string) error
	ListSecret(ctx context.Context, namespace string) (*v1.SecretList, error)
	CreateIfNotExistsSecret(ctx context.Context, namespace string, secret *v1.Secret) error
}

func (s *SecretOption) GetSecret(ctx context.Context, namespace string, name string) (*v1.Secret, error) {
	secret := &v1.Secret{}
	if err := s.client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, secret); err != nil {
		return nil, err
	}
	return secret, nil
}

func (s *SecretOption) CreateSecret(ctx context.Context, namespace string, secret *v1.Secret) error {
	err := s.client.Create(ctx, secret)
	return err
}

func (s *SecretOption) UpdateSecret(ctx context.Context, namespace string, secret *v1.Secret) error {
	err := s.client.Update(ctx, secret)
	return err
}

func (s *SecretOption) CreateOrUpdateSecret(ctx context.Context, namespace string, secret *v1.Secret) error {
	secrets, err := s.GetSecret(ctx, namespace, secret.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			return s.CreateSecret(ctx, namespace, secret)
		}
		return err
	}
	secret.ResourceVersion = secrets.ResourceVersion
	return s.UpdateSecret(ctx, namespace, secret)
}

func (s *SecretOption) DeleteSecret(ctx context.Context, namespace string, name string) error {
	secret := &v1.Secret{}
	if err := s.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, secret); err != nil {
		return err
	}
	return s.client.Delete(ctx, secret)
}

func (s *SecretOption) ListSecret(ctx context.Context, namespace string) (*v1.SecretList, error) {
	secret := &v1.SecretList{}
	listOps := &client.ListOptions{
		Namespace: namespace,
	}
	err := s.client.List(ctx, secret, listOps)
	return secret, err
}

func (s *SecretOption) CreateIfNotExistsSecret(ctx context.Context, namespace string, secret *v1.Secret) error {
	if _, err := s.GetSecret(ctx, namespace, secret.Name); err != nil {
		if errors.IsNotFound(err) {
			return s.CreateSecret(ctx, namespace, secret)
		}
		return err
	}
	return nil
}

type SecretOption struct {
	client client.Client
	logger logr.Logger
}

func NewSecret(kubeClient client.Client, logger logr.Logger) Secret {
	logger = logger.WithValues("service", "k8s.configMap")
	return &SecretOption{
		client: kubeClient,
		logger: logger,
	}
}
