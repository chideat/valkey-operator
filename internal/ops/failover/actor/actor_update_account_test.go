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
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder/aclbuilder"
	"github.com/chideat/valkey-operator/pkg/kubernetes/clientset"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// The ACL ConfigMap holds accounts this actor does not own — the custom Users the User
// controller manages. Dispatching the actor must never remove one.
//
// This is a plain two-dispatch test on purpose: no injected competing write, no interceptor.
// The defect it gates did not need a race. The actor rebuilt oldCm from the users it had
// loaded whenever the ConfigMap was NOT absent — the branch was `else`, not
// `else if oldCm == nil` — and wrote that object back whole, so a custom account was erased
// (and a deleted one resurrected) on EVERY dispatch. A test that needs a race to fail would
// have let that through.
func TestUpdateAccountKeepsAccountsItDoesNotOwn(t *testing.T) {
	ctx := context.Background()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	inst := newTestFailoverInstance("test-failover", "default", nil)
	cmName := aclbuilder.GenerateACLConfigMapName(core.ValkeyFailover, inst.GetName())

	const customEntry = `{"name":"app","role":"Developer","rules":[{"categories":["all"],"keyPatterns":["*"]}]}`
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: inst.GetNamespace()},
			Data:       map[string]string{"app": customEntry},
		}).Build()

	a := NewUpdateAccountActor(clientset.New(cli, logr.Discard()), logr.Discard())

	// Two dispatches: the first creates the operator account, the second is the steady-state
	// pass that used to overwrite the ConfigMap with a stale rebuild.
	for i := range 2 {
		if ret := a.Do(ctx, inst); ret != nil {
			require.NoError(t, ret.Err(), "dispatch %d returned an error", i)
		}

		cm := &corev1.ConfigMap{}
		require.NoError(t, cli.Get(ctx, client.ObjectKey{Namespace: inst.GetNamespace(), Name: cmName}, cm))
		assert.Equal(t, customEntry, cm.Data["app"],
			"dispatch %d erased the custom account the User controller owns", i)
	}
}
