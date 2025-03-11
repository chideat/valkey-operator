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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strconv"
	"strings"

	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/core/helper"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/aclbuilder"
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
	"github.com/chideat/valkey-operator/internal/util"
	clientset "github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/security/acl"
	"github.com/chideat/valkey-operator/pkg/slot"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/types/user"
	"github.com/chideat/valkey-operator/pkg/version"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_ types.Instance        = (*ValkeyCluster)(nil)
	_ types.ClusterInstance = (*ValkeyCluster)(nil)
)

type ValkeyCluster struct {
	v1alpha1.Cluster

	client        clientset.ClientSet
	eventRecorder record.EventRecorder
	userInsts     []*v1alpha1.User
	shards        []types.ClusterShard
	users         types.Users
	tlsConfig     *tls.Config

	logger logr.Logger
}

// NewValkeyCluster
func NewCluster(ctx context.Context, k8sClient clientset.ClientSet, eventRecorder record.EventRecorder, def *v1alpha1.Cluster, logger logr.Logger) (*ValkeyCluster, error) {
	cluster := ValkeyCluster{
		Cluster: *def,

		client:        k8sClient,
		eventRecorder: eventRecorder,
		logger:        logger.WithName("C").WithValues("instance", client.ObjectKeyFromObject(def).String()),
	}

	var err error
	// load after shard
	if cluster.users, err = cluster.loadUsers(ctx); err != nil {
		cluster.logger.Error(err, "loads users failed")
		return nil, err
	}

	// load tls
	if cluster.tlsConfig, err = cluster.loadTLS(ctx); err != nil {
		cluster.logger.Error(err, "loads tls failed")
		return nil, err
	}

	// load shards
	if cluster.shards, err = LoadValkeyClusterShards(ctx, k8sClient, &cluster, cluster.logger); err != nil {
		cluster.logger.Error(err, "loads cluster shards failed", "cluster", def.Name)
		return nil, err
	}
	cluster.LoadUsers(ctx)

	return &cluster, nil
}

func (c *ValkeyCluster) Arch() core.Arch {
	return core.ValkeyCluster
}

func (c *ValkeyCluster) Issuer() *certmetav1.ObjectReference {
	if c.Spec.Access.EnableTLS {
		return &certmetav1.ObjectReference{
			Name: c.Spec.Access.CertIssuer,
			Kind: c.Spec.Access.CertIssuerType,
		}
	}
	return nil
}

func (c *ValkeyCluster) NamespacedName() client.ObjectKey {
	return client.ObjectKey{Namespace: c.GetNamespace(), Name: c.GetName()}
}

func (c *ValkeyCluster) LoadUsers(ctx context.Context) {
	oldOpUser, _ := c.client.GetUser(ctx, c.GetNamespace(), aclbuilder.GenerateOperatorUserResourceName(c.Arch(), c.GetName()))
	oldDefultUser, _ := c.client.GetUser(ctx, c.GetNamespace(), aclbuilder.GenerateDefaultUserResourceName(c.Arch(), c.GetName()))
	c.userInsts = []*v1alpha1.User{oldOpUser, oldDefultUser}
}

// ctx
func (c *ValkeyCluster) Restart(ctx context.Context, annotationKeyVal ...string) error {
	if c == nil {
		return nil
	}
	for _, shard := range c.shards {
		if err := shard.Restart(ctx); errors.IsNotFound(err) {
			continue
		} else {
			return err
		}
	}
	return nil
}

// Refresh refresh users, shards
func (c *ValkeyCluster) Refresh(ctx context.Context) error {
	if c == nil {
		return nil
	}
	logger := c.logger.WithName("Refresh")

	// load cr
	var cr v1alpha1.Cluster
	if err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		if errors.IsInternalError(err) ||
			errors.IsServerTimeout(err) ||
			errors.IsTimeout(err) ||
			errors.IsTooManyRequests(err) ||
			errors.IsServiceUnavailable(err) {
			return true
		}
		return false
	}, func() error {
		return c.client.Client().Get(ctx, client.ObjectKeyFromObject(&c.Cluster), &cr)
	}); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		logger.Error(err, "get DistributedValkeyCluster failed")
		return err
	}
	// TODO: reset default
	// _ = cr.Default()
	c.Cluster = cr

	var err error
	if c.users, err = c.loadUsers(ctx); err != nil {
		logger.Error(err, "load users failed")
		return err
	}

	if c.shards, err = LoadValkeyClusterShards(ctx, c.client, c, logger); err != nil {
		logger.Error(err, "refresh cluster shards failed", "cluster", c.GetName())
		return err
	}
	return nil
}

// RewriteShards
func (c *ValkeyCluster) RewriteShards(ctx context.Context, shards []*v1alpha1.ClusterShards) error {
	if c == nil || len(shards) == 0 {
		return nil
	}
	logger := c.logger.WithName("RewriteShards")

	if err := c.Refresh(ctx); err != nil {
		return err
	}
	cr := &c.Cluster
	if len(cr.Status.Shards) == 0 || c.IsInService() {
		// only update shards when cluster in service
		cr.Status.Shards = shards
	}
	if err := c.client.UpdateClusterStatus(ctx, cr); err != nil {
		logger.Error(err, "update DistributedValkeyCluster status failed")
		return err
	}
	return c.UpdateStatus(ctx, types.Any, "")
}

func (c *ValkeyCluster) UpdateStatus(ctx context.Context, st types.InstanceStatus, message string) error {
	if c == nil {
		return nil
	}
	logger := c.logger.WithName("UpdateStatus")

	if err := c.Refresh(ctx); err != nil {
		return err
	}

	var (
		cr               = &c.Cluster
		status           v1alpha1.ClusterPhase
		isResourceReady  = (len(c.shards) == int(cr.Spec.Replicas.Shards))
		isRollingRestart = false
		isSlotMigrating  = false
		allSlots         = slot.NewSlots()
		unSchedulePods   []string
		messages         []string
	)
	switch st {
	case types.OK:
		status = v1alpha1.ClusterPhaseReady
	case types.Fail:
		status = v1alpha1.ClusterPhaseFailed
	case types.Paused:
		status = v1alpha1.ClusterPhasePaused
	}
	if message != "" {
		messages = append(messages, message)
	}
	cr.Status.ServiceStatus = v1alpha1.ClusterOutOfService
	if c.IsInService() {
		cr.Status.ServiceStatus = v1alpha1.ClusterInService
	}

__end_slot_migrating__:
	for _, shards := range cr.Status.Shards {
		for _, status := range shards.Slots {
			if status.Status == slot.SlotMigrating.String() || status.Status == slot.SlotImporting.String() {
				isSlotMigrating = true
				break __end_slot_migrating__
			}
		}
	}

	// check if all resources fullfilled
	for i, shard := range c.shards {
		if i != shard.Index() ||
			shard.Status().ReadyReplicas != cr.Spec.Replicas.ReplicasOfShard+1 ||
			len(shard.Replicas()) != int(cr.Spec.Replicas.ReplicasOfShard+1) {
			isResourceReady = false
		}

		if shard.Status().CurrentRevision != shard.Status().UpdateRevision &&
			(*shard.Definition().Spec.Replicas != shard.Status().ReadyReplicas ||
				shard.Status().UpdatedReplicas != shard.Status().Replicas ||
				shard.Status().ReadyReplicas != shard.Status().CurrentReplicas) {
			isRollingRestart = true
		}
		slots := shard.Slots()
		if i < len(cr.Status.Shards) {
			allSlots = allSlots.Union(slots)
		}

		// output message for pending pods
		for _, node := range shard.Nodes() {
			if node.Status() == corev1.PodPending {
				for _, cond := range node.Definition().Status.Conditions {
					if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse {
						unSchedulePods = append(unSchedulePods, node.GetName())
					}
				}
			}
		}
	}
	if len(unSchedulePods) > 0 {
		messages = append(messages, fmt.Sprintf("pods %s unschedulable", strings.Join(unSchedulePods, ",")))
	}

	if cr.Status.ServiceStatus == v1alpha1.ClusterOutOfService && allSlots.Count(slot.SlotAssigned) > 0 {
		subSlots := slot.NewFullSlots().Sub(allSlots)
		messages = append(messages, fmt.Sprintf("slots %s missing", subSlots.String()))
	}

	if status != "" {
		cr.Status.Phase = status
	} else {
		if isRollingRestart {
			cr.Status.Phase = v1alpha1.ClusterPhaseRollingUpdate
		} else if isSlotMigrating {
			cr.Status.Phase = v1alpha1.ClusterPhaseRebalancing
		} else if isResourceReady {
			if cr.Status.ServiceStatus == v1alpha1.ClusterInService {
				cr.Status.Phase = v1alpha1.ClusterPhaseReady
				cr.Status.Message = ""
			}
		} else {
			cr.Status.Phase = v1alpha1.ClusterPhaseCreating
		}
	}
	if cr.Status.Phase == v1alpha1.ClusterPhaseRebalancing {
		var migratingSlots []string
		for _, shards := range cr.Status.Shards {
			for _, status := range shards.Slots {
				if status.Status == slot.SlotMigrating.String() {
					migratingSlots = append(migratingSlots, status.Slots)
				}
			}
		}
		if len(migratingSlots) > 0 {
			message = fmt.Sprintf("slots %s migrating", strings.Join(migratingSlots, ","))
			messages = append(messages, message)
		}
	}
	cr.Status.Message = strings.Join(messages, "; ")

	if cr.Status.Phase == v1alpha1.ClusterPhaseReady &&
		c.Spec.Access.ServiceType == corev1.ServiceTypeNodePort &&
		c.Spec.Access.Ports != "" {

		nodeports := map[int32]struct{}{}
		for _, node := range c.Nodes() {
			if port := node.Definition().Labels[builder.AnnouncePortLabelKey]; port != "" {
				val, _ := strconv.ParseInt(port, 10, 32)
				nodeports[int32(val)] = struct{}{}
			}
		}

		assignedPorts, _ := helper.ParsePorts(cr.Spec.Access.Ports)
		// check nodeport applied
		notAppliedPorts := []string{}
		for _, port := range assignedPorts {
			if _, ok := nodeports[port]; !ok {
				notAppliedPorts = append(notAppliedPorts, fmt.Sprintf("%d", port))
			}
		}
		if len(notAppliedPorts) > 0 {
			cr.Status.Phase = v1alpha1.ClusterPhaseRollingUpdate
			cr.Status.Message = fmt.Sprintf("nodeport %s not applied", notAppliedPorts)
		}
	}

	cr.Status.Nodes = cr.Status.Nodes[0:0]
	var (
		nodePlacement = map[string]struct{}{}
	)

	// update master count and node info
	for _, shard := range c.shards {
		for _, node := range shard.Nodes() {
			if _, ok := nodePlacement[node.NodeIP().String()]; !ok {
				nodePlacement[node.NodeIP().String()] = struct{}{}
			}
			rnode := core.ValkeyNode{
				ID:          node.ID(),
				Role:        node.Role(),
				MasterRef:   node.MasterID(),
				IP:          node.DefaultIP().String(),
				Port:        fmt.Sprintf("%d", node.Port()),
				PodName:     node.GetName(),
				StatefulSet: shard.GetName(),
				NodeName:    node.NodeIP().String(),
				Slots:       node.Slots().String(),
			}
			cr.Status.Nodes = append(cr.Status.Nodes, rnode)
		}
	}

	if err := c.client.UpdateClusterStatus(ctx, cr); errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		logger.Error(err, "get DistributedValkeyCluster failed")
		return err
	}
	return nil
}

// Status return the status of the cluster
func (c *ValkeyCluster) Status() *v1alpha1.ClusterStatus {
	if c == nil {
		return nil
	}
	return &c.Cluster.Status
}

// Definition
func (c *ValkeyCluster) Definition() *v1alpha1.Cluster {
	if c == nil {
		return nil
	}
	return &c.Cluster
}

// Version
func (c *ValkeyCluster) Version() version.ValkeyVersion {
	if c == nil {
		return version.ValkeyVersionUnknown
	}

	if ver, err := version.ParseValkeyVersionFromImage(c.Spec.Image); err != nil {
		c.logger.Error(err, "parse valkey version failed")
		return version.ValkeyVersionUnknown
	} else {
		return ver
	}
}

func (c *ValkeyCluster) Shards() []types.ClusterShard {
	if c == nil {
		return nil
	}
	return c.shards
}

func (c *ValkeyCluster) Nodes() []types.ValkeyNode {
	var ret []types.ValkeyNode
	for _, shard := range c.shards {
		ret = append(ret, shard.Nodes()...)
	}
	return ret
}

func (c *ValkeyCluster) RawNodes(ctx context.Context) ([]corev1.Pod, error) {
	if c == nil {
		return nil, nil
	}

	selector := clusterbuilder.GenerateClusterStatefulSetSelectors(c.GetName(), -1)
	// load pods by statefulset selector
	ret, err := c.client.GetStatefulSetPodsByLabels(ctx, c.GetNamespace(), selector)
	if err != nil {
		c.logger.Error(err, "loads pods of sentinel statefulset failed")
		return nil, err
	}
	return ret.Items, nil
}

func (c *ValkeyCluster) Masters() []types.ValkeyNode {
	var ret []types.ValkeyNode
	for _, shard := range c.shards {
		ret = append(ret, shard.Master())
	}
	return ret
}

// IsInService
func (c *ValkeyCluster) IsInService() bool {
	if c == nil {
		return false
	}

	slots := slot.NewSlots()
	// check is cluster slots is fullfilled
	for _, shard := range c.Shards() {
		slots = slots.Union(shard.Slots())
	}
	return slots.IsFullfilled()
}

// IsReady
func (c *ValkeyCluster) IsReady() bool {
	for _, shard := range c.shards {
		status := shard.Status()
		if !(status.ReadyReplicas == *shard.Definition().Spec.Replicas &&
			((status.CurrentRevision == status.UpdateRevision && status.CurrentReplicas == status.ReadyReplicas) || status.UpdateRevision == "")) {

			return false
		}
	}
	return true
}

func (c *ValkeyCluster) Users() (us types.Users) {
	if c == nil {
		return nil
	}

	// clone before return
	for _, user := range c.users {
		u := *user
		if u.Password != nil {
			p := *u.Password
			u.Password = &p
		}
		us = append(us, &u)
	}
	return
}

func (c *ValkeyCluster) TLSConfig() *tls.Config {
	if c == nil {
		return nil
	}
	return c.tlsConfig
}

// TLS
func (c *ValkeyCluster) TLS() *tls.Config {
	if c == nil {
		return nil
	}
	return c.tlsConfig
}

// loadUsers
func (c *ValkeyCluster) loadUsers(ctx context.Context) (types.Users, error) {
	var (
		name  = aclbuilder.GenerateACLConfigMapName(c.Arch(), c.GetName())
		users types.Users
	)
	// NOTE: load acl config first. if acl config not exists, then this may be
	// an old instance(upgrade from old valkey or operator version).
	// migrate old password account to acl
	if cm, err := c.client.GetConfigMap(ctx, c.GetNamespace(), name); errors.IsNotFound(err) {
		var (
			username       string
			passwordSecret string
			currentSecret  string = c.Spec.Access.DefaultPasswordSecret
			secret         *corev1.Secret
		)

		// load current tls secret.
		// because previous cr not recorded the secret name, we should load it from statefulset
		exists := false
		for i := 0; i < int(c.Spec.Replicas.Shards); i += 2 {
			statefulsetName := clusterbuilder.ClusterStatefulSetName(c.GetName(), i)
			sts, err := c.client.GetStatefulSet(ctx, c.GetNamespace(), statefulsetName)
			if err != nil {
				if !errors.IsNotFound(err) {
					c.logger.Error(err, "load statefulset failed", "target", util.ObjectKey(c.GetNamespace(), c.GetName()))
				}
				continue
			}
			exists = true
			spec := sts.Spec.Template.Spec
			if container := util.GetContainerByName(&spec, builder.ServerContainerName); container != nil {
				for _, env := range container.Env {
					if env.Name == builder.PasswordEnvName && env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
						passwordSecret = env.ValueFrom.SecretKeyRef.LocalObjectReference.Name
					} else if env.Name == builder.OperatorSecretName && env.Value != "" {
						passwordSecret = env.Value
					} else if env.Name == builder.OperatorUsername {
						username = env.Value
					}
				}
			}
			if passwordSecret != currentSecret {
				break
			}
		}
		if !exists {
			username = user.DefaultOperatorUserName
			passwordSecret = aclbuilder.GenerateACLOperatorSecretName(c.Arch(), c.GetName())
		}
		if passwordSecret != "" {
			objKey := client.ObjectKey{Namespace: c.GetNamespace(), Name: passwordSecret}
			if secret, err = c.loadUserSecret(ctx, objKey); err != nil {
				c.logger.Error(err, "load user secret failed", "target", objKey)
				return nil, err
			}
		}
		role := user.RoleDeveloper
		if username == user.DefaultOperatorUserName {
			role = user.RoleOperator
		} else if username == "" {
			username = user.DefaultUserName
		}
		if role == user.RoleOperator {
			if u, err := types.NewOperatorUser(secret); err != nil {
				c.logger.Error(err, "init users failed")
				return nil, err
			} else {
				users = append(users, u)
			}
			u, _ := user.NewUser(user.DefaultUserName, user.RoleDeveloper, nil)
			users = append(users, u)
		} else {
			if u, err := user.NewUser(username, role, secret); err != nil {
				c.logger.Error(err, "init users failed")
				return nil, err
			} else {
				users = append(users, u)
			}
		}
	} else if err != nil {
		c.logger.Error(err, "get acl configmap failed", "name", name)
		return nil, err
	} else if users, err = acl.LoadACLUsers(ctx, c.client, cm); err != nil {
		c.logger.Error(err, "load acl failed")
		return nil, err
	}
	return users, nil
}

// loadUserSecret
func (c *ValkeyCluster) loadUserSecret(ctx context.Context, objKey client.ObjectKey) (*corev1.Secret, error) {
	secret, err := c.client.GetSecret(ctx, objKey.Namespace, objKey.Name)
	if err != nil && !errors.IsNotFound(err) {
		c.logger.Error(err, "load default users's password secret failed", "target", objKey.String())
		return nil, err
	} else if errors.IsNotFound(err) {
		if objKey.Name == aclbuilder.GenerateACLOperatorSecretName(c.Arch(), c.GetName()) {
			secret = aclbuilder.GenerateOperatorSecret(c)
			err := c.client.CreateSecret(ctx, objKey.Namespace, secret)
			if err != nil {
				return nil, err
			}
		}
	} else if _, ok := secret.Data[user.PasswordSecretKey]; !ok {
		return nil, fmt.Errorf("no password found")
	}
	return secret, nil
}

func (c *ValkeyCluster) IsACLUserExists() bool {
	if len(c.userInsts) == 0 {
		return false
	}
	for _, v := range c.userInsts {
		if v == nil {
			return false
		}
		if v.Spec.Username == "" {
			return false
		}
	}
	return true
}

func (c *ValkeyCluster) IsResourceFullfilled(ctx context.Context) (bool, error) {
	var (
		serviceKey = corev1.SchemeGroupVersion.WithKind("Service")
		stsKey     = appsv1.SchemeGroupVersion.WithKind("StatefulSet")
	)
	resources := map[schema.GroupVersionKind][]string{
		serviceKey: {c.GetName()}, // <name>
	}
	for i := 0; i < int(c.Spec.Replicas.Shards); i++ {
		headlessSvcName := clusterbuilder.ClusterHeadlessSvcName(c.GetName(), i)
		resources[serviceKey] = append(resources[serviceKey], headlessSvcName) // <name>-<index>

		stsName := clusterbuilder.ClusterStatefulSetName(c.GetName(), i)
		resources[stsKey] = append(resources[stsKey], stsName)
	}
	if c.Spec.Access.ServiceType == corev1.ServiceTypeLoadBalancer || c.Spec.Access.ServiceType == corev1.ServiceTypeNodePort {
		for i := 0; i < int(c.Spec.Replicas.Shards); i++ {
			stsName := clusterbuilder.ClusterStatefulSetName(c.GetName(), i)
			for j := 0; j < int(c.Spec.Replicas.ReplicasOfShard+1); j++ {
				resources[serviceKey] = append(resources[serviceKey], fmt.Sprintf("%s-%d", stsName, j))
			}
		}
	}

	for gvk, names := range resources {
		for _, name := range names {
			var obj unstructured.Unstructured
			obj.SetGroupVersionKind(gvk)

			err := c.client.Client().Get(ctx, client.ObjectKey{Namespace: c.GetNamespace(), Name: name}, &obj)
			if errors.IsNotFound(err) {
				c.logger.V(3).Info("resource not found", "target", util.ObjectKey(c.GetNamespace(), name))
				return false, nil
			} else if err != nil {
				c.logger.Error(err, "get resource failed", "target", util.ObjectKey(c.GetNamespace(), name))
				return false, err
			}
		}
	}

	for i := 0; i < int(c.Spec.Replicas.Shards); i++ {
		stsName := clusterbuilder.ClusterStatefulSetName(c.GetName(), i)
		sts, err := c.client.GetStatefulSet(ctx, c.GetNamespace(), stsName)
		if err != nil {
			if errors.IsNotFound(err) {
				c.logger.V(3).Info("statefulset not found", "target", util.ObjectKey(c.GetNamespace(), stsName))
				return false, nil
			}
			c.logger.Error(err, "get statefulset failed", "target", util.ObjectKey(c.GetNamespace(), stsName))
			return false, err
		}
		if sts.Spec.Replicas == nil || *sts.Spec.Replicas != c.Spec.Replicas.ReplicasOfShard+1 {
			return false, nil
		}
	}
	return true, nil
}

func (c *ValkeyCluster) IsACLAppliedToAll() bool {
	if c == nil {
		return false
	}
	for _, shard := range c.Shards() {
		for _, node := range shard.Nodes() {
			if !node.IsACLApplied() {
				return false
			}
		}
	}
	return true
}

func (c *ValkeyCluster) Logger() logr.Logger {
	if c == nil {
		return logr.Discard()
	}
	return c.logger
}

func (c *ValkeyCluster) SendEventf(eventtype, reason, messageFmt string, args ...interface{}) {
	if c == nil {
		return
	}
	c.eventRecorder.Eventf(c.Definition(), eventtype, reason, messageFmt, args...)
}

// loadTLS
func (c *ValkeyCluster) loadTLS(ctx context.Context) (*tls.Config, error) {
	if c == nil {
		return nil, nil
	}
	logger := c.logger.WithName("loadTLS")

	var secretName string

	// load current tls secret.
	// because previous cr not recorded the secret name, we should load it from statefulset
	for i := 0; i < int(c.Spec.Replicas.Shards); i += 2 {
		statefulsetName := clusterbuilder.ClusterStatefulSetName(c.GetName(), i)
		if sts, err := c.client.GetStatefulSet(ctx, c.GetNamespace(), statefulsetName); err != nil {
			if !errors.IsNotFound(err) {
				c.logger.Error(err, "load statefulset failed", "target", util.ObjectKey(c.GetNamespace(), c.GetName()))
			}
			continue
		} else {
			for _, vol := range sts.Spec.Template.Spec.Volumes {
				if vol.Name == builder.ValkeyTLSVolumeName {
					secretName = vol.VolumeSource.Secret.SecretName
				}
			}
		}
		break
	}

	if secretName == "" {
		return nil, nil
	}

	if secret, err := c.client.GetSecret(ctx, c.GetNamespace(), secretName); err != nil {
		logger.Error(err, "secret not found", "name", secretName)
		return nil, err
	} else if secret.Data[corev1.TLSCertKey] == nil || secret.Data[corev1.TLSPrivateKeyKey] == nil ||
		secret.Data["ca.crt"] == nil {

		logger.Error(fmt.Errorf("invalid tls secret"), "tls secret is invaid")
		return nil, fmt.Errorf("tls secret is invalid")
	} else {
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
}
