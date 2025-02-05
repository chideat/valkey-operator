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
package helper

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chideat/valkey-operator/cmd/helper/commands"
	"github.com/urfave/cli/v2"
)

func NewCommand(ctx context.Context) *cli.Command {
	return &cli.Command{
		Name:  "helper",
		Usage: "Helper includes some usable tools",
		Subcommands: []*cli.Command{
			{
				Name:  "generate",
				Usage: "Generate include tools to generate config/log",
				Subcommands: []*cli.Command{
					{
						Name:  "acl",
						Usage: "Generate acl config from the acl config file",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "namespace",
								Usage:   "Namespace of current pod",
								EnvVars: []string{"NAMESPACE"},
							},
							&cli.StringFlag{
								Name:    "name",
								Usage:   "The name of the configmap for acl",
								EnvVars: []string{"ACL_CONFIGMAP_NAME"},
							},
						},
						Action: func(c *cli.Context) error {
							var (
								namespace = c.String("namespace")
								cmName    = c.String("name")
							)

							ctx, cancel := context.WithCancel(ctx)
							defer cancel()

							logger := commands.NewLogger(c)

							client, err := commands.NewClient()
							if err != nil {
								logger.Error(err, "create k8s client failed, error=%s", err)
								return cli.Exit(err, 1)
							}
							ret, err := GenerateACL(ctx, client, namespace, cmName)
							if err != nil {
								logger.Error(err, "load acl failed")
								return cli.Exit(err, 1)
							}
							// print to stdout
							for _, item := range ret {
								fmt.Fprintf(os.Stdout, "%s\n", item)
							}
							return nil
						},
					},
				},
			},
			{
				Name:  "get-password",
				Usage: "Get user password according to acl config",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "namespace",
						Usage:   "Namespace of current pod",
						EnvVars: []string{"NAMESPACE"},
					},
					&cli.StringFlag{
						Name:    "pod-name",
						Usage:   "pod name",
						EnvVars: []string{"POD_NAME"},
					},
					&cli.StringFlag{
						Name:    "name",
						Usage:   "The name of the configmap for acl",
						EnvVars: []string{"ACL_CONFIGMAP_NAME"},
					},
					&cli.StringFlag{
						Name:    "username",
						Usage:   "Operator user name",
						EnvVars: []string{"OPERATOR_USERNAME"},
					},
					&cli.StringFlag{
						Name:    "password-secret",
						Usage:   "Operator password secret",
						EnvVars: []string{"OPERATOR_SECRET_NAME"},
					},
				},
				Action: func(c *cli.Context) error {
					var (
						namespace  = c.String("namespace")
						podName    = c.String("pod-name")
						cmName     = c.String("name")
						username   = c.String("username")
						secretName = c.String("password-secret")
					)

					if cmName == "" {
						podName = strings.TrimPrefix(podName, "drc-")
						if index := strings.LastIndex(podName, "-"); index > 0 {
							if index = strings.LastIndex(podName[0:index], "-"); index > 0 {
								cmName = fmt.Sprintf("drc-acl-%s", podName[0:index])
							}
						}
					}

					ctx, cancel := context.WithCancel(ctx)
					defer cancel()

					logger := commands.NewLogger(c)

					client, err := commands.NewClient()
					if err != nil {
						logger.Error(err, "create k8s client failed, error=%s", err)
						return cli.Exit(err, 1)
					}
					pwd, err := GetUserPassword(ctx, client, namespace, cmName, username, secretName)
					if err != nil {
						logger.Error(err, "load acl failed")
						return cli.Exit(err, 1)
					}
					fmt.Fprint(os.Stdout, pwd)
					return nil
				},
			},
			{
				Name:  "healthcheck",
				Usage: "Health check",
				Flags: []cli.Flag{
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
						Name:  "addr",
						Usage: "Instance service address",
						Value: "local.inject:6379",
					},
					&cli.IntFlag{
						Name:    "timeout",
						Aliases: []string{"t"},
						Usage:   "Timeout time of ping",
						Value:   3,
					},
				},
				Subcommands: []*cli.Command{
					{
						Name:  "ping",
						Usage: "ping instance to check if it is alive",
						Action: func(c *cli.Context) error {
							var (
								serviceAddr = c.String("addr")
								timeout     = c.Int64("timeout")
							)
							logger := commands.NewLogger(c).WithName("liveness")

							if timeout <= 0 {
								timeout = 5
							}
							if serviceAddr == "" {
								serviceAddr = "local.inject:6379"
							}
							ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
							defer cancel()

							info, err := commands.LoadAuthInfo(c, ctx)
							if err != nil {
								logger.Error(err, "load auth info failed")
								return cli.Exit(err, 1)
							}

							if err := Ping(ctx, serviceAddr, *info); err != nil {
								if strings.HasPrefix(err.Error(), "LOADING") ||
									strings.HasPrefix(err.Error(), "BUSY") {
									return nil
								}
								logger.Error(err, "ping failed")
								return cli.Exit(err, 1)
							}
							return nil
						},
					},
				},
			},
		},
	}
}
