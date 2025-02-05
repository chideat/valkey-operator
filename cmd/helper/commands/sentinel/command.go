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
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/chideat/valkey-operator/cmd/helper/commands"
	"github.com/chideat/valkey-operator/cmd/helper/commands/runner"
	"github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/urfave/cli/v2"
	corev1 "k8s.io/api/core/v1"
)

func NewCommand(ctx context.Context) *cli.Command {
	return &cli.Command{
		Name:  "sentinel",
		Usage: "Sentinel set commands",
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
				Usage:   "The id of current pod",
				EnvVars: []string{"POD_UID"},
			},
			&cli.StringFlag{
				Name:    "service-name",
				Usage:   "Service name of the statefulset",
				EnvVars: []string{"SERVICE_NAME"},
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
				Name:    "ip-family",
				Usage:   "IP_FAMILY for servie access",
				EnvVars: []string{"IP_FAMILY_PREFER"},
			},
			&cli.StringFlag{
				Name:    "service-type",
				Usage:   "Service type for sentinel service",
				EnvVars: []string{"SERVICE_TYPE"},
				Value:   "ClusterIP",
			},
		},
		Subcommands: []*cli.Command{
			{
				Name:  "expose",
				Usage: "Create nodeport service for current pod to announce",
				Flags: []cli.Flag{},
				Action: func(c *cli.Context) error {
					var (
						namespace   = c.String("namespace")
						podName     = c.String("pod-name")
						ipFamily    = c.String("ip-family")
						serviceType = corev1.ServiceType(c.String("service-type"))
					)
					if namespace == "" {
						return cli.Exit("require namespace", 1)
					}
					if podName == "" {
						return cli.Exit("require podname", 1)
					}

					logger := commands.NewLogger(c).WithName("Access")

					client, err := commands.NewClient()
					if err != nil {
						logger.Error(err, "create k8s client failed, error=%s", err)
						return cli.Exit(err, 1)
					}

					if err := Access(ctx, client, namespace, podName, ipFamily, serviceType, logger); err != nil {
						logger.Error(err, "enable nodeport service access failed")
						return cli.Exit(err, 1)
					}
					return nil
				},
			},
			{
				Name:  "get-master-addr",
				Usage: "Get current master address",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "monitor-uri",
						Usage:   "Monitor URI",
						EnvVars: []string{"MONITOR_URI"},
					},
					&cli.StringFlag{
						Name:  "name",
						Usage: "Sentinel master name",
						Value: "mymaster",
					},
					&cli.StringFlag{
						Name:    "monitor-operator-secret-name",
						Usage:   "Monitor operator secret name",
						EnvVars: []string{"MONITOR_OPERATOR_SECRET_NAME"},
					},
					&cli.BoolFlag{
						Name:  "healthy",
						Usage: "Require the master is healthy",
					},
					&cli.IntFlag{
						Name:  "timeout",
						Usage: "Timeout for get master address",
						Value: 10,
					},
				},
				Action: func(c *cli.Context) error {
					var (
						sentinelUri    = c.String("monitor-uri")
						clusterName    = c.String("name")
						timeout        = c.Int("timeout")
						requireHealthy = c.Bool("healthy")
					)

					client, err := commands.NewClient()
					if err != nil {
						return cli.Exit(err, 1)
					}

					authInfo, err := commands.LoadAuthInfo(c, ctx)
					if err != nil {
						return cli.Exit(err, 1)
					}

					senAuthInfo, err := commands.LoadMonitorAuthInfo(c, ctx, client)
					if err != nil {
						return cli.Exit(err, 1)
					}

					ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
					defer cancel()

					var sentinelNodes []string
					if val, err := url.Parse(sentinelUri); err != nil {
						return cli.Exit(fmt.Sprintf("parse sentinel uri failed, error=%s", err), 1)
					} else {
						sentinelNodes = strings.Split(val.Host, ",")
					}

					var valkeyCli valkey.ValkeyClient
					for _, node := range sentinelNodes {
						valkeyCli = valkey.NewValkeyClient(node, *senAuthInfo)
						if _, err = valkeyCli.Do(ctx, "PING"); err != nil {
							valkeyCli.Close()
							valkeyCli = nil
							continue
						}
						break
					}
					if valkeyCli == nil {
						return cli.Exit("no sentinel node available", 1)
					}
					defer valkeyCli.Close()

					if vals, err := valkey.StringMap(valkeyCli.Do(ctx, "SENTINEL", "master", clusterName)); err != nil {
						return cli.Exit(fmt.Sprintf("connect to sentinel failed, error=%s", err), 1)
					} else {
						addr := net.JoinHostPort(vals["ip"], vals["port"])
						if !requireHealthy {
							fmt.Println(addr)
						} else if vals["flags"] == "master" {
							valkeyCli = valkey.NewValkeyClient(addr, *authInfo)
							defer valkeyCli.Close()

							if err := func() error {
								var (
									err  error
									info *valkey.NodeInfo
								)
								for i := 0; i < 3; i++ {
									if info, err = valkeyCli.Info(ctx); err != nil {
										time.Sleep(time.Second)
										continue
									} else if info.Role != "master" {
										return fmt.Errorf("role of node %s is %s", addr, info.Role)
									}
									return nil
								}
								return err
							}(); err != nil {
								return cli.Exit(err, 1)
							} else {
								fmt.Println(addr)
							}
						}
					}
					return nil
				},
			},
			{
				Name:  "failover",
				Usage: "Do sentinel failover",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "monitor-uri",
						Usage:   "Monitor URI",
						EnvVars: []string{"MONITOR_URI"},
					},
					&cli.StringFlag{
						Name:    "monitor-operator-secret-name",
						Usage:   "Monitor operator secret name",
						EnvVars: []string{"MONITOR_OPERATOR_SECRET_NAME"},
					},
					&cli.StringSliceFlag{
						Name:  "escape",
						Usage: "Addresses need to be escape",
					},
					&cli.StringFlag{
						Name:  "name",
						Usage: "Sentinel monitor name",
						Value: "mymaster",
					},
					&cli.IntFlag{
						Name:  "timeout",
						Usage: "Timeout for get master address",
						Value: 120,
					},
				},
				Action: func(c *cli.Context) error {
					logger := commands.NewLogger(c).WithName("Failover")

					client, err := commands.NewClient()
					if err != nil {
						logger.Error(err, "create k8s client failed, error=%s", err)
						return cli.Exit(err, 1)
					}

					var timeout = c.Int("timeout")
					ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
					defer cancel()

					return Failover(ctx, c, client, logger)
				},
			},
			{
				Name:  "shutdown",
				Usage: "Shutdown sentinel node",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "workspace",
						Usage: "Workspace of this container",
						Value: "/data",
					},
					&cli.StringFlag{
						Name:  "config-name",
						Usage: "sentinel config file",
						Value: "sentinel.conf",
					},
					&cli.StringFlag{
						Name:  "prefix",
						Usage: "Etcd sync resource name prefix",
						Value: "sync-",
					},
					&cli.IntFlag{
						Name:    "timeout",
						Aliases: []string{"t"},
						Usage:   "Timeout time of shutdown",
						Value:   30,
					},
				},
				Action: func(c *cli.Context) error {
					logger := commands.NewLogger(c).WithName("Shutdown")

					client, err := commands.NewClient()
					if err != nil {
						logger.Error(err, "create k8s client failed, error=%s", err)
						return cli.Exit(err, 1)
					}
					if err := Shutdown(ctx, c, client, logger); err != nil {
						return cli.Exit(err, 1)
					}
					return nil
				},
			},
			{
				Name:  "merge-config",
				Usage: "merge config from local and cached",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "workspace",
						Usage: "valkey server data workdir",
						Value: "/data",
					},
					&cli.StringFlag{
						Name:  "local-conf-file",
						Usage: "Local sentinel config file",
						Value: "/conf/sentinel.conf",
					},
					&cli.StringFlag{
						Name:  "config-name",
						Usage: "Sentinel config file name",
						Value: "sentinel.conf",
					},
					&cli.StringFlag{
						Name:  "prefix",
						Usage: "Resource name prefix",
						Value: "sync-",
					},
				},
				Action: func(c *cli.Context) error {

					logger := commands.NewLogger(c).WithName("Access")

					client, err := commands.NewClient()
					if err != nil {
						logger.Error(err, "create k8s client failed, error=%s", err)
						return cli.Exit(err, 1)
					}
					if err := MergeConfig(ctx, c, client, logger); err != nil {
						logger.Error(err, "enable nodeport service access failed")
						return cli.Exit(err, 1)
					}
					return nil
				},
			},
			{
				Name:  "agent",
				Usage: "Sentinel agent",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "workspace",
						Usage: "valkey server data workdir",
						Value: "/data",
					},
					&cli.StringFlag{
						Name:  "config-name",
						Usage: "Sentinel config file name",
						Value: "sentinel.conf",
					},
					&cli.StringFlag{
						Name:  "prefix",
						Usage: "Resource name prefix",
						Value: "sync-",
					},
					&cli.Int64Flag{
						Name:        "interval",
						Usage:       "Sync interval",
						Value:       5,
						DefaultText: "5s",
					},
				},
				Action: func(c *cli.Context) error {
					ctx, cancel := context.WithCancel(ctx)
					defer cancel()

					logger := commands.NewLogger(c)
					if err := runner.SyncFromLocalToEtcd(c, ctx, "secret", true, logger); err != nil {
						return cli.Exit(err, 1)
					}
					return nil
				},
			},
		},
	}
}
