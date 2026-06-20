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
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestUpdatePodMergesLabels is the R4 regression: UpdatePod merges labels and
// annotations into the freshly-fetched oldPod, so it must submit oldPod (with the
// server ResourceVersion) — not the caller's stale pod argument.
func TestUpdatePodMergesLabels(t *testing.T) {
	ctx := context.Background()
	ns := "default"

	newPodClient := func(objs ...*corev1.Pod) Pod {
		builder := fake.NewClientBuilder()
		for _, o := range objs {
			builder = builder.WithObjects(o)
		}
		return NewPod(builder.Build(), nil, logr.Discard())
	}

	t.Run("merges new labels/annotations onto the stored pod", func(t *testing.T) {
		existing := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-pod",
				Namespace:   ns,
				Labels:      map[string]string{"app": "v1"},
				Annotations: map[string]string{"a": "1"},
			},
		}
		podClient := newPodClient(existing)

		err := podClient.UpdatePod(ctx, ns, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-pod",
				Namespace:   ns,
				Labels:      map[string]string{"app": "v2", "new-key": "new-val"},
				Annotations: map[string]string{"b": "2"},
			},
		})
		require.NoError(t, err)

		fetched, err := podClient.GetPod(ctx, ns, "test-pod")
		require.NoError(t, err)
		assert.Equal(t, "v2", fetched.Labels["app"])
		assert.Equal(t, "new-val", fetched.Labels["new-key"])
		assert.Equal(t, "1", fetched.Annotations["a"])
		assert.Equal(t, "2", fetched.Annotations["b"])
	})

	t.Run("returns nil when the pod does not exist", func(t *testing.T) {
		podClient := newPodClient()
		err := podClient.UpdatePod(ctx, ns, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "missing", Namespace: ns},
		})
		assert.NoError(t, err)
	})

	t.Run("preserves stored labels not present in the update", func(t *testing.T) {
		existing := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "keep-pod",
				Namespace: ns,
				Labels:    map[string]string{"existing": "kept", "app": "v1"},
			},
		}
		podClient := newPodClient(existing)

		err := podClient.UpdatePod(ctx, ns, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "keep-pod",
				Namespace: ns,
				Labels:    map[string]string{"app": "v2"},
			},
		})
		require.NoError(t, err)

		fetched, err := podClient.GetPod(ctx, ns, "keep-pod")
		require.NoError(t, err)
		assert.Equal(t, "kept", fetched.Labels["existing"])
		assert.Equal(t, "v2", fetched.Labels["app"])
	})
}
