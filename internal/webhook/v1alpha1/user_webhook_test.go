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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/chideat/valkey-operator/pkg/types/user"
)

// Mock mockBuildOwnerReferencesWithParents to return a valid owner reference
func mockBuildOwnerReferencesWithParents(object metav1.Object) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: "v1alpha1",
			Kind:       "MockObject",
			Name:       object.GetName(),
			UID:        object.GetUID(),
		},
	}
}

var _ = Describe("User Webhook", func() {
	var (
		ctx       context.Context
		obj       *v1alpha1.User
		validator UserCustomValidator
		defaulter UserCustomDefaulter
		namespace string
	)

	createSecret := func(name string, password string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"password": []byte(password),
			},
		}
	}

	createFailover := func(name string, isReady bool) *v1alpha1.Failover {
		phase := v1alpha1.FailoverPhaseCreating
		if isReady {
			phase = v1alpha1.FailoverPhaseReady
		}

		return &v1alpha1.Failover{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1alpha1",
						Kind:       "Valkey",
						Name:       "test-user",
						Controller: ptr.To(true),
						UID:        types.UID("test-failover-uid"),
					},
				},
			},
			Spec: v1alpha1.FailoverSpec{
				Image:    "valkey/valkey:8.0",
				Replicas: 2,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
				Sentinel: &v1alpha1.SentinelSettings{
					SentinelSpec: v1alpha1.SentinelSpec{
						Image:    "valkey/valkey:8.0",
						Replicas: 3,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
						},
					},
				},
			},
			Status: v1alpha1.FailoverStatus{
				Phase: phase,
			},
		}
	}

	createCluster := func(name string, isReady bool) *v1alpha1.Cluster {
		phase := v1alpha1.ClusterPhaseCreating
		if isReady {
			phase = v1alpha1.ClusterPhaseReady
		}

		return &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1alpha1",
						Kind:       "Valkey",
						Name:       "test-user",
						Controller: ptr.To(true),
						UID:        types.UID("test-failover-uid"),
					},
				},
			},
			Spec: v1alpha1.ClusterSpec{
				Image: "valkey/valkey:8.0",
				Replicas: v1alpha1.ClusterReplicas{
					Shards:          3,
					ReplicasOfShard: 2,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			Status: v1alpha1.ClusterStatus{
				Phase: phase,
			},
		}
	}

	BeforeEach(func() {
		ctx = context.Background()
		namespace = "default"

		obj = &v1alpha1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-user",
				Namespace: namespace,
			},
			Spec: v1alpha1.UserSpec{
				InstanceName:    "test-failover",
				Arch:            core.ValkeyFailover,
				Username:        "test-user",
				AccountType:     v1alpha1.CustomAccount,
				PasswordSecrets: []string{"test-password-secret"},
				AclRules:        "allkeys +@all -acl +get +set +zadd +zrange",
			},
		}

		validator = UserCustomValidator{
			mgrClient: k8sClient,
		}
		defaulter = UserCustomDefaulter{
			mgrClient: k8sClient,
		}

		// Create a test password secret
		{
			secret := createSecret("test-password-secret", "Password12345@")
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		}

		{
			secret := createSecret("test-password-secret-notsec", "Password")
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		}

		{
			// Create a ready failover instance
			failover := createFailover("test-failover", true)
			Expect(k8sClient.Create(ctx, failover.DeepCopy())).To(Succeed())
			var tmpObj v1alpha1.Failover
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(failover), &tmpObj)).To(Succeed())
			tmpObj.Status = failover.Status
			Expect(k8sClient.Status().Update(ctx, &tmpObj)).To(Succeed())
		}

		{
			// Create a not ready failover instance
			failoverNotReady := createFailover("test-failover-notready", false)
			Expect(k8sClient.Create(ctx, failoverNotReady.DeepCopy())).To(Succeed())
			var tmpObj v1alpha1.Failover
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(failoverNotReady), &tmpObj)).To(Succeed())
			tmpObj.Status = failoverNotReady.Status
			Expect(k8sClient.Status().Update(ctx, &tmpObj)).To(Succeed())
		}

		{
			// Create a ready cluster instance
			cluster := createCluster("test-cluster", true)
			Expect(k8sClient.Create(ctx, cluster.DeepCopy())).To(Succeed())
			var tmpObj v1alpha1.Cluster
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &tmpObj)).To(Succeed())
			tmpObj.Status = cluster.Status
			Expect(k8sClient.Status().Update(ctx, &tmpObj)).To(Succeed())
		}

		{
			// Create a not ready cluster instance
			clusterNotReady := createCluster("test-cluster-notready", false)
			Expect(k8sClient.Create(ctx, clusterNotReady.DeepCopy())).To(Succeed())
			var tmpObj v1alpha1.Cluster
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(clusterNotReady), &tmpObj)).To(Succeed())
			tmpObj.Status = clusterNotReady.Status
			Expect(k8sClient.Status().Update(ctx, &tmpObj)).To(Succeed())
		}
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-password-secret",
				Namespace: namespace,
			},
		})).To(Succeed())

		Expect(k8sClient.Delete(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-password-secret-notsec",
				Namespace: namespace,
			},
		})).To(Succeed())

		Expect(k8sClient.Delete(ctx, &v1alpha1.Failover{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-failover",
				Namespace: namespace,
			},
		})).To(Succeed())
		Expect(k8sClient.Delete(ctx, &v1alpha1.Failover{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-failover-notready",
				Namespace: namespace,
			},
		})).To(Succeed())

		Expect(k8sClient.Delete(ctx, &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: namespace,
			},
		})).To(Succeed())
		Expect(k8sClient.Delete(ctx, &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster-notready",
				Namespace: namespace,
			},
		})).To(Succeed())

		k8sClient.Delete(ctx, obj)
	})

	Context("When applying defaults to User", func() {
		It("Should set default labels", func() {
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			Expect(obj.Labels).To(HaveKeyWithValue(builder.ManagedByLabelKey, config.AppName))
			Expect(obj.Labels).To(HaveKeyWithValue(builder.InstanceNameLabelKey, obj.Spec.InstanceName))
		})

		It("Should modify ACL rules for non-system accounts", func() {
			obj.Spec.AclRules = "allkeys +@all +acl +get +set +zadd +zrange"

			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			// Non-system accounts should have 'acl' command disabled
			Expect(obj.Spec.AclRules).To(ContainSubstring("acl"))
			Expect(obj.Spec.AclRules).To(ContainSubstring("-acl"))
		})

		It("Should set owner references for Failover architecture", func() {
			{
				obj.Spec.Arch = core.ValkeyFailover
				obj.Spec.InstanceName = "test-failover"

				err := defaulter.Default(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj.OwnerReferences).NotTo(BeEmpty())
			}

			{
				obj.OwnerReferences = nil
				obj.Spec.Arch = core.ValkeyFailover
				obj.Spec.InstanceName = "test-failover-notexists"

				err := defaulter.Default(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj.OwnerReferences).To(BeEmpty())
			}
		})

		It("Should set owner references for Cluster architecture", func() {
			{
				obj.Spec.Arch = core.ValkeyCluster
				obj.Spec.InstanceName = "test-cluster"

				err := defaulter.Default(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj.OwnerReferences).NotTo(BeEmpty())
			}

			{
				obj.OwnerReferences = nil
				obj.Spec.Arch = core.ValkeyCluster
				obj.Spec.InstanceName = "test-cluster-notexists"

				err := defaulter.Default(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj.OwnerReferences).To(BeEmpty())
			}
		})

		It("Should preserve ACL commands for system accounts", func() {
			obj.Spec.AccountType = v1alpha1.SystemAccount
			obj.Spec.Username = user.DefaultOperatorUserName
			obj.Spec.AclRules = "allkeys +@all +acl"

			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			Expect(obj.Spec.AclRules).To(ContainSubstring("+acl"))
			Expect(obj.Spec.AclRules).NotTo(ContainSubstring("-acl"))
		})

		It("Should set nil obj", func() {
			err := defaulter.Default(ctx, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected User object but got"))
		})
	})

	Context("When validating User creation", func() {
		It("Should validate system account has correct username", func() {
			obj.Spec.AccountType = v1alpha1.SystemAccount
			obj.Spec.Username = "not-operator"

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("system account username must be operator"))
		})

		It("Should validate system account has ACL commands disabled", func() {
			obj.Spec.AccountType = v1alpha1.SystemAccount
			obj.Spec.Username = user.DefaultOperatorUserName
			obj.Spec.AclRules = "allkeys +@all -acl"

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("system account must enable acl command"))
		})

		It("Should validate system account has ACL commands enabled", func() {
			obj.Spec.AccountType = v1alpha1.SystemAccount
			obj.Spec.Username = user.DefaultOperatorUserName
			obj.Spec.AclRules = "allkeys +@all +acl"

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate password secret exists", func() {
			obj.Spec.PasswordSecrets = []string{}

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("password secret can not be empty"))
		})

		It("Should validate password secret is valid", func() {
			obj.Spec.PasswordSecrets = []string{"non-existent-secret"}

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should validate password secret name is empty", func() {
			obj.Spec.PasswordSecrets = []string{""}

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("password secret can not be empty"))
		})

		It("Should validate password secret not secure", func() {
			obj.Spec.PasswordSecrets = []string{"test-password-secret-notsec"}

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("password should consists of letters, number and special characters"))
		})

		It("Should validate acl rule is invalid", func() {
			{
				obj.Spec.AclRules = "afefwef @fwefae"

				_, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported "))
			}

			{
				obj.Spec.AclRules = "+afefwef +@fwefae"

				_, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported category"))
			}

			{
				obj.Spec.AclRules = "-@all"

				_, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("at least one category or command should be enabled"))
			}

			{
				obj.Spec.AclRules = "+@all -acl"

				_, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("at least one key pattern or channel pattern should be enabled"))
			}
		})

		It("Should validate instance is ready for Replica architecture", func() {
			obj.Spec.AccountType = v1alpha1.CustomAccount
			obj.Spec.Arch = core.ValkeyReplica
			obj.Spec.InstanceName = "test-failover"

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate instance is not ready for Replica architecture", func() {
			obj.Spec.AccountType = v1alpha1.CustomAccount
			obj.Spec.Arch = core.ValkeyReplica
			obj.Spec.InstanceName = "test-failover-notready"

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(Not(HaveOccurred()))
		})

		It("Should validate instance is ready for Failover architecture", func() {
			obj.Spec.Arch = core.ValkeyFailover
			obj.Spec.InstanceName = "test-failover"

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate instance is not ready for Failover architecture", func() {
			obj.Spec.Arch = core.ValkeyFailover
			obj.Spec.InstanceName = "test-failover-notready"

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(Not(HaveOccurred()))
		})

		It("Should validate instance is not exists for Failover architecture", func() {
			obj.Spec.Arch = core.ValkeyFailover
			obj.Spec.InstanceName = "test-failover-notexists"

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("Should validate instance is ready for Cluster architecture", func() {
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.InstanceName = "test-cluster"

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate instance is not ready for Cluster architecture", func() {
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.InstanceName = "test-cluster-notready"

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(Not(HaveOccurred()))
		})

		It("Should validate instance is not exists for Cluster architecture", func() {
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.InstanceName = "test-cluster-notexists"

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})

	Context("When validating User updates", func() {
		It("Should validate updates with the same rules as creation", func() {
			obj.Spec.Arch = core.ValkeyCluster
			obj.Spec.InstanceName = "test-cluster"

			_, err := validator.ValidateUpdate(ctx, obj.DeepCopy(), obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate updates with nil obj", func() {
			_, err := validator.ValidateUpdate(ctx, nil, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected User object but got"))
		})
	})

	Context("When validating User deletion", func() {
		It("Should prevent deletion of operator users when failover instance exists", func() {
			{
				obj.Spec.Username = user.DefaultOperatorUserName
				obj.Spec.InstanceName = "test-failover"

				_, err := validator.ValidateDelete(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("user test-user can not be deleted"))
			}

			{
				obj.Spec.Username = user.DefaultOperatorUserName
				obj.Spec.InstanceName = "test-failover-notexists"

				_, err := validator.ValidateDelete(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("Should prevent deletion of non-default users when failover instance exists", func() {
			{
				obj.Spec.Username = "test"
				obj.Spec.InstanceName = "test-failover"

				_, err := validator.ValidateDelete(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
			}

			{
				obj.Spec.Username = "test"
				obj.Spec.InstanceName = "test-failover-notexists"

				_, err := validator.ValidateDelete(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("Should prevent deletion of operator users when cluster instance exists", func() {
			{
				obj.Spec.Username = user.DefaultOperatorUserName
				obj.Spec.InstanceName = "test-cluster"
				obj.Spec.Arch = core.ValkeyCluster

				_, err := validator.ValidateDelete(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("user test-user can not be deleted"))
			}

			{
				obj.Spec.Username = user.DefaultOperatorUserName
				obj.Spec.InstanceName = "test-cluster-notexists"
				obj.Spec.Arch = core.ValkeyCluster

				_, err := validator.ValidateDelete(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("Should prevent deletion of non-default users when cluster instance exists", func() {
			{
				obj.Spec.Username = "test"
				obj.Spec.InstanceName = "test-cluster"
				obj.Spec.Arch = core.ValkeyCluster

				_, err := validator.ValidateDelete(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
			}

			{
				obj.Spec.Username = "test"
				obj.Spec.InstanceName = "test-cluster-notexists"
				obj.Spec.Arch = core.ValkeyCluster

				_, err := validator.ValidateDelete(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("Should prevent deletion of nil", func() {
			_, err := validator.ValidateDelete(ctx, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected User object but got"))
		})
	})
})
