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

package runner

import (
	"context"
	"time"

	"github.com/chideat/valkey-operator/cmd/helper/commands"
	"github.com/urfave/cli/v2"
)

func NewCommand(ctx context.Context) *cli.Command {
	return &cli.Command{
		Name:  "runner",
		Usage: "Runner for cluster or sentinel",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "namespace",
				Usage:   "Namespace of current pod",
				EnvVars: []string{"NAMESPACE"},
			},
			&cli.StringFlag{
				Name:    "pod-name",
				Usage:   "The name of current pod",
				EnvVars: []string{"POD_NAME"},
			},
			&cli.StringFlag{
				Name:    "pod-uid",
				Usage:   "The uid of current pod",
				EnvVars: []string{"POD_UID"},
			},
			&cli.StringFlag{
				Name:    "operator-username",
				Usage:   "Operator username",
				EnvVars: []string{"OPERATOR_USERNAME"},
			},
			&cli.StringFlag{
				Name:    "operator-secret-name",
				Usage:   "Operator user password secret name",
				EnvVars: []string{"OPERATOR_SECRET_NAME"},
			},
			&cli.BoolFlag{
				Name:    "tls",
				Usage:   "Enable tls",
				EnvVars: []string{"TLS_ENABLED"},
			},
			&cli.StringFlag{
				Name:    "tls-key-file",
				Usage:   "Name of the client key file (including full path)",
				EnvVars: []string{"TLS_CLIENT_KEY_FILE"},
				Value:   "/tls/tls.key",
			},
			&cli.StringFlag{
				Name:    "tls-cert-file",
				Usage:   "Name of the client certificate file (including full path)",
				EnvVars: []string{"TLS_CLIENT_CERT_FILE"},
				Value:   "/tls/tls.crt",
			},
			&cli.StringFlag{
				Name:    "tls-ca-file",
				Usage:   "Name of the ca file (including full path)",
				EnvVars: []string{"TLS_CA_CERT_FILE"},
				Value:   "/tls/ca.crt",
			},
			&cli.StringFlag{
				Name:    "address",
				Usage:   "server address",
				EnvVars: []string{"VALKEY_ADDRESS"},
				Value:   "local.inject:6379",
			},
		},
		Subcommands: []*cli.Command{
			{
				Name:  "cluster",
				Usage: "Cluster sidercar",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "sync-l2c",
						Usage: "Enable sync local nodes.conf to configmap",
					},
					&cli.StringFlag{
						Name:  "workspace",
						Usage: "Valkey server data workdir",
						Value: "/data",
					},
					&cli.StringFlag{
						Name:  "config-name",
						Usage: "Node config file name",
						Value: "nodes.conf",
					},
					&cli.StringFlag{
						Name:  "prefix",
						Usage: "Configmap name prefix",
						Value: "sync-",
					},
					&cli.Int64Flag{
						Name:        "interval",
						Usage:       "Configmap sync interval",
						Value:       5,
						DefaultText: "5s",
					},
				},
				Action: func(c *cli.Context) error {
					ctx, cancel := context.WithCancel(ctx)
					defer cancel()

					logger := commands.NewLogger(c)

					if c.Bool("sync-l2c") {
						go func() {
							time.Sleep(time.Second * 60)
							_ = SyncFromLocalToEtcd(c, ctx, "", true, logger)
						}()
					}

					authInfo, err := commands.LoadAuthInfo(c, ctx)
					if err != nil {
						logger.Error(err, "load authinfo failed")
						return cli.Exit(err, 1)
					}
					_ = RebalanceSlots(ctx, *authInfo, c.String("address"), logger)
					return nil
				},
			},
		},
	}
}
