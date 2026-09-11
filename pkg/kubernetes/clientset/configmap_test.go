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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testCMNamespace = "valkey-test"
	testCMName      = "vkf-acl-inst"
)

// newTestConfigMapClient returns a ConfigMap client backed by a fake API server holding one
// ACL-shaped ConfigMap. The fake client enforces ResourceVersion preconditions the same way
// the API server does, which is what these tests rely on.
func newTestConfigMapClient(t *testing.T, data map[string]string) ConfigMap {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	objs := []runtime.Object{}
	if data != nil {
		objs = append(objs, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: testCMName, Namespace: testCMNamespace},
			Data:       data,
		})
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return NewConfigMap(cli, logr.Discard())
}

// TestUpdateConfigMapRejectsStaleSnapshot is the regression gate for the ACL ConfigMap
// defect: an operator actor loads the ConfigMap, spends several API round-trips elsewhere,
// and by the time it writes back, the User controller has removed a user. Writing the stale
// snapshot used to put the deleted user straight back, permanently.
func TestUpdateConfigMapRejectsStaleSnapshot(t *testing.T) {
	ctx := context.Background()
	svc := newTestConfigMapClient(t, map[string]string{
		"default": "d", "operator": "o", "user1": "u1", "user2": "u2",
	})

	stale, err := svc.GetConfigMap(ctx, testCMNamespace, testCMName)
	require.NoError(t, err)

	// the User controller deletes user1 while the actor is still working
	current, err := svc.GetConfigMap(ctx, testCMNamespace, testCMName)
	require.NoError(t, err)
	delete(current.Data, "user1")
	require.NoError(t, svc.UpdateConfigMap(ctx, testCMNamespace, current))

	// the actor writes back its snapshot, which still carries user1
	stale.Data["operator"] = "o-updated"
	err = svc.UpdateConfigMap(ctx, testCMNamespace, stale)
	assert.True(t, errors.IsConflict(err), "stale write must be reported as a conflict, got %v", err)

	got, err := svc.GetConfigMap(ctx, testCMNamespace, testCMName)
	require.NoError(t, err)
	assert.NotContains(t, got.Data, "user1", "a deleted user must not be resurrected")
}

// TestUpdateConfigMapDoesNotDropConcurrentKey covers the other direction of the same race:
// a stale snapshot must not silently erase a user that was added after it was taken. This
// side is worse for users — the User keeps reporting Ready while access is gone.
func TestUpdateConfigMapDoesNotDropConcurrentKey(t *testing.T) {
	ctx := context.Background()
	svc := newTestConfigMapClient(t, map[string]string{"default": "d", "operator": "o"})

	stale, err := svc.GetConfigMap(ctx, testCMNamespace, testCMName)
	require.NoError(t, err)

	// the User controller adds a new custom user
	current, err := svc.GetConfigMap(ctx, testCMNamespace, testCMName)
	require.NoError(t, err)
	current.Data["user3"] = "u3"
	require.NoError(t, svc.UpdateConfigMap(ctx, testCMNamespace, current))

	stale.Data["operator"] = "o-updated"
	err = svc.UpdateConfigMap(ctx, testCMNamespace, stale)
	assert.True(t, errors.IsConflict(err), "stale write must be reported as a conflict, got %v", err)

	got, err := svc.GetConfigMap(ctx, testCMNamespace, testCMName)
	require.NoError(t, err)
	assert.Contains(t, got.Data, "user3", "a concurrently added user must not be dropped")
}

func TestUpdateConfigMapAcceptsCurrentObject(t *testing.T) {
	ctx := context.Background()
	svc := newTestConfigMapClient(t, map[string]string{"default": "d"})

	current, err := svc.GetConfigMap(ctx, testCMNamespace, testCMName)
	require.NoError(t, err)
	current.Data["default"] = "d-updated"
	require.NoError(t, svc.UpdateConfigMap(ctx, testCMNamespace, current))

	got, err := svc.GetConfigMap(ctx, testCMNamespace, testCMName)
	require.NoError(t, err)
	assert.Equal(t, "d-updated", got.Data["default"])
}

// TestUpdateConfigMapUnversionedObjectOverwrites pins the behaviour the callers that build a
// complete desired object rely on: with no ResourceVersion there is no precondition, so the
// write replaces whatever is stored. ConfigMap's registry strategy allows an unconditional
// update, so the API server adopts the stored version itself — dropping the client-side stamp
// does NOT oblige those callers to route through CreateOrUpdateConfigMap. This test is the
// gate for that assumption; without it, removing the stamp reads as though every builder-fresh
// write had to be rerouted.
func TestUpdateConfigMapUnversionedObjectOverwrites(t *testing.T) {
	ctx := context.Background()
	svc := newTestConfigMapClient(t, map[string]string{"default": "d", "user1": "u1"})

	built := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: testCMName, Namespace: testCMNamespace},
		Data:       map[string]string{"default": "rebuilt"},
	}
	require.NoError(t, svc.UpdateConfigMap(ctx, testCMNamespace, built))

	got, err := svc.GetConfigMap(ctx, testCMNamespace, testCMName)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"default": "rebuilt"}, got.Data)
}

func TestCreateOrUpdateConfigMap(t *testing.T) {
	ctx := context.Background()

	t.Run("creates when absent", func(t *testing.T) {
		svc := newTestConfigMapClient(t, nil)
		built := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: testCMName, Namespace: testCMNamespace},
			Data:       map[string]string{"default": "d"},
		}
		require.NoError(t, svc.CreateOrUpdateConfigMap(ctx, testCMNamespace, built))

		got, err := svc.GetConfigMap(ctx, testCMNamespace, testCMName)
		require.NoError(t, err)
		assert.Equal(t, "d", got.Data["default"])
	})

	// What the builder-fresh callers rely on: an object with no ResourceVersion is the
	// whole desired content, so the stored version is adopted and the write replaces it.
	t.Run("adopts stored version for a freshly built object", func(t *testing.T) {
		svc := newTestConfigMapClient(t, map[string]string{"default": "d", "user1": "u1"})
		built := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: testCMName, Namespace: testCMNamespace},
			Data:       map[string]string{"default": "rebuilt"},
		}
		require.NoError(t, svc.CreateOrUpdateConfigMap(ctx, testCMNamespace, built))

		got, err := svc.GetConfigMap(ctx, testCMNamespace, testCMName)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"default": "rebuilt"}, got.Data)
	})

	// The defect path: the actor passed an object it had read minutes earlier. Adopting the
	// stored version there is what made every such write unconditional.
	t.Run("keeps the caller's version for an object it read", func(t *testing.T) {
		svc := newTestConfigMapClient(t, map[string]string{"default": "d", "user1": "u1"})

		stale, err := svc.GetConfigMap(ctx, testCMNamespace, testCMName)
		require.NoError(t, err)

		current, err := svc.GetConfigMap(ctx, testCMNamespace, testCMName)
		require.NoError(t, err)
		delete(current.Data, "user1")
		require.NoError(t, svc.UpdateConfigMap(ctx, testCMNamespace, current))

		err = svc.CreateOrUpdateConfigMap(ctx, testCMNamespace, stale)
		assert.True(t, errors.IsConflict(err), "stale write must be reported as a conflict, got %v", err)

		got, err := svc.GetConfigMap(ctx, testCMNamespace, testCMName)
		require.NoError(t, err)
		assert.NotContains(t, got.Data, "user1", "a deleted user must not be resurrected")
	})
}

// UpdateIfConfigMapChanged is handed builder-fresh objects by its callers, so it rides the
// unconditional-overwrite path: replacing the content it just found to differ is the intent.
func TestUpdateIfConfigMapChanged(t *testing.T) {
	ctx := context.Background()
	svc := newTestConfigMapClient(t, map[string]string{"valkey.conf": "old"})

	built := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: testCMName, Namespace: testCMNamespace},
		Data:       map[string]string{"valkey.conf": "new"},
	}
	require.NoError(t, svc.UpdateIfConfigMapChanged(ctx, built))

	got, err := svc.GetConfigMap(ctx, testCMNamespace, testCMName)
	require.NoError(t, err)
	assert.Equal(t, "new", got.Data["valkey.conf"])
}
