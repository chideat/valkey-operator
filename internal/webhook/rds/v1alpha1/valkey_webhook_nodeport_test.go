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

package v1alpha1

import (
	"context"
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	rdsv1alpha1 "github.com/chideat/valkey-operator/api/rds/v1alpha1"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func npSvc(ns, name, instance string, svcType corev1.ServiceType, ports ...int32) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, Labels: map[string]string{}},
		Spec:       corev1.ServiceSpec{Type: svcType},
	}
	if instance != "" {
		svc.Labels[builder.InstanceNameLabelKey] = instance
	}
	for _, p := range ports {
		svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{NodePort: p})
	}
	return svc
}

func npValkey(ns, name, ports string) *rdsv1alpha1.Valkey {
	r := &rdsv1alpha1.Valkey{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name}}
	r.Spec.Arch = core.ValkeyCluster
	r.Spec.Access.ServiceType = corev1.ServiceTypeNodePort
	r.Spec.Access.Ports = ports
	return r
}

func TestCollectRequestedNodePorts(t *testing.T) {
	t.Run("not nodeport -> empty", func(t *testing.T) {
		r := npValkey("ns", "a", "30001,30002")
		r.Spec.Access.ServiceType = corev1.ServiceTypeClusterIP
		if got := collectRequestedNodePorts(r); len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})
	t.Run("data ports + sentinel ports", func(t *testing.T) {
		r := npValkey("ns", "a", "30001,30002")
		r.Spec.Sentinel = &v1alpha1.SentinelSettings{}
		r.Spec.Sentinel.Access.ServiceType = corev1.ServiceTypeNodePort
		r.Spec.Sentinel.Access.Ports = "32001,32002"
		got := collectRequestedNodePorts(r)
		for _, p := range []int32{30001, 30002, 32001, 32002} {
			if _, ok := got[p]; !ok {
				t.Errorf("expected port %d in %v", p, got)
			}
		}
	})
}

func TestValidateNodePortConflicts(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	// instance "a" in ns1 owns nodeports 30001/30002
	owned := npSvc("ns1", "drc-a-0", "a", corev1.ServiceTypeNodePort, 30001, 30002)
	// a NodePort service in another namespace using 30003 (cluster-scoped conflict source)
	otherNs := npSvc("ns2", "drc-c-0", "c", corev1.ServiceTypeNodePort, 30003)
	// a ClusterIP service that happens to record a port — must be ignored
	clusterIP := npSvc("ns1", "plain", "z", corev1.ServiceTypeClusterIP, 30001)

	tests := []struct {
		name    string
		objs    []client.Object
		valkey  *rdsv1alpha1.Valkey
		wantErr bool
	}{
		{name: "conflict same ns, different instance", objs: []client.Object{owned}, valkey: npValkey("ns1", "b", "30001,30009"), wantErr: true},
		{name: "conflict cross namespace (cluster-scoped)", objs: []client.Object{otherNs}, valkey: npValkey("ns1", "b", "30003"), wantErr: true},
		{name: "no conflict (free ports)", objs: []client.Object{owned, otherNs}, valkey: npValkey("ns1", "b", "30007,30008"), wantErr: false},
		{name: "self excluded (same ns + instance label)", objs: []client.Object{owned}, valkey: npValkey("ns1", "a", "30001,30002"), wantErr: false},
		{name: "clusterIP service ignored", objs: []client.Object{clusterIP}, valkey: npValkey("ns1", "b", "30001"), wantErr: false},
		{name: "non-nodeport valkey skips check", objs: []client.Object{owned}, valkey: func() *rdsv1alpha1.Valkey {
			r := npValkey("ns1", "b", "30001")
			r.Spec.Access.ServiceType = corev1.ServiceTypeClusterIP
			return r
		}(), wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &ValkeyCustomValidator{
				apiReader: fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objs...).Build(),
			}
			err := v.validateNodePortConflicts(context.Background(), tt.valkey)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateNodePortConflicts() err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}
