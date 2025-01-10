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

	v1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Failover the RF service that knows how to interact with k8s to get them
type Failover interface {
	// ListFailovers
	ListFailovers(ctx context.Context, namespace string, opts client.ListOptions) (*v1alpha1.FailoverList, error)
	// GetFailover
	GetFailover(ctx context.Context, namespace, name string) (*v1alpha1.Failover, error)
	// UpdateFailover
	UpdateFailover(ctx context.Context, inst *v1alpha1.Failover) error
	// UpdateFailoverStatus
	UpdateFailoverStatus(ctx context.Context, inst *v1alpha1.Failover) error
}

// FailoverService is the Failover service implementation using API calls to kubernetes.
type FailoverService struct {
	client client.Client
	logger logr.Logger
}

// NewFailoverService returns a new Workspace KubeService.
func NewFailoverService(client client.Client, logger logr.Logger) *FailoverService {
	logger = logger.WithName("Failover")

	return &FailoverService{
		client: client,
		logger: logger,
	}
}

// ListFailovers
func (r *FailoverService) ListFailovers(ctx context.Context, namespace string, opts client.ListOptions) (*v1alpha1.FailoverList, error) {
	ret := v1alpha1.FailoverList{}
	err := r.client.List(ctx, &ret, &opts)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

// GetFailover
func (r *FailoverService) GetFailover(ctx context.Context, namespace, name string) (*v1alpha1.Failover, error) {
	ret := v1alpha1.Failover{}
	err := r.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &ret)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

// UpdateFailover
func (r *FailoverService) UpdateFailover(ctx context.Context, inst *v1alpha1.Failover) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var oldInst v1alpha1.Failover
		if err := r.client.Get(ctx, client.ObjectKeyFromObject(inst), &oldInst); err != nil {
			r.logger.Error(err, "get Failover failed")
			return err
		}
		inst.ResourceVersion = oldInst.ResourceVersion
		return r.client.Update(ctx, inst)
	})
}

func (r *FailoverService) UpdateFailoverStatus(ctx context.Context, inst *v1alpha1.Failover) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var oldInst v1alpha1.Failover
		if err := r.client.Get(ctx, client.ObjectKeyFromObject(inst), &oldInst); err != nil {
			r.logger.Error(err, "get Failover failed")
			return err
		}
		if !reflect.DeepEqual(oldInst.Status, inst.Status) {
			inst.ResourceVersion = oldInst.ResourceVersion
			return r.client.Status().Update(ctx, inst)
		}
		return nil
	})
}

func (r *FailoverService) DeleteFailover(ctx context.Context, namespace string, name string) error {
	ret := v1alpha1.Failover{}
	if err := r.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &ret); err != nil {
		return err
	}
	return r.client.Delete(ctx, &ret)
}
