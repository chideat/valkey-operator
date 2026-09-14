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
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/types"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	logger = logr.New(nil)
)

// Mock dependencies
type MockClientSet struct {
	mock.Mock
}

var (
	CmdClustetrEnsureResource = NewCommand(core.ValkeyCluster, "CommandEnsureResource")
	CmdClustetrHealPod        = NewCommand(core.ValkeyCluster, "CommandHealPod")
	CmdFailoverEnsureResource = NewCommand(core.ValkeyFailover, "CommandEnsureResource")
	CmdFailoverHealPod        = NewCommand(core.ValkeyFailover, "CommandHealPod")
)

type MockClusterEnsureResource struct {
	mock.Mock
}

func MockNewClusterEnsureResource(cs kubernetes.ClientSet, logger logr.Logger) Actor {
	return &MockClusterEnsureResource{}
}

func (m *MockClusterEnsureResource) SupportedCommands() []Command {
	return []Command{CmdClustetrEnsureResource}
}

func (m *MockClusterEnsureResource) Version() *semver.Version {
	return semver.MustParse("3.18.0")
}

func (m *MockClusterEnsureResource) Do(ctx context.Context, inst types.Instance) *ActorResult {
	args := m.Called()
	return args.Get(0).(*ActorResult)
}

type MockClusterHealPodActor struct {
	mock.Mock
}

func MockNewClusterHealPodActor(cs kubernetes.ClientSet, logger logr.Logger) Actor {
	return &MockClusterHealPodActor{}
}

func (m *MockClusterHealPodActor) SupportedCommands() []Command {
	return []Command{CmdClustetrHealPod}
}

func (m *MockClusterHealPodActor) Version() *semver.Version {
	return semver.MustParse("3.18.0")
}

func (m *MockClusterHealPodActor) Do(ctx context.Context, inst types.Instance) *ActorResult {
	args := m.Called()
	return args.Get(0).(*ActorResult)
}

type MockClusterEnsureResource314 struct {
	mock.Mock
}

func MockNewClusterEnsureResource314(cs kubernetes.ClientSet, logger logr.Logger) Actor {
	return &MockClusterEnsureResource314{}
}

func (m *MockClusterEnsureResource314) SupportedCommands() []Command {
	return []Command{CmdClustetrEnsureResource}
}

func (m *MockClusterEnsureResource314) Version() *semver.Version {
	return semver.MustParse("3.14.0")
}

func (m *MockClusterEnsureResource314) Do(ctx context.Context, inst types.Instance) *ActorResult {
	args := m.Called()
	return args.Get(0).(*ActorResult)
}

type MockClusterHealPodActor314 struct {
	mock.Mock
}

func MockNewClusterHealPodActor314(cs kubernetes.ClientSet, logger logr.Logger) Actor {
	return &MockClusterHealPodActor314{}
}

func (m *MockClusterHealPodActor314) SupportedCommands() []Command {
	return []Command{CmdClustetrHealPod}
}

func (m *MockClusterHealPodActor314) Version() *semver.Version {
	return semver.MustParse("3.14.0")
}

func (m *MockClusterHealPodActor314) Do(ctx context.Context, inst types.Instance) *ActorResult {
	args := m.Called()
	return args.Get(0).(*ActorResult)
}

type MockFailoverEnsureResource struct {
	mock.Mock
}

func MockNewFailoverEnsureResource(cs kubernetes.ClientSet, logger logr.Logger) Actor {
	return &MockFailoverEnsureResource{}
}

func (m *MockFailoverEnsureResource) SupportedCommands() []Command {
	return []Command{CmdFailoverEnsureResource}
}

func (m *MockFailoverEnsureResource) Version() *semver.Version {
	return semver.MustParse("3.18.0")
}

func (m *MockFailoverEnsureResource) Do(ctx context.Context, inst types.Instance) *ActorResult {
	args := m.Called()
	return args.Get(0).(*ActorResult)
}

type MockFailoverHealPodActor struct {
	mock.Mock
}

func MockNewFailoverHealPodActor(cs kubernetes.ClientSet, logger logr.Logger) Actor {
	return &MockFailoverHealPodActor{}
}

func (m *MockFailoverHealPodActor) SupportedCommands() []Command {
	return []Command{CmdFailoverHealPod}
}

func (m *MockFailoverHealPodActor) Version() *semver.Version {
	return semver.MustParse("3.18.0")
}

func (m *MockFailoverHealPodActor) Do(ctx context.Context, inst types.Instance) *ActorResult {
	args := m.Called()
	return args.Get(0).(*ActorResult)
}

type MockFailoverEnsureResource314 struct {
	mock.Mock
}

func MockNewFailoverEnsureResource314(cs kubernetes.ClientSet, logger logr.Logger) Actor {
	return &MockFailoverEnsureResource314{}
}

func (m *MockFailoverEnsureResource314) SupportedCommands() []Command {
	return []Command{CmdFailoverEnsureResource}
}

func (m *MockFailoverEnsureResource314) Version() *semver.Version {
	return semver.MustParse("3.14.0")
}

func (m *MockFailoverEnsureResource314) Do(ctx context.Context, inst types.Instance) *ActorResult {
	args := m.Called()
	return args.Get(0).(*ActorResult)
}

type MockFailoverHealPodActor314 struct {
	mock.Mock
}

func MockNewFailoverHealPodActor314(cs kubernetes.ClientSet, logger logr.Logger) Actor {
	return &MockFailoverHealPodActor314{}
}

func (m *MockFailoverHealPodActor314) SupportedCommands() []Command {
	return []Command{CmdFailoverHealPod}
}

func (m *MockFailoverHealPodActor314) Version() *semver.Version {
	return semver.MustParse("3.14.0")
}

func (m *MockFailoverHealPodActor314) Do(ctx context.Context, inst types.Instance) *ActorResult {
	args := m.Called()
	return args.Get(0).(*ActorResult)
}

type MockObject struct {
	annotations map[string]string
	arch        core.Arch
}

func (m *MockObject) GetAnnotations() map[string]string {
	return m.annotations
}

func (m *MockObject) Arch() core.Arch {
	return m.arch
}

func init() {
	Register(core.ValkeyCluster, MockNewClusterEnsureResource)
	Register(core.ValkeyCluster, MockNewClusterHealPodActor)
	Register(core.ValkeyFailover, MockNewFailoverEnsureResource)
	Register(core.ValkeyFailover, MockNewFailoverHealPodActor)

	Register(core.ValkeyCluster, MockNewClusterEnsureResource314)
	Register(core.ValkeyCluster, MockNewClusterHealPodActor314)
	Register(core.ValkeyFailover, MockNewFailoverEnsureResource314)
	Register(core.ValkeyFailover, MockNewFailoverHealPodActor314)
}

func TestRegister(t *testing.T) {
	assert.NotNil(t, registeredActorInitializer[core.ValkeyCluster])
	assert.Equal(t, 4, len(registeredActorInitializer[core.ValkeyCluster]))
	assert.NotNil(t, registeredActorInitializer[core.ValkeyFailover])
	assert.Equal(t, 4, len(registeredActorInitializer[core.ValkeyFailover]))
}

func TestNewActorManager(t *testing.T) {
	am := NewActorManager(nil, logger)
	assert.NotNil(t, am)
	assert.NotNil(t, am.actors[core.ValkeyCluster])
	assert.NotNil(t, am.actors[core.ValkeyFailover])
	assert.Nil(t, am.actors[core.ValkeySentinel])
}

func TestActorManager_Print(t *testing.T) {
	am := NewActorManager(nil, logger)
	am.Print()
	for arch, ag := range am.actors {
		if len(ag.All()) == 0 {
			t.Errorf("arch %s has no actors", arch)
		}
	}
}

func TestActorManager_Search(t *testing.T) {
	am := NewActorManager(nil, logger)
	for _, ver := range []string{"3.18.0", "3.18.10", "3.18.10-11111", "3.18.1-1111-bbbb"} {
		inst := &MockObject{
			annotations: map[string]string{builder.CRVersionKey: ver},
			arch:        core.ValkeyCluster,
		}

		{
			cmd := &MockClusterEnsureResource{}
			foundActor := am.Search(CmdClustetrEnsureResource, inst)
			assert.NotNil(t, foundActor)
			assert.Equal(t, cmd.Version().String(), foundActor.Version().String())
		}

		{
			cmd := MockClusterHealPodActor{}
			foundActor := am.Search(CmdClustetrHealPod, inst)
			assert.NotNil(t, foundActor)
			assert.Equal(t, cmd.Version().String(), foundActor.Version().String())
		}
	}

	for _, ver := range []string{
		"3.14.0", "3.14.10", "3.14.10-11111", "3.14.1-1111-bbbb",
		"3.15.0", "3.15.10", "3.15.10-11111", "3.15.1-1111-bbbb",
		"3.16.0", "3.16.10", "3.16.10-11111", "3.16.1-1111-bbbb",
		"3.17.0", "3.17.10", "3.17.10-11111", "3.17.1-1111-bbbb",
	} {
		inst := &MockObject{
			annotations: map[string]string{builder.CRVersionKey: ver},
			arch:        core.ValkeyCluster,
		}

		{
			t.Logf("version %s", ver)
			cmd := &MockClusterEnsureResource314{}
			foundActor := am.Search(CmdClustetrEnsureResource, inst)
			assert.NotNil(t, foundActor)
			assert.Equal(t, cmd.Version().String(), foundActor.Version().String())
		}

		{
			cmd := &MockClusterHealPodActor314{}
			foundActor := am.Search(CmdClustetrHealPod, inst)
			assert.NotNil(t, foundActor)
			assert.Equal(t, cmd.Version().String(), foundActor.Version().String())
		}
	}

	for _, ver := range []string{
		"3.14.0", "3.14.10", "3.14.10-11111", "3.14.1-1111-bbbb",
		"3.15.0", "3.15.10", "3.15.10-11111", "3.15.1-1111-bbbb",
		"3.16.0", "3.16.10", "3.16.10-11111", "3.16.1-1111-bbbb",
		"3.17.0", "3.17.10", "3.17.10-11111", "3.17.1-1111-bbbb",
	} {
		inst := &MockObject{
			annotations: map[string]string{builder.CRVersionKey: ver},
			arch:        core.ValkeyFailover,
		}

		{
			t.Logf("version %s", ver)
			cmd := &MockFailoverEnsureResource314{}
			foundActor := am.Search(CmdFailoverEnsureResource, inst)
			assert.NotNil(t, foundActor)
			assert.Equal(t, cmd.Version().String(), foundActor.Version().String())
		}

		{
			cmd := &MockFailoverHealPodActor314{}
			foundActor := am.Search(CmdFailoverHealPod, inst)
			assert.NotNil(t, foundActor)
			assert.Equal(t, cmd.Version().String(), foundActor.Version().String())
		}
	}

	{
		inst := &MockObject{
			annotations: map[string]string{builder.CRVersionKey: "3.18.0"},
		}

		{
			foundActor := am.Search(CmdFailoverHealPod, inst)
			assert.Nil(t, foundActor)
		}
	}

	inst := &MockObject{
		annotations: map[string]string{},
		arch:        core.ValkeyFailover,
	}

	foundActor := am.Search(CmdFailoverHealPod, inst)
	assert.NotNil(t, foundActor)

	am = nil
	am.Print()
	if am.Search(CmdClustetrHealPod, inst) != nil {
		t.Errorf("Search should return nil")
	}
	am.Add(core.ValkeyCluster, MockNewClusterEnsureResource(nil, logger))
}

// TestReleasePipelineTagIsParseableVersion guards the contract between the
// release pipeline and Search: every tag branch-pipeline.yaml can emit must be a
// version Search can parse.
//
// SemVer forbids leading zeros in numeric pre-release identifiers, so a bare
// '+%m%d%H%M' timestamp yields an unparseable tag for every build in January to
// September (e.g. v2.0.0-rc.09121529.d812ee9). Search then matches no actor for
// any command and the operator silently stops creating workloads.
//
// The format is read out of the workflow rather than duplicated here so the two
// cannot drift apart.
func TestReleasePipelineTagIsParseableVersion(t *testing.T) {
	const workflow = "../../.github/workflows/branch-pipeline.yaml"

	data, err := os.ReadFile(workflow)
	if err != nil {
		t.Fatalf("read %s: %v", workflow, err)
	}

	matches := regexp.MustCompile(`TS=\$\(date '\+([^']+)'\)`).FindSubmatch(data)
	if matches == nil {
		t.Fatalf("no timestamp format found in %s; update this test if the tag scheme changed", workflow)
	}
	strftime := string(matches[1])

	// Only the directives the workflow is allowed to use.
	layout := strftime
	for _, r := range []struct{ directive, goLayout string }{
		{"%Y", "2006"}, {"%m", "01"}, {"%d", "02"},
		{"%H", "15"}, {"%M", "04"}, {"%S", "05"},
	} {
		layout = strings.ReplaceAll(layout, r.directive, r.goLayout)
	}
	if strings.Contains(layout, "%") {
		t.Fatalf("unsupported strftime directive in %q; extend this test", strftime)
	}

	// One sample per month, at the smallest day/hour/minute, which is when
	// leading zeros are most likely.
	for month := 1; month <= 12; month++ {
		ts := time.Date(2026, time.Month(month), 1, 0, 5, 0, 0, time.UTC).Format(layout)
		tag := fmt.Sprintf("v2.0.0-rc.%s.d812ee9", ts)

		t.Run(tag, func(t *testing.T) {
			ver, err := semver.NewVersion(tag)
			assert.NoError(t, err, "tag %q emitted by %s is not a parseable version", tag, workflow)
			assert.NotNil(t, ver)
		})
	}
}
