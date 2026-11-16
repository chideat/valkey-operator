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
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const sentinelPasswordTestName = "vk-failover"

// The name the operator generates the sentinel password under, for the instance above.
const reservedSentinelSecret = "sentinel-acl-vk-failover-default-secret"

func TestDefaultSentinelPasswordSecret(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		objName string
		current string
		want    string
	}{
		{
			// The case the change exists for: a new failover instance whose sentinel
			// nodes would otherwise accept any client without a password.
			name:    "create names the secret the operator generates",
			ctx:     admissionCtx(admissionv1.Create),
			objName: sentinelPasswordTestName,
			want:    reservedSentinelSecret,
		},
		{
			name:    "a secret the user names is kept",
			ctx:     admissionCtx(admissionv1.Create),
			objName: sentinelPasswordTestName,
			current: "my-sentinel-password",
			want:    "my-sentinel-password",
		},
		{
			// The sentinel nodes require the password as soon as the field is set, so
			// an existing instance without one keeps accepting its clients.
			name:    "update leaves an existing instance without a password",
			ctx:     admissionCtx(admissionv1.Update),
			objName: sentinelPasswordTestName,
			want:    "",
		},
		{
			name:    "a context with no admission request is not a create",
			ctx:     context.Background(),
			objName: sentinelPasswordTestName,
			want:    "",
		},
		{
			// generateName: the API server assigns the name after admission, so there
			// is nothing yet to name the secret after.
			name:    "a name yet to be generated leaves it unset",
			ctx:     admissionCtx(admissionv1.Create),
			objName: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := &rdsv1alpha1.Valkey{
				ObjectMeta: metav1.ObjectMeta{Name: tt.objName},
				Spec: rdsv1alpha1.ValkeySpec{
					Arch: core.ValkeyFailover,
					Sentinel: &v1alpha1.SentinelSettings{
						SentinelSpec: v1alpha1.SentinelSpec{
							Access: v1alpha1.SentinelInstanceAccess{DefaultPasswordSecret: tt.current},
						},
					},
				},
			}
			defaultSentinelPasswordSecret(tt.ctx, inst)
			if got := inst.Spec.Sentinel.Access.DefaultPasswordSecret; got != tt.want {
				t.Errorf("DefaultPasswordSecret = %q, want %q", got, tt.want)
			}
		})
	}
}

// Default() reaches the sentinel password only for sentinel nodes the operator runs.
func TestDefaultSentinelPasswordSecretOnlyForOperatorRunSentinels(t *testing.T) {
	t.Run("failover instance", func(t *testing.T) {
		inst := &rdsv1alpha1.Valkey{
			ObjectMeta: metav1.ObjectMeta{Name: sentinelPasswordTestName},
			Spec:       rdsv1alpha1.ValkeySpec{Arch: core.ValkeyFailover},
		}
		if err := (&ValkeyCustomDefaulter{}).Default(admissionCtx(admissionv1.Create), inst); err != nil {
			t.Fatalf("Default() error = %v", err)
		}
		if got := inst.Spec.Sentinel.Access.DefaultPasswordSecret; got != reservedSentinelSecret {
			t.Errorf("DefaultPasswordSecret = %q, want %q", got, reservedSentinelSecret)
		}
	})

	t.Run("sentinel nodes the operator does not run", func(t *testing.T) {
		inst := &rdsv1alpha1.Valkey{
			ObjectMeta: metav1.ObjectMeta{Name: sentinelPasswordTestName},
			Spec: rdsv1alpha1.ValkeySpec{
				Arch: core.ValkeyFailover,
				Sentinel: &v1alpha1.SentinelSettings{
					SentinelReference: &v1alpha1.SentinelReference{},
				},
			},
		}
		if err := (&ValkeyCustomDefaulter{}).Default(admissionCtx(admissionv1.Create), inst); err != nil {
			t.Fatalf("Default() error = %v", err)
		}
		if got := inst.Spec.Sentinel.Access.DefaultPasswordSecret; got != "" {
			t.Errorf("DefaultPasswordSecret = %q, want it unset", got)
		}
	})
}

func newSentinelPasswordTestFailover(secretName string) *rdsv1alpha1.Valkey {
	return &rdsv1alpha1.Valkey{
		ObjectMeta: metav1.ObjectMeta{Name: sentinelPasswordTestName, Namespace: "default"},
		Spec: rdsv1alpha1.ValkeySpec{
			Version: "8.0",
			Arch:    core.ValkeyFailover,
			Replicas: &rdsv1alpha1.ValkeyReplicas{
				Shards:          1,
				ReplicasOfShard: 2,
			},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
			Access: core.InstanceAccess{ServiceType: corev1.ServiceTypeClusterIP},
			Sentinel: &v1alpha1.SentinelSettings{
				SentinelSpec: v1alpha1.SentinelSpec{
					Replicas: 3,
					Access: v1alpha1.SentinelInstanceAccess{
						InstanceAccess:        core.InstanceAccess{ServiceType: corev1.ServiceTypeClusterIP},
						DefaultPasswordSecret: secretName,
					},
				},
			},
		},
	}
}

// The webhook names the secret on create and the operator generates it just before it
// creates the failover; until then every create and update of the instance has to be
// admitted. Once the failover exists the operator generates nothing, so the reserved
// secret is checked like any other: missing, it would leave the sentinel nodes reading a
// secret nobody creates.
func TestValidateSentinelPasswordSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		secret  string
		objs    []client.Object
		wantErr string
	}{
		{
			name:   "the reserved secret before the operator generates it",
			secret: reservedSentinelSecret,
		},
		{
			// Named by hand on an instance that is already running: nothing will
			// generate it.
			name:   "the reserved secret missing once the failover exists",
			secret: reservedSentinelSecret,
			objs: []client.Object{&v1alpha1.Failover{
				ObjectMeta: metav1.ObjectMeta{Name: sentinelPasswordTestName, Namespace: "default"},
			}},
			wantErr: "sentinel password secret",
		},
		{
			name:    "a missing secret the user names",
			secret:  "my-sentinel-password",
			wantErr: "sentinel password secret",
		},
		{
			name:   "the reserved secret holding a weak password",
			secret: reservedSentinelSecret,
			objs: []client.Object{&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: reservedSentinelSecret, Namespace: "default"},
				Data:       map[string][]byte{"password": []byte("password")},
			}},
			wantErr: "sentinel password secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objs...).Build()
			v := &ValkeyCustomValidator{mgrClient: cli, apiReader: cli}
			_, err := v.ValidateCreate(context.Background(), newSentinelPasswordTestFailover(tt.secret))
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("ValidateCreate() error = %v, want none", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Errorf("ValidateCreate() error = %v, want one containing %q", err, tt.wantErr)
			}
		})
	}
}
