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
	"context"

	"github.com/Masterminds/semver/v3"
	"github.com/chideat/valkey-operator/pkg/types"
)

// ActorResult
type ActorResult struct {
	next   Command
	result interface{}
}

func NewResult(cmd Command) *ActorResult {
	return &ActorResult{next: cmd}
}

func NewResultWithValue(cmd Command, val interface{}) *ActorResult {
	return &ActorResult{next: cmd, result: val}
}

func NewResultWithError(cmd Command, err error) *ActorResult {
	return &ActorResult{next: cmd, result: err}
}

func Requeue() *ActorResult {
	return &ActorResult{next: CommandRequeue}
}

func RequeueWithError(err error) *ActorResult {
	return &ActorResult{next: CommandRequeue, result: err}
}

func Pause() *ActorResult {
	return &ActorResult{next: CommandPaused}
}

func AbortWithError(err error) *ActorResult {
	return &ActorResult{next: CommandAbort, result: err}
}

// Next
func (c *ActorResult) NextCommand() Command {
	if c == nil {
		return nil
	}
	return c.next
}

// Result
func (c *ActorResult) Result() interface{} {
	if c == nil {
		return nil
	}
	return c.result
}

// Err
func (c *ActorResult) Err() error {
	if c == nil || c.result == nil {
		return nil
	}
	if e, ok := c.result.(error); ok {
		return e
	}
	return nil
}

// Actor actor is used process instance with specified state
type Actor interface {
	// SupportedCommands return the supported commands of the actor
	SupportedCommands() []Command
	// Version return the version of the actor
	// if the version is different from the previous version, the actor will be reloaded
	Version() *semver.Version
	// Do run the actor
	Do(ctx context.Context, cluster types.Instance) *ActorResult
}
