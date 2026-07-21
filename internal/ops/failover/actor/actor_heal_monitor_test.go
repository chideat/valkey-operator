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

package actor

import (
	"context"
	"testing"

	ops "github.com/chideat/valkey-operator/internal/ops/failover"
	"github.com/chideat/valkey-operator/internal/valkey/failover/monitor"
	"github.com/chideat/valkey-operator/pkg/types"
	vkcli "github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/go-logr/logr"
)

// fakeHealNode is a minimal ValkeyNode. The no-candidate path only exercises IsReady/Refresh
// (candidatePicker skips a not-ready node before touching anything else), so the embedded nil
// interface is never called.
type fakeHealNode struct {
	types.ValkeyNode
	ready bool
}

func (f *fakeHealNode) IsReady() bool                 { return f.ready }
func (f *fakeHealNode) Refresh(context.Context) error { return nil }

// fakeHealMonitor reports "no master" with no replicas — the all-Pending state.
type fakeHealMonitor struct {
	types.FailoverMonitor
}

func (fakeHealMonitor) Master(context.Context, ...bool) (*vkcli.SentinelMonitorNode, error) {
	return nil, monitor.ErrNoMaster
}
func (fakeHealMonitor) Inited(context.Context) (bool, error) { return false, nil }
func (fakeHealMonitor) Replicas(context.Context) ([]*vkcli.SentinelMonitorNode, error) {
	return nil, nil
}

type fakeHealInst struct {
	types.FailoverInstance
	nodes []types.ValkeyNode
}

func (f *fakeHealInst) Logger() logr.Logger            { return logr.Discard() }
func (f *fakeHealInst) Nodes() []types.ValkeyNode      { return f.nodes }
func (f *fakeHealInst) Monitor() types.FailoverMonitor { return fakeHealMonitor{} }

// TestHealMonitorEnsuresResourcesWhenNoCandidate covers the Pending-recovery fix: when every
// valkey node is not-ready (e.g. all Pending after an oversized-resource create) there is no
// usable master candidate, and HealMonitor must fall through to EnsureResource — so a later
// resource downscale reaches the StatefulSet template (Parallel pod management then reschedules
// the Pending pods) — instead of idle-Requeue looping forever.
func TestHealMonitorEnsuresResourcesWhenNoCandidate(t *testing.T) {
	notReady := func(n int) []types.ValkeyNode {
		nodes := make([]types.ValkeyNode, n)
		for i := range nodes {
			nodes[i] = &fakeHealNode{ready: false}
		}
		return nodes
	}
	tests := []struct {
		name  string
		nodes []types.ValkeyNode
	}{
		{name: "no nodes yet (statefulset just created)", nodes: nil},
		{name: "all nodes pending / not-ready", nodes: notReady(2)},
	}
	a := &actorHealMaster{logger: logr.Discard()}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := &fakeHealInst{nodes: tt.nodes}
			res := a.Do(context.Background(), inst)
			if got := res.NextCommand(); got != ops.CommandEnsureResource {
				t.Fatalf("Do() command = %v, want CommandEnsureResource", got)
			}
		})
	}
}
