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

package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rdsv1alpha1 "github.com/chideat/valkey-operator/api/rds/v1alpha1"
	"github.com/chideat/valkey-operator/test/utils"
)

var (
	skipDeployOperator    = (utils.GetEnv("SKIP_DEPLOY_OPERATOR") != "")
	skipDeployPrometheus  = (utils.GetEnv("SKIP_DEPLOY_PROMETHEUS") != "")
	skipDeployCertManager = (utils.GetEnv("SKIP_DEPLOY_CERT_MANAGER") != "")
	skipClusterTests      = (utils.GetEnv("SKIP_CLUSTER_TESTS") != "")
	skipFailoverTests     = (utils.GetEnv("SKIP_FAILOVER_TESTS") != "")
	skipReplicationTests  = (utils.GetEnv("SKIP_REPLICATION_TESTS") != "")
	skipSentinelTests     = (utils.GetEnv("SKIP_SENTINEL_TESTS") != "")
	skipVersions          = strings.Split(utils.GetEnv("SKIP_VERSIONS"), ",")

	namespace = utils.GetEnv("NAMESPACE", "valkey-system")
)

var (
	supportedVersions = []string{"7.2", "8.0", "8.1"}
)

type TestParameter struct {
	Version     string
	ServiceType corev1.ServiceType
}

var _ = Describe("controller", Ordered, func() {
	BeforeAll(func() {
		if !skipDeployPrometheus {
			By("installing prometheus operator")
			Expect(utils.InstallPrometheusOperator()).To(Succeed())
		}

		if !skipDeployCertManager {
			By("installing the cert-manager")
			Expect(utils.InstallCertManager()).To(Succeed())
		}

		By("creating manager namespace")
		_ = k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		})
	})

	AfterAll(func() {
		if !skipDeployPrometheus {
			By("uninstalling the Prometheus manager bundle")
			utils.UninstallPrometheusOperator()
		}

		if !skipDeployCertManager {
			By("uninstalling the cert-manager bundle")
			utils.UninstallCertManager()
		}
	})

	Context("Operator", func() {
		if skipDeployOperator {
			Skip("skipping the operator deployment")
			return
		}

		It("should run successfully", func() {
			var controllerPodName string
			var err error

			// projectimage stores the name of the image used in the example
			var projectimage = "example.com/valkey-operator:v0.0.1"

			By("building the manager(Operator) image")
			cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", projectimage))
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("loading the the manager(Operator) image on Kind")
			err = utils.LoadImageToKindClusterWithName(projectimage)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("installing CRDs")
			cmd = exec.Command("make", "install")
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("deploying the controller-manager")
			cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectimage))
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func() error {
				// Get pod name
				cmd = exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				podNames := utils.GetNonEmptyLines(string(podOutput))
				if len(podNames) != 1 {
					return fmt.Errorf("expect 1 controller pods running, but got %d", len(podNames))
				}
				controllerPodName = podNames[0]
				ExpectWithOffset(2, controllerPodName).Should(ContainSubstring("controller-manager"))

				// Validate pod status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				status, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				if string(status) != "Running" {
					return fmt.Errorf("controller pod in %s status", status)
				}
				return nil
			}
			EventuallyWithOffset(1, verifyControllerUp, time.Minute, time.Second).Should(Succeed())
		})
	})

	Context("create and test cluster instance", func() {
		if skipClusterTests {
			Skip("skipping the cluster tests")
		}

		var (
			inst        *rdsv1alpha1.Valkey
			ctx, cancel = context.WithTimeout(context.Background(), time.Hour)
		)
		defer cancel()

		for _, param := range []TestParameter{} {
			if slices.Contains(skipVersions, param.Version) {
				continue
			}

			for _, cases := range clusterTestCases {
				Context(cases.When, func() {
					if cases.BeforeEach != nil {
						BeforeEach(func() {
							inst = cases.BeforeEach(param.Version, param.ServiceType)
						})
					}
					for _, spec := range cases.Specs {
						It(spec.Name, func() {
							spec.Func(ctx, inst)
						}, SpecTimeout(time.Minute*30))
					}

					if cases.AfterEach != nil {
						AfterEach(func() {
							cases.AfterEach(inst)
						})
					}
				})
			}
		}
	})

	Context("create and test failover instance", func() {
		if skipFailoverTests {
			Skip("skipping the failover tests")
		}

		var (
			inst        *rdsv1alpha1.Valkey
			ctx, cancel = context.WithTimeout(context.Background(), time.Hour)
		)
		defer cancel()

		for _, param := range []TestParameter{} {
			if slices.Contains(skipVersions, param.Version) {
				continue
			}

			for _, cases := range clusterTestCases {
				Context(cases.When, func() {
					if cases.BeforeEach != nil {
						BeforeEach(func() {
							inst = cases.BeforeEach(param.Version, param.ServiceType)
						})
					}
					for _, spec := range cases.Specs {
						It(spec.Name, func() {
							spec.Func(ctx, inst)
						}, SpecTimeout(time.Minute*30))
					}

					if cases.AfterEach != nil {
						AfterEach(func() {
							cases.AfterEach(inst)
						})
					}
				})
			}
		}
	})

	Context("create and test replica instance", func() {
		if skipFailoverTests {
			Skip("skipping the replica tests")
		}

		var (
			inst        *rdsv1alpha1.Valkey
			ctx, cancel = context.WithTimeout(context.Background(), time.Hour)
		)
		defer cancel()

		for _, param := range []TestParameter{} {
			if slices.Contains(skipVersions, param.Version) {
				continue
			}

			for _, cases := range clusterTestCases {
				Context(cases.When, func() {
					if cases.BeforeEach != nil {
						BeforeEach(func() {
							inst = cases.BeforeEach(param.Version, param.ServiceType)
						})
					}
					for _, spec := range cases.Specs {
						It(spec.Name, func() {
							spec.Func(ctx, inst)
						}, SpecTimeout(time.Minute*30))
					}

					if cases.AfterEach != nil {
						AfterEach(func() {
							cases.AfterEach(inst)
						})
					}
				})
			}
		}
	})
})
