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
	"crypto/tls"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/core/helper"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	databasesv1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
	"github.com/chideat/valkey-operator/internal/builder/failoverbuilder"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/internal/valkey/failover/monitor"
	clientset "github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/security/acl"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/types/user"
	"github.com/chideat/valkey-operator/pkg/version"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ types.FailoverInstance = (*Failover)(nil)

type Failover struct {
	v1alpha1.Failover

	client        clientset.ClientSet
	eventRecorder record.EventRecorder

	users       types.Users
	tlsConfig   *tls.Config
	configmap   map[string]string
	replication types.Replication
	monitor     types.FailoverMonitor

	valkeyUsers []*v1alpha1.User

	logger logr.Logger
}

func NewFailover(ctx context.Context, k8sClient clientset.ClientSet, eventRecorder record.EventRecorder, def *v1alpha1.Failover, logger logr.Logger) (*Failover, error) {
	inst := &Failover{
		Failover: *def,

		client:        k8sClient,
		eventRecorder: eventRecorder,
		configmap:     make(map[string]string),
		logger:        logger.WithName("F").WithValues("instance", client.ObjectKeyFromObject(def).String()),
	}
	var err error
	if inst.users, err = inst.loadUsers(ctx); err != nil {
		inst.logger.Error(err, "load user failed")
		return nil, err
	}
	if inst.tlsConfig, err = inst.loadTLS(ctx); err != nil {
		inst.logger.Error(err, "loads tls failed")
		return nil, err
	}

	if inst.replication, err = LoadValkeyReplication(ctx, k8sClient, inst, inst.logger); err != nil {
		inst.logger.Error(err, "load replicas failed")
		return nil, err
	}
	if inst.monitor, err = monitor.LoadFailoverMonitor(ctx, k8sClient, inst, inst.logger); err != nil {
		inst.logger.Error(err, "load monitor failed")
		return nil, err
	}

	if inst.Version().IsACLSupported() {
		inst.LoadUsers(ctx)
	}
	return inst, nil
}

func (s *Failover) Arch() core.Arch {
	return core.ValkeyFailover
}

func (c *Failover) Issuer() *certmetav1.ObjectReference {
	if c == nil {
		return nil
	}
	if c.Spec.Access.EnableTLS {
		return &certmetav1.ObjectReference{
			Name: c.Spec.Access.CertIssuer,
			Kind: c.Spec.Access.CertIssuerType,
		}
	}
	return nil
}

func (s *Failover) NamespacedName() client.ObjectKey {
	if s == nil {
		return client.ObjectKey{}
	}
	return client.ObjectKey{Namespace: s.GetNamespace(), Name: s.GetName()}
}

func (s *Failover) UpdateStatus(ctx context.Context, st types.InstanceStatus, msg string) error {
	if s == nil {
		return nil
	}

	var (
		err       error
		nodeports = map[int32]struct{}{}
		rs        = lo.IfF(s != nil && s.replication != nil, func() *appsv1.StatefulSetStatus {
			return s.replication.Status()
		}).Else(nil)
		sentinel *v1alpha1.Sentinel
		cr       = s.Definition()
		status   = cr.Status.DeepCopy()
	)

	switch st {
	case types.OK:
		status.Phase = databasesv1.Ready
	case types.Fail:
		status.Phase = databasesv1.Fail
	case types.Paused:
		status.Phase = databasesv1.Paused
	default:
		status.Phase = ""
	}
	status.Message = msg

	if s.IsBindedSentinel() {
		if sentinel, err = s.client.GetSentinel(ctx, s.GetNamespace(), s.GetName()); err != nil && !errors.IsNotFound(err) {
			s.logger.Error(err, "get ValkeySentinel failed")
			return err
		}
	}

	// collect instance statistics
	status.Nodes = status.Nodes[:0]
	for _, node := range s.Nodes() {
		rnode := core.ValkeyNode{
			Role:        node.Role(),
			MasterRef:   node.MasterID(),
			IP:          node.DefaultIP().String(),
			Port:        fmt.Sprintf("%d", node.Port()),
			PodName:     node.GetName(),
			StatefulSet: s.replication.GetName(),
			NodeName:    node.NodeIP().String(),
		}
		status.Nodes = append(status.Nodes, rnode)
		if port := node.Definition().Labels[builder.PodAnnouncePortLabelKey]; port != "" {
			val, _ := strconv.ParseInt(port, 10, 32)
			nodeports[int32(val)] = struct{}{}
		}
	}

	phase, msg := func() (databasesv1.Phase, string) {
		// use passed status if provided
		if status.Phase == databasesv1.Fail || status.Phase == databasesv1.Paused {
			return status.Phase, status.Message
		}

		if sentinel != nil {
			switch sentinel.Status.Phase {
			case databasesv1.SentinelCreating:
				return databasesv1.Creating, sentinel.Status.Message
			case databasesv1.SentinelFail:
				return databasesv1.Fail, sentinel.Status.Message
			}
		}

		// check creating
		if rs == nil || rs.CurrentReplicas != s.Definition().Spec.Replicas ||
			rs.Replicas != s.Definition().Spec.Replicas {
			return databasesv1.Creating, ""
		}

		var pendingPods []string
		// check pending
		for _, node := range s.Nodes() {
			for _, cond := range node.Definition().Status.Conditions {
				if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse {
					pendingPods = append(pendingPods, node.GetName())
				}
			}
		}
		if len(pendingPods) > 0 {
			return databasesv1.Pending, fmt.Sprintf("pods %s pending", strings.Join(pendingPods, ","))
		}

		// check nodeport applied
		if seq := s.Spec.Access.Ports; s.Spec.Access.ServiceType == corev1.ServiceTypeNodePort {
			var (
				notAppliedPorts = []string{}
				customPorts, _  = helper.ParsePorts(seq)
			)
			for _, port := range customPorts {
				if _, ok := nodeports[port]; !ok {
					notAppliedPorts = append(notAppliedPorts, strconv.Itoa(int(port)))
				}
			}
			if len(notAppliedPorts) > 0 {
				return databasesv1.WaitingPodReady, fmt.Sprintf("nodeport %s not applied", strings.Join(notAppliedPorts, ","))
			}
		}

		var notReadyPods []string
		for _, node := range s.Nodes() {
			if !node.IsReady() {
				notReadyPods = append(notReadyPods, node.GetName())
			}
		}
		if len(notReadyPods) > 0 {
			return databasesv1.WaitingPodReady, fmt.Sprintf("pods %s not ready", strings.Join(notReadyPods, ","))
		}

		if isAllMonitored, _ := s.Monitor().AllNodeMonitored(ctx); !isAllMonitored {
			return databasesv1.Creating, "not all nodes monitored"
		}

		// make sure all is ready
		if (rs != nil &&
			rs.ReadyReplicas == s.Definition().Spec.Replicas &&
			rs.CurrentReplicas == rs.ReadyReplicas &&
			rs.CurrentRevision == rs.UpdateRevision) &&
			// sentinel
			(sentinel == nil || sentinel.Status.Phase == databasesv1.SentinelReady) {

			return databasesv1.Ready, ""
		}
		return databasesv1.WaitingPodReady, ""
	}()
	status.Phase, status.Message = phase, lo.If(msg == "", status.Message).Else(msg)

	// update status
	s.Failover.Status = *status
	if err := s.client.UpdateFailoverStatus(ctx, &s.Failover); err != nil {
		s.logger.Error(err, "update Failover status failed")
		return err
	}
	return nil
}

func (s *Failover) Definition() *v1alpha1.Failover {
	if s == nil {
		return nil
	}
	return &s.Failover
}

func (s *Failover) Version() version.ValkeyVersion {
	if s == nil {
		return version.ValkeyVersionUnknown
	}

	if ver, err := version.ParseValkeyVersionFromImage(s.Spec.Image); err != nil {
		s.logger.Error(err, "parse valkey version failed")
		return version.ValkeyVersionUnknown
	} else {
		return ver
	}
}

func (s *Failover) Masters() []types.ValkeyNode {
	if s == nil || s.replication == nil {
		return nil
	}
	var ret []types.ValkeyNode
	for _, v := range s.replication.Nodes() {
		if v.Role() == core.NodeRoleMaster {
			ret = append(ret, v)
		}
	}
	return ret
}

func (s *Failover) Nodes() []types.ValkeyNode {
	if s == nil || s.replication == nil {
		return nil
	}
	return append([]types.ValkeyNode{}, s.replication.Nodes()...)
}

func (s *Failover) RawNodes(ctx context.Context) ([]corev1.Pod, error) {
	if s == nil {
		return nil, nil
	}
	name := failoverbuilder.GetFailoverStatefulSetName(s.GetName())
	sts, err := s.client.GetStatefulSet(ctx, s.GetNamespace(), name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		s.logger.Error(err, "load statefulset failed", "name", name)
		return nil, err
	}
	// load pods by statefulset selector
	ret, err := s.client.GetStatefulSetPodsByLabels(ctx, sts.GetNamespace(), sts.Spec.Selector.MatchLabels)
	if err != nil {
		s.logger.Error(err, "loads pods of shard failed")
		return nil, err
	}
	return ret.Items, nil
}

func (s *Failover) IsInService() bool {
	if s == nil || s.Monitor() == nil {
		return false
	}

	master, err := s.Monitor().Master(context.TODO())
	if err == monitor.ErrNoMaster || err == monitor.ErrMultipleMaster {
		return false
	} else if err != nil {
		s.logger.Error(err, "get master failed")
		return false
	} else if master != nil && master.Flags == "master" {
		return true
	}
	return false
}

func (s *Failover) IsReady() bool {
	if s == nil {
		return false
	}
	if s.Failover.Status.Phase == databasesv1.Ready {
		return true
	}
	return false
}

func (s *Failover) Users() (us types.Users) {
	if s == nil {
		return nil
	}

	// clone before return
	for _, user := range s.users {
		u := *user
		if u.Password != nil {
			p := *u.Password
			u.Password = &p
		}
		us = append(us, &u)
	}
	return
}

func (s *Failover) TLSConfig() *tls.Config {
	if s == nil {
		return nil
	}
	return s.tlsConfig
}

func (s *Failover) Restart(ctx context.Context, annotationKeyVal ...string) error {
	if s == nil || s.replication == nil {
		return nil
	}
	return s.replication.Restart(ctx, annotationKeyVal...)
}

func (s *Failover) Refresh(ctx context.Context) (err error) {
	if s == nil {
		return nil
	}
	logger := s.logger.WithName("Refresh")
	logger.V(3).Info("refreshing sentinel", "target", util.ObjectKey(s.GetNamespace(), s.GetName()))

	// load cr
	var cr v1alpha1.Failover
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
		return s.client.Client().Get(ctx, client.ObjectKeyFromObject(&s.Failover), &cr)
	}); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		logger.Error(err, "get Failover failed")
		return err
	}
	if cr.Name == "" {
		return fmt.Errorf("Failover is nil")
	}
	s.Failover = cr

	if s.users, err = s.loadUsers(ctx); err != nil {
		logger.Error(err, "load users failed")
		return err
	}

	if s.replication, err = LoadValkeyReplication(ctx, s.client, s, logger); err != nil {
		logger.Error(err, "load replicas failed")
		return err
	}
	return nil
}

func (s *Failover) LoadUsers(ctx context.Context) {
	oldOpUser, _ := s.client.GetUser(ctx, s.GetNamespace(), failoverbuilder.GenerateFailoverOperatorsUserName(s.GetName()))
	oldDefultUser, _ := s.client.GetUser(ctx, s.GetNamespace(), failoverbuilder.GenerateFailoverDefaultUserName(s.GetName()))
	s.valkeyUsers = []*v1alpha1.User{oldOpUser, oldDefultUser}
}

func (s *Failover) loadUsers(ctx context.Context) (types.Users, error) {
	var (
		name  = failoverbuilder.GenerateFailoverACLConfigMapName(s.GetName())
		users types.Users
	)

	if s.Version().IsACLSupported() {
		getPassword := func(secretName string) (*user.Password, error) {
			if secret, err := s.loadUserSecret(ctx, client.ObjectKey{
				Namespace: s.GetNamespace(),
				Name:      secretName,
			}); err != nil {
				return nil, err
			} else {
				if password, err := user.NewPassword(secret); err != nil {
					return nil, err
				} else {
					return password, nil
				}
			}
		}
		for _, name := range []string{
			failoverbuilder.GenerateFailoverOperatorsUserName(s.GetName()),
			failoverbuilder.GenerateFailoverDefaultUserName(s.GetName()),
		} {
			if ru, err := s.client.GetUser(ctx, s.GetNamespace(), name); err != nil {
				s.logger.Error(err, "load operator user failed")
				users = nil
				break
			} else {
				var password *user.Password
				if len(ru.Spec.PasswordSecrets) > 0 {
					if password, err = getPassword(ru.Spec.PasswordSecrets[0]); err != nil {
						s.logger.Error(err, "load operator user password failed")
						return nil, err
					}
				}
				if u, err := types.NewUserFromValkeyUser(ru.Spec.Username, ru.Spec.AclRules, password); err != nil {
					s.logger.Error(err, "load operator user failed")
					return nil, err
				} else {
					users = append(users, u)
				}
			}
		}
	}
	if len(users) == 0 {
		if cm, err := s.client.GetConfigMap(ctx, s.GetNamespace(), name); errors.IsNotFound(err) {
			var (
				username       string
				passwordSecret string
				secret         *corev1.Secret
			)
			statefulSetName := failoverbuilder.GetFailoverStatefulSetName(s.GetName())
			sts, err := s.client.GetStatefulSet(ctx, s.GetNamespace(), statefulSetName)
			if err != nil {
				if !errors.IsNotFound(err) {
					s.logger.Error(err, "load statefulset failed", "target", util.ObjectKey(s.GetNamespace(), s.GetName()))
				}
				if s.Version().IsACLSupported() {
					passwordSecret = failoverbuilder.GenerateFailoverACLOperatorSecretName(s.GetName())
					username = user.DefaultOperatorUserName
				}
			} else {
				spec := sts.Spec.Template.Spec
				if container := util.GetContainerByName(&spec, failoverbuilder.ServerContainerName); container != nil {
					for _, env := range container.Env {
						if env.Name == failoverbuilder.PasswordENV && env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
							passwordSecret = env.ValueFrom.SecretKeyRef.LocalObjectReference.Name
						} else if env.Name == failoverbuilder.OperatorSecretName && env.Value != "" {
							passwordSecret = env.Value
						} else if env.Name == failoverbuilder.OperatorUsername {
							username = env.Value
						}
					}
				}
				if passwordSecret == "" {
					// COMPAT: for old sentinel version, the secret is mounted to the pod
					for _, vol := range spec.Volumes {
						if vol.Name == "valkey-auth" && vol.Secret != nil {
							passwordSecret = vol.Secret.SecretName
							break
						}
					}
				}
			}

			if passwordSecret != "" {
				objKey := client.ObjectKey{Namespace: s.GetNamespace(), Name: passwordSecret}
				if secret, err = s.loadUserSecret(ctx, objKey); err != nil {
					s.logger.Error(err, "load user secret failed", "target", objKey)
					return nil, err
				}
			} else if passwordSecret := s.Spec.Access.DefaultPasswordSecret; passwordSecret != "" {
				secret, err = s.client.GetSecret(ctx, s.GetNamespace(), passwordSecret)
				if err != nil {
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
				if u, err := types.NewOperatorUser(secret, s.Version().IsACLSupported()); err != nil {
					s.logger.Error(err, "init users failed")
					return nil, err
				} else {
					users = append(users, u)
				}

				if passwordSecret := s.Spec.Access.DefaultPasswordSecret; passwordSecret != "" {
					secret, err = s.client.GetSecret(ctx, s.GetNamespace(), passwordSecret)
					if err != nil {
						return nil, err
					}
					u, _ := user.NewUser(user.DefaultUserName, user.RoleDeveloper, secret, s.Version().IsACLSupported())
					users = append(users, u)
				} else {
					u, _ := user.NewUser(user.DefaultUserName, user.RoleDeveloper, nil, s.Version().IsACLSupported())
					users = append(users, u)
				}
			} else {
				if u, err := user.NewUser(username, role, secret, s.Version().IsACLSupported()); err != nil {
					s.logger.Error(err, "init users failed")
					return nil, err
				} else {
					users = append(users, u)
				}
			}
		} else if err != nil {
			s.logger.Error(err, "load default users's password secret failed", "target", util.ObjectKey(s.GetNamespace(), name))
			return nil, err
		} else if users, err = acl.LoadACLUsers(ctx, s.client, cm); err != nil {
			s.logger.Error(err, "load acl failed")
			return nil, err
		}
	}

	var (
		defaultUser = users.GetDefaultUser()
		rule        *user.Rule
	)
	if len(defaultUser.Rules) > 0 {
		rule = defaultUser.Rules[0]
	} else {
		rule = &user.Rule{}
	}
	if s.Version().IsACLSupported() {
		rule.Channels = []string{"*"}
	}

	renameVal := s.Definition().Spec.CustomConfigs[failoverbuilder.ValkeyConfig_RenameCommand]
	renames, _ := clusterbuilder.ParseRenameConfigs(renameVal)
	if len(renameVal) > 0 {
		rule.DisallowedCommands = []string{}
		for key, val := range renames {
			if key != val && !slices.Contains(rule.DisallowedCommands, key) {
				rule.DisallowedCommands = append(rule.DisallowedCommands, key)
			}
		}
	}
	defaultUser.Rules = append(defaultUser.Rules[0:0], rule)

	return users, nil
}

func (s *Failover) loadUserSecret(ctx context.Context, objKey client.ObjectKey) (*corev1.Secret, error) {
	secret, err := s.client.GetSecret(ctx, objKey.Namespace, objKey.Name)
	if err != nil && !errors.IsNotFound(err) {
		s.logger.Error(err, "load default users's password secret failed", "target", objKey.String())
		return nil, err
	} else if errors.IsNotFound(err) {
		secret = failoverbuilder.NewFailoverOpSecret(s.Definition())
		err := s.client.CreateSecret(ctx, objKey.Namespace, secret)
		if err != nil {
			return nil, err
		}
	} else if _, ok := secret.Data[user.PasswordSecretKey]; !ok {
		return nil, fmt.Errorf("no password found")
	}
	return secret, nil
}

func (s *Failover) loadTLS(ctx context.Context) (*tls.Config, error) {
	if s == nil {
		return nil, nil
	}
	logger := s.logger.WithName("loadTLS")

	secretName := s.Status.TLSSecret
	if secretName == "" {
		// load current tls secret.
		// because previous cr not recorded the secret name, we should load it from statefulset
		stsName := failoverbuilder.GetFailoverStatefulSetName(s.GetName())
		if sts, err := s.client.GetStatefulSet(ctx, s.GetNamespace(), stsName); err != nil {
			s.logger.Error(err, "load statefulset failed", "target", util.ObjectKey(s.GetNamespace(), s.GetName()))
		} else {
			for _, vol := range sts.Spec.Template.Spec.Volumes {
				if vol.Name == failoverbuilder.ValkeyTLSVolumeName {
					secretName = vol.VolumeSource.Secret.SecretName
				}
			}
		}
	}
	if secretName == "" {
		return nil, nil
	}
	if secret, err := s.client.GetSecret(ctx, s.GetNamespace(), secretName); err != nil {
		logger.Error(err, "secret not found", "name", secretName)
		return nil, err
	} else if cert, err := util.LoadCertConfigFromSecret(secret); err != nil {
		logger.Error(err, "load cert config failed")
		return nil, err
	} else {
		return cert, nil
	}
}

func (s *Failover) IsBindedSentinel() bool {
	if s == nil || s.Failover.Spec.Sentinel == nil {
		return false
	}
	return s.Failover.Spec.Sentinel.SentinelReference == nil
}

func (s *Failover) Selector() map[string]string {
	if s == nil {
		return nil
	}
	if s.replication != nil && s.replication.Definition() != nil {
		return s.replication.Definition().Spec.Selector.MatchLabels
	}
	return nil
}

func (s *Failover) IsACLUserExists() bool {
	if !s.Version().IsACLSupported() {
		return false
	}
	if len(s.valkeyUsers) == 0 {
		return false
	}
	for _, v := range s.valkeyUsers {
		if v == nil {
			return false
		}
	}
	return true
}

func (s *Failover) IsResourceFullfilled(ctx context.Context) (bool, error) {
	var (
		serviceKey  = corev1.SchemeGroupVersion.WithKind("Service")
		stsKey      = appsv1.SchemeGroupVersion.WithKind("StatefulSet")
		sentinelKey = databasesv1.GroupVersion.WithKind("ValkeySentinel")
	)
	resources := map[schema.GroupVersionKind][]string{
		serviceKey: {
			failoverbuilder.GetFailoverStatefulSetName(s.GetName()), // rfr-<name>
			failoverbuilder.GetValkeyROServiceName(s.GetName()),     // rfr-<name>-read-only
			failoverbuilder.GetValkeyRWServiceName(s.GetName()),     // rfr-<name>-read-write
		},
		stsKey: {
			failoverbuilder.GetFailoverStatefulSetName(s.GetName()),
		},
	}
	if s.IsBindedSentinel() {
		resources[sentinelKey] = []string{s.GetName()}
	}

	if s.Spec.Access.ServiceType == corev1.ServiceTypeLoadBalancer ||
		s.Spec.Access.ServiceType == corev1.ServiceTypeNodePort {
		for i := 0; i < int(s.Spec.Replicas); i++ {
			stsName := failoverbuilder.GetFailoverStatefulSetName(s.GetName())
			resources[serviceKey] = append(resources[serviceKey], fmt.Sprintf("%s-%d", stsName, i))
		}
	}

	for gvk, names := range resources {
		for _, name := range names {
			var obj unstructured.Unstructured
			obj.SetGroupVersionKind(gvk)

			err := s.client.Client().Get(ctx, client.ObjectKey{Namespace: s.GetNamespace(), Name: name}, &obj)
			if errors.IsNotFound(err) {
				s.logger.V(3).Info("resource not found", "kind", gvk.Kind, "target", util.ObjectKey(s.GetNamespace(), name))
				return false, nil
			} else if err != nil {
				s.logger.Error(err, "get resource failed", "kind", gvk.Kind, "target", util.ObjectKey(s.GetNamespace(), name))
				return false, err
			}
		}
	}

	if !s.IsStandalone() {
		// check sentinel
		newSen := failoverbuilder.NewFailoverSentinel(s)
		oldSen, err := s.client.GetSentinel(ctx, s.GetNamespace(), s.GetName())
		if errors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			s.logger.Error(err, "get sentinel failed", "target", client.ObjectKeyFromObject(newSen))
			return false, err
		}
		if !reflect.DeepEqual(newSen.Spec, oldSen.Spec) ||
			!reflect.DeepEqual(newSen.Labels, oldSen.Labels) ||
			!reflect.DeepEqual(newSen.Annotations, oldSen.Annotations) {
			oldSen.Spec = newSen.Spec
			oldSen.Labels = newSen.Labels
			oldSen.Annotations = newSen.Annotations
			return false, nil
		}
	}
	return true, nil
}

func (s *Failover) IsACLAppliedToAll() bool {
	if s == nil || !s.Version().IsACLSupported() {
		return false
	}
	for _, node := range s.Nodes() {
		if !node.CurrentVersion().IsACLSupported() || !node.IsACLApplied() {
			return false
		}
	}
	return true
}

func (c *Failover) Logger() logr.Logger {
	if c == nil {
		return logr.Discard()
	}
	return c.logger
}

func (c *Failover) SendEventf(eventtype, reason, messageFmt string, args ...interface{}) {
	if c == nil {
		return
	}
	c.eventRecorder.Eventf(c.Definition(), eventtype, reason, messageFmt, args...)
}

func (s *Failover) Monitor() types.FailoverMonitor {
	if s == nil {
		return nil
	}
	return s.monitor
}

func (c *Failover) IsStandalone() bool {
	if c == nil {
		return false
	}
	return c.Failover.Spec.Sentinel == nil
}
