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

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/chideat/valkey-operator/cmd/helper/commands/cluster"
	"github.com/chideat/valkey-operator/cmd/helper/commands/failover"
	"github.com/chideat/valkey-operator/cmd/helper/commands/helper"
	"github.com/chideat/valkey-operator/cmd/helper/commands/runner"
	"github.com/chideat/valkey-operator/cmd/helper/commands/sentinel"
	"github.com/urfave/cli/v2"
)

func NewApp(ctx context.Context, cmds ...*cli.Command) *cli.App {
	return &cli.App{
		Name:     filepath.Base(os.Args[0]),
		Usage:    "helper [command] [flags]",
		Commands: cmds,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name: "zap-devel",
				Usage: "Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). \n" +
					"Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error)",
				Value: true,
			},
			&cli.StringFlag{
				Name:   "zap-encoder",
				Usage:  "Zap log encoding (one of 'json' or 'console')",
				Value:  "console",
				Hidden: true,
			},
			&cli.StringFlag{
				Name: "zap-log-level",
				Usage: "Zap Level to configure the verbosity of logging. Can be one of 'debug', 'info', 'error', \n" +
					"or any integer value > 0 which corresponds to custom debug levels of increasing verbosity",
			},
			&cli.StringFlag{
				Name:   "zap-stacktrace-level",
				Usage:  "Zap Level at and above which stacktraces are captured (one of 'info', 'error', 'panic').",
				Hidden: true,
			},
			&cli.StringFlag{
				Name:  "zap-time-encoding",
				Usage: "Zap time encoding (one of 'epoch', 'millis', 'nano', 'iso8601', 'rfc3339' or 'rfc3339nano'). Defaults to 'rfc3339'.",
				Value: "rfc3339",
			},
		},
	}
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	app := NewApp(
		ctx,
		cluster.NewCommand(ctx),
		sentinel.NewCommand(ctx),
		failover.NewCommand(ctx),
		helper.NewCommand(ctx),
		runner.NewCommand(ctx),
	)

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
