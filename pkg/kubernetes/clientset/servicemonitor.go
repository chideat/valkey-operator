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
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceMonitor interface {
	GetServiceMonitor(ctx context.Context, namespace string, name string) (*monitoring.ServiceMonitor, error)
	CreateServiceMonitor(ctx context.Context, namespace string, sm *monitoring.ServiceMonitor) error
	CreateOrUpdateServiceMonitor(ctx context.Context, namespace string, sm *monitoring.ServiceMonitor) error
	UpdateServiceMonitor(ctx context.Context, namespace string, sm *monitoring.ServiceMonitor) error
}

type ServiceMonitorOption struct {
	client client.Client
	logger logr.Logger
}

func NewServiceMonitor(kubeClient client.Client, logger logr.Logger) ServiceMonitor {
	logger = logger.WithValues("service", "k8s.serviceAccount")
	return &ServiceMonitorOption{
		client: kubeClient,
		logger: logger,
	}
}

func (s *ServiceMonitorOption) GetServiceMonitor(ctx context.Context, namespace string, name string) (*monitoring.ServiceMonitor, error) {
	sm := &monitoring.ServiceMonitor{}
	err := s.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, sm)

	if err != nil {
		return nil, err
	}
	return sm, err
}

func (s *ServiceMonitorOption) CreateServiceMonitor(ctx context.Context, namespace string, sm *monitoring.ServiceMonitor) error {
	sm.Namespace = namespace
	err := s.client.Create(ctx, sm)
	if err != nil {
		return err
	}
	s.logger.WithValues("namespace", namespace, "ServiceMonitor", sm.Name).V(3).Info("ServiceMonitor created")
	return nil
}

func (s *ServiceMonitorOption) CreateOrUpdateServiceMonitor(ctx context.Context, namespace string, sm *monitoring.ServiceMonitor) error {
	storedSm, err := s.GetServiceMonitor(ctx, namespace, sm.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			return s.CreateServiceMonitor(ctx, namespace, sm)
		}
		return err
	}
	sm.Namespace = namespace
	sm.ResourceVersion = storedSm.ResourceVersion
	return s.UpdateServiceMonitor(ctx, namespace, sm)
}

func (s *ServiceMonitorOption) UpdateServiceMonitor(ctx context.Context, namespace string, sm *monitoring.ServiceMonitor) error {
	sm.Namespace = namespace
	err := s.client.Update(ctx, sm)
	if err != nil {
		return err
	}
	s.logger.WithValues("namespace", namespace, "ServiceMonitor", sm.Name).V(3).Info("ServiceMonitor updated")
	return nil
}
