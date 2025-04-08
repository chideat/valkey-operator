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

package builder

import (
	"reflect"
	"sort"
	"strings"

	"github.com/chideat/valkey-operator/internal/util"
)

var MustQuoteValkeyConfig = map[string]struct{}{
	"tls-protocols": {},
}

var MustUpperValkeyConfig = map[string]struct{}{
	"tls-ciphers":      {},
	"tls-ciphersuites": {},
	"tls-protocols":    {},
}

var SentinelMonitorDefaultConfigs = map[string]string{
	"down-after-milliseconds": "30000",
	"failover-timeout":        "180000",
	"parallel-syncs":          "1",
}

const (
	ValkeyConfig_MaxMemory               = "maxmemory"
	ValkeyConfig_MaxMemoryPolicy         = "maxmemory-policy"
	ValkeyConfig_ReplBacklogSize         = "repl-backlog-size"
	ValkeyConfig_ClientOutputBufferLimit = "client-output-buffer-limit"
	ValkeyConfig_Save                    = "save"
	ValkeyConfig_RenameCommand           = "rename-command"
	ValkeyConfig_Appendonly              = "appendonly"
	ValkeyConfig_ReplDisklessSync        = "repl-diskless-sync"
)

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
