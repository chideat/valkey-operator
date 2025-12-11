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
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServiceOption_CRUD(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	logger := logr.Discard()
	svcClient := NewService(client, logger)
	ctx := context.Background()
	ns := "default"
	name := "test-service"

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"app": "test",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Port: 80},
			},
		},
	}

	// Test Create
	err := svcClient.CreateService(ctx, ns, svc)
	assert.NoError(t, err)

	// Test Get
	found, err := svcClient.GetService(ctx, ns, name)
	assert.NoError(t, err)
	assert.Equal(t, name, found.Name)

	// Test CreateIfNotExists (exists)
	err = svcClient.CreateIfNotExistsService(ctx, ns, svc)
	assert.NoError(t, err)

	// Test Update
	svc.Labels["new-label"] = "new-value"
	err = svcClient.UpdateService(ctx, ns, svc)
	assert.NoError(t, err)

	found, err = svcClient.GetService(ctx, ns, name)
	assert.NoError(t, err)
	assert.Equal(t, "new-value", found.Labels["new-label"])

	// Test CreateOrUpdateService
	svc.Annotations = map[string]string{"anno": "val"}
	err = svcClient.CreateOrUpdateService(ctx, ns, svc)
	assert.NoError(t, err)

	found, err = svcClient.GetService(ctx, ns, name)
	assert.NoError(t, err)
	assert.Equal(t, "val", found.Annotations["anno"])

	// Test Delete
	err = svcClient.DeleteService(ctx, ns, name)
	assert.NoError(t, err)

	_, err = svcClient.GetService(ctx, ns, name)
	assert.Error(t, err)
}

func TestServiceOption_UpdateIfSelectorChangedService(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	logger := logr.Discard()
	svcClient := NewService(client, logger)
	ctx := context.Background()
	ns := "default"

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "selector-service",
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "v1"},
		},
	}

	err := svcClient.CreateService(ctx, ns, svc)
	assert.NoError(t, err)

	// No change
	err = svcClient.UpdateIfSelectorChangedService(ctx, ns, svc)
	assert.NoError(t, err)

	// Change selector
	svc.Spec.Selector = map[string]string{"app": "v2"}
	err = svcClient.UpdateIfSelectorChangedService(ctx, ns, svc)
	assert.NoError(t, err)

	found, err := svcClient.GetService(ctx, ns, "selector-service")
	assert.NoError(t, err)
	assert.Equal(t, "v2", found.Spec.Selector["app"])
}
