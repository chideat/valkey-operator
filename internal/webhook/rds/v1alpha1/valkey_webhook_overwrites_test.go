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
	"strings"
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	rdsv1alpha1 "github.com/chideat/valkey-operator/api/rds/v1alpha1"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// owValkey is a failover Valkey that passes every other validation, with the
// given overwrites for the Valkey and the sentinel nodes.
func owValkey(overwrites, sentinelOverwrites string) *rdsv1alpha1.Valkey {
	patches := func(doc string) []core.Overwrite {
		if doc == "" {
			return nil
		}
		return []core.Overwrite{{Kind: core.OverwriteKindStatefulSet, Patch: apiextensionsv1.JSON{Raw: []byte(doc)}}}
	}
	resources := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("200m"),
		corev1.ResourceMemory: resource.MustParse("512Mi"),
	}
	return &rdsv1alpha1.Valkey{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "demo"},
		Spec: rdsv1alpha1.ValkeySpec{
			Arch:       core.ValkeyFailover,
			Replicas:   &rdsv1alpha1.ValkeyReplicas{Shards: 1, ReplicasOfShard: 2},
			Resources:  corev1.ResourceRequirements{Limits: resources, Requests: resources},
			Access:     core.InstanceAccess{ServiceType: corev1.ServiceTypeClusterIP},
			Sentinel:   &v1alpha1.SentinelSettings{SentinelSpec: v1alpha1.SentinelSpec{Replicas: 3, Overwrites: patches(sentinelOverwrites)}},
			Overwrites: patches(overwrites),
		},
	}
}

func TestValidateChecksOverwrites(t *testing.T) {
	const (
		userField = `{"spec":{"template":{"spec":{"priorityClassName":"critical"}}}}`
		protected = `{"spec":{"replicas":5}}`
	)
	tests := []struct {
		name    string
		valkey  *rdsv1alpha1.Valkey
		wantErr string
	}{
		{name: "user fields", valkey: owValkey(userField, userField)},
		{name: "protected field", valkey: owValkey(protected, ""), wantErr: "spec.overwrites[0].patch: Forbidden: spec.replicas is protected"},
		{name: "protected field on the sentinel nodes", valkey: owValkey("", protected), wantErr: "spec.sentinel.overwrites[0].patch: Forbidden: spec.replicas is protected"},
	}
	validator := &ValkeyCustomValidator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for action, validate := range map[string]func() error{
				"create": func() error { _, err := validator.ValidateCreate(context.Background(), tt.valkey); return err },
				"update": func() error {
					_, err := validator.ValidateUpdate(context.Background(), owValkey("", ""), tt.valkey)
					return err
				},
			} {
				err := validate()
				switch {
				case tt.wantErr == "" && err != nil:
					t.Errorf("%s: error = %v, want none", action, err)
				case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
					t.Errorf("%s: error = %v, want it to contain %q", action, err, tt.wantErr)
				}
			}
		})
	}
}
