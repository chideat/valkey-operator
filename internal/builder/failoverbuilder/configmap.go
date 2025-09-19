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
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/types"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ConfigMapName(name string) string {
	return FailoverStatefulSetName(name)
}

func GenerateConfigMap(inst types.FailoverInstance) (*corev1.ConfigMap, error) {
	rf := inst.Definition()
	if rf == nil {
		return nil, fmt.Errorf("failover instance definition is nil")
	}
	var (
		customConfig      = rf.Spec.CustomConfigs
		keys              = make([]string, 0, len(rf.Spec.CustomConfigs))
		innerValkeyConfig = inst.Version().CustomConfigs(core.ValkeyFailover)
		configMap         = lo.Assign(rf.Spec.CustomConfigs, innerValkeyConfig)
	)

	// check if it's need to set default save
	// check if aof enabled
	if customConfig[builder.ValkeyConfig_Appendonly] != "yes" &&
		customConfig[builder.ValkeyConfig_ReplDisklessSync] != "yes" &&
		(customConfig[builder.ValkeyConfig_Save] == "" || customConfig[builder.ValkeyConfig_Save] == `""`) {

		configMap["save"] = "60 10000 300 100 600 1"
	}

	if rf.Spec.Resources.Limits != nil {
		var osMem int64
		for _, res := range []*resource.Quantity{rf.Spec.Resources.Limits.Memory(), rf.Spec.Resources.Requests.Memory()} {
			if res == nil && res.IsZero() {
				continue
			}
			osMem, _ = res.AsInt64()
			break
		}
		if osMem > 0 {
			if configedMem := configMap[builder.ValkeyConfig_MaxMemory]; configedMem == "" {
				var recommendMem int64
				if policy := configMap[builder.ValkeyConfig_MaxMemoryPolicy]; policy == "noeviction" {
					recommendMem = int64(float64(osMem) * 0.8)
				} else {
					recommendMem = int64(float64(osMem) * 0.7)
				}
				configMap[builder.ValkeyConfig_MaxMemory] = fmt.Sprintf("%d", recommendMem)
			}
			if backlogSize := configMap[builder.ValkeyConfig_ReplBacklogSize]; backlogSize == "" {
				val := int64(0.01 * float64(osMem))
				if val > 256*1024*1024 {
					val = 256 * 1024 * 1024
				} else if val < 1024*1024 {
					val = 1024 * 1024
				}
				configMap[builder.ValkeyConfig_ReplBacklogSize] = fmt.Sprintf("%d", val)
			}
		}
	}

	for k, v := range configMap {
		if policy := builder.ValkeyConfigRestartPolicy[k]; policy == builder.Forbid {
			continue
		}

		lowerKey, trimVal := strings.ToLower(k), strings.TrimSpace(v)
		keys = append(keys, lowerKey)
		if lowerKey != k || trimVal != v {
			configMap[lowerKey] = trimVal
		}
		// TODO: filter illgal config
	}
	sort.Strings(keys)

	var buffer bytes.Buffer
	for _, k := range keys {
		v := configMap[k]
		if v == "" || v == `""` {
			buffer.WriteString(fmt.Sprintf("%s \"\"\n", k))
			continue
		}
		switch k {
		case builder.ValkeyConfig_ClientOutputBufferLimit:
			fields := strings.Fields(v)
			if len(fields)%4 != 0 {
				continue
			}
			for i := 0; i < len(fields); i += 4 {
				buffer.WriteString(fmt.Sprintf("%s %s %s %s %s\n", k, fields[i], fields[i+1], fields[i+2], fields[i+3]))
			}
		case builder.ValkeyConfig_Save:
			fields := strings.Fields(v)
			if len(fields)%2 != 0 {
				continue
			}
			for i := 0; i < len(fields); i += 2 {
				buffer.WriteString(fmt.Sprintf("%s %s %s\n", k, fields[i], fields[i+1]))
			}
		case builder.ValkeyConfig_RenameCommand:
			continue
		default:
			if _, ok := builder.MustQuoteValkeyConfig[k]; ok && !strings.HasPrefix(v, `"`) {
				v = fmt.Sprintf(`"%s"`, v)
			}
			if _, ok := builder.MustUpperValkeyConfig[k]; ok {
				v = strings.ToUpper(v)
			}
			buffer.WriteString(fmt.Sprintf("%s %s\n", k, v))
		}
	}

	for _, mod := range rf.Spec.Modules {
		args := append([]string{"loadmodule", mod.Path}, mod.Args...)
		buffer.WriteString(strings.Join(args, " ") + "\n")
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            ConfigMapName(rf.Name),
			Namespace:       rf.Namespace,
			Labels:          GenerateCommonLabels(rf.Name),
			OwnerReferences: util.BuildOwnerReferences(rf),
			Annotations:     map[string]string{},
		},
		Data: map[string]string{
			builder.ValkeyConfigKey: buffer.String(),
		},
	}, nil
}
