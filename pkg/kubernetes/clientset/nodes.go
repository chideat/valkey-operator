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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Node interface {
	ListNodesByLabels(ctx context.Context, label_map map[string]string) (*corev1.NodeList, error)
	GetNode(ctx context.Context, name string) (*corev1.Node, error)
}

type NodeOption struct {
	client client.Client
	logger logr.Logger
}

func NewNode(kubeClient client.Client, logger logr.Logger) Node {
	logger = logger.WithValues("service", "k8s.pod")
	return &NodeOption{
		client: kubeClient,
		logger: logger,
	}
}

func (p *NodeOption) ListNodesByLabels(ctx context.Context, label_map map[string]string) (*corev1.NodeList, error) {
	listOps := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(label_map),
	}
	nodelist := &corev1.NodeList{}
	err := p.client.List(context.TODO(), nodelist, listOps)
	return nodelist, err
}

func (p *NodeOption) GetNode(ctx context.Context, name string) (*corev1.Node, error) {
	node := &corev1.Node{}
	err := p.client.Get(context.TODO(), types.NamespacedName{
		Name: name,
	}, node)
	if err != nil {
		return nil, err
	}
	return node, err
}
