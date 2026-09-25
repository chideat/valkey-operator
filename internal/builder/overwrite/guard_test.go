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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func doc(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(s), &m))
	return m
}

// runRestore restores merged from base with g and returns the result and the
// reports, one "location msg" string each.
func runRestore(t *testing.T, g guard, base, merged string) (map[string]any, []string) {
	t.Helper()
	b, m := doc(t, base), doc(t, merged)
	var got []string
	g.restore(b, m, nil, func(at location, msg string) { got = append(got, at.String()+" "+msg) })
	return m, got
}

func runCheck(t *testing.T, g guard, patch string) []string {
	t.Helper()
	var got []string
	g.check(doc(t, patch), nil, func(at location, msg string) { got = append(got, at.String()+" "+msg) })
	return got
}

func TestFixed(t *testing.T) {
	g := fixed{"spec", "replicas"}

	t.Run("restores a changed value", func(t *testing.T) {
		m, reports := runRestore(t, g, `{"spec":{"replicas":3}}`, `{"spec":{"replicas":5}}`)
		assert.Equal(t, doc(t, `{"spec":{"replicas":3}}`), m)
		assert.Equal(t, []string{"spec.replicas restored"}, reports)
	})
	t.Run("restores a value whose parent the patch deleted", func(t *testing.T) {
		m, reports := runRestore(t, g, `{"spec":{"replicas":3}}`, `{}`)
		assert.Equal(t, doc(t, `{"spec":{"replicas":3}}`), m)
		assert.Equal(t, []string{"spec.replicas restored"}, reports)
	})
	t.Run("removes a value the generated object does not set", func(t *testing.T) {
		m, reports := runRestore(t, g, `{"spec":{}}`, `{"spec":{"replicas":5}}`)
		assert.Equal(t, doc(t, `{"spec":{}}`), m)
		assert.Equal(t, []string{"spec.replicas removed"}, reports)
	})
	t.Run("reports nothing when unchanged", func(t *testing.T) {
		_, reports := runRestore(t, g, `{"spec":{"replicas":3,"x":1}}`, `{"spec":{"replicas":3,"x":2}}`)
		assert.Empty(t, reports)
	})
	t.Run("check", func(t *testing.T) {
		assert.Equal(t, []string{"spec.replicas is protected"}, runCheck(t, g, `{"spec":{"replicas":1}}`))
		assert.Equal(t, []string{"spec.replicas is protected"}, runCheck(t, g, `{"spec":{"replicas":null}}`))
		assert.Equal(t, []string{"spec deletes protected fields"}, runCheck(t, g, `{"spec":null}`))
		assert.Empty(t, runCheck(t, g, `{"spec":{"minReadySeconds":10}}`))
	})
}

func TestEntries(t *testing.T) {
	g := entries{path: []string{"labels"}, keys: []string{"app"}, prefixes: []string{"sum-"}, fromBase: true}

	t.Run("restores generated and listed entries, keeps the user's", func(t *testing.T) {
		m, reports := runRestore(t, g,
			`{"labels":{"app":"a","gen":"g"}}`,
			`{"labels":{"app":"x","gen":"y","sum-1":"z","team":"t"}}`)
		assert.Equal(t, doc(t, `{"labels":{"app":"a","gen":"g","team":"t"}}`), m)
		assert.Equal(t, []string{"labels[app] restored", "labels[gen] restored", "labels[sum-1] removed"}, reports)
	})
	t.Run("restores entries the patch deleted with the whole map", func(t *testing.T) {
		m, _ := runRestore(t, g, `{"labels":{"app":"a"}}`, `{}`)
		assert.Equal(t, doc(t, `{"labels":{"app":"a"}}`), m)
	})
	t.Run("check reports listed keys and prefixes only", func(t *testing.T) {
		assert.Equal(t, []string{"labels[app] is protected", "labels[sum-1] is protected"},
			runCheck(t, g, `{"labels":{"app":"x","sum-1":"z","team":"t"}}`))
		assert.Equal(t, []string{"labels deletes protected entries"}, runCheck(t, g, `{"labels":null}`))
	})
}

func TestItems(t *testing.T) {
	g := items{path: []string{"env"}, mergeKey: "name", ids: []string{"POD_IP"}, fromBase: true}

	t.Run("restores generated items and keeps the user's", func(t *testing.T) {
		m, reports := runRestore(t, g,
			`{"env":[{"name":"A","value":"1"},{"name":"B","value":"2"}]}`,
			`{"env":[{"name":"A","value":"x"},{"name":"USER","value":"u"},{"name":"POD_IP","value":"p"}]}`)
		assert.Equal(t, doc(t, `{"env":[{"name":"A","value":"1"},{"name":"USER","value":"u"},{"name":"B","value":"2"}]}`), m)
		assert.Equal(t, []string{"[name=A] restored", "[name=B] restored", "[name=POD_IP] removed"}, stripPath(reports, "env"))
	})
	t.Run("restores the generated items of a deleted list", func(t *testing.T) {
		m, _ := runRestore(t, g, `{"env":[{"name":"A","value":"1"}]}`, `{}`)
		assert.Equal(t, doc(t, `{"env":[{"name":"A","value":"1"}]}`), m)
	})
	t.Run("check reports listed ids only", func(t *testing.T) {
		assert.Equal(t, []string{"env[name=POD_IP] is protected"}, runCheck(t, g, `{"env":[{"name":"POD_IP","value":"x"},{"name":"USER"}]}`))
		assert.Equal(t, []string{"env deletes protected items"}, runCheck(t, g, `{"env":null}`))
	})
}

func TestMounts(t *testing.T) {
	g := mounts{path: []string{"volumeMounts"}, paths: []string{"/tls", "/mnt/opt/"}}

	t.Run("restores generated mounts and drops mounts at or under a protected path", func(t *testing.T) {
		m, reports := runRestore(t, g,
			`{"volumeMounts":[{"name":"conf","mountPath":"/etc/valkey"},{"name":"opt","mountPath":"/mnt/opt/"}]}`,
			`{"volumeMounts":[
				{"name":"x","mountPath":"/etc/valkey"},
				{"name":"x","mountPath":"/etc/valkey/valkey.conf","subPath":"valkey.conf"},
				{"name":"x","mountPath":"/mnt/opt"},
				{"name":"x","mountPath":"/tls/../tls/ca.crt"},
				{"name":"scripts","mountPath":"/etc/valkey-scripts"}]}`)
		assert.Equal(t, doc(t, `{"volumeMounts":[
			{"name":"conf","mountPath":"/etc/valkey"},
			{"name":"scripts","mountPath":"/etc/valkey-scripts"},
			{"name":"opt","mountPath":"/mnt/opt/"}]}`), m)
		assert.Equal(t, []string{
			"[mountPath=/etc/valkey] restored",
			"[mountPath=/etc/valkey/valkey.conf] removed",
			"[mountPath=/mnt/opt] removed",
			"[mountPath=/tls/../tls/ca.crt] removed",
			"[mountPath=/mnt/opt/] restored",
		}, stripPath(reports, "volumeMounts"))
	})
	t.Run("restores the generated mounts of a deleted list", func(t *testing.T) {
		m, _ := runRestore(t, g, `{"volumeMounts":[{"name":"conf","mountPath":"/etc/valkey"}]}`, `{}`)
		assert.Equal(t, doc(t, `{"volumeMounts":[{"name":"conf","mountPath":"/etc/valkey"}]}`), m)
	})
	t.Run("check reports listed paths and paths under them", func(t *testing.T) {
		assert.Equal(t, []string{
			"volumeMounts[mountPath=/tls/] is protected",
			"volumeMounts[mountPath=/mnt/opt] is protected",
			"volumeMounts[mountPath=/tls/ca.crt] is under /tls, which the operator mounts",
		}, runCheck(t, g, `{"volumeMounts":[
			{"name":"x","mountPath":"/tls/"},
			{"name":"x","mountPath":"/mnt/opt"},
			{"name":"x","mountPath":"/tls/ca.crt"},
			{"name":"x","mountPath":"/tlsx"}]}`))
		assert.Equal(t, []string{"volumeMounts deletes protected items"}, runCheck(t, g, `{"volumeMounts":null}`))
	})
}

// stripPath drops the leading path from reports, for readability.
func stripPath(reports []string, path string) []string {
	out := make([]string, len(reports))
	for i, r := range reports {
		out[i] = r[len(path):]
	}
	return out
}

func TestGeneratedOnly(t *testing.T) {
	g := generatedOnly{path: []string{"containers"}, mergeKey: "name", ids: []string{"valkey", "exporter"}}

	t.Run("drops items the operator does not generate", func(t *testing.T) {
		m, reports := runRestore(t, g,
			`{"containers":[{"name":"valkey"}]}`,
			`{"containers":[{"name":"valkey"},{"name":"exporter"},{"name":"sidecar"}]}`)
		assert.Equal(t, doc(t, `{"containers":[{"name":"valkey"}]}`), m)
		assert.Len(t, reports, 2)
	})
	t.Run("check reports names outside the list and items without one", func(t *testing.T) {
		assert.Equal(t, []string{
			"containers[name=sidecar] is not generated by the operator",
			"containers has an item without name",
		}, runCheck(t, g, `{"containers":[{"name":"exporter"},{"name":"sidecar"},{"image":"x"}]}`))
	})
}

func TestWithin(t *testing.T) {
	g := within{path: []string{"containers"}, mergeKey: "name", id: "valkey", guards: []guard{fixed{"command"}}}

	t.Run("guards fields inside the item", func(t *testing.T) {
		m, reports := runRestore(t, g,
			`{"containers":[{"name":"valkey","command":["valkey-server"]}]}`,
			`{"containers":[{"name":"valkey","command":["sh"],"env":[{"name":"X"}]}]}`)
		assert.Equal(t, doc(t, `{"containers":[{"name":"valkey","command":["valkey-server"],"env":[{"name":"X"}]}]}`), m)
		assert.Equal(t, []string{"containers[name=valkey].command restored"}, reports)
	})
	t.Run("puts a deleted generated item back whole", func(t *testing.T) {
		m, reports := runRestore(t, g, `{"containers":[{"name":"valkey","command":["valkey-server"]}]}`, `{"containers":[]}`)
		assert.Equal(t, doc(t, `{"containers":[{"name":"valkey","command":["valkey-server"]}]}`), m)
		assert.Equal(t, []string{"containers[name=valkey] restored"}, reports)
	})
	t.Run("does nothing for an item the operator does not generate", func(t *testing.T) {
		_, reports := runRestore(t, g, `{"containers":[]}`, `{"containers":[{"name":"valkey","command":["sh"]}]}`)
		assert.Empty(t, reports)
	})
	t.Run("check", func(t *testing.T) {
		assert.Equal(t, []string{"containers[name=valkey].command is protected"},
			runCheck(t, g, `{"containers":[{"name":"valkey","command":["sh"]},{"name":"exporter","command":["sh"]}]}`))
	})
}

func TestFlags(t *testing.T) {
	g := flags{path: []string{"args"}, names: []string{"redis.addr", "web.listen-address"}}

	t.Run("restores the whole list when a reserved flag shows up", func(t *testing.T) {
		m, reports := runRestore(t, g, `{}`, `{"args":["--include-system-metrics","--redis.addr","redis://x"]}`)
		assert.Equal(t, doc(t, `{}`), m)
		assert.Equal(t, []string{"args removed"}, reports)
	})
	t.Run("keeps other flags", func(t *testing.T) {
		m, reports := runRestore(t, g, `{}`, `{"args":["--include-system-metrics=true","redis.addr"]}`)
		assert.Equal(t, doc(t, `{"args":["--include-system-metrics=true","redis.addr"]}`), m)
		assert.Empty(t, reports)
	})
	t.Run("check recognises every flag spelling", func(t *testing.T) {
		assert.Len(t, runCheck(t, g, `{"args":["--redis.addr=x","-redis.addr","--web.listen-address",":1"]}`), 3)
		assert.Empty(t, runCheck(t, g, `{"args":["--count-keys=db0=a*"]}`))
	})
}

func TestProbe(t *testing.T) {
	set := probe{key: "livenessProbe", operatorSets: true}
	unset := probe{key: "readinessProbe", operatorSets: false}
	generated := `{"livenessProbe":{"exec":{"command":["healthcheck"]},"periodSeconds":10}}`

	t.Run("keeps the handler and lets timing change", func(t *testing.T) {
		m, reports := runRestore(t, set, generated,
			`{"livenessProbe":{"exec":{"command":["true"]},"periodSeconds":30,"httpGet":{"port":1}}}`)
		assert.Equal(t, doc(t, `{"livenessProbe":{"exec":{"command":["healthcheck"]},"periodSeconds":30}}`), m)
		assert.Equal(t, []string{"livenessProbe.exec restored", "livenessProbe.httpGet removed"}, reports)
	})
	t.Run("puts a deleted probe back whole", func(t *testing.T) {
		m, _ := runRestore(t, set, generated, `{}`)
		assert.Equal(t, doc(t, generated), m)
	})
	t.Run("removes a probe the operator does not set", func(t *testing.T) {
		m, reports := runRestore(t, unset, `{}`, `{"readinessProbe":{"exec":{"command":["true"]}}}`)
		assert.Equal(t, doc(t, `{}`), m)
		assert.Equal(t, []string{"readinessProbe removed"}, reports)
	})
	t.Run("check", func(t *testing.T) {
		assert.Empty(t, runCheck(t, set, `{"livenessProbe":{"periodSeconds":30,"failureThreshold":5}}`))
		assert.Equal(t, []string{"livenessProbe.exec is protected"}, runCheck(t, set, `{"livenessProbe":{"exec":{"command":["true"]}}}`))
		assert.Equal(t, []string{"livenessProbe deletes protected fields"}, runCheck(t, set, `{"livenessProbe":null}`))
		assert.Equal(t, []string{"readinessProbe is protected"}, runCheck(t, unset, `{"readinessProbe":{"periodSeconds":5}}`))
	})
}

func TestOrdered(t *testing.T) {
	g := ordered{path: []string{"env"}, mergeKey: "name"}

	m, reports := runRestore(t, g,
		`{"env":[{"name":"A"},{"name":"B"}]}`,
		`{"env":[{"name":"USER"},{"name":"B"},{"name":"A","value":"x"}]}`)
	assert.Equal(t, doc(t, `{"env":[{"name":"A","value":"x"},{"name":"B"},{"name":"USER"}]}`), m,
		"generated items keep their order, the patch's follow")
	assert.Empty(t, reports)
	assert.Empty(t, runCheck(t, g, `{"env":[{"name":"A"}]}`))
}

func TestExclusive(t *testing.T) {
	g := exclusive{path: []string{"spec"}, generated: "maxUnavailable", others: []string{"minAvailable"}}
	generated := `{"spec":{"maxUnavailable":1}}`

	t.Run("the other key replaces the generated one", func(t *testing.T) {
		m, reports := runRestore(t, g, generated, `{"spec":{"maxUnavailable":1,"minAvailable":2}}`)
		assert.Equal(t, doc(t, `{"spec":{"minAvailable":2}}`), m)
		assert.Empty(t, reports)
	})
	t.Run("a patch that changes both keeps the generated one", func(t *testing.T) {
		m, reports := runRestore(t, g, generated, `{"spec":{"maxUnavailable":3,"minAvailable":2}}`)
		assert.Equal(t, doc(t, `{"spec":{"maxUnavailable":3}}`), m)
		assert.Equal(t, []string{"spec.minAvailable removed, it cannot be set together with maxUnavailable"}, reports)
	})
	t.Run("leaves one key alone", func(t *testing.T) {
		m, reports := runRestore(t, g, generated, `{"spec":{"minAvailable":2}}`)
		assert.Equal(t, doc(t, `{"spec":{"minAvailable":2}}`), m)
		assert.Empty(t, reports)
	})
	t.Run("check", func(t *testing.T) {
		assert.Empty(t, runCheck(t, g, `{"spec":{"minAvailable":2}}`))
		assert.Empty(t, runCheck(t, g, `{"spec":{"minAvailable":2,"maxUnavailable":null}}`))
		assert.Empty(t, runCheck(t, g, `{"spec":{"maxUnavailable":"50%"}}`))
		assert.Equal(t, []string{"spec.minAvailable cannot be set together with maxUnavailable"},
			runCheck(t, g, `{"spec":{"minAvailable":2,"maxUnavailable":1}}`))
	})
}
