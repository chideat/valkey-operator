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

package commands

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chideat/valkey-operator/pkg/valkey"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/go-logr/logr"
	"github.com/urfave/cli/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/retry"
)

const (
	DefaultSecretMountPath = "/account/password" // #nosec G101
	InjectedPasswordPath   = "/tmp/newpass"      // #nosec G101
)

func LoadMonitorAuthInfo(c *cli.Context, ctx context.Context, client *kubernetes.Clientset) (*valkey.AuthInfo, error) {
	var (
		namespace      = c.String("namespace")
		passwordSecret = c.String("monitor-operator-secret-name")
		// tls
		isTLSEnabled = c.Bool("tls")
		tlsKeyFile   = c.String("tls-key-file")
		tlsCertFile  = c.String("tls-cert-file")
	)

	var (
		err      error
		tlsConf  *tls.Config
		password string
	)
	if passwordSecret != "" {
		if err := RetryGet(func() error {
			if data, err := client.CoreV1().Secrets(namespace).Get(ctx, passwordSecret, metav1.GetOptions{}); err != nil {
				return err
			} else {
				password = string(data.Data["password"])
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	if isTLSEnabled {
		if tlsConf, err = LoadTLSCofig(tlsKeyFile, tlsCertFile); err != nil {
			return nil, err
		}
	}
	return &valkey.AuthInfo{
		Password:  password,
		TLSConfig: tlsConf,
	}, nil
}

func LoadAuthInfo(c *cli.Context, ctx context.Context) (*valkey.AuthInfo, error) {
	var (
		// acl
		opUsername = c.String("operator-username")
		// tls
		isTLSEnabled = c.Bool("tls")
		tlsKeyFile   = c.String("tls-key-file")
		tlsCertFile  = c.String("tls-cert-file")
	)

	var (
		err          error
		tlsConf      *tls.Config
		password     string
		passwordPath = DefaultSecretMountPath
	)
	if opUsername == "" || opUsername == "default" {
		if _, err := os.Stat(InjectedPasswordPath); err == nil {
			passwordPath = InjectedPasswordPath
		}
	}
	if data, err := os.ReadFile(passwordPath); err != nil && !os.IsNotExist(err) {
		return nil, err
	} else {
		password = string(data)
	}

	if isTLSEnabled {
		if tlsConf, err = LoadTLSCofig(tlsKeyFile, tlsCertFile); err != nil {
			return nil, err
		}
	}
	return &valkey.AuthInfo{
		Username:  opUsername,
		Password:  password,
		TLSConfig: tlsConf,
	}, nil
}

func LoadTLSCofig(tlsKeyFile, tlsCertFile string) (*tls.Config, error) {
	if tlsKeyFile == "" || tlsCertFile == "" {
		return nil, fmt.Errorf("tls file path not configed")
	}
	cert, err := tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true, // #nosec G402
	}, nil
}

// NewOwnerReference
func NewOwnerReference(ctx context.Context, client *kubernetes.Clientset, namespace, podName string) ([]metav1.OwnerReference, error) {
	if client == nil {
		return nil, nil
	}

	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	var name string
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "StatefulSet" {
			name = ownerRef.Name
			break
		}
	}
	if sts, err := client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{}); err != nil {
		return nil, err
	} else {
		return sts.OwnerReferences, nil
	}
}

func RetryGet(f func() error, steps ...int) error {
	step := 5
	if len(steps) > 0 && steps[0] > 0 {
		step = steps[0]
	}
	return retry.OnError(wait.Backoff{
		Steps:    step,
		Duration: 400 * time.Millisecond,
		Factor:   2.0,
		Jitter:   2,
	}, func(err error) bool {
		return errors.IsInternalError(err) || errors.IsServerTimeout(err) || errors.IsServiceUnavailable(err) ||
			errors.IsTimeout(err) || errors.IsTooManyRequests(err)
	}, f)
}

func GetPod(ctx context.Context, client *kubernetes.Clientset, namespace, name string, logger logr.Logger) (*corev1.Pod, error) {
	var pod *corev1.Pod
	if err := RetryGet(func() (err error) {
		logger.Info("get pods ip", "namespace", namespace, "name", name)
		if pod, err = client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{}); err != nil {
			logger.Error(err, "get pods failed")
			return err
		} else if pod.Status.PodIP == "" {
			return errors.NewTimeoutError("pod have not assigied pod ip", 0)
		} else if pod.Status.HostIP == "" {
			return errors.NewTimeoutError("pod have not assigied host ip", 0)
		}
		return
	}, 20); err != nil {
		return nil, err
	}
	return pod, nil
}

func RetryGetService(ctx context.Context, clientset *kubernetes.Clientset, svcNamespace, svcName string, typ corev1.ServiceType,
	count int, logger logr.Logger) (*corev1.Service, error) {

	serviceChecker := func(svc *corev1.Service, typ corev1.ServiceType) error {
		if svc == nil {
			return fmt.Errorf("service not found")
		}
		if len(svc.Spec.Ports) < 1 {
			return fmt.Errorf("service port not found")
		}

		if svc.Spec.Type != typ {
			return fmt.Errorf("service type not match")
		}

		switch svc.Spec.Type {
		case corev1.ServiceTypeNodePort:
			for _, port := range svc.Spec.Ports {
				if port.NodePort == 0 {
					return fmt.Errorf("service nodeport not found for port %d", port.Port)
				}
			}
		case corev1.ServiceTypeLoadBalancer:
			if len(svc.Status.LoadBalancer.Ingress) < 1 {
				return fmt.Errorf("service loadbalancer ip not found")
			} else {
				for _, v := range svc.Status.LoadBalancer.Ingress {
					if v.IP == "" {
						return fmt.Errorf("service loadbalancer ip is empty")
					}
				}
			}
		}
		return nil
	}

	logger.Info("retry get service", "target", fmt.Sprintf("%s/%s", svcNamespace, svcName), "count", count)
	for i := 0; i < count+1; i++ {
		svc, err := clientset.CoreV1().Services(svcNamespace).Get(ctx, svcName, metav1.GetOptions{})
		if err != nil {
			logger.Error(err, "get service failed", "target", fmt.Sprintf("%s/%s", svcNamespace, svcName))
			return nil, err
		}
		if serviceChecker(svc, typ) != nil {
			logger.Error(err, "service check failed", "target", fmt.Sprintf("%s/%s", svcNamespace, svcName))
		} else {
			return svc, nil
		}
		time.Sleep(time.Second * 3)
	}
	return nil, fmt.Errorf("service %s/%s not ready", svcNamespace, svcName)
}

func NewClient() (*kubernetes.Clientset, error) {
	var (
		err  error
		conf *rest.Config
	)

	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" && port == "" {
		if fp := os.Getenv("KUBE_CONFIG"); fp != "" {
			if conf, err = clientcmd.BuildConfigFromFlags("", fp); err != nil {
				return nil, fmt.Errorf("load config from $KUBE_CONFIG failed, error=%s", err)
			}
		} else {
			if home := homedir.HomeDir(); home != "" {
				fp := filepath.Join(home, ".kube", "config")
				if conf, err = clientcmd.BuildConfigFromFlags("", fp); err != nil {
					return nil, fmt.Errorf("load config from local .kube/config failed, error=%s", err)
				}
			} else {
				return nil, fmt.Errorf("no local config found")
			}
		}
	} else {
		conf, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}
	return kubernetes.NewForConfig(conf)
}

func NewLogger(c *cli.Context) logr.Logger {
	var (
		level, _    = zapcore.ParseLevel(c.String("zap-log-level"))
		timeEncoder zapcore.TimeEncoder
		dev         = false
	)
	if c != nil {
		dev = c.Bool("zap-devel")
		if err := timeEncoder.UnmarshalText([]byte(c.String("zap-time-encoding"))); err != nil {
			panic(err)
		}
	}

	opts := &zap.Options{
		Development:     dev,
		Level:           level,
		TimeEncoder:     timeEncoder,
		StacktraceLevel: zapcore.PanicLevel,
	}
	return logr.New(zap.New(zap.UseFlagOptions(opts)).GetSink())
}
