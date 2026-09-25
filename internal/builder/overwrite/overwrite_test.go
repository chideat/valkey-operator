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
	"strings"
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
