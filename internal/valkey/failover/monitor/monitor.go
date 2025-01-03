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
package monitor

import (
	"context"
	"fmt"

	v1 "github.com/chideat/valkey-operator/api/v1alpha1"
	clientset "github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/go-logr/logr"
)

func LoadFailoverMonitor(ctx context.Context, client clientset.ClientSet, inst types.FailoverInstance, logger logr.Logger) (types.FailoverMonitor, error) {
	def := inst.Definition()
	if def.Status.Monitor.Policy == v1.ManualFailoverPolicy {
		mon, err := NewManualMonitor(ctx, client, inst)
		if err != nil {
			logger.Error(err, "load manual monitor failed")
		}
		return mon, nil
	} else if def.Status.Monitor.Policy == v1.SentinelFailoverPolicy {
		repl, err := NewSentinelMonitor(ctx, client, inst)
		if err != nil {
			logger.Error(err, "load sentinel monitor failed")
		}
		return repl, nil
	}
	return nil, fmt.Errorf("unknown monitor policy %s", def.Status.Monitor.Policy)
}
