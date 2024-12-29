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
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PVC the ServiceAccount service that knows how to interact with k8s to manage them
type PVC interface {
	GetPVC(ctx context.Context, namespace string, name string) (*corev1.PersistentVolumeClaim, error)
	CreatePVC(ctx context.Context, namespace string, pvc *corev1.PersistentVolumeClaim) error
	ListPvcByLabel(ctx context.Context, namespace string, label_map map[string]string) (*corev1.PersistentVolumeClaimList, error)
}

// PVCService is the pvc service implementation using API calls to kubernetes.
type PVCService struct {
	kubeClient client.Client
	logger     logr.Logger
}

// NewPVCService returns a new PVC KubeService.
func NewPVCService(kubeClient client.Client, logger logr.Logger) *PVCService {
	logger = logger.WithName("k8s.pvc")
	return &PVCService{
		kubeClient: kubeClient,
		logger:     logger,
	}
}

func (p *PVCService) GetPVC(ctx context.Context, namespace string, name string) (*corev1.PersistentVolumeClaim, error) {
	pvc := corev1.PersistentVolumeClaim{}
	if err := p.kubeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &pvc); err != nil {
		return nil, err
	}
	return &pvc, nil
}

func (p *PVCService) CreatePVC(ctx context.Context, namespace string, pvc *corev1.PersistentVolumeClaim) error {
	if err := p.kubeClient.Create(ctx, pvc); err != nil {
		return err
	}
	p.logger.V(3).Info("pvc created", "namespace", namespace, "pvc", pvc.Name)

	return nil
}

func (p *PVCService) ListPvcByLabel(ctx context.Context, namespace string, label_map map[string]string) (*corev1.PersistentVolumeClaimList, error) {
	listOps := &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labels.SelectorFromSet(label_map),
	}
	pvclist := &corev1.PersistentVolumeClaimList{}
	err := p.kubeClient.List(ctx, pvclist, listOps)
	return pvclist, err
}
