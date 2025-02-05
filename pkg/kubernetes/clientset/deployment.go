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
	"encoding/json"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Deployment the client that knows how to interact with kubernetes to manage them
type Deployment interface {
	// GetDeployment get deployment from kubernetes with namespace and name
	GetDeployment(ctx context.Context, namespace, name string) (*appsv1.Deployment, error)
	// GetDeploymentPods will retrieve the pods managed by a given deployment
	GetDeploymentPods(ctx context.Context, namespace, name string) (*corev1.PodList, error)
	// CreateDeployment will create the given deployment
	CreateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) error
	// UpdateDeployment will update the given deployment
	UpdateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) error
	// CreateOrUpdateDeployment will update the given deployment or create it if does not exist
	CreateOrUpdateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) error
	// DeleteDeployment will delete the given deployment
	DeleteDeployment(ctx context.Context, namespace string, name string) error
	// ListDeployments get set of deployment on a given namespace
	ListDeployments(ctx context.Context, namespace string) (*appsv1.DeploymentList, error)
	// RestartDeployment
	RestartDeployment(ctx context.Context, namespace, name string) (*appsv1.Deployment, error)

	CreateIfNotExistsDeployment(ctx context.Context, namespace string, deploy *appsv1.Deployment) error
}

// DeploymentOption is the deployment client interface implementation that using API calls to kubernetes.
type DeploymentOption struct {
	client client.Client
	logger logr.Logger
}

// NewDeployment returns a new Deployment client.
func NewDeployment(kubeClient client.Client, logger logr.Logger) Deployment {
	logger = logger.WithValues("service", "k8s.deployment")
	return &DeploymentOption{
		client: kubeClient,
		logger: logger,
	}
}

// GetDeployment implement the Deployment.Interface
func (d *DeploymentOption) GetDeployment(ctx context.Context, namespace string, name string) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	err := d.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, deployment)
	if err != nil {
		return nil, err
	}
	return deployment, err
}

func (d *DeploymentOption) RestartDeployment(ctx context.Context, namespace string, name string) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	err := d.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, deployment)
	if err != nil {
		return nil, err
	}

	date := time.Now().Format(time.RFC3339Nano)
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = date
	old_deployment := &appsv1.Deployment{}
	err = d.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, old_deployment)
	if err != nil {
		return nil, err
	}
	currentPodJSON, err := json.Marshal(old_deployment)
	if err != nil {
		return nil, err
	}
	updatedPodJSON, err := json.Marshal(deployment)
	if err != nil {
		return nil, err
	}
	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(currentPodJSON, updatedPodJSON, appsv1.Deployment{})
	if err != nil {
		return nil, err
	}
	err = d.client.Patch(ctx, old_deployment, client.RawPatch(types.StrategicMergePatchType, patchBytes))
	return deployment, err
}

// GetDeploymentPods implement the Deployment.Interface
func (d *DeploymentOption) GetDeploymentPods(ctx context.Context, namespace string, name string) (*corev1.PodList, error) {
	deployment, err := d.GetDeployment(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	labelSet := make(map[string]string)
	for k, v := range deployment.Spec.Selector.MatchLabels {
		labelSet[k] = v
	}
	labelSelector := labels.SelectorFromSet(labelSet)
	foundPods := &corev1.PodList{}
	err = d.client.List(ctx, foundPods, &client.ListOptions{Namespace: namespace, LabelSelector: labelSelector})
	return foundPods, err
}

// CreateDeployment implement the Deployment.Interface
func (d *DeploymentOption) CreateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) error {
	err := d.client.Create(ctx, deployment)
	if err != nil {
		return err
	}
	d.logger.WithValues("namespace", namespace, "deployment", deployment.ObjectMeta.Name).V(3).Info("deployment created")
	return err
}

// UpdateDeployment implement the Deployment.Interface
func (d *DeploymentOption) UpdateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) error {
	err := d.client.Update(ctx, deployment)
	if err != nil {
		return err
	}
	d.logger.WithValues("namespace", namespace, "deployment", deployment.ObjectMeta.Name).V(3).Info("deployment updated")
	return err
}

// CreateOrUpdateDeployment implement the Deployment.Interface
func (d *DeploymentOption) CreateOrUpdateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		storedDeployment, err := d.GetDeployment(ctx, namespace, deployment.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				return d.CreateDeployment(ctx, namespace, deployment)
			}
			return err
		}
		deployment.ResourceVersion = storedDeployment.ResourceVersion
		return d.UpdateDeployment(ctx, namespace, deployment)
	})
}

// DeleteDeployment implement the Deployment.Interface
func (d *DeploymentOption) DeleteDeployment(ctx context.Context, namespace string, name string) error {
	deployment := &appsv1.Deployment{}
	if err := d.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, deployment); err != nil {
		return err
	}
	return d.client.Delete(ctx, deployment)
}

// ListDeployments implement the Deployment.Interface
func (d *DeploymentOption) ListDeployments(ctx context.Context, namespace string) (*appsv1.DeploymentList, error) {
	ds := &appsv1.DeploymentList{}
	listOps := &client.ListOptions{
		Namespace: namespace,
	}
	err := d.client.List(ctx, ds, listOps)
	return ds, err
}

func (d *DeploymentOption) CreateIfNotExistsDeployment(ctx context.Context, namespace string, deploy *appsv1.Deployment) error {
	_, err := d.GetDeployment(ctx, namespace, deploy.Name)
	if err != nil {
		// If no resource we need to create.
		if errors.IsNotFound(err) {
			return d.CreateDeployment(ctx, namespace, deploy)
		}
		return err
	}

	return nil
}
