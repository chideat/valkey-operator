/*
Copyright 2023 The RedisOperator Authors.

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
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/types/redis"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RedisConfig_MaxMemory               = "maxmemory"
	RedisConfig_MaxMemoryPolicy         = "maxmemory-policy"
	RedisConfig_ClientOutputBufferLimit = "client-output-buffer-limit"
	RedisConfig_Save                    = "save"
	RedisConfig_RenameCommand           = "rename-command"
	RedisConfig_Appendonly              = "appendonly"
	RedisConfig_ReplDisklessSync        = "repl-diskless-sync"
)

func NewRedisConfigMap(st types.RedisFailoverInstance, selectors map[string]string) (*corev1.ConfigMap, error) {
	rf := st.Definition()
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

	version, _ := redis.ParseRedisVersionFromImage(rf.Spec.Image)
	innerRedisConfig := version.CustomConfigs(core.ValkeyFailover)
	default_config = lo.Assign(default_config, innerRedisConfig)

	for k, v := range customConfig {
		k = strings.ToLower(k)
		v = strings.TrimSpace(v)
		if k == "save" && v == "60 100" {
			continue
		}
		if k == RedisConfig_RenameCommand {
			continue
		}
		default_config[k] = v
	}

	// check if it's need to set default save
	// check if aof enabled
	if customConfig[RedisConfig_Appendonly] != "yes" &&
		customConfig[RedisConfig_ReplDisklessSync] != "yes" &&
		(customConfig[RedisConfig_Save] == "" || customConfig[RedisConfig_Save] == `""`) {

		default_config["save"] = "60 10000 300 100 600 1"
	}

	if limits := rf.Spec.Resources.Limits; limits != nil {
		if configedMem := customConfig[RedisConfig_MaxMemory]; configedMem == "" {
			memLimit, _ := limits.Memory().AsInt64()
			if policy := customConfig[RedisConfig_MaxMemoryPolicy]; policy == "noeviction" {
				memLimit = int64(float64(memLimit) * 0.8)
			} else {
				memLimit = int64(float64(memLimit) * 0.7)
			}
			if memLimit > 0 {
				default_config[RedisConfig_MaxMemory] = fmt.Sprintf("%d", memLimit)
			}
		}
	}

	if !st.Version().IsACLSupported() {
		var renameVal []string
		if renameConfig, err := clusterbuilder.ParseRenameConfigs(customConfig[RedisConfig_RenameCommand]); err != nil {
			return nil, err
		} else {
			for k, v := range renameConfig {
				if k == "config" {
					continue
				}
				renameVal = append(renameVal, k, v)
			}
			if len(renameVal) > 0 {
				default_config[RedisConfig_RenameCommand] = strings.Join(renameVal, " ")
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
		case RedisConfig_ClientOutputBufferLimit:
			fields := strings.Fields(v)
			if len(fields)%4 != 0 {
				continue
			}
			for i := 0; i < len(fields); i += 4 {
				buffer.WriteString(fmt.Sprintf("%s %s %s %s %s\n", k, fields[i], fields[i+1], fields[i+2], fields[i+3]))
			}
		case RedisConfig_Save, RedisConfig_RenameCommand:
			fields := strings.Fields(v)
			if len(fields)%2 != 0 {
				continue
			}
			for i := 0; i < len(fields); i += 2 {
				buffer.WriteString(fmt.Sprintf("%s %s %s\n", k, fields[i], fields[i+1]))
			}
		default:
			if _, ok := builder.MustQuoteRedisConfig[k]; ok && !strings.HasPrefix(v, `"`) {
				v = fmt.Sprintf(`"%s"`, v)
			}
			if _, ok := builder.MustUpperRedisConfig[k]; ok {
				v = strings.ToUpper(v)
			}
			buffer.WriteString(fmt.Sprintf("%s %s\n", k, v))
		}
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            GetRedisConfigMapName(rf),
			Namespace:       rf.Namespace,
			Labels:          GetCommonLabels(rf.Name, selectors),
			OwnerReferences: util.BuildOwnerReferences(rf),
		},
		Data: map[string]string{
			RedisConfigFileName: buffer.String(),
		},
	}, nil
}

func GetRedisConfigMapName(rf *v1alpha1.Failover) string {
	return GetFailoverStatefulSetName(rf.Name)
}

func GetRedisScriptConfigMapName(name string) string {
	return fmt.Sprintf("rfr-s-%s", name)
}
