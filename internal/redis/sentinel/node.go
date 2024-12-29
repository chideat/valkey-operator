/*
Copyright 2023 The RedisOperator Authors.

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
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
	"github.com/chideat/valkey-operator/internal/builder/sentinelbuilder"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	rediscli "github.com/chideat/valkey-operator/pkg/redis"
	"github.com/chideat/valkey-operator/pkg/types/redis"
	"github.com/chideat/valkey-operator/pkg/types/user"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func LoadRedisSentinelNodes(ctx context.Context, client kubernetes.ClientSet, sts metav1.Object, newUser *user.User, logger logr.Logger) ([]redis.RedisSentinelNode, error) {
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
	nodes := []redis.RedisSentinelNode{}
	for _, pod := range pods.Items {
		pod := pod.DeepCopy()
		if node, err := NewRedisSentinelNode(ctx, client, sts, pod, newUser, logger); err != nil {
			logger.Error(err, "parse redis node failed", "pod", pod.Name)
		} else {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func LoadDeploymentRedisSentinelNodes(ctx context.Context, client kubernetes.ClientSet, obj metav1.Object, newUser *user.User, logger logr.Logger) ([]redis.RedisSentinelNode, error) {
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
	nodes := []redis.RedisSentinelNode{}
	for _, pod := range pods.Items {
		pod := pod.DeepCopy()
		if node, err := NewRedisSentinelNode(ctx, client, obj, pod, newUser, logger); err != nil {
			logger.Error(err, "parse redis node failed", "pod", pod.Name)
		} else {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func NewRedisSentinelNode(ctx context.Context, client kubernetes.ClientSet, obj metav1.Object, pod *corev1.Pod, newUser *user.User, logger logr.Logger) (redis.RedisSentinelNode, error) {
	if client == nil {
		return nil, fmt.Errorf("require clientset")
	}
	if obj == nil {
		return nil, fmt.Errorf("require workload object")
	}
	if pod == nil {
		return nil, fmt.Errorf("require pod")
	}

	node := RedisSentinelNode{
		Pod:     *pod,
		parent:  obj,
		client:  client,
		newUser: newUser,
		logger:  logger.WithName("RedisSentinelNode"),
	}

	var err error
	if node.localUser, err = node.loadLocalUser(ctx); err != nil {
		return nil, err
	}
	if node.tlsConfig, err = node.loadTLS(ctx); err != nil {
		return nil, err
	}

	if node.IsContainerReady() {
		redisCli, err := node.getRedisConnect(ctx, &node)
		if err != nil {
			return nil, err
		}
		defer redisCli.Close()

		if node.info, node.config, err = node.loadRedisInfo(ctx, &node, redisCli); err != nil {
			return nil, err
		}
	}
	return &node, nil
}

var _ redis.RedisSentinelNode = &RedisSentinelNode{}

type RedisSentinelNode struct {
	corev1.Pod
	parent    metav1.Object
	client    kubernetes.ClientSet
	localUser *user.User
	newUser   *user.User
	tlsConfig *tls.Config
	info      *rediscli.RedisInfo
	config    map[string]string
	logger    logr.Logger
}

func (n *RedisSentinelNode) Definition() *corev1.Pod {
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
func (s *RedisSentinelNode) loadLocalUser(ctx context.Context) (*user.User, error) {
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
		if env.Name == sentinelbuilder.OperatorSecretName && env.Value != "" {
			secretName = env.Value
			break
		}
	}
	if secretName != "" {
		if secret, err := s.client.GetSecret(ctx, s.GetNamespace(), secretName); err != nil {
			logger.Error(err, "get user secret failed", "target", util.ObjectKey(s.GetNamespace(), secretName))
			return nil, err
		} else if user, err := user.NewSentinelUser("", user.RoleDeveloper, secret); err != nil {
			return nil, err
		} else {
			return user, nil
		}
	}
	// return default user with out password
	return user.NewSentinelUser("", user.RoleDeveloper, nil)
}

func (n *RedisSentinelNode) loadTLS(ctx context.Context) (*tls.Config, error) {
	if n == nil {
		return nil, nil
	}
	logger := n.logger

	var name string
	for _, vol := range n.Spec.Volumes {
		if vol.Name == clusterbuilder.RedisTLSVolumeName && vol.Secret != nil && vol.Secret.SecretName != "" {
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

func (n *RedisSentinelNode) getRedisConnect(ctx context.Context, node *RedisSentinelNode) (rediscli.RedisClient, error) {
	if n == nil {
		return nil, fmt.Errorf("nil node")
	}
	logger := n.logger.WithName("getRedisConnect")

	if !n.IsContainerReady() {
		logger.Error(fmt.Errorf("get redis info failed"), "pod not ready", "target",
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
		rediscli := rediscli.NewRedisClient(addr, rediscli.AuthConfig{
			Username:  user.Name,
			Password:  user.Password.String(),
			TLSConfig: node.tlsConfig,
		})

		nctx, cancel := context.WithTimeout(ctx, time.Second*10)
		defer cancel()

		if err = rediscli.Ping(nctx); err != nil {
			if strings.Contains(err.Error(), "NOAUTH Authentication required") ||
				strings.Contains(err.Error(), "invalid password") ||
				strings.Contains(err.Error(), "Client sent AUTH, but no password is set") ||
				strings.Contains(err.Error(), "invalid username-password pair") {
				continue
			}
			logger.Error(err, "check connection to redis failed", "address", addr)
			return nil, err
		}
		return rediscli, nil
	}
	if err == nil {
		err = fmt.Errorf("no usable account to connect to redis instance")
	}
	return nil, err
}

// loadRedisInfo
func (n *RedisSentinelNode) loadRedisInfo(ctx context.Context, _ *RedisSentinelNode, redisCli rediscli.RedisClient) (info *rediscli.RedisInfo,
	config map[string]string, err error) {
	// fetch redis info
	if info, err = redisCli.Info(ctx); err != nil {
		n.logger.Error(err, "load redis info failed")
		return nil, nil, err
	}
	return
}

// Index returns the index of the related pod
func (n *RedisSentinelNode) Index() int {
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
func (n *RedisSentinelNode) Refresh(ctx context.Context) (err error) {
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
		redisCli, err := n.getRedisConnect(ctx, n)
		if err != nil {
			return err
		}
		defer redisCli.Close()

		if n.info, n.config, err = n.loadRedisInfo(ctx, n, redisCli); err != nil {
			n.logger.Error(err, "refresh info failed")
			return err
		}
	}
	return nil
}

// IsContainerReady
func (n *RedisSentinelNode) IsContainerReady() bool {
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
func (n *RedisSentinelNode) IsReady() bool {
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
func (n *RedisSentinelNode) IsTerminating() bool {
	if n == nil {
		return false
	}

	return n.DeletionTimestamp != nil
}

func (n *RedisSentinelNode) IsACLApplied() bool {
	// check if acl have been applied to container
	container := util.GetContainerByName(&n.Pod.Spec, sentinelbuilder.SentinelContainerName)
	for _, env := range container.Env {
		if env.Name == "ACL_CONFIGMAP_NAME" {
			return true
		}
	}
	return false
}

func (n *RedisSentinelNode) CurrentVersion() redis.RedisVersion {
	if n == nil {
		return ""
	}

	// parse version from redis image
	container := util.GetContainerByName(&n.Pod.Spec, sentinelbuilder.SentinelContainerName)
	if ver, _ := redis.ParseRedisVersionFromImage(container.Image); ver != redis.RedisVersionUnknown {
		return ver
	}
	v, _ := redis.ParseRedisVersion(n.info.RedisVersion)
	return v
}

func (n *RedisSentinelNode) Config() map[string]string {
	if n == nil || n.config == nil {
		return nil
	}
	return n.config
}

// Setup only return the last command error
func (n *RedisSentinelNode) Setup(ctx context.Context, margs ...[]any) (err error) {
	if n == nil {
		return nil
	}

	redisCli, err := n.getRedisConnect(ctx, n)
	if err != nil {
		return err
	}
	defer redisCli.Close()

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

			if _, err = redisCli.Do(ctx, cmd, args[1:]...); err != nil {
				// ignore forget nodes error
				n.logger.Error(err, "set config failed", "target", n.GetName(), "address", n.DefaultInternalIP().String(), "port", n.InternalPort(), "cmd", cmd)
			}
		}()
	}
	return
}

func (n *RedisSentinelNode) Query(ctx context.Context, cmd string, args ...any) (any, error) {
	if n == nil {
		return nil, nil
	}

	redisCli, err := n.getRedisConnect(ctx, n)
	if err != nil {
		return nil, err
	}
	defer redisCli.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	return redisCli.Do(ctx, cmd, args...)
}

func (n *RedisSentinelNode) Brothers(ctx context.Context, name string) (ret []*rediscli.SentinelMonitorNode, err error) {
	if n == nil {
		return nil, nil
	}

	items, err := rediscli.Values(n.Query(ctx, "SENTINEL", "SENTINELS", name))
	if err != nil {
		if strings.Contains(err.Error(), "No such master with that name") {
			return nil, nil
		}
		return nil, err
	}
	for _, item := range items {
		node := rediscli.ParseSentinelMonitorNode(item)
		ret = append(ret, node)
	}
	return
}

func (n *RedisSentinelNode) MonitoringClusters(ctx context.Context) (clusters []string, err error) {
	if n == nil {
		return nil, nil
	}

	if items, err := rediscli.Values(n.Query(ctx, "SENTINEL", "MASTERS")); err != nil {
		return nil, err
	} else {
		for _, item := range items {
			node := rediscli.ParseSentinelMonitorNode(item)
			if node.Name != "" {
				clusters = append(clusters, node.Name)
			}
		}
	}
	return
}

func (n *RedisSentinelNode) MonitoringNodes(ctx context.Context, name string) (master *rediscli.SentinelMonitorNode,
	replicas []*rediscli.SentinelMonitorNode, err error) {

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
		master = rediscli.ParseSentinelMonitorNode(val)
	}

	// NOTE: require sentinel 5.0.0
	if items, err := rediscli.Values(n.Query(ctx, "SENTINEL", "REPLICAS", name)); err != nil {
		return nil, nil, err
	} else {
		for _, item := range items {
			node := rediscli.ParseSentinelMonitorNode(item)
			replicas = append(replicas, node)
		}
	}
	return
}

func (n *RedisSentinelNode) Info() rediscli.RedisInfo {
	if n == nil || n.info == nil {
		return rediscli.RedisInfo{}
	}
	return *n.info
}

func (n *RedisSentinelNode) Port() int {
	if port := n.Pod.Labels[builder.PodAnnouncePortLabelKey]; port != "" {
		if val, _ := strconv.ParseInt(port, 10, 32); val > 0 {
			return int(val)
		}
	}
	return n.InternalPort()
}

func (n *RedisSentinelNode) InternalPort() int {
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

func (n *RedisSentinelNode) DefaultIP() net.IP {
	if value := n.Pod.Labels[builder.PodAnnounceIPLabelKey]; value != "" {
		address := strings.Replace(value, "-", ":", -1)
		return net.ParseIP(address)
	}
	return n.DefaultInternalIP()
}

func (n *RedisSentinelNode) DefaultInternalIP() net.IP {
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

func (n *RedisSentinelNode) IPs() []net.IP {
	if n == nil {
		return nil
	}
	ips := []net.IP{}
	for _, podIp := range n.Pod.Status.PodIPs {
		ips = append(ips, net.ParseIP(podIp.IP))
	}
	return ips
}

func (n *RedisSentinelNode) NodeIP() net.IP {
	if n == nil {
		return nil
	}
	return net.ParseIP(n.Pod.Status.HostIP)
}

// ContainerStatus
func (n *RedisSentinelNode) ContainerStatus() *corev1.ContainerStatus {
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
func (n *RedisSentinelNode) Status() corev1.PodPhase {
	if n == nil {
		return corev1.PodUnknown
	}
	return n.Pod.Status.Phase
}

func (n *RedisSentinelNode) SetMonitor(ctx context.Context, name, ip, port, user, password, quorum string) error {
	if err := n.Setup(ctx, []interface{}{"SENTINEL", "REMOVE", name}); err != nil {
		n.logger.Error(err, "try remove cluster failed", "name", name)
	}

	n.logger.Info("set monitor", "name", name, "ip", ip, "port", port, "quorum", quorum)
	if err := n.Setup(ctx, []interface{}{"SENTINEL", "MONITOR", name, ip, port, quorum}); err != nil {
		return err
	}
	if password != "" {
		if n.CurrentVersion().IsACLSupported() {
			if user != "" {
				if err := n.Setup(ctx, []interface{}{"SENTINEL", "SET", name, "auth-user", user}); err != nil {
					return err
				}
			}
		}
		if err := n.Setup(ctx, []interface{}{"SENTINEL", "SET", name, "auth-pass", password}); err != nil {
			return err
		}
		if err := n.Setup(ctx, []interface{}{"SENTINEL", "RESET", name}); err != nil {
			return err
		}
	}
	return nil
}
