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

package failover

import (
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/pkg/actor"
)

var (
	CommandRequeue = actor.CommandRequeue
	CommandAbort   = actor.CommandAbort
	CommandPaused  = actor.CommandPaused

	CommandUpdateAccount  = actor.NewCommand(core.ValkeyFailover, "CommandUpdateAccount")
	CommandUpdateConfig   = actor.NewCommand(core.ValkeyFailover, "CommandUpdateConfig")
	CommandEnsureResource = actor.NewCommand(core.ValkeyFailover, "CommandEnsureResource")
	CommandHealPod        = actor.NewCommand(core.ValkeyFailover, "CommandHealPod")
	CommandHealMonitor    = actor.NewCommand(core.ValkeyFailover, "CommandHealMonitor")
	CommandPatchLabels    = actor.NewCommand(core.ValkeyFailover, "CommandPatchLabels")
	CommandCleanResource  = actor.NewCommand(core.ValkeyFailover, "CommandCleanResource")
)
