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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/chideat/valkey-operator/api/core"
	valkeybufredv1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
)

var _ = Describe("Failover Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "valkey-failover"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		failover := &valkeybufredv1alpha1.Failover{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Failover")
			err := k8sClient.Get(ctx, typeNamespacedName, failover)
			if err != nil && errors.IsNotFound(err) {
				resource := &valkeybufredv1alpha1.Failover{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: valkeybufredv1alpha1.FailoverSpec{
						Image:           "valkey/valkey:8.0",
						ImagePullPolicy: "IfNotPresent",
						Replicas:        3,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("200m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("200m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
						},
						Exporter: &core.Exporter{},
						Access:   core.InstanceAccess{},
						Sentinel: &valkeybufredv1alpha1.SentinelSettings{
							SentinelSpec: valkeybufredv1alpha1.SentinelSpec{
								Image:    "valkey/sentinel:8.0",
								Replicas: 3,
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &valkeybufredv1alpha1.Failover{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Failover")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &FailoverReconciler{
				Client:        k8sClient,
				Scheme:        k8sClient.Scheme(),
				EventRecorder: eventRecorder,
				Engine:        engine,
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
