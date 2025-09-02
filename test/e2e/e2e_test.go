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
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	skipVersions          = strings.Split(utils.GetEnv("SKIP_VERSIONS"), ",")

	namespace = utils.GetEnv("NAMESPACE", "valkey-system")
)

var (
	supportedVersions = []string{"7.2", "8.0", "8.1"}
)

func init() {
	// Filter out skipped versions
	filteredVersions := []string{}
	for _, v := range supportedVersions {
		if !slices.Contains(skipVersions, v) {
			filteredVersions = append(filteredVersions, v)
		}
	}
	supportedVersions = filteredVersions
}

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

		By("creating test namespace")
		_ = k8sClient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		})

		// clean all valkey instances in the test namespace
		By("cleaning all valkey instances in the test namespace")
		valkeyList := &rdsv1alpha1.ValkeyList{}
		err := k8sClient.List(context.Background(), valkeyList, &client.ListOptions{
			Namespace: testNamespace,
		})
		Expect(err).To(Succeed())
		for _, item := range valkeyList.Items {
			Expect(k8sClient.Delete(context.Background(), &item)).To(Succeed())
		}
		if len(valkeyList.Items) > 0 {
			time.Sleep(time.Second * 5)
		}
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

	Context("Operator", Label("operator"), func() {
		BeforeEach(func() {
			if skipDeployOperator {
				Skip("skipping the operator deployment")
			}
		})

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

	Context("create and test cluster instance", Label("cluster"), func() {
		BeforeEach(func() {
			if skipClusterTests {
				Skip("skipping the cluster tests")
			}
		})

		var (
			inst *rdsv1alpha1.Valkey
		)

		// Generate test parameters for all supported versions with ClusterIP service type
		testParameters := []TestParameter{}
		for _, version := range supportedVersions {
			testParameters = append(testParameters, TestParameter{
				Version:     version,
				ServiceType: corev1.ServiceTypeNodePort,
			})
		}

		for _, param := range testParameters {
			for _, cases := range clusterTestCases {
				Context(cases.When, func() {
					if cases.BeforeEach != nil {
						BeforeEach(func() {
							inst = cases.BeforeEach(param.Version, param.ServiceType)
						})
					}
					for _, spec := range cases.Specs {
						if spec.Skip {
							continue
						}
						opts := []any{
							func(ctx context.Context) {
								spec.Func(ctx, inst)
							},
						}
						if len(spec.Labels) > 0 {
							opts = append(opts, Label(spec.Labels...))
						}
						if spec.Timeout > 0 {
							opts = append(opts, SpecTimeout(spec.Timeout))
						} else {
							opts = append(opts, SpecTimeout(time.Minute*30))
						}
						It(spec.Name, opts...)
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

	Context("create and test failover instance", Label("failover"), func() {
		BeforeEach(func() {
			if skipFailoverTests {
				Skip("skipping the failover tests")
			}
		})

		var (
			inst *rdsv1alpha1.Valkey
		)

		// Generate test parameters for all supported versions with ClusterIP service type
		testParameters := []TestParameter{}
		for _, version := range supportedVersions {
			testParameters = append(testParameters, TestParameter{
				Version:     version,
				ServiceType: corev1.ServiceTypeNodePort,
			})
		}

		for _, param := range testParameters {
			for _, cases := range failoverTestCases {
				Context(cases.When, func() {
					if cases.BeforeEach != nil {
						BeforeEach(func() {
							inst = cases.BeforeEach(param.Version, param.ServiceType)
						})
					}
					for _, spec := range cases.Specs {
						if spec.Skip {
							continue
						}
						opts := []any{
							func(ctx context.Context) {
								spec.Func(ctx, inst)
							},
						}
						if len(spec.Labels) > 0 {
							opts = append(opts, Label(spec.Labels...))
						}
						if spec.Timeout > 0 {
							opts = append(opts, SpecTimeout(spec.Timeout))
						} else {
							opts = append(opts, SpecTimeout(time.Minute*30))
						}
						It(spec.Name, opts...)
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

	Context("create and test replica instance", Label("replica"), func() {
		BeforeEach(func() {
			if skipReplicationTests {
				Skip("skipping the replica tests")
			}
		})

		var (
			inst *rdsv1alpha1.Valkey
		)

		// Generate test parameters for all supported versions with ClusterIP service type
		testParameters := []TestParameter{}
		for _, version := range supportedVersions {
			testParameters = append(testParameters, TestParameter{
				Version:     version,
				ServiceType: corev1.ServiceTypeNodePort,
			})
		}

		for _, param := range testParameters {
			for _, cases := range replicationTestCases {
				Context(cases.When, func() {
					if cases.BeforeEach != nil {
						BeforeEach(func() {
							inst = cases.BeforeEach(param.Version, param.ServiceType)
						})
					}
					for _, spec := range cases.Specs {
						if spec.Skip {
							continue
						}
						opts := []any{
							func(ctx context.Context) {
								spec.Func(ctx, inst)
							},
						}
						if len(spec.Labels) > 0 {
							opts = append(opts, Label(spec.Labels...))
						}
						if spec.Timeout > 0 {
							opts = append(opts, SpecTimeout(spec.Timeout))
						} else {
							opts = append(opts, SpecTimeout(time.Minute*30))
						}
						It(spec.Name, opts...)
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