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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/chideat/valkey-operator/api/core"
	rdsv1alpha1 "github.com/chideat/valkey-operator/api/rds/v1alpha1"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Valkey Webhook", func() {
	var (
		obj       *rdsv1alpha1.Valkey
		oldObj    *rdsv1alpha1.Valkey
		validator ValkeyCustomValidator
		defaulter ValkeyCustomDefaulter
		ctx       context.Context
	)

	BeforeEach(func() {
		obj = &rdsv1alpha1.Valkey{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-valkey",
				Namespace: "default",
			},
			Spec: rdsv1alpha1.ValkeySpec{
				Arch: core.ValkeyFailover,
				Access: core.InstanceAccess{
					DefaultPasswordSecret: "valkey-secret",
					ServiceType:           corev1.ServiceTypeClusterIP,
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
			},
		}
		oldObj = &rdsv1alpha1.Valkey{}

		validator = ValkeyCustomValidator{
			mgrClient: k8sClient,
		}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		defaulter = ValkeyCustomDefaulter{}
		Expect(defaulter).NotTo(BeNil(), "Expected defaulter to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")

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
		By("Deleting the secret for the default password")
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

	Context("When creating Valkey under Defaulting Webhook", func() {
		It("Should apply default annotations and configurations when they are empty", func() {
			By("Creating a basic Valkey object with empty annotations and configs")
			obj.Annotations = nil
			obj.Spec.CustomConfigs = nil
			obj.Spec.PodAnnotations = nil

			By("Applying defaults")
			err := defaulter.Default(ctx, obj)

			By("Verifying defaults were applied")
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Annotations).NotTo(BeNil())
			Expect(obj.Spec.CustomConfigs).NotTo(BeNil())
			Expect(obj.Spec.PodAnnotations).NotTo(BeNil())
		})

		It("Should set default exporter settings", func() {
			By("Creating a Valkey object with no exporter configurations")
			obj.Spec.Exporter = nil

			By("Applying defaults")
			err := defaulter.Default(ctx, obj)

			By("Verifying exporter settings were set with default resources")
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Spec.Exporter).NotTo(BeNil())
			Expect(obj.Spec.Exporter.Resources).NotTo(BeNil())
			Expect(obj.Spec.Exporter.Resources.Limits.Cpu().String()).To(Equal("100m"))
			Expect(obj.Spec.Exporter.Resources.Limits.Memory().String()).To(Equal("384Mi"))
		})

		It("Should set default storage capacity based on memory limits", func() {
			By("Creating a Valkey object with StorageClassName but no capacity")
			obj.Spec.Storage = &core.Storage{
				StorageClassName: func() *string { s := "standard"; return &s }(),
				Capacity:         nil,
			}

			By("Applying defaults")
			err := defaulter.Default(ctx, obj)

			By("Verifying storage capacity was set to 2x memory limits")
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Spec.Storage.Capacity).NotTo(BeNil())
			// Memory limit is 512Mi, so storage should be 1Gi
			Expect(obj.Spec.Storage.Capacity.Value()).To(Equal(int64(1024 * 1024 * 1024)))
		})

		It("Should set default failover architecture configuration", func() {
			By("Creating a Valkey object with failover architecture but no replicas or sentinel config")
			obj.Spec.Arch = core.ValkeyFailover
			obj.Spec.Replicas = nil
			obj.Spec.Sentinel = nil

			By("Applying defaults")
			err := defaulter.Default(ctx, obj)

			By("Verifying failover specific defaults were applied")
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Spec.Replicas).NotTo(BeNil())
			Expect(obj.Spec.Replicas.Shards).To(Equal(int32(1)))
			Expect(obj.Spec.Replicas.ReplicasOfShard).To(Equal(int32(2)))

			Expect(obj.Spec.Sentinel).NotTo(BeNil())
			Expect(obj.Spec.Sentinel.Replicas).To(Equal(int32(3)))
			Expect(obj.Spec.Sentinel.Access.ServiceType).To(Equal(obj.Spec.Access.ServiceType))
			Expect(obj.Spec.Sentinel.Resources.Limits.Cpu().String()).To(Equal("200m"))
		})

		It("Should set default replica architecture configuration", func() {
			By("Creating a Valkey object with replica architecture but no replicas config")
			obj.Spec.Arch = core.ValkeyReplica
			obj.Spec.Replicas = nil

			By("Applying defaults")
			err := defaulter.Default(ctx, obj)

			By("Verifying replica specific defaults were applied")
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Spec.Replicas).NotTo(BeNil())
			Expect(obj.Spec.Replicas.Shards).To(Equal(int32(1)))
			Expect(obj.Spec.Sentinel).To(BeNil())
		})
	})

	Context("When creating Valkey Cluster under Validating Webhook", func() {
		It("Should validate cluster architecture configuration", func() {
			By("Creating a Valkey object with cluster architecture but nil replicas")
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.Replicas = nil

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation error for nil replicas")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.replicas not specified"))
		})

		It("Should validate cluster architecture configuration", func() {
			By("Creating a Valkey object with cluster architecture but invalid shard count")
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          2, // Should be >= 3
				ReplicasOfShard: 1,
			}

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation error for shard count")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.replicas.shards must >= 3"))
		})

		It("Should validate port assignments in NodePort service type", func() {
			By("Creating a Valkey object with NodePort service type and not enough ports")
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.Access.ServiceType = corev1.ServiceTypeNodePort
			obj.Spec.Access.Ports = "30001,30002,30003"
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          3,
				ReplicasOfShard: 1,
			}

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation error for not enough ports")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected 6 nodes, but got 3 ports to assign"))
		})

		It("Should validate port assignments in NodePort service type", func() {
			By("Creating a Valkey object with NodePort service type and duplicate ports")
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.Access.ServiceType = corev1.ServiceTypeNodePort
			obj.Spec.Access.Ports = "30001,30002,30003,30004,30005,30001"
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          3,
				ReplicasOfShard: 1,
			}

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation error for duplicate ports")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("duplicate port 30001 found"))
		})

		It("Should validate port assignments in NodePort service type", func() {
			By("Creating a Valkey object with NodePort service type")
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.Access.ServiceType = corev1.ServiceTypeNodePort
			obj.Spec.Access.Ports = "30001,30002,30003,30004,30005,30006-30008"
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          4,
				ReplicasOfShard: 1,
			}

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation not error")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate custom shards config", func() {
			By("Creating a Valkey object with custom shards config but shards config not match shards count")
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          4,
				ReplicasOfShard: 1,
				ShardsConfig: []*v1alpha1.ShardConfig{
					{
						Slots: "0-5461",
					},
					{
						Slots: "5462-10922",
					},
					{
						Slots: "10923-16383",
					},
				},
			}

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation error")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("specified shard configs not not match shards count"))
		})

		It("Should validate custom shards config", func() {
			By("Creating a Valkey object with custom shards config but shards config not match shards count")
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          3,
				ReplicasOfShard: 1,
				ShardsConfig: []*v1alpha1.ShardConfig{
					{
						Slots: "0-5461",
					},
					{
						Slots: "5462-10922",
					},
					{
						Slots: "10923-15000",
					},
					{
						Slots: "15001-16383",
					},
				},
			}

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation error")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("specified shard configs not not match shards count"))
		})

		It("Should validate custom shards config", func() {
			By("Creating a Valkey object with custom shards config but configed slots not fullfilled")
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          3,
				ReplicasOfShard: 1,
				ShardsConfig: []*v1alpha1.ShardConfig{
					{
						Slots: "0-5461",
					},
					{
						Slots: "5462-10922",
					},
					{
						Slots: "10923-15000",
					},
				},
			}

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation error")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("specified shard slots not fullfilled"))
		})

		It("Should validate custom shards config", func() {
			By("Creating a Valkey object with custom shards config but slots duplicated")
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          3,
				ReplicasOfShard: 1,
				ShardsConfig: []*v1alpha1.ShardConfig{
					{
						Slots: "0-5461",
					},
					{
						Slots: "5462-10922",
					},
					{
						Slots: "10922-16383",
					},
				},
			}

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation error")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("specified shard slots duplicated"))
		})

		It("Should validate custom shards config", func() {
			By("Creating a Valkey object with custom shards config but slots not match")
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          3,
				ReplicasOfShard: 1,
				ShardsConfig: []*v1alpha1.ShardConfig{
					{
						Slots: "0-5461",
					},
					{
						Slots: "5462-10922",
					},
					{
						Slots: "10923-16384",
					},
				},
			}

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation not error")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to load shard slots: invalid slot 16384"))
		})

		It("Should validate custom shards config", func() {
			By("Creating a Valkey object with custom shards config")
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          3,
				ReplicasOfShard: 1,
				ShardsConfig: []*v1alpha1.ShardConfig{
					{
						Slots: "0-5461",
					},
					{
						Slots: "5462-10922",
					},
					{
						Slots: "10923-16383",
					},
				},
			}

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate resources", func() {
			By("Creating a Valkey object with nil resources")
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.Resources = corev1.ResourceRequirements{}
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          3,
				ReplicasOfShard: 1,
			}

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation error")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.resources is specified"))
		})

		It("Should validate resources", func() {
			By("Creating a Valkey object with nil memory")
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("1"),
				},
			}
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          3,
				ReplicasOfShard: 1,
			}

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation error")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.resources.limits.memory is required"))
		})

		It("Should validate resources", func() {
			By("Creating a Valkey object with nil cpu")
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			}
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          3,
				ReplicasOfShard: 1,
			}

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation error")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.resources.limits.cpu is required"))
		})

		It("Should validate password", func() {
			By("Creating a Valkey object with invalid password")
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          3,
				ReplicasOfShard: 1,
			}
			obj.Spec.Access = core.InstanceAccess{
				DefaultPasswordSecret: "valkey-secret-nosec",
			}

			By("Validating creation with invalid password")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation error")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("password should consists of"))
		})
	})

	Context("When creating Valkey Failover under Validating Webhook", func() {
		It("Should validate failover architecture configuration", func() {
			By("Creating a Valkey object with failover architecture but invalid shard count")
			obj.Spec.Arch = core.ValkeyFailover
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards: 3, // Should be 1
			}

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation error for shard count")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.replicas.shards must be 1"))
		})

		It("Should validate failover architecture configuration", func() {
			By("Creating a Valkey object with failover architecture but nil sentinel")
			obj.Spec.Arch = core.ValkeyFailover
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards: 1, // Should be 1
			}
			obj.Spec.Sentinel = nil

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation error for nil sentinel")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.sentinel not specified"))
		})

		It("Should validate failover architecture configuration", func() {
			By("Creating a Valkey object with failover architecture but invalid sentinel replicas")
			obj.Spec.Arch = core.ValkeyFailover
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          1,
				ReplicasOfShard: 2,
			}
			obj.Spec.Sentinel = &v1alpha1.SentinelSettings{
				SentinelSpec: v1alpha1.SentinelSpec{
					Replicas: 2, // Should be >= 3 and odd
				},
			}

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation error for sentinel replicas")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("sentinel replicas must be odd and greater >= 3"))
		})

		It("Should validate port assignments in NodePort service type", func() {
			By("Creating a Valkey object with NodePort service type and duplicate ports")
			obj.Spec.Arch = core.ValkeyFailover
			obj.Spec.Access.ServiceType = corev1.ServiceTypeNodePort
			obj.Spec.Access.Ports = "30001,30002,30003"
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          1,
				ReplicasOfShard: 2,
			}
			obj.Spec.Sentinel = &v1alpha1.SentinelSettings{
				SentinelSpec: v1alpha1.SentinelSpec{
					Replicas: 3,
					Access: v1alpha1.SentinelInstanceAccess{
						InstanceAccess: core.InstanceAccess{
							ServiceType: corev1.ServiceTypeNodePort,
							Ports:       "30001,30002,30003", // Same as obj.Spec.Access.Ports
						},
					},
				},
			}

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation error for duplicate ports")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("port 30001 has been assigned"))
		})

		It("Should validate port assignments in NodePort service type", func() {
			By("Creating a Valkey object with NodePort service type and duplicate sentinel ports")
			obj.Spec.Arch = core.ValkeyFailover
			obj.Spec.Access.ServiceType = corev1.ServiceTypeNodePort
			obj.Spec.Access.Ports = "30001,30002,30003"
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          1,
				ReplicasOfShard: 2,
			}
			obj.Spec.Sentinel = &v1alpha1.SentinelSettings{
				SentinelSpec: v1alpha1.SentinelSpec{
					Replicas: 3,
					Access: v1alpha1.SentinelInstanceAccess{
						InstanceAccess: core.InstanceAccess{
							ServiceType: corev1.ServiceTypeNodePort,
							Ports:       "30004,30005,30005", // Same as obj.Spec.Access.Ports
						},
					},
				},
			}

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation error for duplicate ports")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse nodeports: duplicate port 30005 found"))
		})

		It("Should validate sentinel with invalid password", func() {
			By("Creating a Valkey Failover with sentinel password but invalid password")
			obj.Spec.Arch = core.ValkeyFailover
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          1,
				ReplicasOfShard: 2,
			}
			obj.Spec.Sentinel = &v1alpha1.SentinelSettings{
				SentinelSpec: v1alpha1.SentinelSpec{
					Replicas: 3,
					Access: v1alpha1.SentinelInstanceAccess{
						InstanceAccess: core.InstanceAccess{
							DefaultPasswordSecret: "valkey-secret-nosec",
						},
					},
				},
			}

			By("Validating creation failover")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation error for invalid sentinel password")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("sentinel password secret"))
		})
	})

	Context("When creating Valkey Replia under Validating Webhook", func() {
		It("Should validate replica architecture configuration", func() {
			By("Creating a Valkey object with replica architecture but invalid shard count")
			obj.Spec.Arch = core.ValkeyReplica
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards: 2, // Should be 1
			}

			By("Validating creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("Verifying validation error for shard count")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.replicas.shards must be 1"))
		})
	})

	Context("When updating Valkey under Validating Webhook", func() {
		It("Should validate cluster architecture configuration", func() {
			By("Updating a Valkey object with cluster architecture")
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          3, // Should be >= 3
				ReplicasOfShard: 1,
			}

			By("Validating creation")
			_, err := validator.ValidateUpdate(ctx, nil, obj)

			By("Verifying validation no error")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When deleting Valkey under Validating Webhook", func() {
		It("Should validate cluster architecture configuration", func() {
			By("Creating a Valkey object with cluster architecture but invalid shard count")
			obj.Spec.Arch = core.ValkeyCluster

			By("Validating deleting")
			_, err := validator.ValidateDelete(ctx, obj)

			By("Verifying validation error for deleting")
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
