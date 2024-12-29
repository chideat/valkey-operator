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
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestDefinedCommands(t *testing.T) {
	if (*ActorResult)(nil).NextCommand() != nil {
		t.Errorf("ActorResult.NextCommand() = %v, want %v", (*ActorResult)(nil).NextCommand(), nil)
	}
	if Requeue().NextCommand() != CommandRequeue {
		t.Errorf("Requeue() = %v, want %v", Requeue().NextCommand(), CommandRequeue)
	}

	if RequeueWithError(errors.New("test")).NextCommand() != CommandRequeue {
		t.Errorf("RequeueWithError() = %v, want %v", RequeueWithError(errors.New("test")).NextCommand(), CommandRequeue)
	}

	if Pause().NextCommand() != CommandPaused {
		t.Errorf("Pause() = %v, want %v", Pause().NextCommand(), CommandPaused)
	}
	if AbortWithError(errors.New("test")).NextCommand() != CommandAbort {
		t.Errorf("AbortWithError() = %v, want %v", AbortWithError(errors.New("test")).NextCommand(), CommandAbort)
	}
}

func TestNewResult(t *testing.T) {
	type args struct {
		cmd Command
	}
	tests := []struct {
		name string
		args args
		want *ActorResult
	}{
		{
			name: "nil command",
			args: args{},
			want: &ActorResult{},
		},
		{
			name: "TestNewResult",
			args: args{
				cmd: CmdFailoverEnsureResource,
			},
			want: &ActorResult{
				next: CmdFailoverEnsureResource,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewResult(tt.args.cmd); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewResult() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewResultWithValue(t *testing.T) {
	type args struct {
		cmd Command
		val interface{}
	}
	tests := []struct {
		name string
		args args
		want *ActorResult
	}{
		{
			name: "TestNewResultWithValue",
			args: args{
				cmd: CommandRequeue,
				val: time.Second,
			},
			want: &ActorResult{
				next:   CommandRequeue,
				result: time.Second,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewResultWithValue(tt.args.cmd, tt.args.val)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewResultWithValue() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got.Result(), tt.args.val) {
				t.Errorf("NewResultWithValue() = %v, want %v", got.Result(), tt.args.val)
			}
			if got.Err() != nil {
				t.Errorf("NewResultWithValue() = %v, want %v", got.Err(), nil)
			}
		})
	}
	if (*ActorResult)(nil).Result() != nil {
		t.Errorf("ActorResult.Result() = %v, want %v", (&ActorResult{}).Result(), nil)
	}
}

func TestNewResultWithError(t *testing.T) {
	type args struct {
		cmd Command
		err error
	}
	err := errors.New("invalid resource")
	tests := []struct {
		name string
		args args
		want *ActorResult
	}{
		{
			name: "nil error",
			args: args{
				cmd: CommandRequeue,
				err: nil,
			},
			want: &ActorResult{
				next:   CommandRequeue,
				result: nil,
			},
		},
		{
			name: "TestNewResultWithError",
			args: args{
				cmd: CommandRequeue,
				err: err,
			},
			want: &ActorResult{
				next:   CommandRequeue,
				result: err,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewResultWithError(tt.args.cmd, tt.args.err)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewResultWithError() = %v, want %v", got, tt.want)
			}
			if got.Err() != tt.args.err {
				t.Errorf("NewResultWithError() = %v, want %v", got.Err(), tt.args.err)
			}
		})
	}
}
