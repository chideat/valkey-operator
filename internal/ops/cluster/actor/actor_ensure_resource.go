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

package actor

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/core/helper"
	"github.com/chideat/valkey-operator/internal/actor"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/certbuilder"
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
	"github.com/chideat/valkey-operator/internal/builder/sabuilder"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/chideat/valkey-operator/internal/ops/cluster"
	cops "github.com/chideat/valkey-operator/internal/ops/cluster"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ actor.Actor = (*actorEnsureResource)(nil)

func init() {
	actor.Register(core.ValkeyCluster, NewEnsureResourceActor)
}

func NewEnsureResourceActor(client kubernetes.ClientSet, logger logr.Logger) actor.Actor {
	return &actorEnsureResource{
		client: client,
		logger: logger,
	}
}

type actorEnsureResource struct {
	client kubernetes.ClientSet
	logger logr.Logger
}

func (a *actorEnsureResource) SupportedCommands() []actor.Command {
	return []actor.Command{cluster.CommandEnsureResource}
}

func (a *actorEnsureResource) Version() *semver.Version {
	return semver.MustParse("0.1.0")
}

// Do
func (a *actorEnsureResource) Do(ctx context.Context, val types.Instance) *actor.ActorResult {
	logger := val.Logger().WithValues("actor", cops.CommandEnsureResource.String())
	cluster := val.(types.ClusterInstance)

	if cluster.Definition().Spec.PodAnnotations[builder.PauseAnnotationKey] != "" {
		if ret := a.pauseStatefulSet(ctx, cluster, logger); ret != nil {
			return ret
		}
		return actor.NewResult(cops.CommandPaused)
	}

	if ret := a.ensureServiceAccount(ctx, cluster, logger); ret != nil {
		return ret
	}

	if ret := a.ensureService(ctx, cluster, logger); ret != nil {
		return ret
	}

	if ret := a.ensureTLS(ctx, cluster, logger); ret != nil {
		return ret
	}

	if ret := a.ensureConfigMap(ctx, cluster, logger); ret != nil {
		return ret
	}

	if ret := a.ensureStatefulset(ctx, cluster, logger); ret != nil {
		return ret
	}
	return nil
}

func (a *actorEnsureResource) pauseStatefulSet(ctx context.Context, cluster types.ClusterInstance, logger logr.Logger) *actor.ActorResult {
	cr := cluster.Definition()
	labels := clusterbuilder.GenerateClusterLabels(cr.Name, nil)
	stss, err := a.client.ListStatefulSetByLabels(ctx, cr.Namespace, labels)
	if err != nil {
		logger.Error(err, "load statefulsets failed")
		return actor.RequeueWithError(err)
	}
	if len(stss.Items) == 0 {
		return nil
	}

	pausedCount := 0
	for _, item := range stss.Items {
		if *item.Spec.Replicas == 0 {
			continue
		}
		sts := item.DeepCopy()
		*sts.Spec.Replicas = 0
		if err = a.client.UpdateStatefulSet(ctx, cr.Namespace, sts); err != nil {
			return actor.RequeueWithError(err)
		}
		pausedCount += 1
	}

	if pausedCount > 0 {
		cluster.SendEventf(corev1.EventTypeNormal, config.EventPause, "paused instance statefulsets")
	}
	return nil
}

// ensureServiceAccount
func (a *actorEnsureResource) ensureServiceAccount(ctx context.Context, cluster types.ClusterInstance, logger logr.Logger) *actor.ActorResult {
	cr := cluster.Definition()

	sa := sabuilder.GenerateServiceAccount(cr)
	role := sabuilder.GenerateRole(cr)
	binding := sabuilder.GenerateRoleBinding(cr)
	clusterRole := sabuilder.GenerateClusterRole(cr)
	clusterBinding := sabuilder.GenerateClusterRoleBinding(cr)

	if err := a.client.CreateOrUpdateServiceAccount(ctx, cluster.GetNamespace(), sa); err != nil {
		logger.Error(err, "create serviceaccount failed", "target", client.ObjectKeyFromObject(sa))
		return actor.RequeueWithError(err)
	}
	if err := a.client.CreateOrUpdateRole(ctx, cluster.GetNamespace(), role); err != nil {
		return actor.RequeueWithError(err)
	}
	if err := a.client.CreateOrUpdateRoleBinding(ctx, cluster.GetNamespace(), binding); err != nil {
		return actor.RequeueWithError(err)
	}
	if err := a.client.CreateOrUpdateClusterRole(ctx, clusterRole); err != nil {
		return actor.RequeueWithError(err)
	}
	if oldClusterRb, err := a.client.GetClusterRoleBinding(ctx, clusterBinding.Name); err != nil {
		if errors.IsNotFound(err) {
			if err := a.client.CreateClusterRoleBinding(ctx, clusterBinding); err != nil {
				return actor.RequeueWithError(err)
			}
		} else {
			return actor.RequeueWithError(err)
		}
	} else {
		exists := false
		for _, sub := range oldClusterRb.Subjects {
			if sub.Namespace == cluster.GetNamespace() {
				exists = true
			}
		}
		if !exists && len(oldClusterRb.Subjects) > 0 {
			oldClusterRb.Subjects = append(oldClusterRb.Subjects,
				rbacv1.Subject{Kind: "ServiceAccount",
					Name:      sabuilder.ValkeyInstanceServiceAccountName,
					Namespace: cluster.GetNamespace()},
			)
			err := a.client.CreateOrUpdateClusterRoleBinding(ctx, oldClusterRb)
			if err != nil {
				return actor.RequeueWithError(err)
			}
		}
	}
	return nil
}

// ensureConfigMap
func (a *actorEnsureResource) ensureConfigMap(ctx context.Context, cluster types.ClusterInstance, logger logr.Logger) *actor.ActorResult {
	cm, err := clusterbuilder.NewConfigMapForCR(cluster)
	if err != nil {
		logger.Error(err, "new configmap failed")
		// this should not return errors, else there must be a breaking error
		return actor.NewResultWithError(cops.CommandAbort, err)
	}
	if oldCm, err := a.client.GetConfigMap(ctx, cluster.GetNamespace(), cm.GetName()); errors.IsNotFound(err) {
		if err := a.client.CreateConfigMap(ctx, cluster.GetNamespace(), cm); err != nil {
			logger.Error(err, "update configmap failed")
			return actor.RequeueWithError(err)
		}
	} else if !reflect.DeepEqual(oldCm.Data, cm.Data) {
		if err := a.client.UpdateConfigMap(ctx, cluster.GetNamespace(), cm); err != nil {
			return actor.RequeueWithError(err)
		}
	}
	return nil
}

// ensureTLS
func (a *actorEnsureResource) ensureTLS(ctx context.Context, cluster types.ClusterInstance, logger logr.Logger) *actor.ActorResult {
	if !cluster.Definition().Spec.Access.EnableTLS {
		return nil
	}
	dnsNames := []string{
		cluster.GetName(),
		fmt.Sprintf("%s.%s", cluster.GetName(), cluster.GetNamespace()),
	}
	for i := 0; i < int(cluster.Definition().Spec.Replicas.Shards); i++ {
		name := clusterbuilder.ClusterHeadlessSvcName(cluster.GetName(), i)
		dnsNames = append(dnsNames, name)
		dnsNames = append(dnsNames, fmt.Sprintf("%s.%s", name, cluster.GetNamespace()))
	}
	cc, err := certbuilder.NewCertificate(cluster, dnsNames, clusterbuilder.GenerateClusterLabels(cluster.GetName(), nil))
	if err != nil {
		logger.Error(err, "generate certificate failed")
		return actor.NewResultWithError(cops.CommandAbort, err)
	}

	oldCc, err := a.client.GetCertificate(ctx, cluster.GetNamespace(), cc.GetName())
	if err != nil && !errors.IsNotFound(err) {
		return actor.RequeueWithError(err)
	}

	if err := a.client.CreateIfNotExistsCertificate(ctx, cluster.GetNamespace(), cc); err != nil {
		logger.Error(err, "request for certificate failed")
		return actor.RequeueWithError(err)
	}

	var (
		found      = false
		secretName = certbuilder.GenerateSSLSecretName(cluster.GetName())
	)
	for i := 0; i < 5; i++ {
		time.Sleep(time.Second * time.Duration(i))

		if secret, _ := a.client.GetSecret(ctx, cluster.GetNamespace(), secretName); secret != nil {
			found = true
			break
		}

		// check when the certificate created
		if oldCc != nil && time.Since(oldCc.GetCreationTimestamp().Time) > time.Minute*5 {
			return actor.NewResultWithError(cops.CommandAbort, fmt.Errorf("issue for tls certificate failed, please check the cert-manager"))
		}
	}
	if !found {
		return actor.NewResult(cops.CommandRequeue)
	}
	return nil
}

// ensureStatefulset
func (a *actorEnsureResource) ensureStatefulset(ctx context.Context, cluster types.ClusterInstance, logger logr.Logger) *actor.ActorResult {
	cr := cluster.Definition()

	var (
		updated = false
	)

	for i := 0; i < int(cr.Spec.Replicas.Shards); i++ {
		// statefulset
		name := clusterbuilder.ClusterStatefulSetName(cr.GetName(), i)

		pdb := clusterbuilder.GeneratePodDisruptionBudget(cluster, i)
		if oldPdb, err := a.client.GetPodDisruptionBudget(ctx, cr.GetNamespace(), pdb.Name); errors.IsNotFound(err) {
			if err = a.client.CreatePodDisruptionBudget(ctx, cr.GetNamespace(), pdb); err != nil {
				logger.Error(err, "create poddisruptionbudget failed", "target", client.ObjectKeyFromObject(pdb))
				return actor.RequeueWithError(err)
			}
		} else if err != nil {
			logger.Error(err, "get poddisruptionbudget failed", "target", client.ObjectKeyFromObject(pdb))
			return actor.RequeueWithError(err)
		} else if !reflect.DeepEqual(oldPdb.Spec, pdb.Spec) {
			pdb.ResourceVersion = oldPdb.ResourceVersion
			if err = a.client.UpdatePodDisruptionBudget(ctx, cr.GetNamespace(), pdb); err != nil {
				logger.Error(err, "update poddisruptionbudget failed", "target", client.ObjectKeyFromObject(pdb))
				return actor.RequeueWithError(err)
			}
		}

		newSts, err := clusterbuilder.GenerateStatefulSet(cluster, i)
		if err != nil {
			logger.Error(err, "generate statefulset failed")
			return actor.NewResultWithError(cops.CommandAbort, err)
		}

		if oldSts, err := a.client.GetStatefulSet(ctx, cr.GetNamespace(), name); errors.IsNotFound(err) {
			if err := a.client.CreateStatefulSet(ctx, cr.GetNamespace(), newSts); err != nil {
				logger.Error(err, "create statefulset failed")
				return actor.NewResultWithError(cops.CommandAbort, err)
			}
			updated = true
		} else if err != nil {
			logger.Error(err, "get statefulset failed")
			return actor.RequeueWithError(err)
		} else {
			// overwrite persist fields
			// keep old affinity for topolvm cases
			// TODO: remove in future
			newSts.Spec.Template.Spec.Affinity = oldSts.Spec.Template.Spec.Affinity

			// keep old selector for upgrade
			if !reflect.DeepEqual(oldSts.Spec.Selector.MatchLabels, newSts.Spec.Selector.MatchLabels) {
				newSts.Spec.Selector.MatchLabels = oldSts.Spec.Selector.MatchLabels
			}
			// keep old pvc
			newSts.Spec.VolumeClaimTemplates = oldSts.Spec.VolumeClaimTemplates

			// merge restart annotations, if statefulset is more new, not restart statefulset
			newSts.Spec.Template.Annotations = MergeAnnotations(newSts.Spec.Template.Annotations, oldSts.Spec.Template.Annotations)

			if util.IsStatefulsetChanged(newSts, oldSts, logger) {
				if err := a.client.UpdateStatefulSet(ctx, cr.GetNamespace(), newSts); err != nil {
					logger.Error(err, "update statefulset failed", "target", client.ObjectKeyFromObject(newSts))
					return actor.RequeueWithError(err)
				}
				updated = true
			}
		}
	}
	if updated {
		return actor.NewResult(cops.CommandRequeue)
	}
	return nil
}

// ensureService ensure other services
func (a *actorEnsureResource) ensureService(ctx context.Context, cluster types.ClusterInstance, logger logr.Logger) *actor.ActorResult {
	cr := cluster.Definition()

	for i := 0; i < int(cr.Spec.Replicas.Shards); i++ {
		// init headless service
		// use serviceName and selectors from statefulset
		svc := clusterbuilder.GenerateHeadlessService(cr, i)
		// TODO: check if service changed
		if err := a.client.CreateIfNotExistsService(ctx, cr.GetNamespace(), svc); err != nil {
			logger.Error(err, "create headless service failed", "target", client.ObjectKeyFromObject(svc))
		}
	}

	svc := clusterbuilder.GenerateInstanceService(cr)
	if err := a.client.CreateIfNotExistsService(ctx, cr.GetNamespace(), svc); err != nil {
		logger.Error(err, "create service failed", "target", client.ObjectKeyFromObject(svc))
		return actor.RequeueWithError(err)
	}

	if ret := a.cleanUselessService(ctx, cluster, logger); ret != nil {
		return ret
	}
	switch cr.Spec.Access.ServiceType {
	case corev1.ServiceTypeNodePort:
		if err := a.ensureValkeyNodePortService(ctx, cluster, logger); err != nil {
			return err
		}
	case corev1.ServiceTypeLoadBalancer:
		if ret := a.ensureValkeyPodService(ctx, cluster, logger); ret != nil {
			return ret
		}
	}
	return nil
}

func (a *actorEnsureResource) ensureValkeyNodePortService(ctx context.Context, cluster types.ClusterInstance, logger logr.Logger) *actor.ActorResult {
	if cluster.Definition().Spec.Access.ServiceType != corev1.ServiceTypeNodePort {
		return nil
	}

	if cluster.Definition().Spec.Access.Ports == "" {
		return a.ensureValkeyPodService(ctx, cluster, logger)
	}

	cr := cluster.Definition()
	configedPorts, err := helper.ParsePorts(cr.Spec.Access.Ports)
	if err != nil {
		return actor.RequeueWithError(err)
	}
	getClientPort := func(svc *corev1.Service) int32 {
		if port := util.GetServicePortByName(svc, "client"); port != nil {
			return port.NodePort
		}
		return 0
	}
	getGossipPort := func(svc *corev1.Service) int32 {
		if port := util.GetServicePortByName(svc, "gossip"); port != nil {
			return port.NodePort
		}
		return 0
	}

	serviceNameRange := map[string]struct{}{}
	for shard := 0; shard < int(cr.Spec.Replicas.Shards); shard++ {
		for replica := 0; replica < int(cr.Spec.Replicas.ReplicasOfShard); replica++ {
			serviceName := clusterbuilder.ClusterNodeServiceName(cr.Name, shard, replica)
			serviceNameRange[serviceName] = struct{}{}
		}
	}

	labels := clusterbuilder.GenerateClusterLabels(cr.Name, nil)

	// the whole process is divided into three steps:
	// 1. delete service not in nodeport range
	// 2. create new service without gossip port
	// 3. update existing service and restart pod (only one pod is restarted at a same time for each shard)
	// 4. check again if all gossip port is added

	services, ret := a.fetchAllPodBindedServices(ctx, cr.Namespace, labels)
	if ret != nil {
		return ret
	}

	// 1. delete service not in nodeport range
	//
	// when pod not exists and service not in nodeport range, delete service
	// NOTE: only delete service whose pod is not found
	//       let statefulset auto scale up/down for pods
	for _, svc := range services {
		svc := svc.DeepCopy()
		if _, exists := serviceNameRange[svc.Name]; !exists || !slices.Contains(configedPorts, getClientPort(svc)) {
			_, err := a.client.GetPod(ctx, svc.Namespace, svc.Name)
			if errors.IsNotFound(err) {
				logger.Info("release nodeport service", "service", svc.Name, "port", getClientPort(svc))
				if err = a.client.DeleteService(ctx, svc.Namespace, svc.Name); err != nil {
					return actor.RequeueWithError(err)
				}
			} else if err != nil {
				logger.Error(err, "get pods failed", "target", client.ObjectKeyFromObject(svc))
				return actor.RequeueWithError(err)
			}
		}
	}

	if services, ret = a.fetchAllPodBindedServices(ctx, cr.Namespace, labels); ret != nil {
		return ret
	}

	// 2. create new service
	var (
		newPorts                 []int32
		bindedNodeports          []int32
		needUpdateServices       []*corev1.Service
		needUpdateGossipServices []*corev1.Service
	)
	for _, svc := range services {
		svc := svc.DeepCopy()
		bindedNodeports = append(bindedNodeports, getClientPort(svc), getGossipPort(svc))
	}
	// filter used ports
	for _, port := range configedPorts {
		if !slices.Contains(bindedNodeports, port) {
			newPorts = append(newPorts, port)
		}
	}
	for shard := range int(cr.Spec.Replicas.Shards) {
		for replica := range int(cr.Spec.Replicas.ReplicasOfShard) {
			serviceName := clusterbuilder.ClusterNodeServiceName(cr.Name, shard, replica)
			oldService, err := a.client.GetService(ctx, cr.Namespace, serviceName)
			if errors.IsNotFound(err) {
				if len(newPorts) == 0 {
					continue
				}
				port := newPorts[0]
				svc := clusterbuilder.GenerateNodePortSerivce(cr, serviceName, labels, port)
				if err = a.client.CreateService(ctx, svc.Namespace, svc); err != nil {
					a.logger.Error(err, "create nodeport service failed", "target", client.ObjectKeyFromObject(svc))
					return actor.NewResultWithValue(cops.CommandRequeue, err)
				}
				newPorts = newPorts[1:]
				continue
			} else if err != nil {
				return actor.RequeueWithError(err)
			}

			// check old service for compatibility
			if len(oldService.OwnerReferences) == 0 || oldService.OwnerReferences[0].Kind == "Pod" {
				oldService.OwnerReferences = util.BuildOwnerReferences(cr)
				if err := a.client.UpdateService(ctx, oldService.Namespace, oldService); err != nil {
					a.logger.Error(err, "update nodeport service failed", "target", client.ObjectKeyFromObject(oldService))
					return actor.NewResultWithValue(cops.CommandRequeue, err)
				}
			}
			if slices.Contains(configedPorts, getGossipPort(oldService)) {
				needUpdateGossipServices = append(needUpdateGossipServices, oldService)
			} else if port := getClientPort(oldService); port != 0 && !slices.Contains(configedPorts, port) {
				needUpdateServices = append(needUpdateServices, oldService)
			}
		}
	}

	// 3. update existing service and restart pod (only one pod is restarted at a same time for each shard)
	if len(needUpdateServices) > 0 && len(newPorts) > 0 {
		port, svc := newPorts[0], needUpdateServices[0]
		if sp := util.GetServicePortByName(svc, "client"); sp != nil {
			sp.NodePort = port
		}

		// NOTE: here not make sure the failover success, because the nodeport updated, the communication will be failed
		//       in k8s, the nodeport can still access for sometime after the nodeport updated
		//
		// update service
		if err = a.client.UpdateService(ctx, svc.Namespace, svc); err != nil {
			a.logger.Error(err, "update nodeport service failed", "target", client.ObjectKeyFromObject(svc), "port", port)
			return actor.NewResultWithValue(cops.CommandRequeue, err)
		}
		if pod, _ := a.client.GetPod(ctx, cr.Namespace, svc.Spec.Selector[builder.PodNameLabelKey]); pod != nil {
			if err := a.client.DeletePod(ctx, cr.Namespace, pod.Name); err != nil {
				return actor.RequeueWithError(err)
			}
			return actor.NewResult(cops.CommandRequeue)
		}
	}
	for _, svc := range needUpdateGossipServices {
		ports := svc.Spec.Ports[0:0]
		for _, port := range svc.Spec.Ports {
			if port.Name == "client" {
				ports = append(ports, port)
				break
			}
		}
		svc.Spec.Ports = ports
		if err = a.client.UpdateService(ctx, svc.Namespace, svc); err != nil {
			a.logger.Error(err, "update nodeport service failed", "target", client.ObjectKeyFromObject(svc))
			return actor.NewResultWithValue(cops.CommandRequeue, err)
		}
		// update gossip service
		svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{Name: "gossip", Port: 16379})
		if err = a.client.UpdateService(ctx, svc.Namespace, svc); err != nil {
			a.logger.Error(err, "update nodeport service failed", "target", client.ObjectKeyFromObject(svc))
			return actor.NewResultWithValue(cops.CommandRequeue, err)
		}
		if pod, _ := a.client.GetPod(ctx, cr.Namespace, svc.Spec.Selector[builder.PodNameLabelKey]); pod != nil {
			if err := a.client.DeletePod(ctx, cr.Namespace, pod.Name); err != nil {
				a.logger.Error(err, "delete pod failed", "target", client.ObjectKeyFromObject(pod))
				return actor.RequeueWithError(err)
			}
			return actor.NewResult(cops.CommandRequeue)
		}
	}

	// 4. check again if all gossip port is added
	for shard := 0; shard < int(cr.Spec.Replicas.Shards); shard++ {
		for replica := 0; replica < int(cr.Spec.Replicas.ReplicasOfShard); replica++ {
			serviceName := clusterbuilder.ClusterNodeServiceName(cr.Name, shard, replica)
			if svc, err := a.client.GetService(ctx, cr.Namespace, serviceName); errors.IsNotFound(err) {
				continue
			} else if err != nil {
				a.logger.Error(err, "get service failed", "target", util.ObjectKey(cr.Namespace, serviceName))
			} else if port := getGossipPort(svc); port == 0 && len(svc.Spec.Ports) == 1 {
				// update gossip service
				svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{Name: "gossip", Port: 16379})
				if err = a.client.UpdateService(ctx, svc.Namespace, svc); err != nil {
					a.logger.Error(err, "update nodeport service failed", "target", client.ObjectKeyFromObject(svc))
					return actor.NewResultWithValue(cops.CommandRequeue, err)
				}
				if pod, _ := a.client.GetPod(ctx, cr.Namespace, svc.Spec.Selector[builder.PodNameLabelKey]); pod != nil {
					if err := a.client.DeletePod(ctx, cr.Namespace, pod.Name); err != nil {
						a.logger.Error(err, "delete pod failed", "target", client.ObjectKeyFromObject(pod))
						return actor.RequeueWithError(err)
					}
					return actor.NewResult(cops.CommandRequeue)
				}
			}
		}
	}
	return nil
}

func (a *actorEnsureResource) ensureValkeyPodService(ctx context.Context, cluster types.ClusterInstance, logger logr.Logger) *actor.ActorResult {
	cr := cluster.Definition()

	for shard := 0; shard < int(cr.Spec.Replicas.Shards); shard++ {
		for replica := 0; replica < int(cr.Spec.Replicas.ReplicasOfShard); replica++ {
			serviceName := clusterbuilder.ClusterNodeServiceName(cr.Name, shard, replica)
			newSvc := clusterbuilder.GeneratePodService(cr, serviceName, cr.Spec.Access.ServiceType, cr.Spec.Access.Annotations)
			if svc, err := a.client.GetService(ctx, cr.Namespace, serviceName); errors.IsNotFound(err) {
				if err = a.client.CreateService(ctx, cr.Namespace, newSvc); err != nil {
					a.logger.Error(err, "create service failed", "target", client.ObjectKeyFromObject(newSvc))
					return actor.RequeueWithError(err)
				}
			} else if err != nil {
				a.logger.Error(err, "get service failed", "target", client.ObjectKeyFromObject(newSvc))
				return actor.NewResult(cops.CommandRequeue)
			} else if svc.Spec.Type != newSvc.Spec.Type ||
				!reflect.DeepEqual(svc.Spec.Selector, newSvc.Spec.Selector) ||
				!reflect.DeepEqual(svc.Labels, newSvc.Labels) ||
				!reflect.DeepEqual(svc.Annotations, newSvc.Annotations) {
				if err = a.client.UpdateService(ctx, newSvc.Namespace, newSvc); err != nil {
					a.logger.Error(err, "update service failed", "target", client.ObjectKeyFromObject(newSvc))
					return actor.RequeueWithError(err)
				}
			}
		}
	}
	return nil
}

func (a *actorEnsureResource) cleanUselessService(ctx context.Context, cluster types.ClusterInstance, logger logr.Logger) *actor.ActorResult {
	cr := cluster.Definition()
	if cr.Spec.Access.ServiceType != corev1.ServiceTypeLoadBalancer && cr.Spec.Access.ServiceType != corev1.ServiceTypeNodePort {
		return nil
	}

	labels := clusterbuilder.GenerateClusterLabels(cr.Name, nil)
	services, ret := a.fetchAllPodBindedServices(ctx, cr.Namespace, labels)
	if ret != nil {
		return ret
	}

	for _, item := range services {
		svc := item.DeepCopy()
		shard, index, err := builder.ParsePodShardAndIndex(svc.Name)
		if err != nil {
			logger.Error(err, "parse svc name failed", "target", client.ObjectKeyFromObject(svc))
			continue
		}
		if shard >= int(cr.Spec.Replicas.Shards) || index >= int(cr.Spec.Replicas.ReplicasOfShard) {
			_, err := a.client.GetPod(ctx, svc.Namespace, svc.Name)
			if errors.IsNotFound(err) {
				if err = a.client.DeleteService(ctx, svc.Namespace, svc.Name); err != nil {
					return actor.RequeueWithError(err)
				}
			} else if err != nil {
				logger.Error(err, "get pods failed", "target", client.ObjectKeyFromObject(svc))
				return actor.RequeueWithError(err)
			}
		}
	}
	return nil
}

func (a *actorEnsureResource) fetchAllPodBindedServices(ctx context.Context, namespace string, labels map[string]string) ([]corev1.Service, *actor.ActorResult) {
	var (
		services []corev1.Service
	)

	if svcRes, err := a.client.GetServiceByLabels(ctx, namespace, labels); err != nil {
		return nil, actor.RequeueWithError(err)
	} else {
		// ignore services without pod selector
		for _, svc := range svcRes.Items {
			if svc.Spec.Selector[builder.PodNameLabelKey] != "" {
				services = append(services, svc)
			}
		}
	}
	return services, nil
}

func MergeAnnotations(t, s map[string]string) map[string]string {
	if t == nil {
		return s
	}
	if s == nil {
		return t
	}

	for k, v := range s {
		if k == builder.RestartAnnotationKey {
			tRestartAnn := t[k]
			if tRestartAnn == "" && v != "" {
				t[k] = v
			}

			tTime, err1 := time.Parse(time.RFC3339Nano, tRestartAnn)
			sTime, err2 := time.Parse(time.RFC3339Nano, v)
			if err1 != nil || err2 != nil || sTime.After(tTime) {
				t[k] = v
			} else {
				t[k] = tRestartAnn
			}
		} else {
			t[k] = v
		}
	}
	return t
}
