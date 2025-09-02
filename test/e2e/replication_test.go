package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chideat/valkey-operator/api/core"
	rdsv1alpha1 "github.com/chideat/valkey-operator/api/rds/v1alpha1"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	security "github.com/chideat/valkey-operator/pkg/security/password"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var replicationTestCases = []TestData{
	{
		BeforeEach: func(version string, accessType corev1.ServiceType) *rdsv1alpha1.Valkey {
			inst := &rdsv1alpha1.Valkey{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("replica-valkey-%s", strings.ReplaceAll(version, ".", "-")),
					Namespace: testNamespace,
				},
				Spec: rdsv1alpha1.ValkeySpec{
					Arch:    core.ValkeyReplica,
					Version: version,
					Replicas: &rdsv1alpha1.ValkeyReplicas{
						ReplicasOfShard: 1,
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("200Mi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("200Mi"),
						},
					},
					CustomConfigs: map[string]string{},
					Access: core.InstanceAccess{
						ServiceType: accessType,
					},
					Exporter: &rdsv1alpha1.ValkeyExporter{},
				},
			}
			if defaltStorageClass != "" {
				inst.Spec.Storage = &core.Storage{
					StorageClassName: ptr.To(defaltStorageClass),
					Capacity:         ptr.To(resource.MustParse("1Gi")),
				}
			}

			var tmpInst rdsv1alpha1.Valkey
			if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(inst), &tmpInst); err == nil {
				return &tmpInst
			} else if !errors.IsNotFound(err) {
				AbortSuite(fmt.Sprintf("failed to get valkey instance: %v", err))
			}
			return inst
		},
		Specs: []Spec{
			{
				Name:   "deploy valkey",
				Labels: []string{"replica", "deploy"},
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					By("create valkey instance")
					Expect(k8sClient.Create(ctx, inst)).To(Succeed())

					time.Sleep(time.Second * 30)

					By("checking valkey instance created")
					waitInstanceStatusReady(ctx, inst, time.Minute*5)
				},
			},
			{
				Name:   "read/write data",
				Labels: []string{"replica", "readwrite"},
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
					checkInstanceRead(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
				},
			},
			{
				Name:   "create default user with password",
				Labels: []string{"replica", "user", "default"},
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					By("create default user")
					createInstanceUser(ctx, inst, "default", valkeyDefaultPassword, "+@all ~* &* -acl")

					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					checkInstanceRead(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
					checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
				},
			},
			{
				Name:   "create and delete users",
				Labels: []string{"replica", "user"},
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					for _, username := range []string{"user1", "user2", "user3"} {
						password, _ := security.GeneratePassword(12)
						By(fmt.Sprintf("create user %s", username))
						createInstanceUser(ctx, inst, username, password, "+@all ~* &* -acl")

						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
						checkInstanceRead(ctx, inst, username, password)
						checkInstanceWrite(ctx, inst, username, password)

						By("delete user")
						Expect(k8sClient.Delete(ctx, &v1alpha1.User{
							ObjectMeta: metav1.ObjectMeta{
								Name:      generateValkeyUserInstanceName(inst, username),
								Namespace: inst.GetNamespace(),
							},
						})).To(Succeed())

						By("check user deleted")
						Eventually(func() bool {
							var user v1alpha1.User
							if err := k8sClient.Get(ctx, client.ObjectKey{
								Name:      generateValkeyUserInstanceName(inst, username),
								Namespace: inst.GetNamespace(),
							}, &user); err != nil {
								if errors.IsNotFound(err) {
									return true
								}
							}
							return false
						}).WithTimeout(time.Minute).WithPolling(time.Second * 10).Should(BeTrue())
					}
				},
			},
			{
				Name:   "restart valkey",
				Labels: []string{"replica", "restart"},
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					By("restart valkey instance to restart")
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					inst.Spec.PodAnnotations = map[string]string{
						builder.RestartAnnotationKey: time.Now().Format(time.RFC3339Nano),
					}
					Expect(k8sClient.Update(ctx, inst)).To(Succeed())

					time.Sleep(time.Minute)

					waitInstanceStatusReady(ctx, inst, time.Minute*10)
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
					checkInstanceRead(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
				},
			},
			{
				Name:   "update valkey",
				Labels: []string{"replica", "update"},
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					By("update valkey instance resources")
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					inst.Spec.Resources = corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("300m"),
							corev1.ResourceMemory: resource.MustParse("300Mi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("300m"),
							corev1.ResourceMemory: resource.MustParse("300Mi"),
						},
					}
					Expect(k8sClient.Update(ctx, inst)).To(Succeed())

					time.Sleep(time.Minute)

					waitInstanceStatusReady(ctx, inst, time.Minute*5)
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
					checkInstanceRead(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
				},
			},
			{
				Name:   "delete instance",
				Labels: []string{"replica", "delete"},
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					deleteInstance(ctx, inst)
				},
			},
		},
	},
}
