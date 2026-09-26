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

package overwrite

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

// generated is a StatefulSet shaped like the failover builder's output.
func generated() *appsv1.StatefulSet {
	labels := map[string]string{builder.AppNameLabelKey: "demo", builder.ManagedByLabelKey: "valkey-operator"}
	probe := func() *corev1.Probe {
		return &corev1.Probe{
			ProbeHandler:  corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{"/opt/valkey-helper", "helper", "healthcheck"}}},
			PeriodSeconds: 10,
		}
	}
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "rfr-demo", Namespace: "default", Labels: labels},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    ptr.To(int32(2)),
			ServiceName: "rfr-demo",
			Selector:    &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					HostAliases:        []corev1.HostAlias{{IP: "127.0.0.1", Hostnames: []string{"local.inject"}}},
					ServiceAccountName: "valkey-instance-account",
					InitContainers:     []corev1.Container{{Name: builder.InitContainerName, Image: "helper:v1", Command: []string{"sh", "/opt/init_failover.sh"}}},
					Containers: []corev1.Container{
						{
							Name:           builder.ServerContainerName,
							Image:          "valkey:8.1",
							Command:        []string{"valkey-server", "/etc/valkey/valkey.conf"},
							Env:            []corev1.EnvVar{{Name: "POD_IP", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"}}}},
							Resources:      corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("200Mi")}},
							StartupProbe:   probe(),
							LivenessProbe:  probe(),
							ReadinessProbe: probe(),
							VolumeMounts:   []corev1.VolumeMount{{Name: "conf", MountPath: "/etc/valkey"}},
						},
						{
							Name:    builder.ExporterContainerName,
							Image:   "redis_exporter:v1.67.0",
							Command: []string{"/redis_exporter", "--web.listen-address", ":9121"},
							Env:     []corev1.EnvVar{{Name: "REDIS_ADDR", Value: "redis://local.inject:6379"}},
						},
					},
					Volumes: []corev1.Volume{{Name: "conf", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}},
				},
			},
		},
	}
}

func overwrites(patches ...string) []core.Overwrite {
	var ret []core.Overwrite
	for _, p := range patches {
		ret = append(ret, core.Overwrite{Kind: core.OverwriteKindStatefulSet, Patch: apiextensionsv1.JSON{Raw: []byte(p)}})
	}
	return ret
}

func container(t *testing.T, sts *appsv1.StatefulSet, name string) corev1.Container {
	t.Helper()
	for _, c := range sts.Spec.Template.Spec.Containers {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("container %s not found", name)
	return corev1.Container{}
}

func TestStatefulSetWithoutOverwrites(t *testing.T) {
	sts := generated()
	got, problems, err := StatefulSet(sts, nil, FailoverNodes)
	require.NoError(t, err)
	assert.Same(t, sts, got, "no overwrites must leave the generated object untouched")
	assert.Empty(t, problems)
	assert.NotContains(t, got.Annotations, ChecksumAnnotation)
}

func TestStatefulSetAppliesUserValues(t *testing.T) {
	sts := generated()
	got, problems, err := StatefulSet(sts, overwrites(`{
		"metadata": {"labels": {"team": "cache"}},
		"spec": {
			"minReadySeconds": 10,
			"template": {
				"metadata": {"annotations": {"example.com/scrape": "true"}},
				"spec": {
					"priorityClassName": "critical",
					"containers": [
						{"name": "exporter", "args": ["--include-system-metrics=true"], "env": [{"name": "REDIS_EXPORTER_COUNT_KEYS", "value": "db0=session:*"}]},
						{"name": "valkey", "livenessProbe": {"periodSeconds": 30}}
					]
				}
			}
		}
	}`), FailoverNodes)
	require.NoError(t, err)
	assert.Empty(t, problems)

	assert.Equal(t, "cache", got.Labels["team"])
	assert.Equal(t, "demo", got.Labels[builder.AppNameLabelKey])
	assert.Equal(t, int32(10), got.Spec.MinReadySeconds)
	assert.Equal(t, "true", got.Spec.Template.Annotations["example.com/scrape"])
	assert.Equal(t, "critical", got.Spec.Template.Spec.PriorityClassName)

	exporter := container(t, got, builder.ExporterContainerName)
	assert.Equal(t, []string{"--include-system-metrics=true"}, exporter.Args)
	assert.Equal(t, []corev1.EnvVar{
		{Name: "REDIS_ADDR", Value: "redis://local.inject:6379"},
		{Name: "REDIS_EXPORTER_COUNT_KEYS", Value: "db0=session:*"},
	}, exporter.Env)

	assert.Equal(t, builder.ServerContainerName, got.Spec.Template.Spec.Containers[0].Name,
		"the patch lists the exporter first, the generated order stays")

	valkey := container(t, got, builder.ServerContainerName)
	assert.Equal(t, int32(30), valkey.LivenessProbe.PeriodSeconds)
	assert.Equal(t, generated().Spec.Template.Spec.Containers[0].LivenessProbe.Exec, valkey.LivenessProbe.Exec)

	assert.NotEmpty(t, got.Annotations[ChecksumAnnotation])
	assert.Equal(t, generated(), sts, "the generated object must not be modified")
}

func TestStatefulSetRestoresProtectedFields(t *testing.T) {
	got, problems, err := StatefulSet(generated(), overwrites(`{
		"metadata": {"name": "other", "annotations": {"valkey.buf.red/checksum-overwrites": "x"}},
		"spec": {
			"replicas": 5,
			"template": {
				"metadata": {"labels": {"app.kubernetes.io/name": "other"}},
				"spec": {
					"serviceAccountName": "default",
					"hostAliases": [{"ip": "127.0.0.1", "hostnames": ["evil"]}],
					"volumes": [{"name": "conf", "hostPath": {"path": "/"}}, {"name": "extra", "emptyDir": {}}],
					"containers": [
						{"name": "valkey", "command": ["sh"], "resources": {"limits": {"memory": "1Gi"}}, "env": [{"name": "POD_IP", "value": "1.2.3.4"}]},
						{"name": "exporter", "args": ["--redis.addr=redis://evil:6379"]},
						{"name": "sidecar", "image": "busybox"}
					]
				}
			}
		}
	}`), FailoverNodes)
	require.NoError(t, err)

	want := generated()
	assert.Equal(t, want.Name, got.Name)
	assert.Equal(t, want.Spec.Replicas, got.Spec.Replicas)
	assert.Equal(t, want.Spec.Template.Labels, got.Spec.Template.Labels)
	assert.Equal(t, want.Spec.Template.Spec.ServiceAccountName, got.Spec.Template.Spec.ServiceAccountName)
	assert.Equal(t, want.Spec.Template.Spec.HostAliases, got.Spec.Template.Spec.HostAliases)
	assert.Equal(t, append(want.Spec.Template.Spec.Volumes, corev1.Volume{Name: "extra", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}),
		got.Spec.Template.Spec.Volumes, "the operator's volume is restored, the user's kept")
	assert.Equal(t, want.Spec.Template.Spec.Containers, got.Spec.Template.Spec.Containers, "operator containers restored, unknown one dropped")
	assert.NotEqual(t, "x", got.Annotations[ChecksumAnnotation])

	for _, loc := range []string{
		"metadata.name restored",
		"metadata.annotations[valkey.buf.red/checksum-overwrites] removed",
		"spec.replicas restored",
		"spec.template.metadata.labels[app.kubernetes.io/name] restored",
		"spec.template.spec.serviceAccountName restored",
		"spec.template.spec.hostAliases[ip=127.0.0.1] restored",
		"spec.template.spec.volumes[name=conf] restored",
		"spec.template.spec.containers[name=sidecar] removed, the operator does not run it",
		"spec.template.spec.containers[name=valkey].command restored",
		"spec.template.spec.containers[name=valkey].resources restored",
		"spec.template.spec.containers[name=valkey].env[name=POD_IP] restored",
		"spec.template.spec.containers[name=exporter].args removed",
	} {
		assert.Contains(t, problems, loc)
	}
}

func TestStatefulSetSkipsPatchesItCannotMerge(t *testing.T) {
	got, problems, err := StatefulSet(generated(), overwrites(
		`{"spec": {"template": {"spec": {"containers": [{"name": "valkey", "$patch": "delete"}]}}}}`,
		`{"spec": {"template": {"spec": {"priorityClassName": 5}}}}`,
		`{"spec": {"minReadySeconds": 10}}`,
	), FailoverNodes)
	require.NoError(t, err)

	require.Len(t, problems, 2)
	assert.True(t, strings.HasPrefix(problems[0], "overwrites[0] skipped: patch directive $patch"), problems[0])
	assert.True(t, strings.HasPrefix(problems[1], "overwrites[1] skipped:"), problems[1])
	assert.Equal(t, int32(10), got.Spec.MinReadySeconds, "the patch that can be merged still is")
	assert.Equal(t, generated().Spec.Template.Spec.Containers, got.Spec.Template.Spec.Containers)
}

func TestStatefulSetChecksum(t *testing.T) {
	sum := func(patches ...string) string {
		t.Helper()
		got, _, err := StatefulSet(generated(), overwrites(patches...), FailoverNodes)
		require.NoError(t, err)
		return got.Annotations[ChecksumAnnotation]
	}

	a := sum(`{"spec":{"minReadySeconds":10,"template":{"spec":{"priorityClassName":"x"}}}}`)
	assert.Equal(t, a, sum("{\n  \"spec\": {\"template\": {\"spec\": {\"priorityClassName\": \"x\"}}, \"minReadySeconds\": 10}\n}"),
		"layout and key order must not change the checksum")
	assert.NotEqual(t, a, sum(`{"spec":{"minReadySeconds":11,"template":{"spec":{"priorityClassName":"x"}}}}`))
	assert.NotEqual(t, a, sum(`{"spec":{"minReadySeconds":10}}`, `{"spec":{"template":{"spec":{"priorityClassName":"x"}}}}`),
		"splitting a patch in two is a different input")
}

// generatedPodDisruptionBudget is shaped like the failover builder's output.
func generatedPodDisruptionBudget() *policyv1.PodDisruptionBudget {
	labels := map[string]string{builder.AppNameLabelKey: "demo", builder.ManagedByLabelKey: "valkey-operator"}
	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: "rfr-demo", Namespace: "default", Labels: labels},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: ptr.To(intstr.FromInt(1)),
			Selector:       &metav1.LabelSelector{MatchLabels: labels},
		},
	}
}

func pdbOverwrites(patches ...string) []core.Overwrite {
	var ret []core.Overwrite
	for _, p := range patches {
		ret = append(ret, core.Overwrite{Kind: core.OverwriteKindPodDisruptionBudget, Patch: apiextensionsv1.JSON{Raw: []byte(p)}})
	}
	return ret
}

func TestPodDisruptionBudget(t *testing.T) {
	t.Run("without overwrites", func(t *testing.T) {
		pdb := generatedPodDisruptionBudget()
		got, problems, err := PodDisruptionBudget(pdb, overwrites(`{"spec":{"minReadySeconds":10}}`))
		require.NoError(t, err)
		assert.Same(t, pdb, got, "StatefulSet overwrites leave the PodDisruptionBudget untouched")
		assert.Empty(t, problems)
	})
	t.Run("applies user values", func(t *testing.T) {
		got, problems, err := PodDisruptionBudget(generatedPodDisruptionBudget(), pdbOverwrites(
			`{"metadata":{"labels":{"team":"cache"}},"spec":{"unhealthyPodEvictionPolicy":"AlwaysAllow","maxUnavailable":"50%"}}`))
		require.NoError(t, err)
		assert.Empty(t, problems)
		assert.Equal(t, "cache", got.Labels["team"])
		assert.Equal(t, ptr.To(policyv1.AlwaysAllow), got.Spec.UnhealthyPodEvictionPolicy)
		assert.Equal(t, ptr.To(intstr.FromString("50%")), got.Spec.MaxUnavailable)
		assert.NotEmpty(t, got.Annotations[ChecksumAnnotation])
	})
	t.Run("minAvailable replaces maxUnavailable", func(t *testing.T) {
		// With or without the null, which merge patches drop before it arrives.
		for _, patch := range []string{`{"spec":{"minAvailable":1}}`, `{"spec":{"minAvailable":1,"maxUnavailable":null}}`} {
			got, problems, err := PodDisruptionBudget(generatedPodDisruptionBudget(), pdbOverwrites(patch))
			require.NoError(t, err)
			assert.Empty(t, problems, patch)
			assert.Equal(t, ptr.To(intstr.FromInt(1)), got.Spec.MinAvailable, patch)
			assert.Nil(t, got.Spec.MaxUnavailable, patch)
		}
	})
	t.Run("keeps the object valid and the selector generated", func(t *testing.T) {
		got, problems, err := PodDisruptionBudget(generatedPodDisruptionBudget(), pdbOverwrites(
			`{"metadata":{"labels":{"app.kubernetes.io/name":"other"}},"spec":{"minAvailable":1,"maxUnavailable":2,"selector":{"matchLabels":{"app":"other"}}}}`))
		require.NoError(t, err)
		want := generatedPodDisruptionBudget()
		assert.Equal(t, want.Labels, got.Labels)
		assert.Equal(t, want.Spec.Selector, got.Spec.Selector)
		assert.Nil(t, got.Spec.MinAvailable, "a patch that sets both keeps maxUnavailable")
		assert.Equal(t, ptr.To(intstr.FromInt(2)), got.Spec.MaxUnavailable)
		assert.ElementsMatch(t, []string{
			"metadata.labels[app.kubernetes.io/name] restored",
			"spec.selector restored",
			"spec.minAvailable removed, it cannot be set together with maxUnavailable",
		}, problems)
	})
}

// generatedService is shaped like the failover builder's readwrite Service.
func generatedService(typ corev1.ServiceType) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rfr-demo-readwrite",
			Namespace: "default",
			Labels:    map[string]string{builder.AppNameLabelKey: "demo", builder.ManagedByLabelKey: "valkey-operator"},
		},
		Spec: corev1.ServiceSpec{
			Type:     typ,
			Ports:    []corev1.ServicePort{{Name: "server", Port: 6379, TargetPort: intstr.FromInt(6379), Protocol: corev1.ProtocolTCP}},
			Selector: map[string]string{builder.AppNameLabelKey: "demo", builder.RoleLabelKey: "master"},
		},
	}
}

func serviceOverwrites(target core.OverwriteTarget, patches ...string) []core.Overwrite {
	var ret []core.Overwrite
	for _, p := range patches {
		ret = append(ret, core.Overwrite{Kind: core.OverwriteKindService, Target: target, Patch: apiextensionsv1.JSON{Raw: []byte(p)}})
	}
	return ret
}

func TestService(t *testing.T) {
	t.Run("without overwrites for the target", func(t *testing.T) {
		svc := generatedService(corev1.ServiceTypeClusterIP)
		others := append(pdbOverwrites(`{"metadata":{"labels":{"team":"cache"}}}`),
			serviceOverwrites(core.OverwriteTargetReadOnly, `{"metadata":{"labels":{"team":"cache"}}}`)...)
		got, problems, err := Service(svc, others, core.OverwriteTargetReadWrite)
		require.NoError(t, err)
		assert.Same(t, svc, got, "overwrites of other kinds and targets leave the Service untouched")
		assert.Empty(t, problems)
	})
	t.Run("applies user values", func(t *testing.T) {
		got, problems, err := Service(generatedService(corev1.ServiceTypeLoadBalancer), serviceOverwrites(core.OverwriteTargetReadWrite, `{
			"metadata": {"labels": {"team": "cache"}, "annotations": {"service.beta.kubernetes.io/load-balancer-source-ranges": "10.0.0.0/8"}},
			"spec": {
				"externalTrafficPolicy": "Local", "loadBalancerSourceRanges": ["10.0.0.0/8"], "allocateLoadBalancerNodePorts": false,
				"sessionAffinity": "ClientIP", "sessionAffinityConfig": {"clientIP": {"timeoutSeconds": 60}},
				"internalTrafficPolicy": "Local", "publishNotReadyAddresses": true
			}}`), core.OverwriteTargetReadWrite)
		require.NoError(t, err)
		assert.Empty(t, problems)
		assert.Equal(t, "cache", got.Labels["team"])
		assert.Equal(t, "10.0.0.0/8", got.Annotations[corev1.AnnotationLoadBalancerSourceRangesKey])
		assert.Equal(t, corev1.ServiceExternalTrafficPolicyLocal, got.Spec.ExternalTrafficPolicy)
		assert.Equal(t, []string{"10.0.0.0/8"}, got.Spec.LoadBalancerSourceRanges)
		assert.Equal(t, ptr.To(false), got.Spec.AllocateLoadBalancerNodePorts)
		assert.Equal(t, corev1.ServiceAffinityClientIP, got.Spec.SessionAffinity)
		assert.Equal(t, ptr.To(int32(60)), got.Spec.SessionAffinityConfig.ClientIP.TimeoutSeconds)
		assert.Equal(t, ptr.To(corev1.ServiceInternalTrafficPolicyLocal), got.Spec.InternalTrafficPolicy)
		assert.True(t, got.Spec.PublishNotReadyAddresses)
		assert.NotEmpty(t, got.Annotations[ChecksumAnnotation])
	})
	t.Run("restores what the operator routes and announces through", func(t *testing.T) {
		got, problems, err := Service(generatedService(corev1.ServiceTypeClusterIP), serviceOverwrites(core.OverwriteTargetReadWrite, `{
			"metadata": {"labels": {"app.kubernetes.io/name": "other"}},
			"spec": {
				"type": "LoadBalancer", "selector": {"app": "other"}, "ports": [{"name": "other", "port": 7000}],
				"clusterIP": "None", "ipFamilyPolicy": "PreferDualStack", "externalIPs": ["192.0.2.1"],
				"loadBalancerClass": "example.com/lb", "loadBalancerSourceRanges": ["10.0.0.0/8"]
			}}`), core.OverwriteTargetReadWrite)
		require.NoError(t, err)
		want := generatedService(corev1.ServiceTypeClusterIP)
		assert.Equal(t, want.Labels, got.Labels)
		assert.Equal(t, want.Spec, got.Spec)
		assert.ElementsMatch(t, []string{
			"metadata.labels[app.kubernetes.io/name] restored",
			"spec.type restored",
			"spec.selector restored",
			"spec.ports restored",
			"spec.clusterIP removed",
			"spec.ipFamilyPolicy removed",
			"spec.externalIPs removed",
			"spec.loadBalancerClass removed",
			// The type stays ClusterIP, so the ranges go too.
			"spec.loadBalancerSourceRanges removed, it applies only when spec.type is LoadBalancer",
		}, problems)
	})
	t.Run("removes what the Service type does not accept", func(t *testing.T) {
		patch := `{
			"metadata": {"annotations": {"service.beta.kubernetes.io/load-balancer-source-ranges": "10.0.0.0/8"}},
			"spec": {
				"externalTrafficPolicy": "Local", "loadBalancerSourceRanges": ["10.0.0.0/8"], "allocateLoadBalancerNodePorts": false,
				"sessionAffinityConfig": {"clientIP": {"timeoutSeconds": 60}}, "internalTrafficPolicy": "Local"
			}}`
		got, problems, err := Service(generatedService(corev1.ServiceTypeClusterIP), serviceOverwrites(core.OverwriteTargetReadWrite, patch),
			core.OverwriteTargetReadWrite)
		require.NoError(t, err)
		want := generatedService(corev1.ServiceTypeClusterIP)
		want.Spec.InternalTrafficPolicy = ptr.To(corev1.ServiceInternalTrafficPolicyLocal)
		assert.Equal(t, want.Spec, got.Spec, "internalTrafficPolicy applies to every type")
		assert.NotContains(t, got.Annotations, corev1.AnnotationLoadBalancerSourceRangesKey)
		assert.ElementsMatch(t, []string{
			"metadata.annotations[service.beta.kubernetes.io/load-balancer-source-ranges] removed, it applies only when spec.type is LoadBalancer",
			"spec.loadBalancerSourceRanges removed, it applies only when spec.type is LoadBalancer",
			"spec.allocateLoadBalancerNodePorts removed, it applies only when spec.type is LoadBalancer",
			"spec.externalTrafficPolicy removed, it applies only when spec.type is NodePort or LoadBalancer",
			"spec.sessionAffinityConfig removed, it applies only when spec.sessionAffinity is ClientIP",
		}, problems)

		got, problems, err = Service(generatedService(corev1.ServiceTypeNodePort), serviceOverwrites(core.OverwriteTargetReadWrite, patch),
			core.OverwriteTargetReadWrite)
		require.NoError(t, err)
		assert.Equal(t, corev1.ServiceExternalTrafficPolicyLocal, got.Spec.ExternalTrafficPolicy, "a NodePort Service takes externalTrafficPolicy")
		assert.Nil(t, got.Spec.LoadBalancerSourceRanges)
		assert.Len(t, problems, 4)
	})
}

func TestFitServiceType(t *testing.T) {
	merged := func(typ corev1.ServiceType) *corev1.Service {
		svc := generatedService(typ)
		svc.Annotations = map[string]string{corev1.AnnotationLoadBalancerSourceRangesKey: "10.0.0.0/8", "note": "x"}
		svc.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
		svc.Spec.LoadBalancerSourceRanges = []string{"10.0.0.0/8"}
		svc.Spec.InternalTrafficPolicy = ptr.To(corev1.ServiceInternalTrafficPolicyLocal)
		return svc
	}

	svc := merged(corev1.ServiceTypeLoadBalancer)
	require.NoError(t, FitServiceType(svc))
	assert.Equal(t, merged(corev1.ServiceTypeLoadBalancer), svc, "a LoadBalancer Service takes them all")

	svc = merged(corev1.ServiceTypeNodePort)
	require.NoError(t, FitServiceType(svc))
	assert.Equal(t, corev1.ServiceExternalTrafficPolicyLocal, svc.Spec.ExternalTrafficPolicy)
	assert.Nil(t, svc.Spec.LoadBalancerSourceRanges)
	assert.Equal(t, map[string]string{"note": "x"}, svc.Annotations)

	svc = merged(corev1.ServiceTypeClusterIP)
	require.NoError(t, FitServiceType(svc))
	want := generatedService(corev1.ServiceTypeClusterIP)
	want.Annotations = map[string]string{"note": "x"}
	want.Spec.InternalTrafficPolicy = ptr.To(corev1.ServiceInternalTrafficPolicyLocal)
	assert.Equal(t, want, svc)
}

func TestChecksumChanged(t *testing.T) {
	with := func(sum string) *policyv1.PodDisruptionBudget {
		pdb := generatedPodDisruptionBudget()
		if sum != "" {
			pdb.Annotations = map[string]string{ChecksumAnnotation: sum, "other": "x"}
		}
		return pdb
	}
	for _, tc := range []struct {
		name            string
		generated, live string
		want            bool
	}{
		{"no overwrites on either side", "", "", false},
		{"the same overwrites", "a", "a", false},
		{"overwrites added", "a", "", true},
		{"overwrites removed", "", "a", true},
		{"overwrites changed", "b", "a", true},
	} {
		assert.Equal(t, tc.want, ChecksumChanged(with(tc.generated), with(tc.live)), tc.name)
	}
}

// textOverwrites gives each patch as a string, the way YAML block text
// arrives from `patch: |`.
func textOverwrites(t *testing.T, kind core.OverwriteKind, texts ...string) []core.Overwrite {
	t.Helper()
	var ret []core.Overwrite
	for _, text := range texts {
		raw, err := json.Marshal(text)
		require.NoError(t, err)
		ret = append(ret, core.Overwrite{Kind: kind, Patch: apiextensionsv1.JSON{Raw: raw}})
	}
	return ret
}

func TestTextPatches(t *testing.T) {
	const periodNull = `
spec:
  template:
    spec:
      containers:
        - name: valkey
          livenessProbe:
            periodSeconds: null
`
	t.Run("a null in text deletes the field", func(t *testing.T) {
		got, problems, err := StatefulSet(generated(), textOverwrites(t, core.OverwriteKindStatefulSet, periodNull), FailoverNodes)
		require.NoError(t, err)
		assert.Empty(t, problems)
		valkey := container(t, got, builder.ServerContainerName)
		assert.Zero(t, valkey.LivenessProbe.PeriodSeconds)
		assert.Equal(t, generated().Spec.Template.Spec.Containers[0].LivenessProbe.Exec, valkey.LivenessProbe.Exec)

		pdb, problems, err := PodDisruptionBudget(generatedPodDisruptionBudget(),
			textOverwrites(t, core.OverwriteKindPodDisruptionBudget, "spec:\n  maxUnavailable: null\n"))
		require.NoError(t, err)
		assert.Empty(t, problems)
		assert.Nil(t, pdb.Spec.MaxUnavailable)
	})
	t.Run("text and object give the same checksum", func(t *testing.T) {
		sum := func(overwrites []core.Overwrite) string {
			t.Helper()
			got, _, err := StatefulSet(generated(), overwrites, FailoverNodes)
			require.NoError(t, err)
			return got.Annotations[ChecksumAnnotation]
		}
		text := sum(textOverwrites(t, core.OverwriteKindStatefulSet, periodNull))
		assert.Equal(t, text, sum(overwrites(`{"spec":{"template":{"spec":{"containers":[{"name":"valkey","livenessProbe":{"periodSeconds":null}}]}}}}`)))
		assert.Equal(t, text, sum(textOverwrites(t, core.OverwriteKindStatefulSet,
			"# the probe period\nspec: {template: {spec: {containers: [{name: valkey, livenessProbe: {periodSeconds: null}}]}}}\n")),
			"layout and comments must not change the checksum")
	})
	t.Run("text that is not an object is skipped", func(t *testing.T) {
		got, problems, err := StatefulSet(generated(), append(
			textOverwrites(t, core.OverwriteKindStatefulSet, "spec: [unclosed", "- a list"),
			overwrites(`{"spec":{"minReadySeconds":10}}`)...), FailoverNodes)
		require.NoError(t, err)
		require.Len(t, problems, 2)
		assert.True(t, strings.HasPrefix(problems[0], "overwrites[0] skipped: patch text is not valid YAML"), problems[0])
		assert.Equal(t, "overwrites[1] skipped: patch must be an object, or a string that holds one in YAML or JSON", problems[1])
		assert.Equal(t, int32(10), got.Spec.MinReadySeconds, "the patch that can be read still applies")
	})
}
