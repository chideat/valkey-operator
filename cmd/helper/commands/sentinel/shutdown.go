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

package sentinel

import (
	"context"
	"time"

	"github.com/chideat/valkey-operator/cmd/helper/commands/runner"
	"github.com/go-logr/logr"
	"github.com/urfave/cli/v2"
	"k8s.io/client-go/kubernetes"
)

// Shutdown
func Shutdown(ctx context.Context, c *cli.Context, client *kubernetes.Clientset, logger logr.Logger) error {
	timeout := time.Duration(c.Int("timeout")) * time.Second
	if timeout == 0 {
		timeout = time.Second * 30
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// sync current nodes.conf to configmap
	logger.Info("persistent sentinel.conf to secret")
	if err := runner.SyncFromLocalToEtcd(c, ctx, "secret", false, logger); err != nil {
		logger.Error(err, "persistent sentinel.conf to configmap failed")
	}
	return nil
}
