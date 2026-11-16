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
	security "github.com/chideat/valkey-operator/pkg/security/password"
	"github.com/chideat/valkey-operator/pkg/types/user"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const defaultSecretName = "failover-acl-inst-default-secret"

// The generated password has to pass every check a password other than the operator's
// meets on its way to the nodes. The helper that renders the ACL file leaves out, without
// an error, an account whose password fails PasswordValidate(8, 32), so a password
// generated at the operator's own length would leave the default user out of the file
// and the instance would start with that user open.
func TestNewDefaultUserGeneratesAnAcceptedPassword(t *testing.T) {
	ctx := context.Background()
	cli := fake.NewClientBuilder().WithScheme(applyTestScheme(t)).Build()

	u, err := NewDefaultUser(ctx, clientset.New(cli, logr.Discard()), defaultSecretName, applyNamespace, nil)
	require.NoError(t, err)

	assert.Equal(t, user.DefaultUserName, u.Name)
	assert.Equal(t, user.RoleDeveloper, u.Role)
	require.Len(t, u.Rules, 1)
	assert.Equal(t, "+@all -acl ~* &*", u.Rules[0].Encode())
	assert.Equal(t, defaultSecretName, u.Password.GetSecretName())
	require.NoError(t, security.PasswordValidate(u.Password.String(), 8, 32))

	secret := &corev1.Secret{}
	require.NoError(t, cli.Get(ctx, ctrlclient.ObjectKey{Namespace: applyNamespace, Name: defaultSecretName}, secret))
	assert.Equal(t, u.Password.String(), string(secret.Data["password"]))
	assert.Equal(t, user.DefaultUserName, string(secret.Data["username"]))
}

// The account actor calls this again after a partial failure. The password must stay
// the one generated first: the secret may already be referenced by the default User.
func TestNewDefaultUserReusesTheExistingSecret(t *testing.T) {
	ctx := context.Background()
	const existing = "Exist1ng!Passw0rd"
	cli := fake.NewClientBuilder().WithScheme(applyTestScheme(t)).WithObjects(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: defaultSecretName, Namespace: applyNamespace},
		Data:       map[string][]byte{"password": []byte(existing)},
	}).Build()

	u, err := NewDefaultUser(ctx, clientset.New(cli, logr.Discard()), defaultSecretName, applyNamespace, nil)
	require.NoError(t, err)
	assert.Equal(t, existing, u.Password.String())

	secret := &corev1.Secret{}
	require.NoError(t, cli.Get(ctx, ctrlclient.ObjectKey{Namespace: applyNamespace, Name: defaultSecretName}, secret))
	assert.Equal(t, existing, string(secret.Data["password"]))
}
