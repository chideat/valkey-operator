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
*/package sentinelbuilder

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

var (
	constConfig = map[string]string{
		"dir": "/data",
	}
)

func SentinelConfigMapName(name string) string {
	// NOTE: use rfs--xxx for compatibility for old name "rfs-xxx" maybe needed
	return fmt.Sprintf("%s-%s", builder.ResourcePrefix(core.ValkeySentinel), name)
}

func GenerateSentinelConfigMap(inst types.SentinelInstance) (*corev1.ConfigMap, error) {
	sen := inst.Definition()

	defaultConfig := make(map[string]string)
	defaultConfig["loglevel"] = "notice"
	defaultConfig["maxclients"] = "10000"
	defaultConfig["tcp-keepalive"] = "300"
	defaultConfig["tcp-backlog"] = "511"

	version, _ := version.ParseValkeyVersionFromImage(sen.Spec.Image)
	innerValkeyConfig := version.CustomConfigs(core.ValkeySentinel)
	defaultConfig = lo.Assign(defaultConfig, innerValkeyConfig)

	for k, v := range sen.Spec.CustomConfigs {
		defaultConfig[strings.ToLower(k)] = strings.TrimSpace(v)
	}
	for k, v := range constConfig {
		defaultConfig[strings.ToLower(k)] = strings.TrimSpace(v)
	}

	keys := make([]string, 0, len(defaultConfig))
	for k := range defaultConfig {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buffer bytes.Buffer
	for _, k := range keys {
		v := defaultConfig[k]
		if v == "" || v == `""` {
			buffer.WriteString(fmt.Sprintf("%s \"\"\n", k))
			continue
		}

		if _, ok := builder.MustQuoteValkeyConfig[k]; ok && !strings.HasPrefix(v, `"`) {
			v = fmt.Sprintf(`"%s"`, v)
		}
		if _, ok := builder.MustUpperValkeyConfig[k]; ok {
			v = strings.ToUpper(v)
		}
		buffer.WriteString(fmt.Sprintf("%s %s\n", k, v))
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            SentinelConfigMapName(sen.GetName()),
			Namespace:       sen.Namespace,
			Labels:          GenerateCommonLabels(sen.Name),
			OwnerReferences: util.BuildOwnerReferences(sen),
		},
		Data: map[string]string{
			SentinelConfigFileName: buffer.String(),
		},
	}, nil
}
