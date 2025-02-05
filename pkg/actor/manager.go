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
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
)

var (
	registeredActorInitializer = map[core.Arch][]func(kubernetes.ClientSet, logr.Logger) Actor{}
)

func Register(a core.Arch, f func(kubernetes.ClientSet, logr.Logger) Actor) {
	if _, ok := registeredActorInitializer[a]; !ok {
		registeredActorInitializer[a] = []func(kubernetes.ClientSet, logr.Logger) Actor{}
	}
	registeredActorInitializer[a] = append(registeredActorInitializer[a], f)
}

type VersionedActor []Actor

type ActorGroup map[string]VersionedActor

func (ag ActorGroup) Add(a Actor) {
	for _, cmd := range a.SupportedCommands() {
		ag[cmd.String()] = append(ag[cmd.String()], a)
	}
}

func (ag ActorGroup) Get(cmd Command) []Actor {
	if actors, ok := ag[cmd.String()]; ok {
		return actors
	}
	return nil
}

func (ag ActorGroup) All() (ret [][]Actor) {
	for _, actors := range ag {
		ret = append(ret, actors)
	}
	return
}

// ActorManager
type ActorManager struct {
	actors map[core.Arch]ActorGroup
	logger logr.Logger
}

// NewActorManager
func NewActorManager(cs kubernetes.ClientSet, logger logr.Logger) *ActorManager {
	m := &ActorManager{
		actors: map[core.Arch]ActorGroup{},
		logger: logger,
	}
	for arch, inits := range registeredActorInitializer {
		for _, init := range inits {
			m.Add(arch, init(cs, logger))
		}
	}
	return m
}

func (m *ActorManager) Print() {
	if m == nil {
		return
	}

	for arch, ag := range m.actors {
		cmds := map[string][]string{}
		for cmd, actors := range ag {
			versions := lo.Map(actors, func(actor Actor, index int) string {
				return actor.Version().String()
			})
			cmds[cmd] = versions
		}
		m.logger.Info("ActorManager", "arch", string(arch), "commands", cmds)
	}
}

type Object interface {
	GetAnnotations() map[string]string
	Arch() core.Arch
}

// Search find the actor which match the version requirement (version >= actor.Version)
func (m *ActorManager) Search(cmd Command, inst Object) Actor {
	if m == nil {
		return nil
	}

	crVersion := inst.GetAnnotations()[config.CRVersionKey]
	if crVersion == "" {
		crVersion = config.GetOperatorVersion()
	}
	ver, _ := semver.NewVersion(crVersion)
	if ver == nil {
		return nil
	}
	if val, err := ver.SetPrerelease(""); err == nil {
		ver = &val
	}
	if val, err := ver.SetMetadata(""); err == nil {
		ver = &val
	}

	if ag := m.actors[core.Arch(inst.Arch())]; ag != nil {
		actors := ag.Get(cmd)
		if len(actors) == 0 {
			return nil
		} else if len(actors) == 1 {
			return actors[0]
		}
		sort.SliceStable(actors, func(i, j int) bool {
			return actors[i].Version().GreaterThan(actors[j].Version())
		})
		for _, actor := range actors {
			if !ver.LessThan(actor.Version()) {
				return actor
			}
		}
	}
	return nil
}

// Add
func (m *ActorManager) Add(arch core.Arch, a Actor) {
	if m == nil {
		return
	}

	ag := m.actors[arch]
	if ag == nil {
		m.actors[arch] = ActorGroup{}
		ag = m.actors[arch]
	}
	ag.Add(a)
}
