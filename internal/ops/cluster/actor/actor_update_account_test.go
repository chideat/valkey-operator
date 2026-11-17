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

package actor

import (
	"context"
	"errors"
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder/aclbuilder"
	cops "github.com/chideat/valkey-operator/internal/ops/cluster"
	"github.com/chideat/valkey-operator/internal/testutil"
	"github.com/chideat/valkey-operator/pkg/kubernetes/clientset"
	security "github.com/chideat/valkey-operator/pkg/security/password"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func newAccountTestCluster(name, ns string) *testutil.FakeClusterInstance {
	return testutil.NewFakeClusterInstance(&v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
	})
}

func newAccountTestClient(t *testing.T, funcs *interceptor.Funcs, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	b := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...)
	if funcs != nil {
		b = b.WithInterceptorFuncs(*funcs)
	}
	return b.Build()
}

// A new instance has no account at all, and its default user is what every client
// without a username reaches. It must get a password when the operator sets the
// instance up, held in a User of its own: that User is the one place the password is
// managed, so the ACL ConfigMap entry is left to the User controller.
func TestUpdateAccountGivesANewInstanceADefaultUser(t *testing.T) {
	ctx := context.Background()
	inst := newAccountTestCluster("test-cluster", "default")
	cli := newAccountTestClient(t, nil)
	a := NewUpdateAccountActor(clientset.New(cli, logr.Discard()), logr.Discard())

	if ret := a.Do(ctx, inst); ret != nil {
		require.NoError(t, ret.Err())
	}

	secretName := aclbuilder.GenerateACLDefaultSecretName(core.ValkeyCluster, inst.GetName())
	secret := &corev1.Secret{}
	require.NoError(t, cli.Get(ctx, client.ObjectKey{Namespace: inst.GetNamespace(), Name: secretName}, secret))
	// The User webhook and controller and the helper that renders the ACL file all cap a
	// password other than the operator's at 32 characters.
	require.NoError(t, security.PasswordValidate(string(secret.Data["password"]), 8, 32))

	ru := &v1alpha1.User{}
	require.NoError(t, cli.Get(ctx, client.ObjectKey{
		Namespace: inst.GetNamespace(),
		Name:      aclbuilder.GenerateDefaultUserResourceName(core.ValkeyCluster, inst.GetName()),
	}, ru))
	assert.Equal(t, "default", ru.Spec.Username)
	assert.Equal(t, v1alpha1.CustomAccount, ru.Spec.AccountType)
	assert.Equal(t, core.ValkeyCluster, ru.Spec.Arch)
	assert.Equal(t, inst.GetName(), ru.Spec.InstanceName)
	assert.Equal(t, []string{secretName}, ru.Spec.PasswordSecrets)
	assert.Equal(t, "+@all -acl ~* &*", ru.Spec.AclRules)

	require.NoError(t, cli.Get(ctx, client.ObjectKey{
		Namespace: inst.GetNamespace(),
		Name:      aclbuilder.GenerateOperatorUserResourceName(core.ValkeyCluster, inst.GetName()),
	}, &v1alpha1.User{}), "the operator account is still set up")

	cm := &corev1.ConfigMap{}
	require.NoError(t, cli.Get(ctx, client.ObjectKey{
		Namespace: inst.GetNamespace(),
		Name:      aclbuilder.GenerateACLConfigMapName(core.ValkeyCluster, inst.GetName()),
	}, cm))
	assert.Contains(t, cm.Data, "operator")
	assert.NotContains(t, cm.Data, "default", "the default user's entry is the User controller's to write")
}

// An instance that already has its operator account was set up before, by this operator
// or an earlier one, and its clients may reach the default user without a password.
// Giving that user one now would lock them out at the next pod restart.
func TestUpdateAccountLeavesTheDefaultUserOfAnExistingInstanceAlone(t *testing.T) {
	ctx := context.Background()
	opSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aclbuilder.GenerateACLOperatorSecretName(core.ValkeyCluster, "test-cluster"),
			Namespace: "default",
		},
		Data: map[string][]byte{"password": []byte("Operat0r!Passw0rd")},
	}
	opUser, err := types.NewOperatorUser(opSecret)
	require.NoError(t, err)
	inst := newAccountTestCluster("test-cluster", "default").WithUsers(opUser)
	cli := newAccountTestClient(t, nil, opSecret)
	a := NewUpdateAccountActor(clientset.New(cli, logr.Discard()), logr.Discard())

	if ret := a.Do(ctx, inst); ret != nil {
		require.NoError(t, ret.Err())
	}

	err = cli.Get(ctx, client.ObjectKey{
		Namespace: inst.GetNamespace(),
		Name:      aclbuilder.GenerateACLDefaultSecretName(core.ValkeyCluster, inst.GetName()),
	}, &corev1.Secret{})
	assert.True(t, apierrors.IsNotFound(err), "default password secret created for an existing instance: %v", err)
	err = cli.Get(ctx, client.ObjectKey{
		Namespace: inst.GetNamespace(),
		Name:      aclbuilder.GenerateDefaultUserResourceName(core.ValkeyCluster, inst.GetName()),
	}, &v1alpha1.User{})
	assert.True(t, apierrors.IsNotFound(err), "default User created for an existing instance: %v", err)
}

// The branch that sets up the default user is entered only while the operator account
// is missing. A failure partway must leave the operator account unmade, so that the
// next dispatch enters the branch again, and that dispatch must reuse the password
// already generated rather than make a second one.
func TestUpdateAccountRetriesTheDefaultUserAfterAFailure(t *testing.T) {
	ctx := context.Background()
	inst := newAccountTestCluster("test-cluster", "default")
	defaultUserName := aclbuilder.GenerateDefaultUserResourceName(core.ValkeyCluster, inst.GetName())
	opUserName := aclbuilder.GenerateOperatorUserResourceName(core.ValkeyCluster, inst.GetName())
	secretKey := client.ObjectKey{
		Namespace: inst.GetNamespace(),
		Name:      aclbuilder.GenerateACLDefaultSecretName(core.ValkeyCluster, inst.GetName()),
	}

	failed := false
	cli := newAccountTestClient(t, &interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if _, ok := obj.(*v1alpha1.User); ok && obj.GetName() == defaultUserName && !failed {
				failed = true
				return errors.New("injected failure creating the default User")
			}
			return c.Create(ctx, obj, opts...)
		},
	})
	a := NewUpdateAccountActor(clientset.New(cli, logr.Discard()), logr.Discard())

	ret := a.Do(ctx, inst)
	require.NotNil(t, ret)
	assert.Equal(t, cops.CommandRequeue, ret.NextCommand())
	err := cli.Get(ctx, client.ObjectKey{Namespace: inst.GetNamespace(), Name: opUserName}, &v1alpha1.User{})
	require.True(t, apierrors.IsNotFound(err),
		"the operator account was created although the default user failed; the next dispatch would skip the default user: %v", err)
	first := &corev1.Secret{}
	require.NoError(t, cli.Get(ctx, secretKey, first))

	if ret := a.Do(ctx, inst); ret != nil {
		require.NoError(t, ret.Err())
	}
	require.NoError(t, cli.Get(ctx, client.ObjectKey{Namespace: inst.GetNamespace(), Name: defaultUserName}, &v1alpha1.User{}))
	require.NoError(t, cli.Get(ctx, client.ObjectKey{Namespace: inst.GetNamespace(), Name: opUserName}, &v1alpha1.User{}))
	second := &corev1.Secret{}
	require.NoError(t, cli.Get(ctx, secretKey, second))
	assert.Equal(t, string(first.Data["password"]), string(second.Data["password"]), "the retry generated a second password")
}
