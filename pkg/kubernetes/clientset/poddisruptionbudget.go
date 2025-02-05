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
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PodDisruptionBudget the client that knows how to interact with kubernetes to manage them
type PodDisruptionBudget interface {
	// GetPodDisruptionBudget get podDisruptionBudget from kubernetes with namespace and name
	GetPodDisruptionBudget(ctx context.Context, namespace string, name string) (*policyv1.PodDisruptionBudget, error)
	// CreatePodDisruptionBudget will create the given podDisruptionBudget
	CreatePodDisruptionBudget(ctx context.Context, namespace string, podDisruptionBudget *policyv1.PodDisruptionBudget) error
	// UpdatePodDisruptionBudget will update the given podDisruptionBudget
	UpdatePodDisruptionBudget(ctx context.Context, namespace string, podDisruptionBudget *policyv1.PodDisruptionBudget) error
	// CreateOrUpdatePodDisruptionBudget will update the given podDisruptionBudget or create it if does not exist
	CreateOrUpdatePodDisruptionBudget(ctx context.Context, namespace string, podDisruptionBudget *policyv1.PodDisruptionBudget) error
	// DeletePodDisruptionBudget will delete the given podDisruptionBudget
	DeletePodDisruptionBudget(ctx context.Context, namespace string, name string) error
	CreateIfNotExistsPodDisruptionBudget(ctx context.Context, namespace string, podDisruptionBudget *policyv1.PodDisruptionBudget) error
}

// PodDisruptionBudgetOption is the podDisruptionBudget client implementation using API calls to kubernetes.
type PodDisruptionBudgetOption struct {
	client client.Client
	logger logr.Logger
}

// NewPodDisruptionBudget returns a new PodDisruptionBudget client.
func NewPodDisruptionBudget(kubeClient client.Client, logger logr.Logger) PodDisruptionBudget {
	logger = logger.WithValues("service", "k8s.podDisruptionBudget")
	return &PodDisruptionBudgetOption{
		client: kubeClient,
		logger: logger,
	}
}

// GetPodDisruptionBudget implement the PodDisruptionBudget.Interface
func (p *PodDisruptionBudgetOption) GetPodDisruptionBudget(ctx context.Context, namespace string, name string) (*policyv1.PodDisruptionBudget, error) {
	podDisruptionBudget := &policyv1.PodDisruptionBudget{}
	err := p.client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, podDisruptionBudget)
	if err != nil {
		return nil, err
	}
	return podDisruptionBudget, nil
}

// CreatePodDisruptionBudget implement the PodDisruptionBudget.Interface
func (p *PodDisruptionBudgetOption) CreatePodDisruptionBudget(ctx context.Context, namespace string, podDisruptionBudget *policyv1.PodDisruptionBudget) error {
	err := p.client.Create(ctx, podDisruptionBudget)
	if err != nil {
		return err
	}
	p.logger.WithValues("namespace", namespace, "podDisruptionBudget", podDisruptionBudget.Name).V(3).Info("podDisruptionBudget created")
	return nil
}

// UpdatePodDisruptionBudget implement the PodDisruptionBudget.Interface
func (p *PodDisruptionBudgetOption) UpdatePodDisruptionBudget(ctx context.Context, namespace string, podDisruptionBudget *policyv1.PodDisruptionBudget) error {
	err := p.client.Update(ctx, podDisruptionBudget)
	if err != nil {
		return err
	}
	p.logger.WithValues("namespace", namespace, "podDisruptionBudget", podDisruptionBudget.Name).V(3).Info("podDisruptionBudget updated")
	return nil
}

// CreateOrUpdatePodDisruptionBudget implement the PodDisruptionBudget.Interface
func (p *PodDisruptionBudgetOption) CreateOrUpdatePodDisruptionBudget(ctx context.Context, namespace string, podDisruptionBudget *policyv1.PodDisruptionBudget) error {
	storedPodDisruptionBudget, err := p.GetPodDisruptionBudget(ctx, namespace, podDisruptionBudget.Name)
	if err != nil {
		// If no resource we need to create.
		if errors.IsNotFound(err) {
			return p.CreatePodDisruptionBudget(ctx, namespace, podDisruptionBudget)
		}
		return err
	}

	// Already exists, need to Update.
	// Set the correct resource version to ensure we are on the latest version. This way the only valid
	// namespace is our spec(https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#concurrency-control-and-consistency),
	// we will replace the current namespace state.
	podDisruptionBudget.ResourceVersion = storedPodDisruptionBudget.ResourceVersion
	return p.UpdatePodDisruptionBudget(ctx, namespace, podDisruptionBudget)
}

// CreateIfNotExistsPodDisruptionBudget implement the PodDisruptionBudget.Interface
func (p *PodDisruptionBudgetOption) CreateIfNotExistsPodDisruptionBudget(ctx context.Context, namespace string, podDisruptionBudget *policyv1.PodDisruptionBudget) error {
	_, err := p.GetPodDisruptionBudget(ctx, namespace, podDisruptionBudget.Name)
	if err != nil {
		// If no resource we need to create.
		if errors.IsNotFound(err) {
			return p.CreatePodDisruptionBudget(ctx, namespace, podDisruptionBudget)
		}
		return err
	}

	return nil
}

func (p *PodDisruptionBudgetOption) UpdateIfSelectorChangedPodDisruptionaBudget(ctx context.Context, namespace string, podDisruptionBudget *policyv1.PodDisruptionBudget) error {
	old_pdb, err := p.GetPodDisruptionBudget(ctx, namespace, podDisruptionBudget.Name)
	if err != nil {
		// If no resource we need to create.
		if errors.IsNotFound(err) {
			return p.CreatePodDisruptionBudget(ctx, namespace, podDisruptionBudget)
		}
		return err
	}
	if !reflect.DeepEqual(old_pdb.Spec.Selector.MatchLabels, podDisruptionBudget.Spec.Selector.MatchLabels) {
		return p.UpdatePodDisruptionBudget(ctx, namespace, podDisruptionBudget)
	}
	return nil
}

// DeletePodDisruptionBudget implement the PodDisruptionBudget.Interface
func (p *PodDisruptionBudgetOption) DeletePodDisruptionBudget(ctx context.Context, namespace string, name string) error {
	podDisruptionBudget := &policyv1.PodDisruptionBudget{}
	if err := p.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, podDisruptionBudget); err != nil {
		return err
	}
	return p.client.Delete(ctx, podDisruptionBudget)
}
