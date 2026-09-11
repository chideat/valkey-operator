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

package sentinelbuilder

import (
	"strings"
	"testing"

	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The Sentinel renderer applied no forbidden-directive filter at all, and unlike the
// Valkey CR there is no sentinel webhook behind it to reject the key earlier. Anything
// a user put in CustomConfigs reached the rendered sentinel.conf verbatim, so a
// password-carrying directive landed in a ConfigMap in cleartext.
func TestGenerateSentinelConfigMapDropsForbiddenCustomConfigs(t *testing.T) {
	forbidden := map[string]string{
		"requirepass": "s3cret",
		// the 8.0+ spellings are aliases of masterauth/masteruser, so they carry a
		// password just as well
		"primaryauth":       "s3cret-primary",
		"primaryuser":       "admin",
		"masterauth":        "s3cret-master",
		"tls-key-file-pass": "s3cret-tls",
		// valkey matches directive names case-insensitively, so a differently-cased
		// key configures the same thing and must not slip past the filter
		"RequirePass": "s3cret-cased",
		"AclFile":     "/data/users.acl",
	}

	sentinel := &v1alpha1.Sentinel{
		ObjectMeta: metav1.ObjectMeta{Name: "test-sentinel", Namespace: "default"},
		Spec: v1alpha1.SentinelSpec{
			Replicas:      3,
			CustomConfigs: forbidden,
		},
	}

	cm, err := GenerateSentinelConfigMap(&mockSentinelInstance{Sentinel: sentinel})
	assert.NoError(t, err)
	assert.NotNil(t, cm)

	rendered := cm.Data[SentinelConfigFileName]
	assert.NotEmpty(t, rendered, "renderer produced no config")

	for key, val := range forbidden {
		assert.NotContains(t, rendered, val,
			"forbidden directive %q rendered its value into the sentinel ConfigMap", key)
		for _, line := range strings.Split(rendered, "\n") {
			name, _, _ := strings.Cut(strings.TrimSpace(line), " ")
			assert.NotEqual(t, strings.ToLower(key), name,
				"forbidden directive %q was rendered into the sentinel ConfigMap", key)
		}
	}
}

// The filter applies to user-supplied keys only. The operator's own defaults must still
// reach the rendered config, or the fix would strip the instance's working configuration.
func TestGenerateSentinelConfigMapKeepsAllowedConfigs(t *testing.T) {
	sentinel := &v1alpha1.Sentinel{
		ObjectMeta: metav1.ObjectMeta{Name: "test-sentinel", Namespace: "default"},
		Spec: v1alpha1.SentinelSpec{
			Replicas: 3,
			CustomConfigs: map[string]string{
				"maxclients": "20000",
				// mixed case on an allowed key is lowercased, not dropped
				"TCP-Backlog": "1024",
			},
		},
	}

	cm, err := GenerateSentinelConfigMap(&mockSentinelInstance{Sentinel: sentinel})
	assert.NoError(t, err)

	rendered := cm.Data[SentinelConfigFileName]
	assert.Contains(t, rendered, "maxclients 20000")
	assert.Contains(t, rendered, "tcp-backlog 1024")
	assert.Contains(t, rendered, "loglevel notice")
}
