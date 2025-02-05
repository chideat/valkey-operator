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

package node

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/slot"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/types/user"
	vkcli "github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/chideat/valkey-operator/pkg/version"
	"github.com/go-logr/logr"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ types.ValkeyNode = (*ValkeyNode)(nil)

// LoadNodes
func LoadNodes(ctx context.Context, client kubernetes.ClientSet, sts *appv1.StatefulSet, newUser *user.User, logger logr.Logger) ([]types.ValkeyNode, error) {
	if client == nil {
		return nil, fmt.Errorf("require clientset")
	}
	if sts == nil {
		return nil, fmt.Errorf("require statefulset")
	}

	// load pods by statefulset selector
	pods, err := client.GetStatefulSetPodsByLabels(ctx, sts.GetNamespace(), sts.Spec.Selector.MatchLabels)
	if err != nil {
		logger.Error(err, "loads pods of shard failed")
		return nil, err
	}

	nodes := []types.ValkeyNode{}
	for _, pod := range pods.Items {
		pod := pod.DeepCopy()
		if !func() bool {
			for _, own := range pod.OwnerReferences {
				if own.UID == sts.GetUID() {
					return true
				}
			}
			return false
		}() {
			continue
		}

		if node, err := NewValkeyNode(ctx, client, sts, pod, newUser, logger); err != nil {
			logger.Error(err, "parse valkey node failed", "pod", pod.Name)
		} else {
			nodes = append(nodes, node)
		}
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		return nodes[i].Index() < nodes[j].Index()
	})
	return nodes, nil
}

type ValkeyNode struct {
	corev1.Pod

	client      kubernetes.ClientSet
	statefulSet *appv1.StatefulSet
	localUser   *user.User
	newUser     *user.User
	tlsConfig   *tls.Config
	info        *vkcli.NodeInfo
	cinfo       *vkcli.ClusterNodeInfo
	nodes       vkcli.ClusterNodes
	config      map[string]string

	// TODO: added a flag to indicate valkey-server is not connectable

	logger logr.Logger
}

func (n *ValkeyNode) Definition() *corev1.Pod {
	if n == nil {
		return nil
	}
	return &n.Pod
}

// NewValkeyNode
func NewValkeyNode(ctx context.Context, client kubernetes.ClientSet, sts *appv1.StatefulSet, pod *corev1.Pod, newUser *user.User, logger logr.Logger) (types.ValkeyNode, error) {
	if client == nil {
		return nil, fmt.Errorf("require clientset")
	}
	if sts == nil {
		return nil, fmt.Errorf("require statefulset")
	}
	if pod == nil {
		return nil, fmt.Errorf("require pod")
	}

	node := ValkeyNode{
		Pod:         *pod,
		client:      client,
		statefulSet: sts,
		newUser:     newUser,
		logger:      logger.WithName("ValkeyNode"),
	}

	var err error
	if node.localUser, err = node.loadLocalUser(ctx); err != nil {
		return nil, err
	}
	if node.tlsConfig, err = node.loadTLS(ctx); err != nil {
		return nil, err
	}

	if node.IsContainerReady() {
		vkcli, err := node.getValkeyConnect(ctx, &node)
		if err != nil {
			return nil, err
		}
		defer vkcli.Close()

		// TODO: list the pod status, but added a flag to indicate valkey-server is not connectable,
		// maybe valkey-server blocked or the host node is down
		if node.info, node.cinfo, node.config, node.nodes, err = node.loadValkeyInfo(ctx, &node, vkcli); err != nil {
			return nil, err
		}
	}
	return &node, nil
}

// loadLocalUser
//
// every pod still mount secret to the pod. for acl supported and not supported versions, the difference is that:
// unsupported: the mount secret is the default user secret, which maybe changed
// supported: the mount secret is operator's secret. the operator secret never changes, which is used only internal
//
// this method is used to fetch pod's operator secret
// for versions without acl supported, there exists cases that the env secret not consistent with the server
func (s *ValkeyNode) loadLocalUser(ctx context.Context) (*user.User, error) {
	if s == nil {
		return nil, nil
	}
	logger := s.logger.WithName("loadLocalUser")

	var (
		secretName string
		username   string
	)
	container := util.GetContainerByName(&s.Spec, builder.ServerContainerName)
	if container == nil {
		return nil, fmt.Errorf("server container not found")
	}
	for _, env := range container.Env {
		if env.Name == builder.PasswordEnvName && env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			secretName = env.ValueFrom.SecretKeyRef.LocalObjectReference.Name
		} else if env.Name == builder.OperatorSecretName && env.Value != "" {
			secretName = env.Value
		} else if env.Name == builder.OperatorUsername {
			username = env.Value
		}
	}
	if secretName == "" {
		// COMPAT: for old sentinel version, the secret is mounted to the pod
		for _, vol := range s.Spec.Volumes {
			if vol.Name == "valkey-auth" && vol.Secret != nil {
				secretName = vol.Secret.SecretName
				break
			}
		}
	}

	if secretName != "" {
		if secret, err := s.client.GetSecret(ctx, s.GetNamespace(), secretName); err != nil {
			logger.Error(err, "get user secret failed", "target", util.ObjectKey(s.GetNamespace(), secretName))
			return nil, err
		} else if user, err := user.NewUser(username, user.RoleDeveloper, secret); err != nil {
			return nil, err
		} else {
			return user, nil
		}
	}
	// return default user with out password
	return user.NewUser("", user.RoleDeveloper, nil)
}

func (n *ValkeyNode) loadTLS(ctx context.Context) (*tls.Config, error) {
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

func (n *ValkeyNode) getValkeyConnect(ctx context.Context, node *ValkeyNode) (vkcli.ValkeyClient, error) {
	if n == nil {
		return nil, fmt.Errorf("nil node")
	}
	logger := n.logger.WithName("getValkeyConnect")

	if !n.IsContainerReady() {
		logger.Error(fmt.Errorf("get valkey info failed"), "pod not ready", "target",
			client.ObjectKey{Namespace: node.Namespace, Name: node.Name})
		return nil, fmt.Errorf("node not ready")
	}

	var err error
	addr := net.JoinHostPort(node.DefaultInternalIP().String(), strconv.Itoa(n.InternalPort()))
	for _, user := range []*user.User{node.newUser, node.localUser} {
		if user == nil {
			continue
		}
		name := user.Name
		password := user.Password.String()
		vkcli := vkcli.NewValkeyClient(addr, vkcli.AuthConfig{
			Username:  name,
			Password:  password,
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
			logger.Error(err, "check connection to valkey failed", "address", node.DefaultInternalIP().String())
			return nil, err
		}
		return vkcli, nil
	}
	if err == nil {
		err = fmt.Errorf("no usable account to connect to valkey instance")
	}
	return nil, err
}

// loadValkeyInfo
func (n *ValkeyNode) loadValkeyInfo(ctx context.Context, node *ValkeyNode, vkcli vkcli.ValkeyClient) (info *vkcli.NodeInfo,
	cinfo *vkcli.ClusterNodeInfo, config map[string]string, nodes vkcli.ClusterNodes, err error) {
	// fetch valkey info
	if info, err = vkcli.Info(ctx); err != nil {
		n.logger.Error(err, "load valkey info failed")
		return nil, nil, nil, nil, err
	}
	// fetch current config
	if config, err = vkcli.ConfigGet(ctx, "*"); err != nil {
		n.logger.Error(err, "get valkey config failed, ignore")
	}

	if info.ClusterEnabled == "1" {
		if cinfo, err = vkcli.ClusterInfo(ctx); err != nil {
			n.logger.Error(err, "load valkey cluster info failed")
		}

		// load cluster nodes
		if items, err := vkcli.Nodes(ctx); err != nil {
			n.logger.Error(err, "load valkey cluster nodes info failed, ignore")
		} else {
			// TODO: port this logic to Clean actor
			for _, n := range items {
				// clean disconnected nodes
				if n.LinkState == "disconnected" && strings.Contains(n.RawFlag, "noaddr") {
					if _, err := vkcli.Do(ctx, "CLUSTER", "FORGET", n.Id); err != nil {
						node.logger.Error(err, "forget disconnected node failed", "id", n.Id)
					}
				} else {
					nodes = append(nodes, n)
				}
			}
		}
	}

	return
}

// Refresh not concurrency safe
func (n *ValkeyNode) Refresh(ctx context.Context) (err error) {
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

	localUser, err := n.loadLocalUser(ctx)
	if err != nil {
		return err
	} else {
		n.localUser = localUser
	}

	if n.IsContainerReady() {
		vkcli, err := n.getValkeyConnect(ctx, n)
		if err != nil {
			return err
		}
		defer vkcli.Close()

		if n.info, n.cinfo, n.config, n.nodes, err = n.loadValkeyInfo(ctx, n, vkcli); err != nil {
			n.logger.Error(err, "refresh info failed")
			return err
		}
	}
	return nil
}

// ID get cluster id
//
// TODO: if it's possible generate a const id, use this id as cluster id
func (n *ValkeyNode) ID() string {
	if n == nil || n.nodes == nil {
		return ""
	}
	if n.nodes.Self() != nil {
		return n.nodes.Self().Id
	}
	return ""
}

func (n *ValkeyNode) MasterID() string {
	if n == nil {
		return ""
	}
	if n.nodes.Self() != nil {
		return n.nodes.Self().MasterId
	}
	return ""
}

// IsMasterFailed
func (n *ValkeyNode) IsMasterFailed() bool {
	if n == nil {
		return false
	}
	self := n.nodes.Self()
	if self == nil {
		return false
	}
	if n.Role() == core.NodeRoleMaster {
		return false
	}
	if self.MasterId != "" {
		for _, info := range n.nodes {
			if info.Id == self.MasterId {
				return strings.Contains(info.RawFlag, "fail")
			}
		}
	}
	return false
}

// IsConnected
func (n *ValkeyNode) IsConnected() bool {
	if n == nil {
		return false
	}
	if n.nodes.Self() != nil {
		return n.nodes.Self().LinkState == "connected"
	}
	return false
}

// IsContainerReady
func (n *ValkeyNode) IsContainerReady() bool {
	if n == nil {
		return false
	}

	for _, cond := range n.Pod.Status.ContainerStatuses {
		if cond.Name == builder.ServerContainerName {
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
func (n *ValkeyNode) IsReady() bool {
	if n == nil {
		return false
	}

	for _, cond := range n.Pod.Status.ContainerStatuses {
		if cond.Name == builder.ServerContainerName {
			return cond.Ready
		}
	}
	return false
}

// IsTerminating
func (n *ValkeyNode) IsTerminating() bool {
	if n == nil {
		return false
	}

	return n.DeletionTimestamp != nil
}

// master_link_status is up
func (n *ValkeyNode) IsMasterLinkUp() bool {
	if n == nil || n.info == nil {
		return false
	}

	if n.Role() == core.NodeRoleMaster {
		return true
	}
	return n.info.MasterLinkStatus == "up"
}

// IsJoined
func (n *ValkeyNode) IsJoined() bool {
	if n == nil {
		return false
	}
	brotherCount := 0
	for _, nn := range n.nodes {
		if nn.LinkState == "connected" && !nn.IsSelf() {
			brotherCount += 1
		}
	}
	return (n.nodes.Self() != nil) && (n.nodes.Self().Addr != "") && brotherCount > 0
}

// Slots
func (n *ValkeyNode) Slots() *slot.Slots {
	if n == nil {
		return nil
	}

	role := n.Role()
	if self := n.nodes.Self(); self != nil && role == core.NodeRoleMaster {
		return self.Slots()
	}
	return nil
}

// Index returns the index of the related pod
func (n *ValkeyNode) Index() int {
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

func (n *ValkeyNode) IsACLApplied() bool {
	// check if acl have been applied to container
	container := util.GetContainerByName(&n.Pod.Spec, builder.ServerContainerName)
	for _, env := range container.Env {
		if env.Name == "ACL_CONFIGMAP_NAME" {
			return true
		}
	}
	return false
}

func (n *ValkeyNode) CurrentVersion() version.ValkeyVersion {
	if n == nil {
		return ""
	}

	// parse version from image
	container := util.GetContainerByName(&n.Pod.Spec, builder.ServerContainerName)
	if ver, _ := version.ParseValkeyVersionFromImage(container.Image); ver != version.ValkeyVersionUnknown {
		return ver
	}

	v, _ := version.ParseValkeyVersion(n.info.Version)
	return v
}

func (n *ValkeyNode) Role() core.NodeRole {
	if n == nil || n.info == nil {
		return core.NodeRoleNone
	}
	return types.NewNodeRole(n.info.Role)
}

func (n *ValkeyNode) Config() map[string]string {
	if n == nil || n.config == nil {
		return nil
	}
	return n.config
}

func (n *ValkeyNode) ConfigedMasterIP() string {
	if n == nil || n.info == nil {
		return ""
	}
	return n.info.MasterHost
}

func (n *ValkeyNode) ConfigedMasterPort() string {
	if n == nil || n.info == nil {
		return ""
	}
	return n.info.MasterPort
}

// Setup only return the last command error
func (n *ValkeyNode) Setup(ctx context.Context, margs ...[]any) (err error) {
	if n == nil {
		return nil
	}

	vkcli, err := n.getValkeyConnect(ctx, n)
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

func (n *ValkeyNode) SetACLUser(ctx context.Context, username string, passwords []string, rules string) (interface{}, error) {
	if n == nil {
		return nil, nil
	}

	vkconn, err := n.getValkeyConnect(ctx, n)
	if err != nil {
		return nil, err
	}
	defer vkconn.Close()
	if acluser, err := vkcli.String(vkconn.Do(ctx, "ACL", "whoami")); err != nil {
		return nil, err
	} else if acluser != user.DefaultOperatorUserName {
		return nil, fmt.Errorf("user not operator")
	}

	cmds := [][]interface{}{{"ACL", "SETUSER", username, "reset"}}
	for _, password := range passwords {
		cmds = append(cmds, []interface{}{"ACL", "SETUSER", username, ">" + password})
	}
	if len(passwords) == 0 {
		cmds = append(cmds, []interface{}{"ACL", "SETUSER", username, "nopass"})
	}

	rule_slice := []string{"ACL", "SETUSER", username}
	rule_slice = append(rule_slice, strings.Split(rules, " ")...)
	interfaceSlice := make([]interface{}, len(rule_slice))
	for i, v := range rule_slice {
		interfaceSlice[i] = v
	}
	cmds = append(cmds, interfaceSlice)
	cmds = append(cmds, []interface{}{"ACL", "SETUSER", username, "on"})
	cmds = append(cmds, []interface{}{"ACL", "LIST"})
	cmd_list := []string{}
	args_list := [][]interface{}{}
	for _, cmd := range cmds {
		_cmd, _ := cmd[0].(string)
		cmd_list = append(cmd_list, _cmd)
		args_list = append(args_list, cmd[1:])
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	results, err := vkconn.Tx(ctx, cmd_list, args_list)
	if err != nil {
		return nil, err
	}

	result_list, err := vkcli.Values(results, err)
	if err != nil {
		return results, err
	}
	acl_list := result_list[len(result_list)-1]
	switch acls := acl_list.(type) {
	case []interface{}:
		interfaceSlice := []interface{}{}
		for _, v := range acls {
			r, _err := vkcli.String(v, err)
			if _err != nil {
				return acl_list, _err
			}
			interfaceSlice = append(interfaceSlice, r)
		}
		return interfaceSlice, nil
	default:
		return acl_list, fmt.Errorf("acl list failed")
	}
}

func (n *ValkeyNode) Query(ctx context.Context, cmd string, args ...any) (any, error) {
	if n == nil {
		return nil, nil
	}

	vkcli, err := n.getValkeyConnect(ctx, n)
	if err != nil {
		return nil, err
	}
	defer vkcli.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	return vkcli.Do(ctx, cmd, args...)
}

func (n *ValkeyNode) Info() vkcli.NodeInfo {
	if n == nil || n.info == nil {
		return vkcli.NodeInfo{}
	}
	return *n.info
}

func (n *ValkeyNode) ClusterInfo() vkcli.ClusterNodeInfo {
	if n == nil || n.cinfo == nil {
		return vkcli.ClusterNodeInfo{}
	}
	return *n.cinfo
}

func (n *ValkeyNode) Port() int {
	if value := n.Pod.Labels[builder.AnnouncePortLabelKey]; value != "" {
		if port, _ := strconv.Atoi(value); port > 0 {
			return port
		}
	}
	return n.InternalPort()
}

func (n *ValkeyNode) InternalPort() int {
	port := 6379
	if container := util.GetContainerByName(&n.Pod.Spec, builder.ServerContainerName); container != nil {
		for _, p := range container.Ports {
			if p.Name == builder.ServerContainerName {
				port = int(p.ContainerPort)
				break
			}
		}
	}
	return port
}

func (n *ValkeyNode) DefaultIP() net.IP {
	if value := n.Pod.Labels[builder.AnnounceIPLabelKey]; value != "" {
		address := strings.Replace(value, "-", ":", -1)
		return net.ParseIP(address)
	}
	return n.DefaultInternalIP()
}

func (n *ValkeyNode) DefaultInternalIP() net.IP {
	ips := n.IPs()
	if len(ips) == 0 {
		return nil
	}

	var ipFamilyPrefer string
	if container := util.GetContainerByName(&n.Pod.Spec, builder.ServerContainerName); container != nil {
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

func (n *ValkeyNode) IPort() int {
	if value := n.Pod.Labels[builder.AnnounceIPortLabelKey]; value != "" {
		port, err := strconv.Atoi(value)
		if err == nil {
			return port
		}
	}
	return n.InternalIPort()
}

func (n *ValkeyNode) InternalIPort() int {
	return n.InternalPort() + 10000
}

func (n *ValkeyNode) IPs() []net.IP {
	if n == nil {
		return nil
	}
	ips := []net.IP{}
	for _, podIp := range n.Pod.Status.PodIPs {
		ips = append(ips, net.ParseIP(podIp.IP))
	}
	return ips
}

func (n *ValkeyNode) GetPod() *corev1.Pod {
	return &n.Pod
}

func (n *ValkeyNode) NodeIP() net.IP {
	if n == nil {
		return nil
	}
	return net.ParseIP(n.Pod.Status.HostIP)
}

// ContainerStatus
func (n *ValkeyNode) ContainerStatus() *corev1.ContainerStatus {
	if n == nil {
		return nil
	}
	for _, status := range n.Pod.Status.ContainerStatuses {
		if status.Name == builder.ServerContainerName {
			return &status
		}
	}
	return nil
}

// Status
func (n *ValkeyNode) Status() corev1.PodPhase {
	if n == nil {
		return corev1.PodUnknown
	}
	return n.Pod.Status.Phase
}

func (n *ValkeyNode) ReplicaOf(ctx context.Context, ip, port string) error {
	if n.DefaultIP().String() == ip && strconv.Itoa(n.Port()) == port {
		return nil
	}
	if n.Info().MasterHost == ip && n.Info().MasterPort == port && n.Info().MasterLinkStatus == "up" {
		return nil
	}
	if err := n.Setup(ctx, []interface{}{"slaveof", ip, port}); err != nil {
		return err
	}
	return nil
}
