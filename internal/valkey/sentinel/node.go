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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/sentinelbuilder"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/types/user"
	vkcli "github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/chideat/valkey-operator/pkg/version"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func LoadValkeySentinelNodes(ctx context.Context, client kubernetes.ClientSet, sts metav1.Object, newUser *user.User, logger logr.Logger) ([]types.SentinelNode, error) {
	if client == nil {
		return nil, fmt.Errorf("require clientset")
	}
	if sts == nil {
		return nil, fmt.Errorf("require statefulset")
	}
	pods, err := client.GetStatefulSetPods(ctx, sts.GetNamespace(), sts.GetName())
	if err != nil {
		logger.Error(err, "loads pods of shard failed")
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	nodes := []types.SentinelNode{}
	for _, pod := range pods.Items {
		pod := pod.DeepCopy()
		if node, err := NewValkeySentinelNode(ctx, client, sts, pod, newUser, logger); err != nil {
			logger.Error(err, "parse node failed", "pod", pod.Name)
		} else {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func LoadDeploymentValkeySentinelNodes(ctx context.Context, client kubernetes.ClientSet, obj metav1.Object, newUser *user.User, logger logr.Logger) ([]types.SentinelNode, error) {
	if client == nil {
		return nil, fmt.Errorf("require clientset")
	}
	if obj == nil {
		return nil, fmt.Errorf("require statefulset")
	}
	pods, err := client.GetDeploymentPods(ctx, obj.GetNamespace(), obj.GetName())
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		logger.Error(err, "loads pods of shard failed")
		return nil, err
	}
	nodes := []types.SentinelNode{}
	for _, pod := range pods.Items {
		pod := pod.DeepCopy()
		if node, err := NewValkeySentinelNode(ctx, client, obj, pod, newUser, logger); err != nil {
			logger.Error(err, "parse node failed", "pod", pod.Name)
		} else {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func NewValkeySentinelNode(ctx context.Context, client kubernetes.ClientSet, obj metav1.Object, pod *corev1.Pod, newUser *user.User, logger logr.Logger) (types.SentinelNode, error) {
	if client == nil {
		return nil, fmt.Errorf("require clientset")
	}
	if obj == nil {
		return nil, fmt.Errorf("require workload object")
	}
	if pod == nil {
		return nil, fmt.Errorf("require pod")
	}

	node := ValkeySentinelNode{
		Pod:     *pod,
		parent:  obj,
		client:  client,
		newUser: newUser,
		logger:  logger.WithName("ValkeySentinelNode"),
	}

	var err error
	if node.localUser, err = node.loadLocalUser(ctx); err != nil {
		return nil, err
	}
	if node.tlsConfig, err = node.loadTLS(ctx); err != nil {
		return nil, err
	}

	if node.IsContainerReady() {
		vkcli, err := node.getConnect(ctx, &node)
		if err != nil {
			return nil, err
		}
		defer vkcli.Close()

		if node.info, node.config, err = node.loadNodeInfo(ctx, &node, vkcli); err != nil {
			return nil, err
		}
	}
	return &node, nil
}

var _ types.SentinelNode = &ValkeySentinelNode{}

type ValkeySentinelNode struct {
	corev1.Pod
	parent    metav1.Object
	client    kubernetes.ClientSet
	localUser *user.User
	newUser   *user.User
	tlsConfig *tls.Config
	info      *vkcli.NodeInfo
	config    map[string]string
	logger    logr.Logger
}

func (n *ValkeySentinelNode) Definition() *corev1.Pod {
	if n == nil {
		return nil
	}
	return &n.Pod
}

// loadLocalUser
//
// every pod still mount secret to the pod. for acl supported and not supported versions, the difference is that:
// unsupported: the mount secret is the default user secret, which maybe changed
// supported: the mount secret is operator's secret. the operator secret never changes, which is used only internal
//
// this method is used to fetch pod's operator secret
// for versions without acl supported, there exists cases that the env secret not consistent with the server
func (s *ValkeySentinelNode) loadLocalUser(ctx context.Context) (*user.User, error) {
	if s == nil {
		return nil, nil
	}
	logger := s.logger.WithName("loadLocalUser")

	var secretName string
	container := util.GetContainerByName(&s.Spec, sentinelbuilder.SentinelContainerName)
	if container == nil {
		return nil, fmt.Errorf("server container not found")
	}
	for _, env := range container.Env {
		if env.Name == builder.OperatorSecretName && env.Value != "" {
			secretName = env.Value
			break
		}
	}
	if secretName != "" {
		if secret, err := s.client.GetSecret(ctx, s.GetNamespace(), secretName); err != nil {
			logger.Error(err, "get user secret failed", "target", util.ObjectKey(s.GetNamespace(), secretName))
			return nil, err
		} else if user, err := types.NewSentinelUser("", user.RoleDeveloper, secret); err != nil {
			return nil, err
		} else {
			return user, nil
		}
	}
	// return default user with out password
	return types.NewSentinelUser("", user.RoleDeveloper, nil)
}

func (n *ValkeySentinelNode) loadTLS(ctx context.Context) (*tls.Config, error) {
	if n == nil {
		return nil, nil
	}
	logger := n.logger

	var name string
	for _, vol := range n.Spec.Volumes {
		if vol.Name == builder.ValkeyTLSVolumeName && vol.Secret != nil && vol.Secret.SecretName != "" {
			name = vol.Secret.SecretName
			break
		}
	}
	if name == "" {
		return nil, nil
	}

	secret, err := n.client.GetSecret(ctx, n.GetNamespace(), name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		logger.Error(err, "get secret failed", "target", name)
		return nil, err
	}

	if secret.Data[corev1.TLSCertKey] == nil || secret.Data[corev1.TLSPrivateKeyKey] == nil ||
		secret.Data["ca.crt"] == nil {
		logger.Error(fmt.Errorf("invalid tls secret"), "tls secret is invaid")
		return nil, fmt.Errorf("tls secret is invalid")
	}
	cert, err := tls.X509KeyPair(secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey])
	if err != nil {
		logger.Error(err, "generate X509KeyPair failed")
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(secret.Data["ca.crt"])

	return &tls.Config{
		InsecureSkipVerify: true, // #nosec
		RootCAs:            caCertPool,
		Certificates:       []tls.Certificate{cert},
	}, nil
}

func (n *ValkeySentinelNode) getConnect(ctx context.Context, node *ValkeySentinelNode) (vkcli.ValkeyClient, error) {
	if n == nil {
		return nil, fmt.Errorf("nil node")
	}
	logger := n.logger.WithName("getConnect")

	if !n.IsContainerReady() {
		logger.Error(fmt.Errorf("get node info failed"), "pod not ready", "target",
			client.ObjectKey{Namespace: node.Namespace, Name: node.Name})
		return nil, fmt.Errorf("node not ready")
	}

	var (
		err  error
		addr = net.JoinHostPort(node.DefaultInternalIP().String(), strconv.Itoa(n.InternalPort()))
	)
	for _, user := range []*user.User{node.newUser, node.localUser} {
		if user == nil {
			continue
		}
		vkcli := vkcli.NewValkeyClient(addr, vkcli.AuthConfig{
			Username:  user.Name,
			Password:  user.Password.String(),
			TLSConfig: node.tlsConfig,
		})

		nctx, cancel := context.WithTimeout(ctx, time.Second*10)
		defer cancel()

		if err = vkcli.Ping(nctx); err != nil {
			if strings.Contains(err.Error(), "NOAUTH Authentication required") ||
				strings.Contains(err.Error(), "invalid password") ||
				strings.Contains(err.Error(), "Client sent AUTH, but no password is set") ||
				strings.Contains(err.Error(), "invalid username-password pair") {
				continue
			}
			logger.Error(err, "check connection to node failed", "address", addr)
			return nil, err
		}
		return vkcli, nil
	}
	if err == nil {
		err = fmt.Errorf("no usable account to connect to node")
	}
	return nil, err
}

// loadNodeInfo
func (n *ValkeySentinelNode) loadNodeInfo(ctx context.Context, _ *ValkeySentinelNode, vkcli vkcli.ValkeyClient) (info *vkcli.NodeInfo,
	config map[string]string, err error) {
	// fetch node info
	if info, err = vkcli.Info(ctx); err != nil {
		n.logger.Error(err, "load node info failed")
		return nil, nil, err
	}
	return
}

// Index returns the index of the related pod
func (n *ValkeySentinelNode) Index() int {
	if n == nil {
		return -1
	}

	name := n.Pod.Name
	if i := strings.LastIndex(name, "-"); i > 0 {
		index, _ := strconv.ParseInt(name[i+1:], 10, 64)
		return int(index)
	}
	return -1
}

// Refresh not concurrency safe
func (n *ValkeySentinelNode) Refresh(ctx context.Context) (err error) {
	if n == nil {
		return nil
	}

	// refresh pod first
	if pod, err := n.client.GetPod(ctx, n.GetNamespace(), n.GetName()); err != nil {
		n.logger.Error(err, "refresh pod failed")
		return err
	} else {
		n.Pod = *pod
	}

	if n.IsContainerReady() {
		vkcli, err := n.getConnect(ctx, n)
		if err != nil {
			return err
		}
		defer vkcli.Close()

		if n.info, n.config, err = n.loadNodeInfo(ctx, n, vkcli); err != nil {
			n.logger.Error(err, "refresh info failed")
			return err
		}
	}
	return nil
}

// IsContainerReady
func (n *ValkeySentinelNode) IsContainerReady() bool {
	if n == nil {
		return false
	}

	for _, cond := range n.Pod.Status.ContainerStatuses {
		if cond.Name == sentinelbuilder.SentinelContainerName {
			// assume the main process is ready in 10s
			if cond.Started != nil && *cond.Started && cond.State.Running != nil &&
				time.Since(cond.State.Running.StartedAt.Time) > time.Second*10 {
				return true
			}
		}
	}
	return false
}

// IsReady
func (n *ValkeySentinelNode) IsReady() bool {
	if n == nil || n.IsTerminating() {
		return false
	}

	for _, cond := range n.Pod.Status.ContainerStatuses {
		if cond.Name == sentinelbuilder.SentinelContainerName {
			return cond.Ready
		}
	}
	return false
}

// IsTerminating
func (n *ValkeySentinelNode) IsTerminating() bool {
	if n == nil {
		return false
	}

	return n.DeletionTimestamp != nil
}

func (n *ValkeySentinelNode) IsACLApplied() bool {
	// check if acl have been applied to container
	container := util.GetContainerByName(&n.Pod.Spec, sentinelbuilder.SentinelContainerName)
	for _, env := range container.Env {
		if env.Name == "ACL_CONFIGMAP_NAME" {
			return true
		}
	}
	return false
}

func (n *ValkeySentinelNode) CurrentVersion() version.ValkeyVersion {
	if n == nil {
		return ""
	}

	// parse version from image
	container := util.GetContainerByName(&n.Pod.Spec, sentinelbuilder.SentinelContainerName)
	if ver, _ := version.ParseValkeyVersionFromImage(container.Image); ver != version.ValkeyVersionUnknown {
		return ver
	}
	v, _ := version.ParseValkeyVersion(n.info.Version)
	return v
}

func (n *ValkeySentinelNode) Config() map[string]string {
	if n == nil || n.config == nil {
		return nil
	}
	return n.config
}

// Setup only return the last command error
func (n *ValkeySentinelNode) Setup(ctx context.Context, margs ...[]any) (err error) {
	if n == nil {
		return nil
	}

	vkcli, err := n.getConnect(ctx, n)
	if err != nil {
		return err
	}
	defer vkcli.Close()

	// TODO: change this to pipeline
	for _, args := range margs {
		if len(args) == 0 {
			continue
		}
		cmd, ok := args[0].(string)
		if !ok {
			return fmt.Errorf("the command must be string")
		}

		func() {
			ctx, cancel := context.WithTimeout(ctx, time.Second*10)
			defer cancel()

			if _, err = vkcli.Do(ctx, cmd, args[1:]...); err != nil {
				// ignore forget nodes error
				n.logger.Error(err, "set config failed", "target", n.GetName(), "address", n.DefaultInternalIP().String(), "port", n.InternalPort(), "cmd", cmd)
			}
		}()
	}
	return
}

func (n *ValkeySentinelNode) Query(ctx context.Context, cmd string, args ...any) (any, error) {
	if n == nil {
		return nil, nil
	}

	vkcli, err := n.getConnect(ctx, n)
	if err != nil {
		return nil, err
	}
	defer vkcli.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	return vkcli.Do(ctx, cmd, args...)
}

func (n *ValkeySentinelNode) Brothers(ctx context.Context, name string) (ret []*vkcli.SentinelMonitorNode, err error) {
	if n == nil {
		return nil, nil
	}

	items, err := vkcli.Values(n.Query(ctx, "SENTINEL", "SENTINELS", name))
	if err != nil {
		if strings.Contains(err.Error(), "No such master with that name") {
			return nil, nil
		}
		return nil, err
	}
	for _, item := range items {
		node := vkcli.ParseSentinelMonitorNode(item)
		ret = append(ret, node)
	}
	return
}

func (n *ValkeySentinelNode) MonitoringClusters(ctx context.Context) (clusters []string, err error) {
	if n == nil {
		return nil, nil
	}

	if items, err := vkcli.Values(n.Query(ctx, "SENTINEL", "MASTERS")); err != nil {
		return nil, err
	} else {
		for _, item := range items {
			node := vkcli.ParseSentinelMonitorNode(item)
			if node.Name != "" {
				clusters = append(clusters, node.Name)
			}
		}
	}
	return
}

func (n *ValkeySentinelNode) MonitoringNodes(ctx context.Context, name string) (master *vkcli.SentinelMonitorNode,
	replicas []*vkcli.SentinelMonitorNode, err error) {

	if n == nil {
		return nil, nil, nil
	}
	if name == "" {
		return nil, nil, fmt.Errorf("empty name")
	}

	if val, err := n.Query(ctx, "SENTINEL", "MASTER", name); err != nil {
		if strings.Contains(err.Error(), "No such master with that name") {
			return nil, nil, nil
		}
		return nil, nil, err
	} else {
		master = vkcli.ParseSentinelMonitorNode(val)
	}

	// NOTE: require sentinel 5.0.0
	if items, err := vkcli.Values(n.Query(ctx, "SENTINEL", "REPLICAS", name)); err != nil {
		return nil, nil, err
	} else {
		for _, item := range items {
			node := vkcli.ParseSentinelMonitorNode(item)
			replicas = append(replicas, node)
		}
	}
	return
}

func (n *ValkeySentinelNode) Info() vkcli.NodeInfo {
	if n == nil || n.info == nil {
		return vkcli.NodeInfo{}
	}
	return *n.info
}

func (n *ValkeySentinelNode) Port() int {
	if port := n.Pod.Labels[builder.AnnouncePortLabelKey]; port != "" {
		if val, _ := strconv.ParseInt(port, 10, 32); val > 0 {
			return int(val)
		}
	}
	return n.InternalPort()
}

func (n *ValkeySentinelNode) InternalPort() int {
	port := 26379
	if container := util.GetContainerByName(&n.Pod.Spec, sentinelbuilder.SentinelContainerName); container != nil {
		for _, p := range container.Ports {
			if p.Name == sentinelbuilder.SentinelContainerPortName {
				port = int(p.ContainerPort)
				break
			}
		}
	}
	return port
}

func (n *ValkeySentinelNode) DefaultIP() net.IP {
	if value := n.Pod.Labels[builder.AnnounceIPLabelKey]; value != "" {
		address := strings.Replace(value, "-", ":", -1)
		return net.ParseIP(address)
	}
	return n.DefaultInternalIP()
}

func (n *ValkeySentinelNode) DefaultInternalIP() net.IP {
	ips := n.IPs()
	if len(ips) == 0 {
		return nil
	}

	var ipFamilyPrefer string
	if container := util.GetContainerByName(&n.Pod.Spec, sentinelbuilder.SentinelContainerName); container != nil {
		for _, env := range container.Env {
			if env.Name == "IP_FAMILY_PREFER" {
				ipFamilyPrefer = env.Value
				break
			}
		}
	}

	if ipFamilyPrefer != "" {
		for _, ip := range n.IPs() {
			addr, err := netip.ParseAddr(ip.String())
			if err != nil {
				continue
			}
			if addr.Is4() && ipFamilyPrefer == string(corev1.IPv4Protocol) ||
				addr.Is6() && ipFamilyPrefer == string(corev1.IPv6Protocol) {
				return ip
			}
		}
	}
	return ips[0]
}

func (n *ValkeySentinelNode) IPs() []net.IP {
	if n == nil {
		return nil
	}
	ips := []net.IP{}
	for _, podIp := range n.Pod.Status.PodIPs {
		ips = append(ips, net.ParseIP(podIp.IP))
	}
	return ips
}

func (n *ValkeySentinelNode) NodeIP() net.IP {
	if n == nil {
		return nil
	}
	return net.ParseIP(n.Pod.Status.HostIP)
}

// ContainerStatus
func (n *ValkeySentinelNode) ContainerStatus() *corev1.ContainerStatus {
	if n == nil {
		return nil
	}
	for _, status := range n.Pod.Status.ContainerStatuses {
		if status.Name == sentinelbuilder.SentinelContainerName {
			return &status
		}
	}
	return nil
}

// Status
func (n *ValkeySentinelNode) Status() corev1.PodPhase {
	if n == nil {
		return corev1.PodUnknown
	}
	return n.Pod.Status.Phase
}

func (n *ValkeySentinelNode) SetMonitor(ctx context.Context, name, ip, port, user, password, quorum string) error {
	if err := n.Setup(ctx, []any{"SENTINEL", "REMOVE", name}); err != nil {
		n.logger.Error(err, "try remove cluster failed", "name", name)
	}

	n.logger.Info("set monitor", "name", name, "ip", ip, "port", port, "quorum", quorum)
	if err := n.Setup(ctx, []any{"SENTINEL", "MONITOR", name, ip, port, quorum}); err != nil {
		return err
	}
	if password != "" {
		if user != "" {
			if err := n.Setup(ctx, []any{"SENTINEL", "SET", name, "auth-user", user}); err != nil {
				return err
			}
		}
		if err := n.Setup(ctx, []any{"SENTINEL", "SET", name, "auth-pass", password}); err != nil {
			return err
		}
		if err := n.Setup(ctx, []any{"SENTINEL", "RESET", name}); err != nil {
			return err
		}
	}
	return nil
}
