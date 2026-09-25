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

package validation

import (
	"context"
	"reflect"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestValidateClusterScalingResource(t *testing.T) {
	dss := int64(1) << 30
	memReq := int64(float64(dss)/float64(MinMaxMemoryPercentage)) + 1

	type args struct {
		shards   int32
		resource *corev1.ResourceRequirements
		datasize []int64
	}
	tests := []struct {
		name      string
		args      args
		wantErr   bool
		wantWarns admission.Warnings
	}{
		{
			name: "just match the maxmemory limit",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(memReq, resource.BinarySI),
					},
				},
				datasize: []int64{dss, dss, dss},
			},
			wantErr: false,
		},
		{
			name: "just not match the maxmemory limit",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(memReq-2, resource.BinarySI),
					},
				},
				datasize: []int64{dss, dss, dss},
			},
			wantErr: true,
		},
		{
			name: "nil resource check",
			args: args{
				shards:   3,
				resource: nil,
			},
			wantErr: false,
		},
		{
			name: "nil resource check with data",
			args: args{
				shards:   3,
				datasize: []int64{dss, dss, dss},
			},
			wantErr: false,
		},
		{
			name: "empty resource check",
			args: args{
				shards:   3,
				resource: &corev1.ResourceRequirements{},
			},
			wantErr: false,
		},
		{
			name: "empty resource check with data",
			args: args{
				shards:   3,
				resource: &corev1.ResourceRequirements{},
				datasize: []int64{dss, dss, dss},
			},
			wantErr: false,
		},
		{
			name: "min memory limit check",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(1<<24, resource.BinarySI),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "min memory limit check with warning",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(1<<25, resource.BinarySI),
					},
				},
			},
			wantErr: false,
			wantWarns: admission.Warnings{
				"memory limit it's recommended to be at least 128Mi",
			},
		},
		{
			name: "max memory limit check with warning",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(1<<36, resource.BinarySI),
					},
				},
			},
			wantErr:   false,
			wantWarns: admission.Warnings{"memory limit it's recommended to be at most 32Gi"},
		},
		{
			name: "3=>6 without change memory limit",
			args: args{
				shards: 6,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(memReq, resource.BinarySI),
					},
				},
				datasize: []int64{dss, dss, dss},
			},
			wantErr: false,
		},
		{
			name: "3=>6 without update the memory limit",
			args: args{
				shards: 6,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(memReq, resource.BinarySI),
					},
				},
				datasize: []int64{dss, dss, dss},
			},
			wantErr: false,
		},
		{
			name: "3=>6 with halve the memory limit",
			args: args{
				shards: 6,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(1+memReq/2, resource.BinarySI),
					},
				},
				datasize: []int64{dss, dss, dss},
			},
			wantErr: true,
		},
		{
			name: "4=>3 with not scaling memory",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(memReq, resource.BinarySI),
					},
				},
				datasize: []int64{dss, dss, dss, dss},
			},
			wantErr: true,
		},
		{
			name: "4=>3 with just match memory",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(int64(float64(dss+dss)/MinMaxMemoryPercentage), resource.BinarySI),
					},
				},
				datasize: []int64{dss, dss, dss, dss},
			},
			wantErr: false,
		},
		{
			name: "4=>3 with only the deleting shards have data",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(int64(float64(dss)/MinMaxMemoryPercentage), resource.BinarySI),
					},
				},
				datasize: []int64{0, 0, 0, dss},
			},
			wantErr: false,
		},
		{
			name: "4=>3 with the deleting shards is empty",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(int64(float64(dss)/MinMaxMemoryPercentage), resource.BinarySI),
					},
				},
				datasize: []int64{dss, dss, dss, 0},
			},
			wantErr: false,
		},
		{
			name: "6=>3 deleting 3 shards",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(int64(float64(dss*4)/MinMaxMemoryPercentage), resource.BinarySI),
					},
				},
				datasize: []int64{dss, dss, dss, dss, dss, dss},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var warns admission.Warnings
			if err := ValidateClusterScalingResource(tt.args.shards, tt.args.resource, tt.args.datasize, &warns); (err != nil) != tt.wantErr {
				t.Errorf("ValidateClusterScalingResource() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(warns, tt.wantWarns) {
				t.Errorf("ValidateClusterScalingResource() warns = %v, want %v", warns, tt.wantWarns)
			}
		})
	}
}

func TestValidateReplicationScalingResource(t *testing.T) {
	dss := int64(1) << 30
	memReq := int64(float64(dss)/float64(MinMaxMemoryPercentage)) + 1

	type args struct {
		resource *corev1.ResourceRequirements
		datasize int64
	}
	tests := []struct {
		name      string
		args      args
		wantErr   bool
		wantWarns admission.Warnings
	}{
		{
			name: "just match the maxmemory limit",
			args: args{
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(memReq, resource.BinarySI),
					},
				},
				datasize: dss,
			},
			wantErr: false,
		},
		{
			name: "just not match the maxmemory limit",
			args: args{
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(memReq-2, resource.BinarySI),
					},
				},
				datasize: dss,
			},
			wantErr: true,
		},
		{
			name: "nil resource check",
			args: args{
				resource: nil,
			},
			wantErr: false,
		},
		{
			name: "nil resource check with data",
			args: args{
				datasize: dss,
			},
			wantErr: false,
		},
		{
			name: "empty resource check",
			args: args{
				resource: &corev1.ResourceRequirements{},
			},
			wantErr: false,
		},
		{
			name: "empty resource check with data",
			args: args{
				resource: &corev1.ResourceRequirements{},
				datasize: dss,
			},
			wantErr: false,
		},
		{
			name: "min memory limit check",
			args: args{
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(1<<24, resource.BinarySI),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "min memory limit check with warning",
			args: args{
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(1<<25, resource.BinarySI),
					},
				},
			},
			wantErr: false,
			wantWarns: admission.Warnings{
				"memory limit it's recommended to be at least 128Mi",
			},
		},
		{
			name: "max memory limit check with warning",
			args: args{
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(1<<36, resource.BinarySI),
					},
				},
			},
			wantErr:   false,
			wantWarns: admission.Warnings{"memory limit it's recommended to be at most 32Gi"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var warns admission.Warnings
			if err := ValidateReplicationScalingResource(tt.args.resource, tt.args.datasize, &warns); (err != nil) != tt.wantErr {
				t.Errorf("ValidateReplicationScalingResource() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(warns, tt.wantWarns) {
				t.Errorf("ValidateClusterScalingResource() warns = %v, want %v", warns, tt.wantWarns)
			}
		})
	}
}

var _ = Describe("Valkey Webhook", func() {
	var ctx = context.Background()

	BeforeEach(func() {
		ctx = context.Background()

		By("Creating a secret for the default password")
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "valkey-secret-nosec",
				Namespace: "default",
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"password": []byte("password"),
			},
		}
		secret2 := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "valkey-secret",
				Namespace: "default",
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"password": []byte("admin@123"),
			},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		Expect(k8sClient.Create(ctx, secret2)).To(Succeed())
	})

	AfterEach(func() {
		By("Deleting the secrets")
		Expect(k8sClient.Delete(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "valkey-secret",
				Namespace: "default",
			},
		})).To(Succeed())
		Expect(k8sClient.Delete(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "valkey-secret-nosec",
				Namespace: "default",
			},
		})).To(Succeed())
	})

	Context("Validate passwords", func() {
		It("Should pass validation", func() {
			By("Check password")
			err := ValidatePasswordSecret("default", "valkey-secret", k8sClient, nil)

			By("Verifying result")
			Expect(err).NotTo(HaveOccurred())
		})
		It("Should not pass validation", func() {
			By("Check password")
			err := ValidatePasswordSecret("default", "valkey-secret-nosec", k8sClient, nil)

			By("Verifying result")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("password should consists of letters"))
		})
		It("Not pass secret", func() {
			By("Check password")
			err := ValidatePasswordSecret("default", "", k8sClient, nil)

			By("Verifying result")
			Expect(err).NotTo(HaveOccurred())
		})
		It("Should not validate", func() {
			By("Check password")
			err := ValidatePasswordSecret("default", "valkey-secret-nosec", nil, nil)

			By("Verifying result")
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

func TestValidateOverwrites(t *testing.T) {
	patches := func(doc string) []core.Overwrite {
		return []core.Overwrite{{Kind: core.OverwriteKindStatefulSet, Patch: apiextensionsv1.JSON{Raw: []byte(doc)}}}
	}
	budget := func(doc string) []core.Overwrite {
		return []core.Overwrite{{Kind: core.OverwriteKindPodDisruptionBudget, Patch: apiextensionsv1.JSON{Raw: []byte(doc)}}}
	}
	sentinel := func(doc string) *v1alpha1.SentinelSettings {
		return &v1alpha1.SentinelSettings{SentinelSpec: v1alpha1.SentinelSpec{Replicas: 3, Overwrites: patches(doc)}}
	}
	const (
		exporterArgs = `{"spec":{"template":{"spec":{"containers":[{"name":"exporter","args":["--include-system-metrics=true"]}]}}}}`
		agentEnv     = `{"spec":{"template":{"spec":{"containers":[{"name":"agent","env":[{"name":"LOG_LEVEL","value":"debug"}]}]}}}}`
	)

	tests := []struct {
		name       string
		arch       core.Arch
		overwrites []core.Overwrite
		sentinel   *v1alpha1.SentinelSettings
		wantErr    []string
	}{
		{name: "nothing to check", arch: core.ValkeyFailover},
		{name: "exporter args on failover", arch: core.ValkeyFailover, overwrites: patches(exporterArgs)},
		{name: "exporter args on replica", arch: core.ValkeyReplica, overwrites: patches(exporterArgs)},
		{name: "agent on cluster", arch: core.ValkeyCluster, overwrites: patches(agentEnv)},
		{
			name: "agent on failover", arch: core.ValkeyFailover, overwrites: patches(agentEnv),
			wantErr: []string{"spec.overwrites[0].patch", "containers[name=agent] is not generated by the operator"},
		},
		{
			name: "protected field", arch: core.ValkeyCluster, overwrites: patches(`{"spec":{"replicas":5}}`),
			wantErr: []string{"spec.overwrites[0].patch", "spec.replicas is protected"},
		},
		{name: "budget policy", arch: core.ValkeyFailover, overwrites: budget(`{"spec":{"unhealthyPodEvictionPolicy":"AlwaysAllow"}}`)},
		{
			name: "budget minAvailable next to maxUnavailable", arch: core.ValkeyCluster, overwrites: budget(`{"spec":{"minAvailable":1}}`),
			wantErr: []string{"spec.overwrites[0].patch", "spec.minAvailable cannot be set with maxUnavailable"},
		},
		{name: "agent on the sentinel nodes", arch: core.ValkeyFailover, sentinel: sentinel(agentEnv)},
		{
			name: "valkey on the sentinel nodes", arch: core.ValkeyFailover,
			sentinel: sentinel(`{"spec":{"template":{"spec":{"containers":[{"name":"valkey","args":["x"]}]}}}}`),
			wantErr:  []string{"spec.sentinel.overwrites[0].patch", "containers[name=valkey] is not generated by the operator"},
		},
		{
			name: "sentinel overwrites on cluster", arch: core.ValkeyCluster, sentinel: sentinel(agentEnv),
			wantErr: []string{"spec.sentinel.overwrites", "the cluster architecture runs no sentinel nodes"},
		},
		{
			name: "sentinel overwrites with a sentinel reference", arch: core.ValkeyFailover,
			sentinel: func() *v1alpha1.SentinelSettings {
				s := sentinel(agentEnv)
				s.SentinelReference = &v1alpha1.SentinelReference{}
				return s
			}(),
			wantErr: []string{"spec.sentinel.overwrites", "spec.sentinel.sentinelReference"},
		},
		{
			name: "sentinel reference without sentinel overwrites", arch: core.ValkeyFailover,
			sentinel: &v1alpha1.SentinelSettings{SentinelReference: &v1alpha1.SentinelReference{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOverwrites(tt.arch, tt.overwrites, tt.sentinel)
			if len(tt.wantErr) == 0 {
				if err != nil {
					t.Fatalf("ValidateOverwrites() error = %v, want none", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateOverwrites() accepted, want %q", tt.wantErr)
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("ValidateOverwrites() error = %v, want it to contain %q", err, want)
				}
			}
		})
	}
}
