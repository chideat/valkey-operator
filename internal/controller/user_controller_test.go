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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/chideat/valkey-operator/api/core"
	valkeybufredv1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
	userHandler "github.com/chideat/valkey-operator/internal/controller/user"
)

var _ = Describe("User Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "vk-user"
		const vkResourceName = "vk-cluster"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		vkTypeNamespacedName := types.NamespacedName{
			Name:      vkResourceName,
			Namespace: "default",
		}
		user := &valkeybufredv1alpha1.User{}
		vkCluster := &valkeybufredv1alpha1.Cluster{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Cluster")
			err := k8sClient.Get(ctx, vkTypeNamespacedName, vkCluster)
			if err != nil && errors.IsNotFound(err) {
				resource := &valkeybufredv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      vkResourceName,
						Namespace: "default",
					},
					Spec: valkeybufredv1alpha1.ClusterSpec{
						Image: "valkey/valkey:8.0",
						Replicas: valkeybufredv1alpha1.ClusterReplicas{
							Shards:          3,
							ReplicasOfShard: 1,
						},
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("200m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("200m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
						},
						Access:   core.InstanceAccess{},
						Exporter: &core.Exporter{},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("creating the custom resource for the Kind User")
			err = k8sClient.Get(ctx, typeNamespacedName, user)
			if err != nil && errors.IsNotFound(err) {
				resource := &valkeybufredv1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: valkeybufredv1alpha1.UserSpec{
						AccountType:  valkeybufredv1alpha1.SystemAccount,
						Arch:         core.ValkeyCluster,
						Username:     "operator",
						AclRules:     "+@all ~* &*",
						InstanceName: "vk-cluster",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &valkeybufredv1alpha1.User{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance User")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			vkResource := &valkeybufredv1alpha1.Cluster{}
			err = k8sClient.Get(ctx, vkTypeNamespacedName, vkResource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Cluster")
			Expect(k8sClient.Delete(ctx, vkResource)).To(Succeed())

		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &UserReconciler{
				Client:        k8sClient,
				Scheme:        k8sClient.Scheme(),
				K8sClient:     k8sClientSet,
				EventRecorder: eventRecorder,
				Handler:       userHandler.NewUserHandler(k8sClientSet, eventRecorder, ctrl.Log.WithName("UserHandler")),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
