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

// TestEnsureServiceNoticesOverwrites: the headless and instance Services are
// only created, so without overwrites a live one that differs stays as it is;
// a change of their overwrites updates them. The node port Services keep the
// ports the operator manages on the live objects.
func TestEnsureServiceNoticesOverwrites(t *testing.T) {
	label := func(target core.OverwriteTarget) core.Overwrite {
		return core.Overwrite{Kind: core.OverwriteKindService, Target: target,
			Patch: apiextensionsv1.JSON{Raw: []byte(`{"metadata":{"labels":{"team":"cache"}}}`)}}
	}
	headless, instance, pod := label(core.OverwriteTargetHeadless), label(core.OverwriteTargetInstance), label(core.OverwriteTargetPod)
	clusterIP := core.InstanceAccess{ServiceType: corev1.ServiceTypeClusterIP}
	nodePort := core.InstanceAccess{ServiceType: corev1.ServiceTypeNodePort, Ports: "30001"}
	newInst := func(access core.InstanceAccess, overwrites ...core.Overwrite) *testutil.FakeClusterInstance {
		return testutil.NewFakeClusterInstance(&v1alpha1.Cluster{
			// A kind and a UID give the Services an owner reference.
			TypeMeta:   metav1.TypeMeta{Kind: "Cluster", APIVersion: v1alpha1.GroupVersion.String()},
			ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "uid"},
			Spec: v1alpha1.ClusterSpec{
				Replicas:   v1alpha1.ClusterReplicas{Shards: 1, ReplicasOfShard: 1},
				Access:     access,
				Overwrites: overwrites,
			},
		})
	}
	// live holds the Services as the operator wrote them for overwrites; a
	// node port Service has the gossip port the actor adds later.
	live := func(access core.InstanceAccess, overwrites ...core.Overwrite) (map[string]*corev1.Service, []corev1.Service) {
		inst := newInst(access, overwrites...)
		name := clusterbuilder.ClusterNodeServiceName("demo", 0, 0)
		headless, err := clusterbuilder.GenerateHeadlessService(inst, 0)
		require.NoError(t, err)
		instance, err := clusterbuilder.GenerateInstanceService(inst)
		require.NoError(t, err)
		var pod *corev1.Service
		if access.ServiceType == corev1.ServiceTypeNodePort {
			pod, err = clusterbuilder.GenerateNodePortService(inst, name, clusterbuilder.GenerateClusterLabels("demo", nil), 30001)
			pod.Spec.Ports = append(pod.Spec.Ports, corev1.ServicePort{Name: "gossip", Port: 16379, NodePort: 31000})
		} else {
			pod, err = clusterbuilder.GeneratePodService(inst, name, access.ServiceType, access.Annotations)
		}
		require.NoError(t, err)
		return map[string]*corev1.Service{headless.Name: headless, instance.Name: instance, pod.Name: pod}, []corev1.Service{*pod}
	}
	for _, tc := range []struct {
		name       string
		access     core.InstanceAccess
		live, inst []core.Overwrite
		changeLive func(map[string]*corev1.Service)
		updated    []string
	}{
		{"no overwrites", clusterIP, nil, nil, nil, nil},
		{"no overwrites, and live Services that differ", clusterIP, nil, nil, func(services map[string]*corev1.Service) {
			services["demo-0"].Spec.Ports = services["demo-0"].Spec.Ports[:1]
			services["demo"].Labels["x"] = "y"
		}, nil},
		{"the same overwrites", clusterIP, []core.Overwrite{headless, instance, pod}, []core.Overwrite{headless, instance, pod}, nil, nil},
		{"overwrites added", clusterIP, nil, []core.Overwrite{headless, instance, pod}, nil, []string{"demo-0", "demo", "drc-demo-0-0"}},
		{"overwrites removed", clusterIP, []core.Overwrite{headless, instance, pod}, nil, nil, []string{"demo-0", "demo", "drc-demo-0-0"}},
		{"node port overwrites added", nodePort, nil, []core.Overwrite{pod}, nil, []string{"drc-demo-0-0"}},
		{"node port overwrites removed", nodePort, []core.Overwrite{pod}, nil, nil, []string{"drc-demo-0-0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			services, pods := live(tc.access, tc.live...)
			if tc.changeLive != nil {
				tc.changeLive(services)
			}
			clientMock := &mocks.ClientSet{}
			clientMock.On("GetServiceByLabels", ctx, "default", mock.Anything).Return(&corev1.ServiceList{Items: pods}, nil)
			for name, svc := range services {
				clientMock.On("GetService", ctx, "default", name).Return(svc, nil)
			}
			var updated []string
			clientMock.On("UpdateService", ctx, "default", mock.Anything).Return(nil).
				Run(func(args mock.Arguments) {
					svc := args.Get(2).(*corev1.Service)
					updated = append(updated, svc.Name)
					if tc.access.ServiceType == corev1.ServiceTypeNodePort {
						assert.Equal(t, services[svc.Name].Spec.Ports, svc.Spec.Ports, "the node ports and the gossip port stay")
					}
				}).Maybe()

			a := &actorEnsureResource{client: clientMock, logger: logr.Discard()}
			assert.Nil(t, a.ensureService(ctx, newInst(tc.access, tc.inst...), logr.Discard()))
			assert.Equal(t, tc.updated, updated)
			clientMock.AssertNotCalled(t, "CreateService", ctx, "default", mock.Anything)
			clientMock.AssertNotCalled(t, "DeletePod", ctx, "default", mock.Anything)
		})
	}
}
