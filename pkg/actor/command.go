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

package actor

import (
	"github.com/chideat/valkey-operator/api/core"
)

type Command interface {
	String() string
}

type opsCommand struct {
	// arch is the architecture of the command, empty means all architectures
	arch    core.Arch
	command string
}

func (c *opsCommand) String() string {
	return c.command
}

func NewCommand(arch core.Arch, command string) Command {
	return &opsCommand{
		arch:    arch,
		command: command,
	}
}

var (
	CommandRequeue = NewCommand("", "CommandRequeue")
	CommandAbort   = NewCommand("", "CommandAbort")
	CommandPaused  = NewCommand("", "CommandPaused")
)
