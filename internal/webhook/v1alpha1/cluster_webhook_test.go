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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	valkeybufredv1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
)

var _ = Describe("Cluster Webhook", func() {
	var (
		obj       *valkeybufredv1alpha1.Cluster
		validator ClusterCustomValidator
	)

	BeforeEach(func() {
		obj = &valkeybufredv1alpha1.Cluster{}
		validator = ClusterCustomValidator{}
	})

	AfterEach(func() {
	})

	Context("When creating or updating Cluster under Validating Webhook", func() {
		It("Should deny creation if no valkey ownerer specified", func() {
			By("simulating an invalid creation scenario")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("please use Valkey API to create Cluster resources"))
		})

		It("Should allow creation if valkey ownerer specified", func() {
			By("simulating an invalid creation scenario")
			obj.OwnerReferences = []metav1.OwnerReference{
				{
					APIVersion: valkeybufredv1alpha1.GroupVersion.String(),
					Kind:       "Valkey",
					Name:       "valkey",
					UID:        "valkey-uid",
					Controller: ptr.To(true),
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
