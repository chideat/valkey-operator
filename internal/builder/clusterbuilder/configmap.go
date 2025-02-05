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
*/package clusterbuilder

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/samber/lo"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewConfigMapForCR creates a new ConfigMap for the given Cluster
func NewConfigMapForCR(cluster types.ClusterInstance) (*corev1.ConfigMap, error) {
	valkeyConfContent, err := buildValkeyConfigs(cluster)
	if err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            ValkeyConfigMapName(cluster.GetName()),
			Namespace:       cluster.GetNamespace(),
			Labels:          GenerateClusterLabels(cluster.GetName(), nil),
			OwnerReferences: util.BuildOwnerReferences(cluster.Definition()),
		},
		Data: map[string]string{
			builder.ValkeyConfigKey: valkeyConfContent,
		},
	}, nil
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

// buildValkeyConfigs
//
// TODO: validate config and config value. check the empty value
func buildValkeyConfigs(cluster types.ClusterInstance) (string, error) {
	cr := cluster.Definition()
	var buffer bytes.Buffer

	var (
		keys              = make([]string, 0, len(cr.Spec.CustomConfigs))
		innerValkeyConfig = cluster.Version().CustomConfigs(core.ValkeyCluster)
		configMap         = lo.Assign(cr.Spec.CustomConfigs, innerValkeyConfig)
	)

	// check memory-policy
	if cluster != nil && cr.Spec.Resources != nil && cr.Spec.Resources.Limits != nil {
		osMem, _ := cr.Spec.Resources.Limits.Memory().AsInt64()
		if configedMem := configMap[ValkeyConfig_MaxMemory]; configedMem == "" {
			var recommendMem int64
			if policy := cr.Spec.CustomConfigs[ValkeyConfig_MaxMemoryPolicy]; policy == "noeviction" {
				recommendMem = int64(float64(osMem) * 0.8)
			} else {
				recommendMem = int64(float64(osMem) * 0.7)
			}
			configMap[ValkeyConfig_MaxMemory] = fmt.Sprintf("%d", recommendMem)
		}
		// TODO: validate user input
	}

	// check if it's needed to set default save
	// check if aof enabled
	if configMap[ValkeyConfig_Appendonly] != "yes" &&
		configMap[ValkeyConfig_ReplDisklessSync] != "yes" &&
		(configMap[ValkeyConfig_Save] == "" || configMap[ValkeyConfig_Save] == `""`) {

		configMap["save"] = "60 10000 300 100 600 1"
	}
	delete(configMap, ValkeyConfig_RenameCommand)

	for k, v := range configMap {
		if policy := ValkeyConfigRestartPolicy[k]; policy == Forbid {
			continue
		}

		lowerKey, trimVal := strings.ToLower(k), strings.TrimSpace(v)
		keys = append(keys, lowerKey)
		if lowerKey != k || !strings.EqualFold(trimVal, v) {
			configMap[lowerKey] = trimVal
		}
		// TODO: filter illgal config
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := configMap[k]

		if v == "" || v == `""` {
			buffer.WriteString(fmt.Sprintf("%s \"\"\n", k))
			continue
		}

		switch k {
		case ValkeyConfig_ClientOutputBufferLimit:
			fields := strings.Fields(v)
			if len(fields)%4 != 0 {
				return "", fmt.Errorf(`value "%s" for config %s is invalid`, v, k)
			}
			for i := 0; i < len(fields); i += 4 {
				buffer.WriteString(fmt.Sprintf("%s %s %s %s %s\n", k, fields[i], fields[i+1], fields[i+2], fields[i+3]))
			}
		case ValkeyConfig_Save:
			fields := strings.Fields(v)
			if len(fields)%2 != 0 {
				return "", fmt.Errorf(`value "%s" for config %s is invalid`, v, k)
			}
			for i := 0; i < len(fields); i += 2 {
				buffer.WriteString(fmt.Sprintf("%s %s %s\n", k, fields[i], fields[i+1]))
			}
		case ValkeyConfig_RenameCommand:
			// DEPRECATED: for consistence of the config
			fields := strings.Fields(v)
			if len(fields)%2 != 0 {
				return "", fmt.Errorf(`value "%s" for config %s is invalid`, v, k)
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
	return buffer.String(), nil
}

func ValkeyConfigMapName(clusterName string) string {
	return fmt.Sprintf("%s-%s", "valkey-cluster", clusterName)
}

type ValkeyConfigSettingRule string

const (
	OK             ValkeyConfigSettingRule = "OK"
	RequireRestart ValkeyConfigSettingRule = "Restart"
	Forbid         ValkeyConfigSettingRule = "Forbid"
)

var ValkeyConfigRestartPolicy = map[string]ValkeyConfigSettingRule{
	// forbid
	"include":          Forbid,
	"loadmodule":       Forbid,
	"bind":             Forbid,
	"protected-mode":   Forbid,
	"port":             Forbid,
	"tls-port":         Forbid,
	"tls-cert-file":    Forbid,
	"tls-key-file":     Forbid,
	"tls-ca-cert-file": Forbid,
	"tls-ca-cert-dir":  Forbid,
	"unixsocket":       Forbid,
	"unixsocketperm":   Forbid,
	"daemonize":        Forbid,
	"supervised":       Forbid,
	"pidfile":          Forbid,
	"logfile":          Forbid,
	"syslog-enabled":   Forbid,
	"syslog-ident":     Forbid,
	"syslog-facility":  Forbid,
	"always-show-logo": Forbid,
	"dbfilename":       Forbid,
	"appendfilename":   Forbid,
	"dir":              Forbid,
	"slaveof":          Forbid,
	"replicaof":        Forbid,
	"gopher-enabled":   Forbid,
	// "ignore-warnings":       Forbid,
	"aclfile":               Forbid,
	"requirepass":           Forbid,
	"masterauth":            Forbid,
	"masteruser":            Forbid,
	"slave-announce-ip":     Forbid,
	"replica-announce-ip":   Forbid,
	"slave-announce-port":   Forbid,
	"replica-announce-port": Forbid,
	"cluster-enabled":       Forbid,
	"cluster-config-file":   Forbid,

	// RequireRestart
	"tcp-backlog":         RequireRestart,
	"databases":           RequireRestart,
	"rename-command":      RequireRestart,
	"rdbchecksum":         RequireRestart,
	"io-threads":          RequireRestart,
	"io-threads-do-reads": RequireRestart,
}

type ValkeyConfigValues []string

func (v *ValkeyConfigValues) String() string {
	if v == nil {
		return ""
	}
	sort.Strings(*v)
	return strings.Join(*v, " ")
}

// ValkeyConfig
type ValkeyConfig map[string]ValkeyConfigValues

// LoadValkeyConfig
func LoadValkeyConfig(data string) (ValkeyConfig, error) {
	conf := ValkeyConfig{}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.SplitN(line, " ", 2)
		if len(fields) != 2 {
			continue
		}
		key := fields[0]
		// filter unsupported config
		if policy := ValkeyConfigRestartPolicy[key]; policy == Forbid {
			continue
		}
		val := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(fields[1]), `"`), `"`)
		conf[key] = append(conf[key], val)
	}
	return conf, nil
}

// Diff return diff two n ValkeyConfig
func (o ValkeyConfig) Diff(n ValkeyConfig) (added, changed, deleted map[string]ValkeyConfigValues) {
	if len(n) == 0 {
		return nil, nil, o
	}
	if len(o) == 0 {
		return n, nil, nil
	}

	if reflect.DeepEqual(o, n) {
		return nil, nil, nil
	}

	added, changed, deleted = map[string]ValkeyConfigValues{}, map[string]ValkeyConfigValues{}, map[string]ValkeyConfigValues{}
	for key, vals := range n {
		val := util.UnifyValueUnit(vals.String())
		if oldVals, ok := o[key]; ok {
			if util.UnifyValueUnit(oldVals.String()) != val {
				changed[key] = vals
			}
		} else {
			added[key] = vals
		}
	}
	for key, vals := range o {
		if _, ok := n[key]; !ok {
			deleted[key] = vals
		}
	}
	return
}
