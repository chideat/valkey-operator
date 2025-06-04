package clusterbuilder

import (
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestGenerateSelectors(t *testing.T) {
	testCases := []struct {
		name        string
		clusterName string
		expected    map[string]string
	}{
		{
			name:        "Basic selector generation",
			clusterName: "test-cluster",
			expected: map[string]string{
				builder.InstanceTypeLabelKey: string(core.ValkeyCluster),
				builder.InstanceNameLabelKey: "test-cluster",
				builder.ManagedByLabelKey:    config.AppName,
				builder.AppComponentLabelKey: string(core.ValkeyCluster),
				builder.AppNameLabelKey:      "test-cluster",
			},
		},
		{
			name:        "Empty cluster name",
			clusterName: "",
			expected: map[string]string{
				builder.InstanceTypeLabelKey: string(core.ValkeyCluster),
				builder.InstanceNameLabelKey: "",
				builder.ManagedByLabelKey:    config.AppName,
				builder.AppComponentLabelKey: string(core.ValkeyCluster),
				builder.AppNameLabelKey:      "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := generateSelectors(tc.clusterName)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGenerateClusterStatefulSetSelectors(t *testing.T) {
	testCases := []struct {
		name        string
		clusterName string
		index       int
		expected    map[string]string
	}{
		{
			name:        "With valid index",
			clusterName: "test-cluster",
			index:       0,
			expected: map[string]string{
				builder.InstanceTypeLabelKey: string(core.ValkeyCluster),
				builder.InstanceNameLabelKey: "test-cluster",
				builder.ManagedByLabelKey:    config.AppName,
				builder.AppComponentLabelKey: string(core.ValkeyCluster),
				builder.AppNameLabelKey:      "test-cluster",
				"statefulset":                ClusterStatefulSetName("test-cluster", 0),
			},
		},
		{
			name:        "With negative index",
			clusterName: "test-cluster",
			index:       -1,
			expected: map[string]string{
				builder.InstanceTypeLabelKey: string(core.ValkeyCluster),
				builder.InstanceNameLabelKey: "test-cluster",
				builder.ManagedByLabelKey:    config.AppName,
				builder.AppComponentLabelKey: string(core.ValkeyCluster),
				builder.AppNameLabelKey:      "test-cluster",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GenerateClusterStatefulSetSelectors(tc.clusterName, tc.index)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGenerateClusterStatefulSetLabels(t *testing.T) {
	testCases := []struct {
		name        string
		clusterName string
		index       int
		extra       []map[string]string
		expected    map[string]string
	}{
		{
			name:        "With valid index, no extra",
			clusterName: "test-cluster",
			index:       0,
			extra:       nil,
			expected: map[string]string{
				builder.InstanceTypeLabelKey: string(core.ValkeyCluster),
				builder.InstanceNameLabelKey: "test-cluster",
				builder.ManagedByLabelKey:    config.AppName,
				builder.AppComponentLabelKey: string(core.ValkeyCluster),
				builder.AppNameLabelKey:      "test-cluster",
				"statefulset":                ClusterStatefulSetName("test-cluster", 0),
			},
		},
		{
			name:        "With valid index, with extra",
			clusterName: "test-cluster",
			index:       1,
			extra:       []map[string]string{{"custom-key": "custom-value"}},
			expected: map[string]string{
				builder.InstanceTypeLabelKey: string(core.ValkeyCluster),
				builder.InstanceNameLabelKey: "test-cluster",
				builder.ManagedByLabelKey:    config.AppName,
				builder.AppComponentLabelKey: string(core.ValkeyCluster),
				builder.AppNameLabelKey:      "test-cluster",
				"statefulset":                ClusterStatefulSetName("test-cluster", 1),
			},
		},
		{
			name:        "With negative index",
			clusterName: "test-cluster",
			index:       -1,
			extra:       nil,
			expected: map[string]string{
				builder.InstanceTypeLabelKey: string(core.ValkeyCluster),
				builder.InstanceNameLabelKey: "test-cluster",
				builder.ManagedByLabelKey:    config.AppName,
				builder.AppComponentLabelKey: string(core.ValkeyCluster),
				builder.AppNameLabelKey:      "test-cluster",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result map[string]string
			if tc.extra == nil {
				result = GenerateClusterStatefulSetLabels(tc.clusterName, tc.index)
			} else {
				result = GenerateClusterStatefulSetLabels(tc.clusterName, tc.index, tc.extra...)
			}
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGenerateClusterLabels(t *testing.T) {
	testCases := []struct {
		name        string
		clusterName string
		extra       map[string]string
		expected    map[string]string
	}{
		{
			name:        "No extra labels",
			clusterName: "test-cluster",
			extra:       nil,
			expected: map[string]string{
				builder.InstanceTypeLabelKey: string(core.ValkeyCluster),
				builder.InstanceNameLabelKey: "test-cluster",
				builder.ManagedByLabelKey:    config.AppName,
				builder.AppComponentLabelKey: string(core.ValkeyCluster),
				builder.AppNameLabelKey:      "test-cluster",
			},
		},
		{
			name:        "With extra labels",
			clusterName: "test-cluster",
			extra: map[string]string{
				"custom-key": "custom-value",
			},
			expected: map[string]string{
				builder.InstanceTypeLabelKey: string(core.ValkeyCluster),
				builder.InstanceNameLabelKey: "test-cluster",
				builder.ManagedByLabelKey:    config.AppName,
				builder.AppComponentLabelKey: string(core.ValkeyCluster),
				builder.AppNameLabelKey:      "test-cluster",
				"custom-key":                 "custom-value",
			},
		},
		{
			name:        "Extra labels override existing labels",
			clusterName: "test-cluster",
			extra: map[string]string{
				builder.AppNameLabelKey: "overridden-name",
			},
			expected: map[string]string{
				builder.InstanceTypeLabelKey: string(core.ValkeyCluster),
				builder.InstanceNameLabelKey: "test-cluster",
				builder.ManagedByLabelKey:    config.AppName,
				builder.AppComponentLabelKey: string(core.ValkeyCluster),
				builder.AppNameLabelKey:      "overridden-name",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GenerateClusterLabels(tc.clusterName, tc.extra)
			assert.Equal(t, tc.expected, result)
		})
	}
}
