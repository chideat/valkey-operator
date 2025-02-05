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
package cluster

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"

	"github.com/chideat/valkey-operator/cmd/helper/commands"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/client-go/kubernetes"
)

// ExposeNodePort
func ExposeNodePort(ctx context.Context, client *kubernetes.Clientset, namespace, podName, ipfamily string, serviceType corev1.ServiceType, logger logr.Logger) error {
	logger.Info("Info", "serviceType", serviceType, "ipfamily", ipfamily)
	pod, err := commands.GetPod(ctx, client, namespace, podName, logger)
	if err != nil {
		logger.Error(err, "get pods failed", "namespace", namespace, "name", podName)
		return err
	}
	if pod.Status.HostIP == "" {
		return fmt.Errorf("pod not found or pod with invalid hostIP")
	}

	var (
		announceIp         = pod.Status.PodIP
		announcePort int32 = 6379
		// announceIPort is this port necessary ?
		announceIPort int32 = 16379
	)
	if serviceType == corev1.ServiceTypeNodePort {
		podSvc, err := commands.RetryGetService(ctx, client, namespace, podName, corev1.ServiceTypeNodePort, 20, logger)
		if errors.IsNotFound(err) {
			if podSvc, err = commands.RetryGetService(ctx, client, namespace, podName, corev1.ServiceTypeNodePort, 20, logger); err != nil {
				logger.Error(err, "get service failed", "target", fmt.Sprintf("%s/%s", namespace, podName))
				return err
			}
		} else if err != nil {
			logger.Error(err, "get service failed", "target", fmt.Sprintf("%s/%s", namespace, podName))
			return err
		}
		for _, v := range podSvc.Spec.Ports {
			if v.Name == "client" {
				announcePort = v.NodePort
			}
			if v.Name == "gossip" {
				announceIPort = v.NodePort
			}
		}

		node, err := client.CoreV1().Nodes().Get(ctx, pod.Spec.NodeName, metav1.GetOptions{})
		if err != nil {
			logger.Error(err, "get nodes err", "node", node.Name)
			return err
		}
		logger.Info("get nodes success", "Name", node.Name)

		var addresses []string
		for _, addr := range node.Status.Addresses {
			if addr.Address == "" {
				continue
			}

			switch addr.Type {
			case corev1.NodeExternalIP:
				ip, err := netip.ParseAddr(addr.Address)
				if err != nil {
					logger.Error(err, "parse address err", "address", addr.Address)
					return err
				}
				if ipfamily == "IPv6" && ip.Is6() {
					addresses = append(addresses, addr.Address)
				} else if ipfamily != "IPv6" && ip.Is4() {
					addresses = append(addresses, addr.Address)
				}
			case corev1.NodeInternalIP:
				// internal ip first
				ip, err := netip.ParseAddr(addr.Address)
				if err != nil {
					logger.Error(err, "parse address err", "address", addr.Address)
					return err
				}
				if ipfamily == "IPv6" && ip.Is6() {
					addresses = append([]string{addr.Address}, addresses...)
					addresses = append(addresses, addr.Address)
				} else if ipfamily != "IPv6" && ip.Is4() {
					addresses = append([]string{addr.Address}, addresses...)
				}
			}
		}
		if len(addresses) > 0 {
			announceIp = addresses[0]
		} else {
			err := fmt.Errorf("no available address")
			logger.Error(err, "get usable address failed")
			return err
		}
	} else if serviceType == corev1.ServiceTypeLoadBalancer {
		podSvc, err := commands.RetryGetService(ctx, client, namespace, podName, corev1.ServiceTypeLoadBalancer, 20, logger)
		if errors.IsNotFound(err) {
			if podSvc, err = commands.RetryGetService(ctx, client, namespace, podName, corev1.ServiceTypeLoadBalancer, 20, logger); err != nil {
				logger.Error(err, "retry get lb service failed")
				return err
			}
		} else if err != nil {
			logger.Error(err, "get lb service failed", "target", fmt.Sprintf("%s/%s", namespace, podName))
			return err
		}

		for _, v := range podSvc.Status.LoadBalancer.Ingress {
			if v.IP != "" {
				ip, err := netip.ParseAddr(v.IP)
				if err != nil {
					logger.Error(err, "parse address err", "address", v.IP)
					return err
				}
				if ipfamily == "IPv6" && ip.Is6() {
					announceIp = v.IP
					break
				}
				if ipfamily != "IPv6" && ip.Is4() {
					announceIp = v.IP
					break
				}
			}
		}
	} else {
		for _, addr := range pod.Status.PodIPs {
			ip, err := netip.ParseAddr(addr.IP)
			if err != nil {
				return err
			}
			if ipfamily == "IPv6" && ip.Is6() {
				announceIp = addr.IP
				break
			} else if ipfamily != "IPv6" && ip.Is4() {
				announceIp = addr.IP
				break
			}
		}
	}

	format_announceIp := strings.Replace(announceIp, ":", "-", -1)
	labelPatch := fmt.Sprintf(`[{"op":"add","path":"/metadata/labels/%s","value":"%s"},{"op":"add","path":"/metadata/labels/%s","value":"%s"},{"op":"add","path":"/metadata/labels/%s","value":"%s"}]`,
		strings.Replace("middleware.alauda.io/announce_ip", "/", "~1", -1), format_announceIp,
		strings.Replace("middleware.alauda.io/announce_port", "/", "~1", -1), strconv.Itoa(int(announcePort)),
		strings.Replace("middleware.alauda.io/announce_iport", "/", "~1", -1), strconv.Itoa(int(announceIPort)))

	logger.Info(labelPatch)
	_, err = client.CoreV1().Pods(pod.Namespace).Patch(ctx, podName, types.JSONPatchType, []byte(labelPatch), metav1.PatchOptions{})
	if err != nil {
		logger.Error(err, "patch pod label failed")
		return err
	}
	configContent := fmt.Sprintf(`cluster-announce-ip %s
cluster-announce-port %d
cluster-announce-bus-port %d`, announceIp, announcePort, announceIPort)

	return os.WriteFile("/data/announce.conf", []byte(configContent), 0644) // #nosec: G306
}
