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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceAccount interface {
	GetServiceAccount(ctx context.Context, namespace string, name string) (*corev1.ServiceAccount, error)
	CreateServiceAccount(ctx context.Context, namespace string, sa *corev1.ServiceAccount) error
	CreateOrUpdateServiceAccount(ctx context.Context, namespace string, sa *corev1.ServiceAccount) error
}

type ServiceAccountOption struct {
	client client.Client
	logger logr.Logger
}

func NewServiceAccount(kubeClient client.Client, logger logr.Logger) ServiceAccount {
	logger = logger.WithValues("service", "k8s.serviceAccount")
	return &ServiceAccountOption{
		client: kubeClient,
		logger: logger,
	}
}

func (s *ServiceAccountOption) GetServiceAccount(ctx context.Context, namespace string, name string) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{}
	err := s.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, sa)

	if err != nil {
		return nil, err
	}
	return sa, err
}

func (s *ServiceAccountOption) CreateServiceAccount(ctx context.Context, namespace string, sa *corev1.ServiceAccount) error {
	err := s.client.Create(ctx, sa)
	if err != nil {
		return err
	}
	s.logger.WithValues("namespace", namespace, "serviceAccountName", sa.Name).V(3).Info("serviceAccount created")
	return nil
}

func (s *ServiceAccountOption) CreateOrUpdateServiceAccount(ctx context.Context, namespace string, sa *corev1.ServiceAccount) error {
	if _, err := s.GetServiceAccount(ctx, namespace, sa.GetName()); errors.IsNotFound(err) {
		return s.CreateServiceAccount(ctx, namespace, sa)
	} else {
		return err
	}
}
