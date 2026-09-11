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
	"reflect"
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	rdsv1alpha1 "github.com/chideat/valkey-operator/api/rds/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
	"github.com/chideat/valkey-operator/internal/builder/failoverbuilder"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newValkeyInst(name string, arch core.Arch, matchLabels map[string]string) *rdsv1alpha1.Valkey {
	return &rdsv1alpha1.Valkey{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       rdsv1alpha1.ValkeySpec{Arch: arch},
		Status:     rdsv1alpha1.ValkeyStatus{MatchLabels: matchLabels},
	}
}

func TestPvcSelectors(t *testing.T) {
	const name = "inst-a"

	clusterLabels := clusterbuilder.GenerateClusterLabels(name, nil)
	failoverLabels := failoverbuilder.GenerateSelectorLabels(name)
	// A selector recorded by an older operator, which must always win over derivation.
	recorded := map[string]string{"legacy/selector": name}

	tests := []struct {
		name string
		inst *rdsv1alpha1.Valkey
		want []map[string]string
	}{
		{
			name: "recorded status wins over derivation",
			inst: newValkeyInst(name, core.ValkeyCluster, recorded),
			want: []map[string]string{recorded},
		},
		{
			name: "empty status, cluster arch",
			inst: newValkeyInst(name, core.ValkeyCluster, nil),
			want: []map[string]string{clusterLabels},
		},
		{
			name: "empty status, failover arch",
			inst: newValkeyInst(name, core.ValkeyFailover, nil),
			want: []map[string]string{failoverLabels},
		},
		{
			name: "empty status, replica arch shares the failover selector",
			inst: newValkeyInst(name, core.ValkeyReplica, nil),
			want: []map[string]string{failoverLabels},
		},
		{
			name: "empty status and unset arch falls back to both selectors",
			inst: newValkeyInst(name, "", nil),
			want: []map[string]string{clusterLabels, failoverLabels},
		},
		{
			name: "empty status and an arch rds never reconciles falls back to both",
			inst: newValkeyInst(name, core.ValkeySentinel, nil),
			want: []map[string]string{clusterLabels, failoverLabels},
		},
		{
			name: "zero-length (non-nil) status still derives",
			inst: newValkeyInst(name, core.ValkeyCluster, map[string]string{}),
			want: []map[string]string{clusterLabels},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pvcSelectors(tt.inst)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("pvcSelectors() = %v, want %v", got, tt.want)
			}
		})
	}
}

// The finalizer must never be blocked by an empty status: every selector it can return has
// to be non-empty and scoped to this instance, or DeleteAllOf would either no-op or match
// PVCs belonging to a different instance.
func TestPvcSelectorsAreScopedToInstance(t *testing.T) {
	for _, arch := range []core.Arch{core.ValkeyCluster, core.ValkeyFailover, core.ValkeyReplica, ""} {
		inst := newValkeyInst("inst-a", arch, nil)
		selectors := pvcSelectors(inst)
		if len(selectors) == 0 {
			t.Fatalf("arch %q: no selector returned, deletion would be blocked", arch)
		}
		for _, selector := range selectors {
			if len(selector) == 0 {
				t.Errorf("arch %q: empty selector would delete every PVC in the namespace", arch)
			}
			if got := selector[builder.InstanceNameLabelKey]; got != inst.Name {
				t.Errorf("arch %q: selector %v is not scoped by %s=%s",
					arch, selector, builder.InstanceNameLabelKey, inst.Name)
			}
		}
	}
}
