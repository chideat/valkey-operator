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
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
	"github.com/chideat/valkey-operator/internal/testutil"
	"github.com/chideat/valkey-operator/pkg/kubernetes/clientset/mocks"
	"github.com/chideat/valkey-operator/pkg/types/user"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_parsePodShardAndIndex(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name      string
		args      args
		wantShard int
		wantIndex int
		wantErr   bool
	}{
		{
			name:      "name ok",
			args:      args{name: "vkc-valkey-1-1"},
			wantShard: 1,
			wantIndex: 1,
			wantErr:   false,
		},
		{
			name:      "name ok",
			args:      args{name: "vkc----valkey-0-0"},
			wantShard: 0,
			wantIndex: 0,
			wantErr:   false,
		},
		{
			name:      "name error",
			args:      args{name: "vkc-valkey-1"},
			wantShard: -1,
			wantIndex: -1,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotShard, gotIndex, err := builder.ParsePodShardAndIndex(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("%s parsePodShardAndIndex() error = %v, wantErr %v", tt.name, err, tt.wantErr)
				return
			}
			if gotShard != tt.wantShard {
				t.Errorf("%s parsePodShardAndIndex() gotShard = %v, want %v", tt.name, gotShard, tt.wantShard)
			}
			if gotIndex != tt.wantIndex {
				t.Errorf("%s parsePodShardAndIndex() gotIndex = %v, want %v", tt.name, gotIndex, tt.wantIndex)
			}
		})
	}
}

// TestEnsureStatefulsetNoticesPodDisruptionBudgetOverwrites: a shard's budget
// is updated when its overwrites are added or removed, although its spec stays
// the same. The StatefulSet lookup fails on purpose, to stop after the budget.
func TestEnsureStatefulsetNoticesPodDisruptionBudgetOverwrites(t *testing.T) {
	labels := core.Overwrite{Kind: core.OverwriteKindPodDisruptionBudget, Patch: apiextensionsv1.JSON{Raw: []byte(`{"metadata":{"labels":{"team":"cache"}}}`)}}
	resources := corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m"), corev1.ResourceMemory: resource.MustParse("200Mi")}
	newInst := func(overwrites ...core.Overwrite) *testutil.FakeClusterInstance {
		return testutil.NewFakeClusterInstance(&v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
			Spec: v1alpha1.ClusterSpec{
				Image:      "valkey/valkey:8.1",
				Replicas:   v1alpha1.ClusterReplicas{Shards: 1, ReplicasOfShard: 2},
				Resources:  corev1.ResourceRequirements{Limits: resources, Requests: resources},
				Overwrites: overwrites,
			},
		}).WithUsers(&user.User{Name: user.DefaultOperatorUserName, Role: user.RoleOperator, Password: &user.Password{SecretName: "operator"}})
	}
	live := func(overwrites ...core.Overwrite) *policyv1.PodDisruptionBudget {
		pdb, err := clusterbuilder.GeneratePodDisruptionBudget(newInst(overwrites...), 0)
		require.NoError(t, err)
		return pdb
	}
	for _, tc := range []struct {
		name   string
		live   *policyv1.PodDisruptionBudget
		inst   *testutil.FakeClusterInstance
		update bool
	}{
		{"no overwrites", live(), newInst(), false},
		{"the same overwrites", live(labels), newInst(labels), false},
		{"overwrites added", live(), newInst(labels), true},
		{"overwrites removed", live(labels), newInst(), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			clientMock := &mocks.ClientSet{}
			clientMock.On("GetPodDisruptionBudget", ctx, "default", tc.live.Name).Return(tc.live, nil)
			clientMock.On("UpdatePodDisruptionBudget", ctx, "default", mock.Anything).Return(nil).Maybe()
			clientMock.On("GetStatefulSet", ctx, "default", mock.Anything).Return(nil, errors.New("stop after the budget"))

			a := &actorEnsureResource{client: clientMock, logger: logr.Discard()}
			assert.NotNil(t, a.ensureStatefulset(ctx, tc.inst, logr.Discard()))
			clientMock.AssertCalled(t, "GetStatefulSet", ctx, "default", mock.Anything)
			if tc.update {
				clientMock.AssertCalled(t, "UpdatePodDisruptionBudget", ctx, "default", mock.Anything)
			} else {
				clientMock.AssertNotCalled(t, "UpdatePodDisruptionBudget", ctx, "default", mock.Anything)
			}
		})
	}
}
