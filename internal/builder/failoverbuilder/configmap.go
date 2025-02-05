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
*/package failoverbuilder

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/version"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ConfigMapName(name string) string {
	return FailoverStatefulSetName(name)
}

const (
	ValkeyConfig_MaxMemory               = "maxmemory"
	ValkeyConfig_MaxMemoryPolicy         = "maxmemory-policy"
	ValkeyConfig_ClientOutputBufferLimit = "client-output-buffer-limit"
	ValkeyConfig_Save                    = "save"
	ValkeyConfig_RenameCommand           = "rename-command"
	ValkeyConfig_Appendonly              = "appendonly"
	ValkeyConfig_ReplDisklessSync        = "repl-diskless-sync"
)

func GenerateConfigMap(inst types.FailoverInstance) (*corev1.ConfigMap, error) {
	rf := inst.Definition()

	customConfig := rf.Spec.CustomConfigs
	default_config := make(map[string]string)
	default_config["loglevel"] = "notice"
	default_config["stop-writes-on-bgsave-error"] = "yes"
	default_config["rdbcompression"] = "yes"
	default_config["rdbchecksum"] = "yes"
	default_config["slave-read-only"] = "yes"
	default_config["repl-diskless-sync"] = "no"
	default_config["slowlog-max-len"] = "128"
	default_config["slowlog-log-slower-than"] = "10000"
	default_config["maxclients"] = "10000"
	default_config["hz"] = "10"
	default_config["tcp-keepalive"] = "300"
	default_config["tcp-backlog"] = "511"
	default_config["protected-mode"] = "no"

	version, _ := version.ParseValkeyVersionFromImage(rf.Spec.Image)
	innerValkeyConfig := version.CustomConfigs(core.ValkeyFailover)
	default_config = lo.Assign(default_config, innerValkeyConfig)

	for k, v := range customConfig {
		k = strings.ToLower(k)
		v = strings.TrimSpace(v)
		if k == "save" && v == "60 100" {
			continue
		}
		if k == ValkeyConfig_RenameCommand {
			continue
		}
		default_config[k] = v
	}

	// check if it's need to set default save
	// check if aof enabled
	if customConfig[ValkeyConfig_Appendonly] != "yes" &&
		customConfig[ValkeyConfig_ReplDisklessSync] != "yes" &&
		(customConfig[ValkeyConfig_Save] == "" || customConfig[ValkeyConfig_Save] == `""`) {

		default_config["save"] = "60 10000 300 100 600 1"
	}

	if limits := rf.Spec.Resources.Limits; limits != nil {
		if configedMem := customConfig[ValkeyConfig_MaxMemory]; configedMem == "" {
			memLimit, _ := limits.Memory().AsInt64()
			if policy := customConfig[ValkeyConfig_MaxMemoryPolicy]; policy == "noeviction" {
				memLimit = int64(float64(memLimit) * 0.8)
			} else {
				memLimit = int64(float64(memLimit) * 0.7)
			}
			if memLimit > 0 {
				default_config[ValkeyConfig_MaxMemory] = fmt.Sprintf("%d", memLimit)
			}
		}
	}

	keys := make([]string, 0, len(default_config))
	for k := range default_config {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buffer bytes.Buffer
	for _, k := range keys {
		v := default_config[k]
		if v == "" || v == `""` {
			buffer.WriteString(fmt.Sprintf("%s \"\"\n", k))
			continue
		}
		switch k {
		case ValkeyConfig_ClientOutputBufferLimit:
			fields := strings.Fields(v)
			if len(fields)%4 != 0 {
				continue
			}
			for i := 0; i < len(fields); i += 4 {
				buffer.WriteString(fmt.Sprintf("%s %s %s %s %s\n", k, fields[i], fields[i+1], fields[i+2], fields[i+3]))
			}
		case ValkeyConfig_Save, ValkeyConfig_RenameCommand:
			fields := strings.Fields(v)
			if len(fields)%2 != 0 {
				continue
			}
			for i := 0; i < len(fields); i += 2 {
				buffer.WriteString(fmt.Sprintf("%s %s %s\n", k, fields[i], fields[i+1]))
			}
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

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            ConfigMapName(rf.Name),
			Namespace:       rf.Namespace,
			Labels:          GenerateCommonLabels(rf.Name),
			OwnerReferences: util.BuildOwnerReferences(rf),
		},
		Data: map[string]string{
			builder.ValkeyConfigKey: buffer.String(),
		},
	}, nil
}
