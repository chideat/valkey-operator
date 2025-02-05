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
*/package clientset

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
)

// Sentinel the sen service that knows how to interact with k8s to get them
type Sentinel interface {
	// ListSentinels
	ListSentinels(ctx context.Context, namespace string, opts client.ListOptions) (*v1alpha1.SentinelList, error)
	// GetSentinel
	GetSentinel(ctx context.Context, namespace, name string) (*v1alpha1.Sentinel, error)
	// UpdateSentinel
	UpdateSentinel(ctx context.Context, sen *v1alpha1.Sentinel) error
	// UpdateSentinelStatus
	UpdateSentinelStatus(ctx context.Context, inst *v1alpha1.Sentinel) error
}

// SentinelService is the Sentinel service implementation using API calls to kubernetes.
type SentinelService struct {
	client client.Client
	logger logr.Logger
}

// NewSentinelService returns a new Workspace KubeService.
func NewSentinelService(client client.Client, logger logr.Logger) *SentinelService {
	logger = logger.WithName("k8s.sentinel")

	return &SentinelService{
		client: client,
		logger: logger,
	}
}

// ListSentinels
func (r *SentinelService) ListSentinels(ctx context.Context, namespace string, opts client.ListOptions) (*v1alpha1.SentinelList, error) {
	ret := v1alpha1.SentinelList{}
	if err := r.client.List(ctx, &ret, &opts); err != nil {
		return nil, err
	}
	return &ret, nil
}

// GetSentinel
func (r *SentinelService) GetSentinel(ctx context.Context, namespace, name string) (*v1alpha1.Sentinel, error) {
	ret := v1alpha1.Sentinel{}
	err := r.client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &ret)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

// UpdateSentinel
func (r *SentinelService) UpdateSentinel(ctx context.Context, inst *v1alpha1.Sentinel) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var oldInst v1alpha1.Sentinel
		if err := r.client.Get(ctx, client.ObjectKeyFromObject(inst), &oldInst); err != nil {
			r.logger.Error(err, "get Sentinel failed")
			return err
		}
		inst.ResourceVersion = oldInst.ResourceVersion
		return r.client.Update(ctx, inst)
	})
}

func (r *SentinelService) UpdateSentinelStatus(ctx context.Context, inst *v1alpha1.Sentinel) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var oldInst v1alpha1.Sentinel
		if err := r.client.Get(ctx, client.ObjectKeyFromObject(inst), &oldInst); err != nil {
			r.logger.Error(err, "get Sentinel failed")
			return err
		}
		if !reflect.DeepEqual(oldInst.Status, inst.Status) {
			inst.ResourceVersion = oldInst.ResourceVersion
			return r.client.Status().Update(ctx, inst)
		}
		return nil
	})
}

func (r *SentinelService) DeleteSentinel(ctx context.Context, namespace string, name string) error {
	ret := v1alpha1.Sentinel{}
	if err := r.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &ret); err != nil {
		return err
	}
	return r.client.Delete(ctx, &ret)
}
