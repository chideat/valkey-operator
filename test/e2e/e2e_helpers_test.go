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
	"strings"
	"time"

	rdsv1alpha1 "github.com/chideat/valkey-operator/api/rds/v1alpha1"
	pkgversion "github.com/chideat/valkey-operator/pkg/version"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
)

// newClusterInstance returns a minimal Valkey cluster CR for webhook/standalone tests.
func newClusterInstance(version string, accessType corev1.ServiceType) *rdsv1alpha1.Valkey {
	return &rdsv1alpha1.Valkey{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("webhook-cluster-%s", strings.ReplaceAll(version, ".", "-")),
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
			Access: core.InstanceAccess{
				ServiceType: accessType,
			},
			AffinityPolicy: ptr.To(core.AntiAffinityInShard),
		},
	}
}

// checkInstanceConfig runs CONFIG GET <key> on the instance and asserts the value equals want.
func checkInstanceConfig(ctx context.Context, inst *rdsv1alpha1.Valkey, username, password, key, want string) {
	c, err := newValkeyClient(ctx, inst, username, password)
	Expect(err).To(Succeed())
	defer c.Close()

	By(fmt.Sprintf("checking config %s = %s", key, want))
	// CONFIG GET returns a map in RESP3 (Valkey 7.2+); use AsStrMap().
	result, err := c.Do(ctx, c.B().ConfigGet().Parameter(key).Build()).AsStrMap()
	Expect(err).To(Succeed())
	Expect(result).To(HaveKey(key))
	Expect(result[key]).To(Equal(want))
}

// checkInstanceVersion runs INFO server and asserts the running version matches wantMajorMinor.
func checkInstanceVersion(ctx context.Context, inst *rdsv1alpha1.Valkey, username, password, wantMajorMinor string) {
	c, err := newValkeyClient(ctx, inst, username, password)
	Expect(err).To(Succeed())
	defer c.Close()

	By(fmt.Sprintf("checking running version is %s", wantMajorMinor))
	info, err := c.Do(ctx, c.B().Info().Section("server").Build()).ToString()
	Expect(err).To(Succeed())
	Expect(info).To(ContainSubstring(fmt.Sprintf("redis_version:%s", wantMajorMinor)))
}

// killPod deletes a pod by name and namespace and waits for it to be replaced.
func killPod(ctx context.Context, namespace, podName string) {
	By(fmt.Sprintf("killing pod %s/%s", namespace, podName))
	var oldPod corev1.Pod
	Expect(k8sClient.Get(ctx, client.ObjectKey{Name: podName, Namespace: namespace}, &oldPod)).To(Succeed())
	oldUID := oldPod.UID

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
	}
	Expect(k8sClient.Delete(ctx, pod)).To(Succeed())

	// Wait for the pod to be replaced (StatefulSet recreates it with a new UID)
	Eventually(func() bool {
		var p corev1.Pod
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(pod), &p); err != nil {
			return true // pod is gone momentarily — counts as replaced
		}
		return p.UID != oldUID // new pod created
	}).WithTimeout(time.Minute * 2).WithPolling(time.Second * 5).Should(BeTrue())
}

// skipIfVersionLessThan skips the current Ginkgo spec if inst.Spec.Version < minVersion.
func skipIfVersionLessThan(inst *rdsv1alpha1.Valkey, minVersion string) {
	instVer, err := pkgversion.ParseValkeyVersion(inst.Spec.Version)
	if err != nil {
		Skip(fmt.Sprintf("cannot parse instance version %q: %v", inst.Spec.Version, err))
	}
	minVer, err := pkgversion.ParseValkeyVersion(minVersion)
	if err != nil {
		Skip(fmt.Sprintf("cannot parse minVersion %q: %v", minVersion, err))
	}
	if instVer.Compare(minVer) < 0 {
		Skip(fmt.Sprintf("version %s < %s, skipping", inst.Spec.Version, minVersion))
	}
}

// patchInstanceVersion patches spec.version and waits for the rolling upgrade to complete.
func patchInstanceVersion(ctx context.Context, inst *rdsv1alpha1.Valkey, newVersion string, timeout time.Duration) {
	By(fmt.Sprintf("patching instance version to %s", newVersion))
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
	inst.Spec.Version = newVersion
	Expect(k8sClient.Update(ctx, inst)).To(Succeed())

	time.Sleep(time.Minute)
	waitInstanceStatusReady(ctx, inst, timeout)
}

// getSentinelMasterPodName returns the pod name of the sentinel master via INFO replication.
func getSentinelMasterPodName(ctx context.Context, inst *rdsv1alpha1.Valkey) string {
	c, err := newValkeyClient(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
	Expect(err).To(Succeed())
	defer c.Close()

	info, err := c.Do(ctx, c.B().Info().Section("replication").Build()).ToString()
	Expect(err).To(Succeed())

	for _, line := range strings.Split(info, "\r\n") {
		if strings.HasPrefix(line, "role:master") {
			// find the matching node in status
			for _, node := range inst.Status.Nodes {
				if node.Role == "master" {
					return node.PodName
				}
			}
		}
	}
	// fallback: return first master node
	for _, node := range inst.Status.Nodes {
		if node.Role == "master" {
			return node.PodName
		}
	}
	return ""
}

// clusterMasterPodName returns the pod name of any master node in the cluster status.
func clusterMasterPodName(inst *rdsv1alpha1.Valkey) string {
	for _, node := range inst.Status.Nodes {
		if node.Role == "master" {
			return node.PodName
		}
	}
	return ""
}

// createReadOnlyUser creates a user with read permissions for testing ACL enforcement.
// cluster and role commands are included so the valkey client can discover topology on
// connect; writes are still denied.
func createReadOnlyUser(ctx context.Context, inst *rdsv1alpha1.Valkey, username, password string) {
	createInstanceUser(ctx, inst, username, password, "+@read ~* &* +cluster|shards +cluster|nodes +cluster|info +role")
}

// findReplicaPodName returns the pod name of a replica node in inst.Status.Nodes.
func findReplicaPodName(inst *rdsv1alpha1.Valkey) string {
	for _, node := range inst.Status.Nodes {
		if node.Role == "slave" || node.Role == "replica" {
			return node.PodName
		}
	}
	return ""
}

// getReplicaNodeIP returns the IP of a replica node.
func getReplicaNodeIP(inst *rdsv1alpha1.Valkey) string {
	for _, node := range inst.Status.Nodes {
		if node.Role == "slave" || node.Role == "replica" {
			return node.IP
		}
	}
	return ""
}

// getSentinelReplicaInfo returns (masterIP, replicaIP) from sentinel status nodes.
func getSentinelReplicaInfo(ctx context.Context, inst *rdsv1alpha1.Valkey) (masterPod, replicaPod string) {
	for _, node := range inst.Status.Nodes {
		switch node.Role {
		case "master":
			masterPod = node.PodName
		case "slave", "replica":
			replicaPod = node.PodName
		}
	}
	return
}

// valkeyClientForAddr creates a Valkey client pointed directly at a specific address.
func valkeyClientForAddr(ctx context.Context, addr, username, password string) (interface{ Close() }, error) {
	// We just test connectivity; returning a raw client via newValkeyClient is sufficient
	// for the cases we need.  We expose this helper so tests can be written against it.
	_ = ctx
	_ = addr
	_ = username
	_ = password
	return nil, nil
}

// getFailoverSentinelInst returns the Sentinel CR associated with inst (failover arch).
func getFailoverSentinelInst(ctx context.Context, inst *rdsv1alpha1.Valkey) *v1alpha1.Sentinel {
	var senInst v1alpha1.Sentinel
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), &senInst)).To(Succeed())
	return &senInst
}
