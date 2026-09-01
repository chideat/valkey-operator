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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConfigMap the client that knows how to interact with kubernetes to manage them
type ConfigMap interface {
	// GetConfigMap get ConfigMap from kubernetes with namespace and name
	GetConfigMap(ctx context.Context, namespace string, name string) (*corev1.ConfigMap, error)
	// CreateConfigMap create the given ConfigMap
	CreateConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) error
	// UpdateConfigMap update the given ConfigMap
	UpdateConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) error
	// CreateOrUpdateConfigMap if the ConfigMap Already exists, create it, otherwise update it
	CreateOrUpdateConfigMap(ctx context.Context, namespace string, np *corev1.ConfigMap) error
	// DeleteConfigMap delete ConfigMap from kubernetes with namespace and name
	DeleteConfigMap(ctx context.Context, namespace string, name string) error
	// ListConfigMaps get set of ConfigMaps on a given namespace
	ListConfigMaps(ctx context.Context, namespace string) (*corev1.ConfigMapList, error)
	CreateIfNotExistsConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) error
	UpdateIfConfigMapChanged(ctx context.Context, newConfigmap *corev1.ConfigMap) error
}

// ConfigMapOption is the configMap client interface implementation that using API calls to kubernetes.
type ConfigMapOption struct {
	client client.Client
	logger logr.Logger
}

// NewConfigMap returns a new ConfigMap client.
func NewConfigMap(kubeClient client.Client, logger logr.Logger) ConfigMap {
	logger = logger.WithValues("service", "k8s.configMap")
	return &ConfigMapOption{
		client: kubeClient,
		logger: logger,
	}
}

// GetConfigMap implement the  ConfigMap.Interface
func (p *ConfigMapOption) GetConfigMap(ctx context.Context, namespace string, name string) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	err := p.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, configMap)

	if err != nil {
		return nil, err
	}
	return configMap, err
}

// CreateConfigMap implement the  ConfigMap.Interface
func (p *ConfigMapOption) CreateConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) error {
	return retry.OnError(retry.DefaultRetry, func(err error) bool {
		return errors.IsInternalError(err) ||
			errors.IsServerTimeout(err) ||
			errors.IsTimeout(err) ||
			errors.IsUnexpectedServerError(err) ||
			errors.IsServiceUnavailable(err)
	}, func() error {
		return p.client.Create(ctx, configMap)
	})
}

// UpdateConfigMap implement the  ConfigMap.Interface
//
// The ResourceVersion carried by configMap is honoured as an optimistic-concurrency
// precondition: a caller passing an object it read asserts it has seen that version, so a
// write that lands in between is reported as a conflict instead of being silently
// discarded. Callers that build a complete desired object from scratch have no version to
// assert and must go through CreateOrUpdateConfigMap, which adopts the stored one.
//
// NOTE: do not reintroduce a Get-then-stamp of the stored ResourceVersion here. It makes
// every update unconditional, so a stale snapshot overwrites newer state and no conflict is
// ever raised — that is what let a deleted User reappear in the ACL ConfigMap.
func (p *ConfigMapOption) UpdateConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) error {
	return retry.OnError(retry.DefaultRetry, func(err error) bool {
		return errors.IsInternalError(err) ||
			errors.IsServerTimeout(err) ||
			errors.IsTimeout(err) ||
			errors.IsUnexpectedServerError(err) ||
			errors.IsServiceUnavailable(err)
	}, func() error {
		return p.client.Update(ctx, configMap)
	})
}

// CreateIfNotExistsConfigMap implement the ConfigMap.Interface
func (p *ConfigMapOption) CreateIfNotExistsConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) error {
	if _, err := p.GetConfigMap(ctx, namespace, configMap.Name); err != nil {
		// If no resource we need to create.
		if errors.IsNotFound(err) {
			return p.CreateConfigMap(ctx, namespace, configMap)
		}
		return err
	}
	return nil
}

// CreateOrUpdateConfigMap implement the  ConfigMap.Interface
func (p *ConfigMapOption) CreateOrUpdateConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) error {
	storedConfigMap, err := p.GetConfigMap(ctx, namespace, configMap.Name)
	if err != nil {
		// If no resource we need to create.
		if errors.IsNotFound(err) {
			return p.CreateConfigMap(ctx, namespace, configMap)
		}
		return err
	}

	// Already exists, need to Update. Adopt the stored version only for an object the caller
	// built from scratch — that is a request to replace the ConfigMap with the given content.
	// An object the caller read keeps its own version, so a concurrent write is reported as a
	// conflict rather than being overwritten.
	if configMap.ResourceVersion == "" {
		configMap.ResourceVersion = storedConfigMap.ResourceVersion
	}
	return p.UpdateConfigMap(ctx, namespace, configMap)
}

// DeleteConfigMap implement the  ConfigMap.Interface
func (p *ConfigMapOption) DeleteConfigMap(ctx context.Context, namespace string, name string) error {
	configMap := &corev1.ConfigMap{}
	if err := p.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, configMap); err != nil {
		return err
	}
	return p.client.Delete(ctx, configMap)
}

// ListConfigMaps implement the  ConfigMap.Interface
func (p *ConfigMapOption) ListConfigMaps(ctx context.Context, namespace string) (*corev1.ConfigMapList, error) {
	cms := &corev1.ConfigMapList{}
	listOps := &client.ListOptions{
		Namespace: namespace,
	}
	err := p.client.List(ctx, cms, listOps)
	return cms, err
}

// UpdateIfConfigMapChanged writes newConfigmap when its Data differs from what is stored.
//
// Its callers hand over a freshly built object, so the version compared against is adopted
// when the caller carries none — the write is the intended replacement of exactly the
// content that was just found to differ.
func (p *ConfigMapOption) UpdateIfConfigMapChanged(ctx context.Context, newConfigmap *corev1.ConfigMap) error {
	oldConfigmap, err := p.GetConfigMap(ctx, newConfigmap.Namespace, newConfigmap.Name)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(newConfigmap.Data, oldConfigmap.Data) {
		if newConfigmap.ResourceVersion == "" {
			newConfigmap.ResourceVersion = oldConfigmap.ResourceVersion
		}
		return p.UpdateConfigMap(ctx, newConfigmap.Namespace, newConfigmap)
	}
	return nil
}
