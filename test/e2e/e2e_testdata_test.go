package e2e

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/chideat/valkey-operator/api/core"
	rdsv1alpha1 "github.com/chideat/valkey-operator/api/rds/v1alpha1"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
	"github.com/chideat/valkey-operator/internal/builder/failoverbuilder"
	"github.com/chideat/valkey-operator/internal/builder/sentinelbuilder"
	security "github.com/chideat/valkey-operator/pkg/security/password"
	"github.com/chideat/valkey-operator/test/utils"
	"github.com/valkey-io/valkey-go"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	testNamespace               = utils.GetEnv("NAMESPACE", "valkey-test")
	defaltStorageClass          = utils.GetEnv("STORAGE_CLASS")
	valkeyDefaultUsername       = "default"
	valkeyDefaultPassword, _    = security.GeneratePassword(12)
	valkeyDefaultSenPassword, _ = security.GeneratePassword(12)
)

func generateValkeyUserInstanceName(inst *rdsv1alpha1.Valkey, username string) string {
	switch inst.Spec.Arch {
	case core.ValkeyCluster:
		return fmt.Sprintf("cluster-acl-%s-%s", inst.GetName(), username)
	}
	return fmt.Sprintf("failover-acl-%s-%s", inst.GetName(), username)
}

func createInstanceUser(ctx context.Context, inst *rdsv1alpha1.Valkey, username, password, aclRules string) {
	secretName := fmt.Sprintf("valkey-user-%s-%s-%d", inst.Name, username, time.Now().Unix())
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: inst.GetNamespace(),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"password": []byte(password),
		},
	}
	Expect(k8sClient.Create(ctx, secret)).To(Succeed())

	if aclRules == "" {
		aclRules = fmt.Sprintf("+@all ~* &*")
	}

	user := v1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateValkeyUserInstanceName(inst, username),
			Namespace: inst.GetNamespace(),
		},
		Spec: v1alpha1.UserSpec{
			AccountType:     v1alpha1.CustomAccount,
			Arch:            inst.Spec.Arch,
			InstanceName:    inst.GetName(),
			Username:        username,
			PasswordSecrets: []string{secretName},
			AclRules:        aclRules,
		},
	}
	Expect(k8sClient.Create(ctx, &user)).To(Succeed())
	Eventually(func() bool {
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&user), &user); err == nil {
			return user.Status.Phase == v1alpha1.UserReady
		} else {
			GinkgoWriter.Printf("failed to get valkey user: %v", err)
		}
		return false
	}).WithTimeout(time.Minute * 5).WithPolling(time.Second * 10).Should(BeTrue())
}

func deleteInstance(ctx context.Context, inst *rdsv1alpha1.Valkey) {
	if inst.Spec.Storage != nil && inst.Spec.Storage.StorageClassName != nil {
		By("update instance with delete pvc")
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
		inst.Finalizers = []string{"delete-pvc"}
		Expect(k8sClient.Update(ctx, inst)).To(Succeed())
		waitInstanceStatusReady(ctx, inst, time.Second*30)
	}

	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
	Expect(k8sClient.Delete(ctx, inst)).To(Succeed())
}

func waitInstanceStatusReady(ctx context.Context, inst *rdsv1alpha1.Valkey, timeout time.Duration) {
	const defaultStatusCheckThreshold = 3
	threshold := defaultStatusCheckThreshold
	Eventually(func() bool {
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst); err == nil {
			if inst.Status.Phase == rdsv1alpha1.Ready {
				threshold -= 1
			} else {
				threshold = defaultStatusCheckThreshold
			}
			return threshold == 0
		} else {
			GinkgoWriter.Printf("failed to get valkey instance: %v", err)
		}
		return false
	}).WithTimeout(timeout).WithPolling(time.Second * 10).Should(BeTrue())

	labels := failoverbuilder.GenerateSelectorLabels(inst.Name)
	if inst.Spec.Arch == core.ValkeyCluster {
		labels = clusterbuilder.GenerateClusterLabels(inst.Name, nil)
	}
	var stsList appsv1.StatefulSetList
	Expect(k8sClient.List(ctx, &stsList, client.InNamespace(testNamespace), client.MatchingLabels(labels))).To(Succeed())
	if inst.Spec.Arch == core.ValkeyCluster {
		Expect(stsList.Items).To(HaveLen(int(inst.Spec.Replicas.Shards)))
	}
	for _, sts := range stsList.Items {
		Expect(sts.Status.ReadyReplicas).To(Equal(*sts.Spec.Replicas))
		Expect(sts.Status.ReadyReplicas).To(Equal(inst.Spec.Replicas.ReplicasOfShard))
		Expect(sts.Status.UpdateRevision).To(Equal(sts.Status.CurrentRevision))
	}

	if inst.Spec.Arch == core.ValkeyFailover {
		labels := sentinelbuilder.GenerateSelectorLabels(inst.Name)

		var stsList appsv1.StatefulSetList
		Expect(k8sClient.List(ctx, &stsList, client.InNamespace(testNamespace), client.MatchingLabels(labels))).To(Succeed())
		Expect(stsList.Items).To(HaveLen(1))
		for _, sts := range stsList.Items {
			Expect(sts.Status.ReadyReplicas).To(Equal(*sts.Spec.Replicas))
			Expect(sts.Status.ReadyReplicas).To(Equal(inst.Spec.Sentinel.Replicas))
			Expect(sts.Status.UpdateRevision).To(Equal(sts.Status.CurrentRevision))
		}
	}
}

func newValkeyClient(ctx context.Context, inst *rdsv1alpha1.Valkey, username, password string) (valkey.Client, error) {
	var addrs []string
	if inst.Spec.Arch == core.ValkeyCluster {
		for _, node := range inst.Status.Nodes {
			addrs = append(addrs, net.JoinHostPort(node.IP, node.Port))
		}
	} else if inst.Spec.Arch == core.ValkeyFailover {
		// get sentinel address
		var senInst v1alpha1.Sentinel
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), &senInst)).To(Succeed())
		for _, node := range senInst.Status.Nodes {
			addrs = append(addrs, net.JoinHostPort(node.IP, node.Port))
		}
	} else if inst.Spec.Arch == core.ValkeyReplica {
		addrs = []string{fmt.Sprintf("rfr-%s-readwrite.%s:6379", inst.GetName(), testNamespace)}
	}
	GinkgoWriter.Printf("valkey instance %s address: %v, username: %s, password: %s", inst.GetName(), addrs, username, password)

	options := valkey.ClientOption{
		Username:    username,
		Password:    password,
		InitAddress: addrs,
		ClientName:  "e2e-tests",
	}
	if inst.Spec.Arch == core.ValkeyFailover {
		options.Sentinel = valkey.SentinelOption{
			MasterSet: "mymaster",
		}
	} else if inst.Spec.Arch == core.ValkeyCluster {
		options.ShuffleInit = true
	}
	return valkey.NewClient(options)
}

func checkInstanceRead(ctx context.Context, inst *rdsv1alpha1.Valkey, username, password string) {
	client, err := newValkeyClient(ctx, inst, username, password)
	Expect(err).To(Succeed())
	defer client.Close()

	By("checking read valkey")
	for i := range 1000 {
		key := fmt.Sprintf("key-%d", i)
		val, err := client.Do(ctx, client.B().Get().Key(key).Build()).ToString()
		Expect(err).To(Succeed())
		Expect(val).To(Equal(key))
	}
}

func checkInstanceWrite(ctx context.Context, inst *rdsv1alpha1.Valkey, username, password string) {
	client, err := newValkeyClient(ctx, inst, username, password)
	Expect(err).To(Succeed())
	defer client.Close()

	By("checking write valkey")
	for i := range 1000 {
		key := fmt.Sprintf("key-%d", i)
		Expect(client.Do(ctx, client.B().Set().Key(key).Value(key).Build()).Error()).To(Succeed())
	}
}

type Spec struct {
	Name    string
	Labels  []string
	Timeout time.Duration
	Skip    bool
	Func    func(context.Context, *rdsv1alpha1.Valkey)
}

type TestData struct {
	When       string
	BeforeEach func(version string, accessType corev1.ServiceType) *rdsv1alpha1.Valkey
	Specs      []Spec
	AfterEach  func(*rdsv1alpha1.Valkey)
}

var clusterTestCases = []TestData{
	{
		BeforeEach: func(version string, accessType corev1.ServiceType) *rdsv1alpha1.Valkey {
			inst := &rdsv1alpha1.Valkey{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("cluster-valkey-%s", strings.ReplaceAll(version, ".", "-")),
					Namespace: testNamespace,
				},
				Spec: rdsv1alpha1.ValkeySpec{
					Arch:    core.ValkeyCluster,
					Version: version,
					Replicas: &rdsv1alpha1.ValkeyReplicas{
						Shards:          3,
						ReplicasOfShard: 2,
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
					Exporter:       &rdsv1alpha1.ValkeyExporter{},
					AffinityPolicy: ptr.To(core.AntiAffinityInShard),
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
				Labels: []string{"cluster", "deploy"},
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
				Labels: []string{"cluster", "readwrite"},
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
					checkInstanceRead(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
				},
			},
			{
				Name:   "create default user with password",
				Labels: []string{"cluster", "user", "default"},
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
				Labels: []string{"cluster", "user"},
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
				Name:   "scale up to 5 shards",
				Labels: []string{"cluster", "scale", "up"},
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					By("update valkey instance to 5 shards")
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					inst.Spec.Replicas.Shards = 5
					Expect(k8sClient.Update(ctx, inst)).To(Succeed())

					time.Sleep(time.Minute)

					waitInstanceStatusReady(ctx, inst, time.Minute*30)
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					checkInstanceRead(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
					checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
				},
			},
			{
				Name:   "scale down to 3 shards",
				Labels: []string{"cluster", "scale", "down"},
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					By("update valkey instance to 3 shards")
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					inst.Spec.Replicas.Shards = 3
					Expect(k8sClient.Update(ctx, inst)).To(Succeed())

					time.Sleep(time.Minute)

					waitInstanceStatusReady(ctx, inst, time.Minute*30)
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					checkInstanceRead(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
					checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
				},
			},
			{
				Name:   "restart valkey",
				Labels: []string{"cluster", "restart"},
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					By("update valkey instance to restart")
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					inst.Spec.PodAnnotations = map[string]string{
						builder.RestartAnnotationKey: time.Now().Format(time.RFC3339Nano),
					}
					Expect(k8sClient.Update(ctx, inst)).To(Succeed())

					time.Sleep(time.Minute)

					waitInstanceStatusReady(ctx, inst, time.Minute*10)
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					checkInstanceRead(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
					checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
				},
			},
			{
				Name:   "update valkey",
				Labels: []string{"cluster", "update"},
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
					checkInstanceRead(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
					checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
				},
			},
			{
				Name:   "delete valkey",
				Labels: []string{"cluster", "delete"},
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					deleteInstance(ctx, inst)
				},
			},
		},
	},
}

var failoverTestCases = []TestData{
	{
		BeforeEach: func(version string, accessType corev1.ServiceType) *rdsv1alpha1.Valkey {
			inst := &rdsv1alpha1.Valkey{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("failover-valkey-%s", strings.ReplaceAll(version, ".", "-")),
					Namespace: testNamespace,
				},
				Spec: rdsv1alpha1.ValkeySpec{
					Arch:    core.ValkeyFailover,
					Version: version,
					Replicas: &rdsv1alpha1.ValkeyReplicas{
						ReplicasOfShard: 2,
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
					Sentinel: &v1alpha1.SentinelSettings{
						SentinelSpec: v1alpha1.SentinelSpec{
							Replicas: 3,
						},
					},
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
				Labels: []string{"failover", "deploy"},
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
				Labels: []string{"failover", "readwrite"},
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
					checkInstanceRead(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
				},
			},
			{
				Name:   "create default user with password",
				Labels: []string{"failover", "user", "default"},
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
				Labels: []string{"failover", "user"},
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
				Name:   "scale up to 3 replicas",
				Labels: []string{"failover", "scale", "up"},
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					By("update valkey instance to 3 replicas")
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					inst.Spec.Replicas.ReplicasOfShard = 3
					Expect(k8sClient.Update(ctx, inst)).To(Succeed())

					time.Sleep(time.Minute)

					waitInstanceStatusReady(ctx, inst, time.Minute*10)
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					checkInstanceRead(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
					checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
				},
			},
			{
				Name:   "scale down to 2 replicas",
				Labels: []string{"failover", "scale", "down"},
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					By("update valkey instance to 1 replicas")
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					inst.Spec.Replicas.ReplicasOfShard = 2
					Expect(k8sClient.Update(ctx, inst)).To(Succeed())

					time.Sleep(time.Minute)

					waitInstanceStatusReady(ctx, inst, time.Minute*5)
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					checkInstanceRead(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
					checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
				},
			},
			{
				Name: "restart valkey",
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					By("update valkey instance to restart")
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					inst.Spec.PodAnnotations = map[string]string{
						builder.RestartAnnotationKey: time.Now().Format(time.RFC3339Nano),
					}
					Expect(k8sClient.Update(ctx, inst)).To(Succeed())

					time.Sleep(time.Minute)

					waitInstanceStatusReady(ctx, inst, time.Minute*10)
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					checkInstanceRead(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
					checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
				},
			},
			{
				Name:   "update valkey",
				Labels: []string{"failover", "update"},
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
					checkInstanceRead(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
					checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
				},
			},
			{
				Name:   "delete valkey",
				Labels: []string{"failover", "delete"},
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					deleteInstance(ctx, inst)
				},
			},
		},
	},
}

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
