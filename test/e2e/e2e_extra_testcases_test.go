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
	"slices"
	"strings"
	"time"

	"github.com/chideat/valkey-operator/api/core"
	rdsv1alpha1 "github.com/chideat/valkey-operator/api/rds/v1alpha1"
	"github.com/chideat/valkey-operator/api/v1alpha1"
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

// isVersionSkipped returns true if the given version is in the SKIP_VERSIONS list.
func isVersionSkipped(v string) bool {
	return slices.Contains(skipVersions, v)
}

// commonAdditionalSpecs returns Spec entries shared across cluster, failover, and replication
// test cases (ACL enforcement, auth rejection, config hot-reload, data types, version-gated).
func commonAdditionalSpecs(archLabel string) []Spec {
	return []Spec{
		{
			Name:   "ACL enforcement: restricted user cannot run write commands",
			Labels: []string{archLabel, "acl"},
			Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
				readPassword, _ := security.GeneratePassword(12)
				createReadOnlyUser(ctx, inst, "e2e-readonly", readPassword)

				c, err := newValkeyClient(ctx, inst, "e2e-readonly", readPassword)
				Expect(err).To(Succeed())
				defer c.Close()

				By("checking write is denied with NOPERM")
				err = c.Do(ctx, c.B().Set().Key("acl-write-test").Value("val").Build()).Error()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("NOPERM"))

				By("checking read is allowed (no NOPERM)")
				getErr := c.Do(ctx, c.B().Get().Key("key-0").Build()).Error()
				if getErr != nil {
					Expect(getErr.Error()).NotTo(ContainSubstring("NOPERM"))
				}
			},
		},
		{
			Name:   "wrong password is rejected",
			Labels: []string{archLabel, "auth"},
			Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
				_, err := newValkeyClient(ctx, inst, valkeyDefaultUsername, "totally-wrong-password-xyz-123")
				Expect(err).To(HaveOccurred())
			},
		},
		{
			Name:   "custom config change propagates to pods",
			Labels: []string{archLabel, "config"},
			Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
				if inst.Spec.CustomConfigs == nil {
					inst.Spec.CustomConfigs = map[string]string{}
				}
				inst.Spec.CustomConfigs["slowlog-log-slower-than"] = "9999"
				Expect(k8sClient.Update(ctx, inst)).To(Succeed())

				time.Sleep(time.Second * 30)
				waitInstanceStatusReady(ctx, inst, time.Minute*5)

				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
				checkInstanceConfig(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword, "slowlog-log-slower-than", "9999")
			},
		},
		{
			Name:   "hash, list, sorted set, and set operations",
			Labels: []string{archLabel, "datatypes"},
			Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
				c, err := newValkeyClient(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
				Expect(err).To(Succeed())
				defer c.Close()

				By("testing hash operations")
				for i := range 10 {
					key := fmt.Sprintf("hash-%d", i)
					Expect(c.Do(ctx, c.B().Hset().Key(key).FieldValue().FieldValue("f", fmt.Sprintf("v%d", i)).Build()).Error()).To(Succeed())
					val, err := c.Do(ctx, c.B().Hget().Key(key).Field("f").Build()).ToString()
					Expect(err).To(Succeed())
					Expect(val).To(Equal(fmt.Sprintf("v%d", i)))
				}

				By("testing list operations")
				for i := range 10 {
					key := fmt.Sprintf("list-%d", i)
					Expect(c.Do(ctx, c.B().Lpush().Key(key).Element(fmt.Sprintf("item%d", i)).Build()).Error()).To(Succeed())
					vals, err := c.Do(ctx, c.B().Lrange().Key(key).Start(0).Stop(-1).Build()).AsStrSlice()
					Expect(err).To(Succeed())
					Expect(vals).To(ContainElement(fmt.Sprintf("item%d", i)))
				}

				By("testing sorted set operations")
				for i := range 10 {
					key := fmt.Sprintf("zset-%d", i)
					Expect(c.Do(ctx, c.B().Zadd().Key(key).ScoreMember().ScoreMember(float64(i), fmt.Sprintf("m%d", i)).Build()).Error()).To(Succeed())
					members, err := c.Do(ctx, c.B().Zrange().Key(key).Min("0").Max("-1").Build()).AsStrSlice()
					Expect(err).To(Succeed())
					Expect(members).To(ContainElement(fmt.Sprintf("m%d", i)))
				}

				By("testing set operations")
				for i := range 10 {
					key := fmt.Sprintf("set-%d", i)
					Expect(c.Do(ctx, c.B().Sadd().Key(key).Member(fmt.Sprintf("m%d", i)).Build()).Error()).To(Succeed())
					members, err := c.Do(ctx, c.B().Smembers().Key(key).Build()).AsStrSlice()
					Expect(err).To(Succeed())
					Expect(members).To(ContainElement(fmt.Sprintf("m%d", i)))
				}
			},
		},
		{
			Name:   "latency-tracking enabled for Valkey 9.0+",
			Labels: []string{archLabel, "config", "version"},
			Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
				skipIfVersionLessThan(inst, "9.0")
				checkInstanceConfig(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword, "latency-tracking", "yes")
			},
		},
	}
}

func newClusterExtraInstance(version string, accessType corev1.ServiceType) *rdsv1alpha1.Valkey {
	inst := &rdsv1alpha1.Valkey{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("cluster-extra-%s", strings.ReplaceAll(version, ".", "-")),
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
			Exporter:       &rdsv1alpha1.ValkeyExporter{},
			AffinityPolicy: ptr.To(core.AntiAffinityInShard),
		},
	}
	if defaltStorageClass != "" {
		inst.Spec.Storage = &core.Storage{
			StorageClassName: ptr.To(defaltStorageClass),
		}
	}
	var tmpInst rdsv1alpha1.Valkey
	if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(inst), &tmpInst); err == nil {
		return &tmpInst
	} else if !errors.IsNotFound(err) {
		AbortSuite(fmt.Sprintf("failed to get valkey instance: %v", err))
	}
	return inst
}

func newFailoverExtraInstance(version string, accessType corev1.ServiceType) *rdsv1alpha1.Valkey {
	inst := &rdsv1alpha1.Valkey{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("failover-extra-%s", strings.ReplaceAll(version, ".", "-")),
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
		}
	}
	var tmpInst rdsv1alpha1.Valkey
	if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(inst), &tmpInst); err == nil {
		return &tmpInst
	} else if !errors.IsNotFound(err) {
		AbortSuite(fmt.Sprintf("failed to get valkey instance: %v", err))
	}
	return inst
}

func newReplicaExtraInstance(version string, accessType corev1.ServiceType) *rdsv1alpha1.Valkey {
	inst := &rdsv1alpha1.Valkey{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("replica-extra-%s", strings.ReplaceAll(version, ".", "-")),
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
			Access: core.InstanceAccess{
				ServiceType: accessType,
			},
			Exporter: &rdsv1alpha1.ValkeyExporter{},
		},
	}
	if defaltStorageClass != "" {
		inst.Spec.Storage = &core.Storage{
			StorageClassName: ptr.To(defaltStorageClass),
		}
	}
	var tmpInst rdsv1alpha1.Valkey
	if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(inst), &tmpInst); err == nil {
		return &tmpInst
	} else if !errors.IsNotFound(err) {
		AbortSuite(fmt.Sprintf("failed to get valkey instance: %v", err))
	}
	return inst
}

func init() {
	// ---- Cluster extra test cases ----
	clusterExtraSpecs := append(
		[]Spec{
			{
				Name:   "deploy cluster for extra tests",
				Labels: []string{"cluster", "deploy"},
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					By("create valkey instance")
					Expect(k8sClient.Create(ctx, inst)).To(Succeed())
					time.Sleep(time.Second * 30)
					waitInstanceStatusReady(ctx, inst, time.Minute*5)
					createInstanceUser(ctx, inst, "default", valkeyDefaultPassword, "+@all ~* &* -acl")
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
				},
			},
		},
		append(
			commonAdditionalSpecs("cluster"),
			[]Spec{
				{
					Name:    "cluster recovers after a master pod is killed",
					Labels:  []string{"cluster", "resilience"},
					Timeout: 15 * time.Minute,
					Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
						masterPod := clusterMasterPodName(inst)
						if masterPod == "" {
							Skip("no master pod found in status nodes")
						}
						killPod(ctx, testNamespace, masterPod)
						time.Sleep(time.Second * 30)
						waitInstanceStatusReady(ctx, inst, time.Minute*10)
						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
						checkInstanceRead(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
						checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
					},
				},
				{
					Name:   "delete cluster extra instance",
					Labels: []string{"cluster", "delete"},
					Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
						deleteInstance(ctx, inst)
					},
				},
			}...,
		)...,
	)

	clusterTestCases = append(clusterTestCases,
		TestData{
			When:       "additional cluster tests",
			BeforeEach: newClusterExtraInstance,
			Specs:      clusterExtraSpecs,
		},
		// Version upgrade: 8.1 → 8.2
		TestData{
			When: "version upgrade 8.1 to 8.2",
			BeforeEach: func(version string, accessType corev1.ServiceType) *rdsv1alpha1.Valkey {
				if version != "8.1" {
					return nil
				}
				inst := &rdsv1alpha1.Valkey{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-upgrade-81-82",
						Namespace: testNamespace,
					},
					Spec: rdsv1alpha1.ValkeySpec{
						Arch:    core.ValkeyCluster,
						Version: "8.1",
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
					Name:   "deploy at 8.1 for upgrade test",
					Labels: []string{"cluster", "upgrade"},
					Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
						if inst == nil || isVersionSkipped("8.2") {
							Skip("upgrade test only runs for version 8.1 when 8.2 is available")
						}
						Expect(k8sClient.Create(ctx, inst)).To(Succeed())
						time.Sleep(time.Second * 30)
						waitInstanceStatusReady(ctx, inst, time.Minute*5)
						checkInstanceWrite(ctx, inst, valkeyDefaultUsername, "")
					},
				},
				{
					Name:    "upgrade from 8.1 to 8.2 completes rolling update",
					Labels:  []string{"cluster", "upgrade"},
					Timeout: 45 * time.Minute,
					Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
						if inst == nil || isVersionSkipped("8.2") {
							Skip("upgrade test only runs for version 8.1 when 8.2 is available")
						}
						patchInstanceVersion(ctx, inst, "8.2", 45*time.Minute)
						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
						checkInstanceRead(ctx, inst, valkeyDefaultUsername, "")
						checkInstanceWrite(ctx, inst, valkeyDefaultUsername, "")
					},
				},
				{
					Name:   "delete upgrade 8.1→8.2 instance",
					Labels: []string{"cluster", "upgrade", "delete"},
					Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
						if inst == nil || isVersionSkipped("8.2") {
							Skip("upgrade test only runs for version 8.1 when 8.2 is available")
						}
						deleteInstance(ctx, inst)
					},
				},
			},
		},
		// Version upgrade: 8.2 → 9.0
		TestData{
			When: "version upgrade 8.2 to 9.0",
			BeforeEach: func(version string, accessType corev1.ServiceType) *rdsv1alpha1.Valkey {
				if version != "8.2" {
					return nil
				}
				inst := &rdsv1alpha1.Valkey{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-upgrade-82-90",
						Namespace: testNamespace,
					},
					Spec: rdsv1alpha1.ValkeySpec{
						Arch:    core.ValkeyCluster,
						Version: "8.2",
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
					Name:   "deploy at 8.2 for upgrade test",
					Labels: []string{"cluster", "upgrade"},
					Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
						if inst == nil || isVersionSkipped("8.2") {
							Skip("upgrade test only runs for version 8.2 when 8.2 is available")
						}
						Expect(k8sClient.Create(ctx, inst)).To(Succeed())
						time.Sleep(time.Second * 30)
						waitInstanceStatusReady(ctx, inst, time.Minute*5)
						checkInstanceWrite(ctx, inst, valkeyDefaultUsername, "")
					},
				},
				{
					Name:    "upgrade from 8.2 to 9.0 completes rolling update",
					Labels:  []string{"cluster", "upgrade"},
					Timeout: 45 * time.Minute,
					Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
						if inst == nil || isVersionSkipped("8.2") {
							Skip("upgrade test only runs for version 8.2 when 8.2 is available")
						}
						patchInstanceVersion(ctx, inst, "9.0", 45*time.Minute)
						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
						checkInstanceConfig(ctx, inst, valkeyDefaultUsername, "", "latency-tracking", "yes")
						checkInstanceRead(ctx, inst, valkeyDefaultUsername, "")
						checkInstanceWrite(ctx, inst, valkeyDefaultUsername, "")
					},
				},
				{
					Name:   "delete upgrade 8.2→9.0 instance",
					Labels: []string{"cluster", "upgrade", "delete"},
					Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
						if inst == nil || isVersionSkipped("8.2") {
							Skip("upgrade test only runs for version 8.2 when 8.2 is available")
						}
						deleteInstance(ctx, inst)
					},
				},
			},
		},
	)

	// ---- Failover extra test cases ----
	failoverExtraSpecs := append(
		[]Spec{
			{
				Name:   "deploy failover for extra tests",
				Labels: []string{"failover", "deploy"},
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					Expect(k8sClient.Create(ctx, inst)).To(Succeed())
					time.Sleep(time.Second * 30)
					waitInstanceStatusReady(ctx, inst, time.Minute*5)
					createInstanceUser(ctx, inst, "default", valkeyDefaultPassword, "+@all ~* &* -acl")
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
				},
			},
		},
		append(
			commonAdditionalSpecs("failover"),
			[]Spec{
				{
					Name:    "sentinel promotes replica when master is killed",
					Labels:  []string{"failover", "resilience"},
					Timeout: 15 * time.Minute,
					Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
						masterPod, _ := getSentinelReplicaInfo(ctx, inst)
						if masterPod == "" {
							Skip("no master pod found in failover status nodes")
						}
						killPod(ctx, testNamespace, masterPod)
						time.Sleep(time.Second * 30)
						waitInstanceStatusReady(ctx, inst, time.Minute*10)
						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
						checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
						checkInstanceRead(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
					},
				},
				{
					Name:   "delete failover extra instance",
					Labels: []string{"failover", "delete"},
					Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
						deleteInstance(ctx, inst)
					},
				},
			}...,
		)...,
	)

	failoverTestCases = append(failoverTestCases,
		TestData{
			When:       "additional failover tests",
			BeforeEach: newFailoverExtraInstance,
			Specs:      failoverExtraSpecs,
		},
	)

	// ---- Replication extra test cases ----
	replicaExtraSpecs := append(
		[]Spec{
			{
				Name:   "deploy replica for extra tests",
				Labels: []string{"replica", "deploy"},
				Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
					Expect(k8sClient.Create(ctx, inst)).To(Succeed())
					time.Sleep(time.Second * 30)
					waitInstanceStatusReady(ctx, inst, time.Minute*5)
					createInstanceUser(ctx, inst, "default", valkeyDefaultPassword, "+@all ~* &* -acl")
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
					checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
				},
			},
		},
		append(
			commonAdditionalSpecs("replica"),
			[]Spec{
				{
					Name:   "scale to 2 replicas and verify reads distribute",
					Labels: []string{"replica", "scale"},
					Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
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
					Name:   "delete replica extra instance",
					Labels: []string{"replica", "delete"},
					Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
						deleteInstance(ctx, inst)
					},
				},
			}...,
		)...,
	)

	replicationTestCases = append(replicationTestCases,
		TestData{
			When:       "additional replication tests",
			BeforeEach: newReplicaExtraInstance,
			Specs:      replicaExtraSpecs,
		},
	)
}
