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

package overwrite_test

import (
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/aclbuilder"
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
	"github.com/chideat/valkey-operator/internal/builder/failoverbuilder"
	"github.com/chideat/valkey-operator/internal/builder/overwrite"
	"github.com/chideat/valkey-operator/internal/builder/sentinelbuilder"
	"github.com/chideat/valkey-operator/internal/testutil"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/types/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
)

// The rules name what the builders generate. These tests render the real
// builders in their main configurations and check that everything the
// operator puts into a StatefulSet or PodDisruptionBudget is refused at
// admission and restored by the builders, so a name added to a builder cannot
// slip past the rules.

type sentinelMonitor struct{ types.FailoverMonitor }

func (sentinelMonitor) Policy() v1alpha1.FailoverPolicy { return v1alpha1.SentinelFailoverPolicy }

type setting struct {
	tls      bool
	storage  bool
	exporter bool
	family   corev1.IPFamily
}

var settings = []setting{
	{exporter: true, family: corev1.IPv4Protocol},
	{tls: true, storage: true, exporter: true, family: corev1.IPv4Protocol},
	{tls: true, family: corev1.IPv6Protocol},
}

func (s setting) String() string {
	return fmt.Sprintf("tls=%t,storage=%t,exporter=%t,%s", s.tls, s.storage, s.exporter, s.family)
}

func (s setting) access() core.InstanceAccess {
	return core.InstanceAccess{EnableTLS: s.tls, IPFamilyPrefer: s.family, ServiceType: corev1.ServiceTypeClusterIP}
}

func (s setting) storageSpec() *core.Storage {
	if !s.storage {
		return nil
	}
	return &core.Storage{StorageClassName: ptr.To("standard"), Capacity: ptr.To(resource.MustParse("1Gi"))}
}

func (s setting) exporterSpec() *core.Exporter {
	if !s.exporter {
		return nil
	}
	return &core.Exporter{Image: "oliver006/redis_exporter:v1.67.0-alpine"}
}

// operatorUser is the user the operator connects as, with its password secret.
func operatorUser(arch core.Arch, name string) *user.User {
	return &user.User{
		Name:     user.DefaultOperatorUserName,
		Role:     user.RoleOperator,
		Password: &user.Password{SecretName: aclbuilder.GenerateACLOperatorSecretName(arch, name)},
	}
}

var resources = corev1.ResourceRequirements{
	Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m"), corev1.ResourceMemory: resource.MustParse("200Mi")},
}

func render(t *testing.T, component overwrite.Component, s setting) *appsv1.StatefulSet {
	t.Helper()
	meta := metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "uid"}
	var (
		sts *appsv1.StatefulSet
		err error
	)
	switch component {
	case overwrite.ClusterNodes:
		cluster := &v1alpha1.Cluster{ObjectMeta: meta, Spec: v1alpha1.ClusterSpec{
			Image:     "valkey/valkey:8.1",
			Replicas:  v1alpha1.ClusterReplicas{Shards: 3, ReplicasOfShard: 1},
			Resources: resources,
			Access:    s.access(),
			Storage:   s.storageSpec(),
			Exporter:  s.exporterSpec(),
		}}
		inst := testutil.NewFakeClusterInstance(cluster).WithUsers(operatorUser(core.ValkeyCluster, meta.Name))
		sts, err = clusterbuilder.GenerateStatefulSet(inst, 0)
	case overwrite.FailoverNodes:
		failover := &v1alpha1.Failover{ObjectMeta: meta, Spec: v1alpha1.FailoverSpec{
			Image:     "valkey/valkey:8.1",
			Replicas:  2,
			Resources: resources,
			Access:    s.access(),
			Storage:   s.storageSpec(),
			Exporter:  s.exporterSpec(),
		}}
		inst := testutil.NewFakeFailoverInstance(failover).
			WithMonitor(sentinelMonitor{}).
			WithUsers(operatorUser(core.ValkeyFailover, meta.Name))
		sts, err = failoverbuilder.GenerateStatefulSet(inst)
	case overwrite.SentinelNodes:
		sentinel := &v1alpha1.Sentinel{ObjectMeta: meta, Spec: v1alpha1.SentinelSpec{
			Image:     "valkey/valkey:8.1",
			Replicas:  3,
			Resources: resources,
			Access: v1alpha1.SentinelInstanceAccess{
				InstanceAccess:        s.access(),
				DefaultPasswordSecret: map[bool]string{true: "demo-password"}[s.tls],
			},
		}}
		sts, err = sentinelbuilder.GenerateSentinelStatefulset(testutil.NewFakeSentinelInstance(sentinel))
	}
	require.NoError(t, err)
	return sts
}

func patchOf(t *testing.T, kind core.OverwriteKind, doc string) []core.Overwrite {
	t.Helper()
	return []core.Overwrite{{Kind: kind, Patch: apiextensionsv1.JSON{Raw: []byte(doc)}}}
}

// roundTrip passes obj through JSON, as applying overwrites does, so that
// comparisons do not trip over representations JSON does not keep, such as
// the cached string of a resource.Quantity.
func roundTrip[T any](t *testing.T, obj *T) *T {
	t.Helper()
	data, err := json.Marshal(obj)
	require.NoError(t, err)
	var ret T
	require.NoError(t, json.Unmarshal(data, &ret))
	return &ret
}

// refused asserts that admission refuses doc, naming want, and that the
// builders restore the StatefulSet to sts.
func refused(t *testing.T, component overwrite.Component, sts *appsv1.StatefulSet, doc, want string) {
	t.Helper()
	errs := overwrite.Validate(patchOf(t, core.OverwriteKindStatefulSet, doc), component, field.NewPath("spec", "overwrites"))
	if assert.NotEmpty(t, errs, "admission accepted %s", doc) {
		assert.Contains(t, errs.ToAggregate().Error(), want, doc)
	}

	got, problems, err := overwrite.StatefulSet(sts, patchOf(t, core.OverwriteKindStatefulSet, doc), component)
	require.NoError(t, err)
	assert.NotEmpty(t, problems, "the builders did not report %s", doc)
	expected := roundTrip(t, sts)
	assert.Equal(t, expected.Spec, got.Spec, "the builders did not restore %s", doc)
	assert.Equal(t, expected.Labels, got.Labels, "the builders did not restore %s", doc)
}

func accepted(t *testing.T, component overwrite.Component, doc string) {
	t.Helper()
	errs := overwrite.Validate(patchOf(t, core.OverwriteKindStatefulSet, doc), component, field.NewPath("spec", "overwrites"))
	assert.Empty(t, errs, doc)
}

func TestRulesCoverWhatTheBuildersGenerate(t *testing.T) {
	for _, component := range []overwrite.Component{overwrite.ClusterNodes, overwrite.FailoverNodes, overwrite.SentinelNodes} {
		for _, s := range settings {
			t.Run(fmt.Sprintf("%s/%s", component, s), func(t *testing.T) {
				sts := render(t, component, s)
				pod := sts.Spec.Template.Spec

				for key := range sts.Labels {
					refused(t, component, sts, fmt.Sprintf(`{"metadata":{"labels":{%q:"x"}}}`, key), fmt.Sprintf("metadata.labels[%s] is protected", key))
				}
				for key := range sts.Spec.Template.Labels {
					refused(t, component, sts, fmt.Sprintf(`{"spec":{"template":{"metadata":{"labels":{%q:"x"}}}}}`, key),
						fmt.Sprintf("spec.template.metadata.labels[%s] is protected", key))
				}
				for _, alias := range pod.HostAliases {
					refused(t, component, sts, fmt.Sprintf(`{"spec":{"template":{"spec":{"hostAliases":[{"ip":%q,"hostnames":["x"]}]}}}}`, alias.IP),
						fmt.Sprintf("hostAliases[ip=%s] is protected", alias.IP))
				}
				for _, v := range pod.Volumes {
					// The operator never mounts a host path, so the patch always changes the volume.
					refused(t, component, sts, fmt.Sprintf(`{"spec":{"template":{"spec":{"volumes":[{"name":%q,"hostPath":{"path":"/"}}]}}}}`, v.Name),
						fmt.Sprintf("volumes[name=%s] is protected", v.Name))
				}

				for list, cs := range map[string][]corev1.Container{"initContainers": pod.InitContainers, "containers": pod.Containers} {
					for _, c := range cs {
						item := func(body string) string {
							return fmt.Sprintf(`{"spec":{"template":{"spec":{%q:[{"name":%q,%s}]}}}}`, list, c.Name, body)
						}
						at := fmt.Sprintf("%s[name=%s]", list, c.Name)

						accepted(t, component, item(`"terminationMessagePath":"/dev/termination-log"`))
						for _, e := range c.Env {
							refused(t, component, sts, item(fmt.Sprintf(`"env":[{"name":%q,"value":"x"}]`, e.Name)),
								fmt.Sprintf("%s.env[name=%s] is protected", at, e.Name))
						}
						for _, m := range c.VolumeMounts {
							refused(t, component, sts, item(fmt.Sprintf(`"volumeMounts":[{"name":"x","mountPath":%q}]`, m.MountPath)),
								fmt.Sprintf("%s.volumeMounts[mountPath=%s] is protected", at, m.MountPath))
							nested := path.Join(m.MountPath, "x")
							refused(t, component, sts, item(fmt.Sprintf(`"volumeMounts":[{"name":"x","mountPath":%q}]`, nested)),
								fmt.Sprintf("%s.volumeMounts[mountPath=%s] is under %s", at, nested, path.Clean(m.MountPath)))
						}
						if c.Name != builder.ServerContainerName && c.Name != builder.SentinelContainerName {
							continue
						}
						for key, p := range map[string]*corev1.Probe{"startupProbe": c.StartupProbe, "livenessProbe": c.LivenessProbe, "readinessProbe": c.ReadinessProbe} {
							if p == nil {
								refused(t, component, sts, item(fmt.Sprintf(`%q:{"periodSeconds":5}`, key)), fmt.Sprintf("%s.%s is protected", at, key))
								continue
							}
							accepted(t, component, item(fmt.Sprintf(`%q:{"periodSeconds":5}`, key)))
							refused(t, component, sts, item(fmt.Sprintf(`%q:{"exec":{"command":["true"]}}`, key)), fmt.Sprintf("%s.%s.exec is protected", at, key))
						}
					}
				}
			})
		}
	}
}

func TestEveryComponentRunsItsListedContainers(t *testing.T) {
	// The other direction: a container the rules allow for a component should
	// be one its builder can generate, or admission accepts patches that do
	// nothing.
	for component, names := range map[overwrite.Component][]string{
		overwrite.ClusterNodes:  {builder.ServerContainerName, builder.ExporterContainerName, builder.AgentContainerName},
		overwrite.FailoverNodes: {builder.ServerContainerName, builder.ExporterContainerName},
		overwrite.SentinelNodes: {builder.SentinelContainerName, builder.AgentContainerName},
	} {
		var generated []string
		for _, s := range settings {
			for _, c := range render(t, component, s).Spec.Template.Spec.Containers {
				if !slices.Contains(generated, c.Name) {
					generated = append(generated, c.Name)
				}
			}
		}
		assert.ElementsMatch(t, names, generated, component)
	}
}

func renderPodDisruptionBudget(t *testing.T, component overwrite.Component) *policyv1.PodDisruptionBudget {
	t.Helper()
	meta := metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "uid"}
	var (
		pdb *policyv1.PodDisruptionBudget
		err error
	)
	switch component {
	case overwrite.ClusterNodes:
		cluster := &v1alpha1.Cluster{ObjectMeta: meta, Spec: v1alpha1.ClusterSpec{Replicas: v1alpha1.ClusterReplicas{Shards: 3, ReplicasOfShard: 1}}}
		pdb, err = clusterbuilder.GeneratePodDisruptionBudget(testutil.NewFakeClusterInstance(cluster), 0)
	case overwrite.FailoverNodes:
		failover := &v1alpha1.Failover{ObjectMeta: meta, Spec: v1alpha1.FailoverSpec{Replicas: 2}}
		pdb, err = failoverbuilder.GeneratePodDisruptionBudget(testutil.NewFakeFailoverInstance(failover))
	case overwrite.SentinelNodes:
		sentinel := &v1alpha1.Sentinel{ObjectMeta: meta, Spec: v1alpha1.SentinelSpec{Replicas: 3}}
		pdb, err = sentinelbuilder.GeneratePodDisruptionBudget(testutil.NewFakeSentinelInstance(sentinel))
	}
	require.NoError(t, err)
	return pdb
}

func TestRulesCoverWhatThePodDisruptionBudgetBuildersGenerate(t *testing.T) {
	for _, component := range []overwrite.Component{overwrite.ClusterNodes, overwrite.FailoverNodes, overwrite.SentinelNodes} {
		t.Run(string(component), func(t *testing.T) {
			pdb := renderPodDisruptionBudget(t, component)
			require.NotNil(t, pdb.Spec.MaxUnavailable, "the rules expect the builders to set maxUnavailable")

			refused := func(doc, want string) {
				t.Helper()
				errs := overwrite.Validate(patchOf(t, core.OverwriteKindPodDisruptionBudget, doc), component, field.NewPath("spec", "overwrites"))
				if assert.NotEmpty(t, errs, "admission accepted %s", doc) {
					assert.Contains(t, errs.ToAggregate().Error(), want, doc)
				}

				got, problems, err := overwrite.PodDisruptionBudget(pdb, patchOf(t, core.OverwriteKindPodDisruptionBudget, doc))
				require.NoError(t, err)
				assert.NotEmpty(t, problems, "the builders did not report %s", doc)
				expected := roundTrip(t, pdb)
				assert.Equal(t, expected.Spec, got.Spec, "the builders did not restore %s", doc)
				assert.Equal(t, expected.Labels, got.Labels, "the builders did not restore %s", doc)
			}
			for key := range pdb.Labels {
				refused(fmt.Sprintf(`{"metadata":{"labels":{%q:"x"}}}`, key), fmt.Sprintf("metadata.labels[%s] is protected", key))
			}
			refused(`{"spec":{"selector":{"matchLabels":{"x":"y"}}}}`, "spec.selector is protected")

			// minAvailable replaces the generated maxUnavailable; a patch that
			// sets both is refused, and the builders keep one of them.
			errs := overwrite.Validate(patchOf(t, core.OverwriteKindPodDisruptionBudget, `{"spec":{"minAvailable":1,"maxUnavailable":2}}`),
				component, field.NewPath("spec", "overwrites"))
			if assert.NotEmpty(t, errs) {
				assert.Contains(t, errs.ToAggregate().Error(), "spec.minAvailable cannot be set together with maxUnavailable")
			}
			for _, doc := range []string{`{"spec":{"minAvailable":1}}`, `{"spec":{"minAvailable":1,"maxUnavailable":2}}`} {
				got, _, err := overwrite.PodDisruptionBudget(pdb, patchOf(t, core.OverwriteKindPodDisruptionBudget, doc))
				require.NoError(t, err)
				assert.True(t, (got.Spec.MinAvailable == nil) != (got.Spec.MaxUnavailable == nil), "%s leaves both or neither set", doc)
			}
		})
	}
}
