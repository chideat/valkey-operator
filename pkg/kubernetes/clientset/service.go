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
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Service the client that knows how to interact with kubernetes to manage them
type Service interface {
	// GetService get service from kubernetes with namespace and name
	GetService(ctx context.Context, namespace string, name string) (*corev1.Service, error)
	// CreateService will create the given service
	CreateService(ctx context.Context, namespace string, service *corev1.Service) error
	//CreateIfNotExistsService create service if it does not exist
	CreateIfNotExistsService(ctx context.Context, namespace string, service *corev1.Service) error
	// UpdateService will update the given service
	UpdateService(ctx context.Context, namespace string, service *corev1.Service) error
	// CreateOrUpdateService will update the given service or create it if does not exist
	CreateOrUpdateService(ctx context.Context, namespace string, service *corev1.Service) error
	// DeleteService will delete the given service
	DeleteService(ctx context.Context, namespace string, name string) error
	// ListServices get set of service on a given namespace
	ListServices(ctx context.Context, namespace string) (*corev1.ServiceList, error)
	GetServiceByLabels(ctx context.Context, namespace string, labelsMap map[string]string) (*corev1.ServiceList, error)
	UpdateIfSelectorChangedService(ctx context.Context, namespace string, service *corev1.Service) error
	CreateOrUpdateIfServiceChanged(ctx context.Context, namespace string, service *corev1.Service) error
}

// ServiceOption is the service client implementation using API calls to kubernetes.
type ServiceOption struct {
	client client.Client
	logger logr.Logger
}

// NewService returns a new Service client.
func NewService(kubeClient client.Client, logger logr.Logger) Service {
	logger = logger.WithValues("service", "k8s.service")
	return &ServiceOption{
		client: kubeClient,
		logger: logger,
	}
}

func (s *ServiceOption) GetServiceByLabels(ctx context.Context, namespace string, labelsMap map[string]string) (*corev1.ServiceList, error) {

	labelSelector := labels.Set(labelsMap).AsSelector()

	services := &corev1.ServiceList{}
	err := s.client.List(context.TODO(), services, &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labelSelector,
	})

	if err != nil {
		return nil, err
	}
	return services, err

}

// GetService implement the Service.Interface
func (s *ServiceOption) GetService(ctx context.Context, namespace string, name string) (*corev1.Service, error) {
	service := &corev1.Service{}
	err := s.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, service)

	if err != nil {
		return nil, err
	}
	return service, err
}

// CreateService implement the Service.Interface
func (s *ServiceOption) CreateService(ctx context.Context, namespace string, service *corev1.Service) error {
	err := s.client.Create(ctx, service)
	if err != nil {
		return err
	}
	s.logger.WithValues("namespace", namespace, "serviceName", service.Name).V(3).Info("service created")
	return nil
}

// CreateIfNotExistsService implement the Service.Interface
func (s *ServiceOption) CreateIfNotExistsService(ctx context.Context, namespace string, service *corev1.Service) error {
	if _, err := s.GetService(ctx, namespace, service.Name); err != nil {
		// If no resource we need to create.
		if errors.IsNotFound(err) {
			return s.CreateService(ctx, namespace, service)
		}
		return err
	}
	return nil
}

// CreateOrUpdateIfServiceChanged implement the Service.Interface
func (s *ServiceOption) CreateOrUpdateIfServiceChanged(ctx context.Context, namespace string, service *corev1.Service) error {
	oldSvc, err := s.GetService(ctx, namespace, service.Name)
	if errors.IsNotFound(err) {
		return s.CreateService(ctx, namespace, service)
	} else if err != nil {
		return err
	}
	if !reflect.DeepEqual(oldSvc.Labels, service.Labels) ||
		!reflect.DeepEqual(oldSvc.Annotations, service.Annotations) ||
		!cmp.Equal(oldSvc.Spec, service.Spec, cmpopts.EquateEmpty()) {

		return s.UpdateService(ctx, namespace, service)
	}
	return nil
}

func (s *ServiceOption) UpdateIfSelectorChangedService(ctx context.Context, namespace string, service *corev1.Service) error {
	oldSvc, err := s.GetService(ctx, namespace, service.Name)
	if errors.IsNotFound(err) {
		return s.CreateService(ctx, namespace, service)
	} else if err != nil {
		return err
	}
	if !reflect.DeepEqual(oldSvc.Spec.Selector, service.Spec.Selector) {
		return s.UpdateService(ctx, namespace, service)
	}
	return nil
}

// UpdateService implement the Service.Interface
func (s *ServiceOption) UpdateService(ctx context.Context, namespace string, service *corev1.Service) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		oldSvc, err := s.GetService(ctx, namespace, service.Name)
		if errors.IsNotFound(err) {
			return nil
		} else if err != nil {
			return err
		}
		service.ResourceVersion = oldSvc.ResourceVersion
		return s.client.Update(ctx, service)
	})
}

// CreateOrUpdateService implement the Service.Interface
func (s *ServiceOption) CreateOrUpdateService(ctx context.Context, namespace string, service *corev1.Service) error {
	oldSvc, err := s.GetService(ctx, namespace, service.Name)
	if errors.IsNotFound(err) {
		return s.CreateService(ctx, namespace, service)
	} else if err != nil {
		return err
	}
	service.ResourceVersion = oldSvc.ResourceVersion
	return s.UpdateService(ctx, namespace, service)
}

// DeleteService implement the Service.Interface
func (s *ServiceOption) DeleteService(ctx context.Context, namespace string, name string) error {
	service := &corev1.Service{}
	if err := s.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, service); err != nil {
		return err
	}

	err := s.client.Delete(ctx, service)
	s.logger.WithValues("namespace", namespace, "serviceName", service.Name).V(3).Info("service deleted")
	return err
}

// ListServices implement the Service.Interface
func (s *ServiceOption) ListServices(ctx context.Context, namespace string) (*corev1.ServiceList, error) {
	services := &corev1.ServiceList{}
	listOps := &client.ListOptions{
		Namespace: namespace,
	}
	err := s.client.List(ctx, services, listOps)
	return services, err
}
