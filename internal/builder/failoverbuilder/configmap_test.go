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

package failoverbuilder

import (
	"strings"
	"testing"

	v1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newFailoverWithResources(resources corev1.ResourceRequirements) *v1alpha1.Failover {
	return &v1alpha1.Failover{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Failover",
			APIVersion: "valkey.buf.red/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rf-test",
			Namespace: "default",
			UID:       "rf-test-uid",
		},
		Spec: v1alpha1.FailoverSpec{
			Resources: resources,
		},
	}
}

func TestGenerateConfigMapMemorySizing(t *testing.T) {
	tests := []struct {
		name           string
		resources      corev1.ResourceRequirements
		expectedConfig []string
		notExpected    []string
	}{
		{
			// R5 regression: Limits has no memory, so sizing must fall back to
			// Requests.Memory(). The old `&&` guard never skipped the zero
			// Limits.Memory() and dropped maxmemory entirely.
			name: "Requests-only memory triggers maxmemory sizing",
			resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("500m"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
			expectedConfig: []string{"maxmemory ", "repl-backlog-size "},
		},
		{
			name: "Limits memory drives sizing",
			resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
			expectedConfig: []string{"maxmemory ", "repl-backlog-size "},
		},
		{
			name:        "No memory configured renders no sizing",
			resources:   corev1.ResourceRequirements{},
			notExpected: []string{"maxmemory ", "repl-backlog-size "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := testutil.NewFakeFailoverInstance(newFailoverWithResources(tt.resources))
			cm, err := GenerateConfigMap(inst)
			require.NoError(t, err)
			data := cm.Data[builder.ValkeyConfigKey]
			require.NotEmpty(t, data)

			for _, expected := range tt.expectedConfig {
				assert.True(t, strings.Contains(data, expected),
					"config should contain %q, got: %s", expected, data)
			}
			for _, notExpected := range tt.notExpected {
				assert.False(t, strings.Contains(data, notExpected),
					"config should not contain %q, got: %s", notExpected, data)
			}
		})
	}
}
