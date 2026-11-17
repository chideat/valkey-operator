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

package rds

import (
	"context"
	"errors"
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	rdsv1alpha1 "github.com/chideat/valkey-operator/api/rds/v1alpha1"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder/aclbuilder"
	security "github.com/chideat/valkey-operator/pkg/security/password"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

const sentinelPasswordInst = "vk-failover"

func newSentinelPasswordReconciler(t *testing.T, funcs *interceptor.Funcs, secretName string) (*ValkeyReconciler, client.Client, *rdsv1alpha1.Valkey) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, rdsv1alpha1.AddToScheme(scheme))
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	inst := &rdsv1alpha1.Valkey{
		ObjectMeta: metav1.ObjectMeta{Name: sentinelPasswordInst, Namespace: "default"},
		Spec: rdsv1alpha1.ValkeySpec{
			Version:  "8.0",
			Arch:     core.ValkeyFailover,
			Replicas: &rdsv1alpha1.ValkeyReplicas{Shards: 1, ReplicasOfShard: 2},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
			Sentinel: &v1alpha1.SentinelSettings{
				SentinelSpec: v1alpha1.SentinelSpec{
					Replicas: 3,
					Access:   v1alpha1.SentinelInstanceAccess{DefaultPasswordSecret: secretName},
				},
			},
		},
	}
	b := fake.NewClientBuilder().WithScheme(scheme).WithObjects(inst)
	if funcs != nil {
		b = b.WithInterceptorFuncs(*funcs)
	}
	cli := b.Build()

	stored := &rdsv1alpha1.Valkey{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(inst), stored))
	return &ValkeyReconciler{Client: cli, Scheme: scheme}, cli, stored
}

// The admission webhook names a secret the operator has to generate, and the sentinel
// nodes read it from their first start. It has to exist before the failover that
// creates them, hold a password the webhook accepts, and survive a second reconcile
// unchanged.
func TestReconcileFailoverGeneratesTheSentinelPassword(t *testing.T) {
	ctx := context.Background()
	reserved := aclbuilder.GenerateACLDefaultSecretName(core.ValkeySentinel, sentinelPasswordInst)
	r, cli, inst := newSentinelPasswordReconciler(t, nil, reserved)

	require.NoError(t, r.reconcileFailover(ctx, inst, logr.Discard()))

	secret := &corev1.Secret{}
	require.NoError(t, cli.Get(ctx, client.ObjectKey{Namespace: "default", Name: reserved}, secret))
	password := string(secret.Data["password"])
	require.NoError(t, security.PasswordValidate(password, 8, 32))
	assert.Equal(t, "default", string(secret.Data["username"]))

	failover := &v1alpha1.Failover{}
	require.NoError(t, cli.Get(ctx, client.ObjectKey{Namespace: "default", Name: sentinelPasswordInst}, failover))
	assert.Equal(t, reserved, failover.Spec.Sentinel.Access.DefaultPasswordSecret)

	require.NoError(t, r.reconcileFailover(ctx, inst, logr.Discard()))
	require.NoError(t, cli.Get(ctx, client.ObjectKey{Namespace: "default", Name: reserved}, secret))
	assert.Equal(t, password, string(secret.Data["password"]), "a later reconcile replaced the password")
}

// The failover must not be created while the secret its sentinel nodes read is missing,
// and the retry must keep the password of a secret that did get created.
func TestReconcileFailoverCreatesNoFailoverWithoutTheSentinelPassword(t *testing.T) {
	ctx := context.Background()
	reserved := aclbuilder.GenerateACLDefaultSecretName(core.ValkeySentinel, sentinelPasswordInst)
	failed := false
	r, cli, inst := newSentinelPasswordReconciler(t, &interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if _, ok := obj.(*corev1.Secret); ok && !failed {
				failed = true
				return errors.New("injected failure creating the sentinel password secret")
			}
			return c.Create(ctx, obj, opts...)
		},
	}, reserved)

	require.Error(t, r.reconcileFailover(ctx, inst, logr.Discard()))
	err := cli.Get(ctx, client.ObjectKey{Namespace: "default", Name: sentinelPasswordInst}, &v1alpha1.Failover{})
	require.True(t, apierrors.IsNotFound(err), "the failover was created without its sentinel password secret: %v", err)

	require.NoError(t, r.reconcileFailover(ctx, inst, logr.Discard()))
	require.NoError(t, cli.Get(ctx, client.ObjectKey{Namespace: "default", Name: reserved}, &corev1.Secret{}))
	require.NoError(t, cli.Get(ctx, client.ObjectKey{Namespace: "default", Name: sentinelPasswordInst}, &v1alpha1.Failover{}))
}

// A secret the user names is the user's to provide: the operator generates only the
// one the webhook reserves.
func TestReconcileFailoverLeavesTheUsersSentinelSecretAlone(t *testing.T) {
	ctx := context.Background()
	r, cli, inst := newSentinelPasswordReconciler(t, nil, "my-sentinel-password")

	require.NoError(t, r.reconcileFailover(ctx, inst, logr.Discard()))

	err := cli.Get(ctx, client.ObjectKey{Namespace: "default", Name: "my-sentinel-password"}, &corev1.Secret{})
	assert.True(t, apierrors.IsNotFound(err), "the operator generated a secret the user named: %v", err)
	err = cli.Get(ctx, client.ObjectKey{
		Namespace: "default",
		Name:      aclbuilder.GenerateACLDefaultSecretName(core.ValkeySentinel, sentinelPasswordInst),
	}, &corev1.Secret{})
	assert.True(t, apierrors.IsNotFound(err), "the operator generated the reserved secret no field names: %v", err)
}
