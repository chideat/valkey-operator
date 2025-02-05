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
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/samber/lo"
)

const (
	SentinelConfigFileName = "sentinel.conf"
)

func GenerateCommonLabels(name string, extra ...map[string]string) map[string]string {
	return lo.Assign(lo.Assign(extra...), GenerateSelectorLabels(name))
}

func GenerateSelectorLabels(name string) map[string]string {
	return map[string]string{
		builder.ManagedByLabelKey:    config.AppName,
		builder.AppComponentLabelKey: string(core.ValkeyFailover),
		builder.AppNameLabelKey:      name,

		builder.InstanceTypeLabelKey: string(core.ValkeySentinel),
		builder.InstanceNameLabelKey: name,
	}
}
