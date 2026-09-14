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
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func admissionCtx(op admissionv1.Operation) context.Context {
	return admission.NewContextWithRequest(context.Background(), admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{Operation: op},
	})
}

func TestDefaultIPFamily(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		podIPs  string
		current corev1.IPFamily
		want    corev1.IPFamily
	}{
		{
			// The case the whole change exists for: nothing asked for a family,
			// so the cluster's own is pinned and every consumer agrees on it.
			name: "create on an IPv6 cluster resolves IPv6",
			ctx:  admissionCtx(admissionv1.Create),
			// A single-stack IPv6 cluster gives the operator pod one address.
			podIPs: "fd00:10:244::4",
			want:   corev1.IPv6Protocol,
		},
		{
			name:   "create on an IPv4 cluster resolves IPv4",
			ctx:    admissionCtx(admissionv1.Create),
			podIPs: "10.244.1.4",
			want:   corev1.IPv4Protocol,
		},
		{
			// An explicit choice is the user's. Never overwritten, even when it
			// disagrees with what the cluster would have given.
			name:    "an explicit preference survives",
			ctx:     admissionCtx(admissionv1.Create),
			podIPs:  "fd00:10:244::4",
			current: corev1.IPv4Protocol,
			want:    corev1.IPv4Protocol,
		},
		{
			// The field feeds cluster-announce-ip and reaches the pod spec as an
			// env var, so writing it into a running instance would re-register
			// every node and roll every pod to do it. Existing resources keep the
			// empty value; builder.IPFamilySpec handles them.
			name:   "update leaves an existing resource alone",
			ctx:    admissionCtx(admissionv1.Update),
			podIPs: "fd00:10:244::4",
			want:   "",
		},
		{
			// `make run` and unit tests have no POD_IPS. Unresolvable must stay
			// unset so the API server decides, never fall back to a family.
			name:   "an unresolvable cluster stays unset",
			ctx:    admissionCtx(admissionv1.Create),
			podIPs: "",
			want:   "",
		},
		{
			// Not an admission call at all: leave the object untouched rather
			// than guess that it is a create.
			name:   "a context with no admission request is not a create",
			ctx:    context.Background(),
			podIPs: "10.244.1.4",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("POD_IPS", tt.podIPs)
			inst := &rdsv1alpha1.Valkey{
				Spec: rdsv1alpha1.ValkeySpec{
					Access: core.InstanceAccess{IPFamilyPrefer: tt.current},
				},
			}
			defaultIPFamily(tt.ctx, inst)
			if got := inst.Spec.Access.IPFamilyPrefer; got != tt.want {
				t.Errorf("IPFamilyPrefer = %q, want %q", got, tt.want)
			}
		})
	}
}

// A sentinel monitors its nodes by a single address, so it has to land on the
// same family as the failover it belongs to — the same reason ServiceType is
// already propagated here.
func TestDefaultPropagatesIPFamilyToSentinel(t *testing.T) {
	t.Setenv("POD_IPS", "fd00:10:244::4")

	inst := &rdsv1alpha1.Valkey{
		Spec: rdsv1alpha1.ValkeySpec{
			Arch:   core.ValkeyFailover,
			Access: core.InstanceAccess{ServiceType: corev1.ServiceTypeClusterIP},
		},
	}
	if err := (&ValkeyCustomDefaulter{}).Default(admissionCtx(admissionv1.Create), inst); err != nil {
		t.Fatalf("Default() error = %v", err)
	}

	if got := inst.Spec.Access.IPFamilyPrefer; got != corev1.IPv6Protocol {
		t.Errorf("instance IPFamilyPrefer = %q, want IPv6", got)
	}
	if inst.Spec.Sentinel == nil {
		t.Fatal("a failover instance must have sentinel settings after defaulting")
	}
	if got := inst.Spec.Sentinel.Access.IPFamilyPrefer; got != corev1.IPv6Protocol {
		t.Errorf("sentinel IPFamilyPrefer = %q, want IPv6", got)
	}
}

// A failover instance and its sentinel share one certificate and dial each
// other, so TLS is not a per-side preference: the valkey pods reach the
// sentinel using the failover's own setting, and the operator reaches it using
// the sentinel's. Leaving the sentinel's unset produced TLS valkey pods talking
// to a plaintext sentinel, and the instance never left Initializing.
func TestDefaultPropagatesTLSToSentinel(t *testing.T) {
	inst := &rdsv1alpha1.Valkey{
		Spec: rdsv1alpha1.ValkeySpec{
			Arch: core.ValkeyFailover,
			Access: core.InstanceAccess{
				ServiceType:    corev1.ServiceTypeNodePort,
				EnableTLS:      true,
				CertIssuer:     "valkey-ca",
				CertIssuerType: "Issuer",
			},
		},
	}
	if err := (&ValkeyCustomDefaulter{}).Default(admissionCtx(admissionv1.Create), inst); err != nil {
		t.Fatalf("Default() error = %v", err)
	}

	if inst.Spec.Sentinel == nil {
		t.Fatal("a failover instance must have sentinel settings after defaulting")
	}
	if !inst.Spec.Sentinel.Access.EnableTLS {
		t.Error("sentinel EnableTLS = false, want true: the valkey pods dial the " +
			"sentinel over TLS, so a plaintext sentinel cannot answer them")
	}
	if got := inst.Spec.Sentinel.Access.CertIssuer; got != "valkey-ca" {
		t.Errorf("sentinel CertIssuer = %q, want %q", got, "valkey-ca")
	}
	if got := inst.Spec.Sentinel.Access.CertIssuerType; got != "Issuer" {
		t.Errorf("sentinel CertIssuerType = %q, want %q", got, "Issuer")
	}
}

// The inverse mismatch is equally broken -- the valkey pods would dial a TLS
// sentinel in plaintext -- so a sentinel-only setting is corrected rather than
// honoured.
func TestDefaultDoesNotLeaveSentinelTLSAheadOfTheInstance(t *testing.T) {
	inst := &rdsv1alpha1.Valkey{
		Spec: rdsv1alpha1.ValkeySpec{
			Arch:   core.ValkeyFailover,
			Access: core.InstanceAccess{ServiceType: corev1.ServiceTypeClusterIP},
			Sentinel: &v1alpha1.SentinelSettings{
				SentinelSpec: v1alpha1.SentinelSpec{
					Access: v1alpha1.SentinelInstanceAccess{
						InstanceAccess: core.InstanceAccess{EnableTLS: true},
					},
				},
			},
		},
	}
	if err := (&ValkeyCustomDefaulter{}).Default(admissionCtx(admissionv1.Create), inst); err != nil {
		t.Fatalf("Default() error = %v", err)
	}
	if inst.Spec.Sentinel.Access.EnableTLS {
		t.Error("sentinel EnableTLS stayed true while the instance is plaintext; " +
			"the valkey pods would dial a TLS listener without TLS")
	}
}
