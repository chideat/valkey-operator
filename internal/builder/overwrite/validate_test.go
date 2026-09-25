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
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func validate(component Component, patch string) field.ErrorList {
	return Validate(overwrites(patch), component, field.NewPath("spec", "overwrites"))
}

func TestValidateAcceptsUserFields(t *testing.T) {
	for _, patch := range []string{
		`{"metadata": {"labels": {"team": "cache"}, "annotations": {"note": "x"}}}`,
		`{"spec": {"minReadySeconds": 10}}`,
		`{"spec": {"template": {"metadata": {"annotations": {"example.com/scrape": "true"}}}}}`,
		`{"spec": {"template": {"spec": {"priorityClassName": "critical", "topologySpreadConstraints": [{"maxSkew": 1, "topologyKey": "zone", "whenUnsatisfiable": "ScheduleAnyway"}]}}}}`,
		`{"spec": {"template": {"spec": {"volumes": [{"name": "scripts", "configMap": {"name": "lua"}}]}}}}`,
		`{"spec": {"template": {"spec": {"containers": [{"name": "exporter", "args": ["--include-system-metrics=true"], "env": [{"name": "REDIS_EXPORTER_COUNT_KEYS", "value": "db0=a*"}], "volumeMounts": [{"name": "scripts", "mountPath": "/scripts"}]}]}}}}`,
		`{"spec": {"template": {"spec": {"containers": [{"name": "valkey", "livenessProbe": {"periodSeconds": 30}, "readinessProbe": {"failureThreshold": 5}}]}}}}`,
		`{"spec": {"template": {"spec": {"initContainers": [{"name": "init", "resources": {"limits": {"cpu": "100m"}}}]}}}}`,
	} {
		assert.Empty(t, validate(FailoverNodes, patch), patch)
	}
}

func TestValidateRejects(t *testing.T) {
	for _, tc := range []struct {
		name      string
		component Component
		patch     string
		want      string
	}{
		{"replicas", FailoverNodes, `{"spec": {"replicas": 3}}`, "spec.replicas is protected"},
		{"object name", FailoverNodes, `{"metadata": {"name": "x"}}`, "metadata.name is protected"},
		{"operator label", FailoverNodes, `{"metadata": {"labels": {"app.kubernetes.io/name": "x"}}}`, "metadata.labels[app.kubernetes.io/name] is protected"},
		{"checksum annotation", FailoverNodes, `{"spec": {"template": {"metadata": {"annotations": {"valkey.buf.red/checksum-secret": "x"}}}}}`, "is protected"},
		{"restart annotation", FailoverNodes, `{"spec": {"template": {"metadata": {"annotations": {"kubectl.kubernetes.io/restartedAt": "x"}}}}}`, "is protected"},
		{"service account", FailoverNodes, `{"spec": {"template": {"spec": {"serviceAccountName": "default"}}}}`, "serviceAccountName is protected"},
		{"typed field", FailoverNodes, `{"spec": {"template": {"spec": {"tolerations": []}}}}`, "tolerations is protected"},
		{"local.inject alias", FailoverNodes, `{"spec": {"template": {"spec": {"hostAliases": [{"ip": "127.0.0.1", "hostnames": ["x"]}]}}}}`, "hostAliases[ip=127.0.0.1] is protected"},
		{"operator volume", FailoverNodes, `{"spec": {"template": {"spec": {"volumes": [{"name": "conf", "emptyDir": {}}]}}}}`, "volumes[name=conf] is protected"},
		{"valkey command", FailoverNodes, `{"spec": {"template": {"spec": {"containers": [{"name": "valkey", "command": ["sh"]}]}}}}`, "containers[name=valkey].command is protected"},
		{"valkey resources", FailoverNodes, `{"spec": {"template": {"spec": {"containers": [{"name": "valkey", "resources": {}}]}}}}`, "containers[name=valkey].resources is protected"},
		{"probe handler", FailoverNodes, `{"spec": {"template": {"spec": {"containers": [{"name": "valkey", "livenessProbe": {"exec": {"command": ["true"]}}}]}}}}`, "livenessProbe.exec is protected"},
		{"operator env", FailoverNodes, `{"spec": {"template": {"spec": {"containers": [{"name": "exporter", "env": [{"name": "REDIS_ADDR", "value": "x"}]}]}}}}`, "env[name=REDIS_ADDR] is protected"},
		{"operator mount", FailoverNodes, `{"spec": {"template": {"spec": {"containers": [{"name": "exporter", "volumeMounts": [{"name": "x", "mountPath": "/tls"}]}]}}}}`, "volumeMounts[mountPath=/tls] is protected"},
		{"exporter flag", FailoverNodes, `{"spec": {"template": {"spec": {"containers": [{"name": "exporter", "args": ["--redis.password=x"]}]}}}}`, "sets --redis.password"},
		{"init args", FailoverNodes, `{"spec": {"template": {"spec": {"initContainers": [{"name": "init", "args": ["x"]}]}}}}`, "initContainers[name=init].args is protected"},
		{"unknown container", FailoverNodes, `{"spec": {"template": {"spec": {"containers": [{"name": "sidecar", "image": "busybox"}]}}}}`, "containers[name=sidecar] is not generated by the operator"},
		{"agent on failover", FailoverNodes, `{"spec": {"template": {"spec": {"containers": [{"name": "agent", "args": []}]}}}}`, "containers[name=agent] is not generated by the operator"},
		{"valkey on sentinel", SentinelNodes, `{"spec": {"template": {"spec": {"containers": [{"name": "valkey"}]}}}}`, "containers[name=valkey] is not generated by the operator"},
		{"sentinel readiness probe", SentinelNodes, `{"spec": {"template": {"spec": {"containers": [{"name": "sentinel", "readinessProbe": {"periodSeconds": 5}}]}}}}`, "readinessProbe is protected"},
		{"deleting the containers", FailoverNodes, `{"spec": {"template": {"spec": {"containers": null}}}}`, "deletes protected"},
		{"deleting the pod spec", FailoverNodes, `{"spec": {"template": {"spec": null}}}`, "spec.template.spec deletes protected fields"},
		{"directive", FailoverNodes, `{"spec": {"template": {"spec": {"$setElementOrder/containers": [{"name": "valkey"}]}}}}`, "patch directive $setElementOrder/containers is not supported"},
		{"unknown field", FailoverNodes, `{"spec": {"template": {"spec": {"priorityClass": "x"}}}}`, `unknown field "spec.template.spec.priorityClass"`},
		{"field in the wrong case", FailoverNodes, `{"spec": {"template": {"spec": {"Containers": []}}}}`, `unknown field "spec.template.spec.Containers"`},
		{"value of the wrong type", FailoverNodes, `{"spec": {"minReadySeconds": "10"}}`, "cannot unmarshal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errs := validate(tc.component, tc.patch)
			if assert.NotEmpty(t, errs) {
				assert.Contains(t, errs.ToAggregate().Error(), tc.want)
			}
		})
	}
}

func TestValidateKinds(t *testing.T) {
	path := field.NewPath("spec", "overwrites")

	errs := Validate([]core.Overwrite{{Kind: "Deployment", Patch: apiextensionsv1.JSON{Raw: []byte(`{}`)}}}, FailoverNodes, path)
	if assert.Len(t, errs, 1) {
		assert.Equal(t, field.ErrorTypeNotSupported, errs[0].Type)
		assert.Equal(t, "spec.overwrites[0].kind", errs[0].Field)
	}

	errs = Validate(overwrites(`{}`, `{}`), FailoverNodes, path)
	if assert.Len(t, errs, 1) {
		assert.Equal(t, field.ErrorTypeDuplicate, errs[0].Type)
		assert.Equal(t, "spec.overwrites[1].kind", errs[0].Field)
	}
}

func TestValidatePodDisruptionBudget(t *testing.T) {
	path := field.NewPath("spec", "overwrites")
	for _, patch := range []string{
		`{"metadata": {"labels": {"team": "cache"}, "annotations": {"note": "x"}}}`,
		`{"spec": {"unhealthyPodEvictionPolicy": "AlwaysAllow"}}`,
		`{"spec": {"maxUnavailable": "50%"}}`,
		`{"spec": {"minAvailable": 1}}`,
		`{"spec": {"minAvailable": 1, "maxUnavailable": null}}`,
	} {
		assert.Empty(t, Validate(pdbOverwrites(patch), FailoverNodes, path), patch)
	}

	for _, tc := range []struct {
		name  string
		patch string
		want  string
	}{
		{"selector", `{"spec": {"selector": {"matchLabels": {"app": "other"}}}}`, "spec.selector is protected"},
		{"operator label", `{"metadata": {"labels": {"app.kubernetes.io/name": "x"}}}`, "metadata.labels[app.kubernetes.io/name] is protected"},
		{"minAvailable and maxUnavailable", `{"spec": {"minAvailable": 1, "maxUnavailable": 1}}`, "spec.minAvailable cannot be set together with maxUnavailable"},
		{"deleting the spec", `{"spec": null}`, "spec deletes protected fields"},
		{"unknown field", `{"spec": {"minReadySeconds": 10}}`, `unknown field "spec.minReadySeconds"`},
		{"directive", `{"spec": {"$retainKeys": ["selector"]}}`, "patch directive $retainKeys is not supported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errs := Validate(pdbOverwrites(tc.patch), FailoverNodes, path)
			if assert.NotEmpty(t, errs) {
				assert.Contains(t, errs.ToAggregate().Error(), tc.want)
			}
		})
	}

	t.Run("one entry per kind", func(t *testing.T) {
		both := append(overwrites(`{}`), pdbOverwrites(`{}`)...)
		assert.Empty(t, Validate(both, FailoverNodes, path))
		errs := Validate(pdbOverwrites(`{}`, `{}`), FailoverNodes, path)
		if assert.Len(t, errs, 1) {
			assert.Equal(t, field.ErrorTypeDuplicate, errs[0].Type)
		}
	})
}
