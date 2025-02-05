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

package failover

import (
	"context"

	"github.com/chideat/valkey-operator/cmd/helper/commands"
	"github.com/urfave/cli/v2"
	corev1 "k8s.io/api/core/v1"
)

func NewCommand(ctx context.Context) *cli.Command {
	return &cli.Command{
		Name:  "failover",
		Usage: "Failover set commands",
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
				Name:    "pod-ips",
				Usage:   "The ips of current pod",
				EnvVars: []string{"POD_IPS"},
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
				Name:    "acl",
				Usage:   "Enable acl",
				EnvVars: []string{"ACL_ENABLED"},
				Hidden:  true,
			},
			&cli.StringFlag{
				Name:    "acl-config",
				Usage:   "Acl config map name",
				EnvVars: []string{"ACL_CONFIGMAP_NAME"},
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
				Name:    "nodeport-enabled",
				Usage:   "nodeport switch",
				EnvVars: []string{"NODEPORT_ENABLED"},
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
				Name:  "shutdown",
				Usage: "Shutdown node",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "monitor-policy",
						Usage:   "Monitor policy for replication",
						EnvVars: []string{"MONITOR_POLICY"},
					},
					&cli.StringFlag{
						Name:    "monitor-uri",
						Usage:   "Monitor uri for failover",
						EnvVars: []string{"MONITOR_URI"},
					},
					&cli.StringFlag{
						Name:    "monitor-operator-secret-name",
						Usage:   "Monitor operator secret name",
						EnvVars: []string{"MONITOR_OPERATOR_SECRET_NAME"},
					},
					&cli.StringFlag{
						Name:  "name",
						Usage: "Monitor name",
						Value: "mymaster",
					},
					&cli.IntFlag{
						Name:    "timeout",
						Aliases: []string{"t"},
						Usage:   "Timeout time of shutdown",
						Value:   300,
					},
				},
				Action: func(c *cli.Context) error {
					logger := commands.NewLogger(c).WithName("Shutdown")

					client, err := commands.NewClient()
					if err != nil {
						logger.Error(err, "create k8s client failed, error=%s", err)
						return cli.Exit(err, 1)
					}
					_ = Shutdown(ctx, c, client, logger)
					return nil
				},
			},
		},
	}
}
