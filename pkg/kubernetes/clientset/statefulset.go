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
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StatefulSet the StatefulSet client that knows how to interact with kubernetes to manage them
type StatefulSet interface {
	// GetStatefulSet get StatefulSet from kubernetes with namespace and name
	GetStatefulSet(ctx context.Context, namespace, name string) (*appsv1.StatefulSet, error)
	// GetStatefulSetPods will retrieve the pods managed by a given StatefulSet
	GetStatefulSetPods(ctx context.Context, namespace string, name string) (*corev1.PodList, error)
	// GetStatefulSetPodsByLabels
	GetStatefulSetPodsByLabels(ctx context.Context, namespace string, labels map[string]string) (*corev1.PodList, error)
	// CreateStatefulSet will create the given StatefulSet
	CreateStatefulSet(ctx context.Context, namespace string, statefulSet *appsv1.StatefulSet) error
	// UpdateStatefulSet will update the given StatefulSet
	UpdateStatefulSet(ctx context.Context, namespace string, statefulSet *appsv1.StatefulSet) error
	// CreateOrUpdateStatefulSet will update the given StatefulSet or create it if does not exist
	CreateOrUpdateStatefulSet(ctx context.Context, namespace string, StatefulSet *appsv1.StatefulSet) error
	// DeleteStatefulSet will delete the given StatefulSet
	DeleteStatefulSet(ctx context.Context, namespace string, name string, opts ...client.DeleteOption) error
	// ListStatefulSets get set of StatefulSet on a given namespace
	ListStatefulSets(ctx context.Context, namespace string) (*appsv1.StatefulSetList, error)
	// ListStatefulsetByLabels
	ListStatefulSetByLabels(ctx context.Context, namespace string, labels map[string]string) (*appsv1.StatefulSetList, error)
	// CreateIfNotExistsStatefulSet
	CreateIfNotExistsStatefulSet(ctx context.Context, namespace string, statefulSet *appsv1.StatefulSet) error
}

// StatefulSetOption is the StatefulSet client implementation using API calls to kubernetes.
type StatefulSetOption struct {
	client client.Client
	logger logr.Logger
}

// NewStatefulSet returns a new StatefulSet client.
func NewStatefulSet(kubeClient client.Client, logger logr.Logger) StatefulSet {
	logger = logger.WithValues("service", "k8s.statefulSet")
	return &StatefulSetOption{
		client: kubeClient,
		logger: logger,
	}
}

// GetStatefulSet implement the StatefulSet.Interface
func (s *StatefulSetOption) GetStatefulSet(ctx context.Context, namespace string, name string) (*appsv1.StatefulSet, error) {
	statefulSet := &appsv1.StatefulSet{}
	if err := s.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, statefulSet); err != nil {
		return nil, err
	}
	return statefulSet, nil
}

// GetStatefulSetPods implement the StatefulSet.Interface
func (s *StatefulSetOption) GetStatefulSetPods(ctx context.Context, namespace string, name string) (*corev1.PodList, error) {
	statefulSet, err := s.GetStatefulSet(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	labelSet := make(map[string]string)
	for k, v := range statefulSet.Spec.Selector.MatchLabels {
		labelSet[k] = v
	}
	labelSelector := labels.SelectorFromSet(labelSet)
	foundPods := &corev1.PodList{}
	err = s.client.List(ctx, foundPods, &client.ListOptions{Namespace: namespace, LabelSelector: labelSelector})
	return foundPods, err
}

// GetStatefulSetPodsByLabels implement the IStatefulSetControl.Interface.
func (s *StatefulSetOption) GetStatefulSetPodsByLabels(ctx context.Context, namespace string, labels map[string]string) (*corev1.PodList, error) {
	foundPods := &corev1.PodList{}
	err := s.client.List(ctx, foundPods, client.InNamespace(namespace), client.MatchingLabels(labels))
	return foundPods, err
}

// CreateStatefulSet implement the StatefulSet.Interface
func (s *StatefulSetOption) CreateStatefulSet(ctx context.Context, namespace string, statefulSet *appsv1.StatefulSet) error {
	err := s.client.Create(ctx, statefulSet)
	if err != nil {
		return err
	}
	s.logger.WithValues("namespace", namespace, "statefulSet", statefulSet.ObjectMeta.Name).V(3).Info("statefulSet created")
	return err
}

// UpdateStatefulSet implement the StatefulSet.Interface
func (s *StatefulSetOption) UpdateStatefulSet(ctx context.Context, namespace string, statefulSet *appsv1.StatefulSet) error {
	err := s.client.Update(ctx, statefulSet)
	if err != nil {
		return err
	}
	s.logger.WithValues("namespace", namespace, "statefulSet", statefulSet.ObjectMeta.Name).V(3).Info("statefulSet updated")
	return err
}

// CreateOrUpdateStatefulSet implement the StatefulSet.Interface
func (s *StatefulSetOption) CreateOrUpdateStatefulSet(ctx context.Context, namespace string, statefulSet *appsv1.StatefulSet) error {
	storedStatefulSet, err := s.GetStatefulSet(ctx, namespace, statefulSet.Name)
	if err != nil {
		// If no resource we need to create.
		if errors.IsNotFound(err) {
			return s.CreateStatefulSet(ctx, namespace, statefulSet)
		}
		return err
	}

	// Already exists, need to Update.
	// Set the correct resource version to ensure we are on the latest version. This way the only valid
	// namespace is our spec(https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#concurrency-control-and-consistency),
	// we will replace the current namespace state.
	s.logger.WithValues("namespace", namespace, "storedStatefulSet", statefulSet.ObjectMeta.Name).V(5).Info(fmt.Sprintf("storedStatefulSet Spec:\n %+v", storedStatefulSet))

	statefulSet.ResourceVersion = storedStatefulSet.ResourceVersion
	s.logger.WithValues("namespace", namespace, "statefulSet", statefulSet.ObjectMeta.Name).V(5).Info(fmt.Sprintf("Stateful Spec:\n %+v", statefulSet))

	return s.UpdateStatefulSet(ctx, namespace, statefulSet)
}

// CreateIfNotExistsStatefulSet implement the StatefulSet.Interface
func (s *StatefulSetOption) CreateIfNotExistsStatefulSet(ctx context.Context, namespace string, statefulSet *appsv1.StatefulSet) error {
	_, err := s.GetStatefulSet(ctx, namespace, statefulSet.Name)
	if err != nil {
		// If no resource we need to create.
		if errors.IsNotFound(err) {
			return s.CreateStatefulSet(ctx, namespace, statefulSet)
		}
		return err
	}

	return nil
}

// DeleteStatefulSet implement the StatefulSet.Interface
func (s *StatefulSetOption) DeleteStatefulSet(ctx context.Context, namespace string, name string, opts ...client.DeleteOption) error {
	statefulset := &appsv1.StatefulSet{}
	if err := s.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, statefulset); err != nil {
		return err
	}
	return s.client.Delete(ctx, statefulset, opts...)
}

// ListStatefulSets implement the StatefulSet.Interface
func (s *StatefulSetOption) ListStatefulSets(ctx context.Context, namespace string) (*appsv1.StatefulSetList, error) {
	statelfulSets := &appsv1.StatefulSetList{}
	listOps := &client.ListOptions{
		Namespace: namespace,
	}
	err := s.client.List(ctx, statelfulSets, listOps)
	return statelfulSets, err
}

// ListStatefulSetByLabels
func (s *StatefulSetOption) ListStatefulSetByLabels(ctx context.Context, namespace string, labels map[string]string) (*appsv1.StatefulSetList, error) {
	foundSts := &appsv1.StatefulSetList{}
	err := s.client.List(ctx, foundSts, client.InNamespace(namespace), client.MatchingLabels(labels))
	return foundSts, err
}
