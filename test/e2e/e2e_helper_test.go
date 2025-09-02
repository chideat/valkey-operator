package e2e

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/chideat/valkey-operator/api/core"
	rdsv1alpha1 "github.com/chideat/valkey-operator/api/rds/v1alpha1"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
	"github.com/chideat/valkey-operator/internal/builder/failoverbuilder"
	"github.com/chideat/valkey-operator/internal/builder/sentinelbuilder"
	"github.com/valkey-io/valkey-go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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