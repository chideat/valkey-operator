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

package acl

import (
	"context"
	"testing"

	"github.com/chideat/valkey-operator/pkg/kubernetes/clientset"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/types/user"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

const (
	applyNamespace = "valkey-test"
	applyCMName    = "vkf-acl-inst"
)

func mustUser(t *testing.T, name string) *user.User {
	t.Helper()
	u, err := types.NewUserFromValkeyUser(name, "+@all ~* &*", &user.Password{})
	require.NoError(t, err)
	return u
}

func applyTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	return scheme
}

func aclConfigMap(data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: applyCMName, Namespace: applyNamespace},
		Data:       data,
	}
}

// TestApplyUsersToConfigMapKeepsForeignEntries pins the property that makes the actor safe:
// it writes only the accounts it owns, so a custom account managed by the User controller
// survives even though the actor never knew about it.
func TestApplyUsersToConfigMapKeepsForeignEntries(t *testing.T) {
	ctx := context.Background()
	cli := fake.NewClientBuilder().WithScheme(applyTestScheme(t)).
		WithObjects(aclConfigMap(map[string]string{
			"default": "stale", "operator": "stale", "custom-user": "owned-by-user-controller",
		})).Build()
	cs := clientset.New(cli, logr.Discard())

	users := types.Users{mustUser(t, "default"), mustUser(t, "operator")}
	require.NoError(t, ApplyUsersToConfigMap(ctx, cs, applyNamespace, applyCMName, users))

	got, err := cs.GetConfigMap(ctx, applyNamespace, applyCMName)
	require.NoError(t, err)
	assert.Equal(t, "owned-by-user-controller", got.Data["custom-user"], "a foreign entry must be left alone")
	assert.NotEqual(t, "stale", got.Data["default"], "the caller's own entries must be written")
	assert.NotEqual(t, "stale", got.Data["operator"], "the caller's own entries must be written")
}

// TestApplyUsersToConfigMapRetriesOnConflict simulates the real race: another writer lands a
// change between this call's read and its write. The update must lose, be retried against the
// newer content, and leave the other writer's change intact.
func TestApplyUsersToConfigMapRetriesOnConflict(t *testing.T) {
	ctx := context.Background()

	var raced bool
	cli := fake.NewClientBuilder().WithScheme(applyTestScheme(t)).
		WithObjects(aclConfigMap(map[string]string{"default": "stale", "operator": "stale"})).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(ctx context.Context, c ctrlclient.WithWatch, obj ctrlclient.Object,
				opts ...ctrlclient.UpdateOption) error {

				if !raced {
					raced = true
					// a User reconcile adds a custom account first, invalidating the
					// version the caller is about to submit
					var cm corev1.ConfigMap
					if err := c.Get(ctx, ctrlclient.ObjectKey{
						Namespace: applyNamespace, Name: applyCMName,
					}, &cm); err != nil {
						return err
					}
					cm.Data["late-user"] = "added-mid-flight"
					if err := c.Update(ctx, &cm); err != nil {
						return err
					}
				}
				return c.Update(ctx, obj, opts...)
			},
		}).Build()
	cs := clientset.New(cli, logr.Discard())

	users := types.Users{mustUser(t, "default"), mustUser(t, "operator")}
	require.NoError(t, ApplyUsersToConfigMap(ctx, cs, applyNamespace, applyCMName, users),
		"the conflict must be absorbed by the retry")
	assert.True(t, raced, "the interceptor should have injected a competing write")

	got, err := cs.GetConfigMap(ctx, applyNamespace, applyCMName)
	require.NoError(t, err)
	assert.Equal(t, "added-mid-flight", got.Data["late-user"], "the competing write must survive")
	assert.NotEqual(t, "stale", got.Data["default"], "the caller's own entries must still land")
}

func TestApplyUsersToConfigMapMissingConfigMap(t *testing.T) {
	ctx := context.Background()
	cli := fake.NewClientBuilder().WithScheme(applyTestScheme(t)).Build()
	cs := clientset.New(cli, logr.Discard())

	err := ApplyUsersToConfigMap(ctx, cs, applyNamespace, applyCMName, types.Users{mustUser(t, "default")})
	assert.Error(t, err, "a missing ConfigMap must be reported, not silently created")
}
