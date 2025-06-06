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
	"github.com/chideat/valkey-operator/internal/builder/sabuilder"
	"github.com/chideat/valkey-operator/internal/builder/sentinelbuilder"
	ops "github.com/chideat/valkey-operator/internal/ops/sentinel"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ actor.Actor = (*actorEnsureResource)(nil)

func init() {
	actor.Register(core.ValkeySentinel, NewEnsureResourceActor)
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
	return []actor.Command{ops.CommandEnsureResource}
}

func (a *actorEnsureResource) Version() *semver.Version {
	return semver.MustParse("0.1.0")
}

// Do
func (a *actorEnsureResource) Do(ctx context.Context, val types.Instance) *actor.ActorResult {
	logger := val.Logger().WithValues("actor", ops.CommandEnsureResource.String())

	var (
		sentinel = val.(types.SentinelInstance)
		inst     = sentinel.Definition()
	)
	if inst.Spec.PodAnnotations[builder.PauseAnnotationKey] != "" {
		if ret := a.ensurePauseStatefulSet(ctx, sentinel, logger); ret != nil {
			return ret
		}
		return actor.NewResult(ops.CommandPaused)
	}

	if ret := a.ensureServiceAccount(ctx, sentinel, logger); ret != nil {
		return ret
	}
	if ret := a.ensureService(ctx, sentinel, logger); ret != nil {
		return ret
	}
	// ensure configMap
	if ret := a.ensureConfigMap(ctx, sentinel, logger); ret != nil {
		return ret
	}
	if ret := a.ensureValkeySSL(ctx, sentinel, logger); ret != nil {
		return ret
	}
	if ret := a.ensureStatefulSet(ctx, sentinel, logger); ret != nil {
		return ret
	}
	return nil
}

func (a *actorEnsureResource) ensureStatefulSet(ctx context.Context, inst types.SentinelInstance, logger logr.Logger) *actor.ActorResult {
	if ret := a.ensurePodDisruptionBudget(ctx, inst, logger); ret != nil {
		return ret
	}
	sen := inst.Definition()

	salt := fmt.Sprintf("%s-%s-%s", sen.GetName(), sen.GetNamespace(), sen.GetName())
	sts, err := sentinelbuilder.GenerateSentinelStatefulset(inst)
	if err != nil {
		logger.Error(err, "generate statefulset failed")
		return actor.NewResultWithError(ops.CommandRequeue, err)
	}
	if passwordSecret := sen.Spec.Access.DefaultPasswordSecret; passwordSecret != "" {
		secret, err := a.client.GetSecret(ctx, sen.Namespace, passwordSecret)
		if err != nil {
			logger.Error(err, "get password secret failed", "target", util.ObjectKey(sen.Namespace, passwordSecret))
			return actor.NewResultWithError(ops.CommandRequeue, err)
		}
		secretSig, err := util.GenerateObjectSig(secret, salt)
		if err != nil {
			logger.Error(err, "generate secret sig failed")
			return actor.NewResultWithError(ops.CommandAbort, err)
		}
		sts.Spec.Template.Labels[builder.ChecksumKey("secret")] = secretSig
	}

	configName := sentinelbuilder.SentinelConfigMapName(sen.Name)
	configMap, err := a.client.GetConfigMap(ctx, sen.Namespace, configName)
	if err != nil {
		logger.Error(err, "get configMap failed", "target", util.ObjectKey(sen.Namespace, configName))
		return actor.NewResultWithError(ops.CommandRequeue, err)
	}
	configSig, err := util.GenerateObjectSig(configMap, salt)
	if err != nil {
		logger.Error(err, "generate configMap sig failed")
		return actor.NewResultWithError(ops.CommandAbort, err)
	}
	sts.Spec.Template.Annotations[builder.ChecksumKey("configmap")] = configSig

	oldSts, err := a.client.GetStatefulSet(ctx, sts.Namespace, sts.Name)
	if errors.IsNotFound(err) {
		if err := a.client.CreateStatefulSet(ctx, sen.Namespace, sts); err != nil {
			logger.Error(err, "create statefulset failed", "target", client.ObjectKeyFromObject(sts))
			return actor.NewResultWithError(ops.CommandRequeue, err)
		}
	} else if err != nil {
		logger.Error(err, "get statefulset failed", "target", client.ObjectKeyFromObject(sts))
		return actor.NewResultWithError(ops.CommandRequeue, err)
	} else if util.IsStatefulsetChanged(sts, oldSts, logger) {
		if *oldSts.Spec.Replicas > *sts.Spec.Replicas {
			oldSts.Spec.Replicas = sts.Spec.Replicas
			if err := a.client.UpdateStatefulSet(ctx, sen.Namespace, oldSts); err != nil {
				logger.Error(err, "scale down statefulset failed", "target", client.ObjectKeyFromObject(oldSts))
				return actor.RequeueWithError(err)
			}
			time.Sleep(time.Second * 3)
		}

		pods, err := inst.RawNodes(ctx)
		if err != nil {
			logger.Error(err, "get pods failed")
			return actor.RequeueWithError(err)
		}
		for _, item := range pods {
			pod := item.DeepCopy()
			pod.Labels = lo.Assign(pod.Labels, inst.Selector())
			if !reflect.DeepEqual(pod.Labels, item.Labels) {
				if err := a.client.UpdatePod(ctx, pod.GetNamespace(), pod); err != nil {
					logger.Error(err, "patch pod label failed", "target", client.ObjectKeyFromObject(pod))
					return actor.RequeueWithError(err)
				}
			}
		}

		if err := a.client.DeleteStatefulSet(ctx, sen.Namespace, sts.GetName(),
			client.PropagationPolicy(metav1.DeletePropagationOrphan)); err != nil && !errors.IsNotFound(err) {

			logger.Error(err, "delete old statefulset failed", "target", client.ObjectKeyFromObject(sts))
			return actor.RequeueWithError(err)
		}
		time.Sleep(time.Second * 3)
		if err = a.client.CreateStatefulSet(ctx, sen.Namespace, sts); err != nil {
			logger.Error(err, "update statefulset failed", "target", client.ObjectKeyFromObject(sts))
			return actor.RequeueWithError(err)
		}
	}
	return nil
}

func (a *actorEnsureResource) ensurePodDisruptionBudget(ctx context.Context, inst types.SentinelInstance, logger logr.Logger) *actor.ActorResult {
	sen := inst.Definition()

	pdb := sentinelbuilder.NewPodDisruptionBudget(sen)
	if oldPdb, err := a.client.GetPodDisruptionBudget(ctx, sen.Namespace, pdb.Name); errors.IsNotFound(err) {
		if err := a.client.CreatePodDisruptionBudget(ctx, sen.Namespace, pdb); err != nil {
			return actor.NewResultWithError(ops.CommandRequeue, err)
		}
	} else if err != nil {
		logger.Error(err, "get poddisruptionbudget failed", "target", client.ObjectKeyFromObject(pdb))
		return actor.NewResultWithError(ops.CommandRequeue, err)
	} else if !reflect.DeepEqual(oldPdb.Spec, pdb.Spec) {
		pdb.ResourceVersion = oldPdb.ResourceVersion
		if err := a.client.UpdatePodDisruptionBudget(ctx, sen.Namespace, pdb); err != nil {
			logger.Error(err, "update poddisruptionbudget failed", "target", client.ObjectKeyFromObject(pdb))
			return actor.NewResultWithError(ops.CommandRequeue, err)
		}
	}
	return nil
}

func (a *actorEnsureResource) ensureConfigMap(ctx context.Context, inst types.SentinelInstance, logger logr.Logger) *actor.ActorResult {
	// ensure sentinel config
	senitnelConfigMap, _ := sentinelbuilder.GenerateSentinelConfigMap(inst)
	if _, err := a.client.GetConfigMap(ctx, inst.GetNamespace(), senitnelConfigMap.Name); errors.IsNotFound(err) {
		if err := a.client.CreateIfNotExistsConfigMap(ctx, inst.GetNamespace(), senitnelConfigMap); err != nil {
			logger.Error(err, "create configMap failed", "target", client.ObjectKeyFromObject(senitnelConfigMap))
			return actor.NewResultWithError(ops.CommandRequeue, err)
		}
	} else if err != nil {
		logger.Error(err, "get configMap failed", "target", client.ObjectKeyFromObject(senitnelConfigMap))
		return actor.NewResultWithError(ops.CommandRequeue, err)
	}
	if err := a.client.UpdateIfConfigMapChanged(ctx, senitnelConfigMap); err != nil {
		return actor.NewResultWithError(ops.CommandRequeue, err)
	}
	return nil
}

func (a *actorEnsureResource) ensureValkeySSL(ctx context.Context, inst types.SentinelInstance, logger logr.Logger) *actor.ActorResult {
	def := inst.Definition()
	if !def.Spec.Access.EnableTLS {
		return nil
	}

	if secretName := def.Spec.Access.ExternalTLSSecret; secretName != "" {
		var (
			err    error
			secret *corev1.Secret
		)
		for i := 0; i < 5; i++ {
			if secret, err = a.client.GetSecret(ctx, def.Namespace, secretName); err != nil {
				logger.Error(err, "get external tls secret failed", "name", secretName)
				time.Sleep(time.Second * time.Duration(i+1))
			} else {
				break
			}
		}
		if secret == nil {
			logger.Error(err, "get external tls secret failed", "name", secretName)
			return actor.RequeueWithError(fmt.Errorf("external tls secret %s not found", secretName))
		}
		return nil
	}

	dnsNames := []string{
		certbuilder.GenerateServiceDNSName(sentinelbuilder.SentinelStatefulSetName(def.Name), def.Namespace),
	}
	for i := 0; i < int(def.Spec.Replicas); i++ {
		dnsNames = append(dnsNames, certbuilder.GenerateHeadlessDNSName(
			sentinelbuilder.SentinelPodServiceName(def.Name, i),
			sentinelbuilder.SentinelHeadlessServiceName(def.Name),
			def.Namespace,
		))
	}
	cc, err := certbuilder.NewCertificate(inst, dnsNames, inst.Selector())
	if err != nil {
		logger.Error(err, "create certificate failed")
		return actor.NewResultWithError(ops.CommandRequeue, err)
	}
	if err := a.client.CreateIfNotExistsCertificate(ctx, def.Namespace, cc); err != nil {
		return actor.NewResultWithError(ops.CommandRequeue, err)
	}
	oldCc, err := a.client.GetCertificate(ctx, def.Namespace, cc.GetName())
	if err != nil && !errors.IsNotFound(err) {
		logger.Error(err, "get certificate failed", "target", client.ObjectKeyFromObject(cc))
		return actor.NewResultWithError(ops.CommandRequeue, err)
	}

	var (
		secretName = certbuilder.GenerateSSLSecretName(def.Name)
		secret     *corev1.Secret
	)
	for i := 0; i < 5; i++ {
		if secret, _ = a.client.GetSecret(ctx, def.Namespace, secretName); secret != nil {
			break
		}
		// check when the certificate created
		if oldCc != nil && time.Since(oldCc.GetCreationTimestamp().Time) > time.Minute*5 {
			return actor.NewResultWithError(ops.CommandAbort, fmt.Errorf("issue for tls certificate failed, please check the cert-manager"))
		}
		time.Sleep(time.Second * time.Duration(i+1))
	}
	if secret == nil {
		return actor.NewResult(ops.CommandRequeue)
	}
	return nil
}

func (a *actorEnsureResource) ensurePauseStatefulSet(ctx context.Context, inst types.SentinelInstance, logger logr.Logger) *actor.ActorResult {
	sen := inst.Definition()
	name := sentinelbuilder.SentinelStatefulSetName(sen.Name)
	if sts, err := a.client.GetStatefulSet(ctx, sen.Namespace, name); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return actor.NewResultWithError(ops.CommandRequeue, err)
	} else {
		if sts.Spec.Replicas == nil || *sts.Spec.Replicas == 0 {
			return nil
		}
		*sts.Spec.Replicas = 0
		if err = a.client.UpdateStatefulSet(ctx, sen.Namespace, sts); err != nil {
			return actor.NewResultWithError(ops.CommandRequeue, err)
		}
	}
	return nil
}

func (a *actorEnsureResource) ensureServiceAccount(ctx context.Context, sentinel types.SentinelInstance, logger logr.Logger) *actor.ActorResult {
	sen := sentinel.Definition()
	sa := sabuilder.GenerateServiceAccount(sen)
	role := sabuilder.GenerateRole(sen)
	binding := sabuilder.GenerateRoleBinding(sen)
	clusterRole := sabuilder.GenerateClusterRole(sen)
	clusterRoleBinding := sabuilder.GenerateClusterRoleBinding(sen)

	if err := a.client.CreateOrUpdateServiceAccount(ctx, sentinel.GetNamespace(), sa); err != nil {
		logger.Error(err, "create service account failed", "target", client.ObjectKeyFromObject(sa))
		return actor.NewResultWithError(ops.CommandRequeue, err)
	}
	if err := a.client.CreateOrUpdateRole(ctx, sentinel.GetNamespace(), role); err != nil {
		return actor.NewResultWithError(ops.CommandRequeue, err)
	}
	if err := a.client.CreateOrUpdateRoleBinding(ctx, sentinel.GetNamespace(), binding); err != nil {
		return actor.NewResultWithError(ops.CommandRequeue, err)
	}
	if err := a.client.CreateOrUpdateClusterRole(ctx, clusterRole); err != nil {
		return actor.NewResultWithError(ops.CommandRequeue, err)
	}
	if oldClusterRb, err := a.client.GetClusterRoleBinding(ctx, clusterRoleBinding.Name); err != nil {
		if errors.IsNotFound(err) {
			if err := a.client.CreateClusterRoleBinding(ctx, clusterRoleBinding); err != nil {
				return actor.NewResultWithError(ops.CommandRequeue, err)
			}
		} else {
			return actor.NewResultWithError(ops.CommandRequeue, err)
		}
	} else {
		exists := false
		for _, sub := range oldClusterRb.Subjects {
			if sub.Namespace == sentinel.GetNamespace() {
				exists = true
			}
		}
		if !exists && len(oldClusterRb.Subjects) > 0 {
			oldClusterRb.Subjects = append(oldClusterRb.Subjects,
				rbacv1.Subject{Kind: "ServiceAccount",
					Name:      sabuilder.ValkeyInstanceServiceAccountName,
					Namespace: sentinel.GetNamespace()},
			)
			err := a.client.CreateOrUpdateClusterRoleBinding(ctx, oldClusterRb)
			if err != nil {
				return actor.NewResultWithError(ops.CommandRequeue, err)
			}
		}
	}
	return nil
}

func (a *actorEnsureResource) ensureService(ctx context.Context, inst types.SentinelInstance, logger logr.Logger) *actor.ActorResult {
	sen := inst.Definition()

	if ret := a.cleanUselessService(ctx, inst, logger); ret != nil {
		return ret
	}

	createService := func(senService *corev1.Service) *actor.ActorResult {
		if oldService, err := a.client.GetService(ctx, sen.GetNamespace(), senService.Name); errors.IsNotFound(err) {
			if err := a.client.CreateService(ctx, sen.GetNamespace(), senService); err != nil {
				return actor.NewResultWithError(ops.CommandRequeue, err)
			}
		} else if err != nil {
			return actor.NewResultWithError(ops.CommandRequeue, err)
		} else if senService.Spec.Type != oldService.Spec.Type ||
			(senService.Spec.Type == corev1.ServiceTypeNodePort && senService.Spec.Ports[0].NodePort != oldService.Spec.Ports[0].NodePort) ||
			!reflect.DeepEqual(senService.Spec.Selector, oldService.Spec.Selector) ||
			!reflect.DeepEqual(senService.Labels, oldService.Labels) ||
			!reflect.DeepEqual(senService.Annotations, oldService.Annotations) {

			if err := a.client.UpdateService(ctx, sen.GetNamespace(), senService); err != nil {
				return actor.NewResultWithError(ops.CommandRequeue, err)
			}
		}
		return nil
	}

	if ret := createService(sentinelbuilder.GenerateSentinelHeadlessService(sen)); ret != nil {
		return ret
	}

	switch sen.Spec.Access.ServiceType {
	case corev1.ServiceTypeNodePort:
		if ret := a.ensureValkeySpecifiedNodePortService(ctx, inst, logger); ret != nil {
			return ret
		}
	case corev1.ServiceTypeLoadBalancer:
		if ret := a.ensureValkeyPodService(ctx, inst, logger); ret != nil {
			return ret
		}
	}
	return nil
}

func (a *actorEnsureResource) ensureValkeySpecifiedNodePortService(ctx context.Context, inst types.SentinelInstance, logger logr.Logger) *actor.ActorResult {
	sen := inst.Definition()

	if sen.Spec.Access.Ports == "" {
		return a.ensureValkeyPodService(ctx, inst, logger)
	}

	logger.V(3).Info("ensure sentinel nodeports", "namepspace", sen.Namespace, "name", sen.Name)
	configedPorts, err := helper.ParsePorts(sen.Spec.Access.Ports)
	if err != nil {
		return actor.NewResultWithError(ops.CommandRequeue, err)
	}
	getClientPort := func(svc *corev1.Service, args ...string) int32 {
		name := "sentinel"
		if len(args) > 0 {
			name = args[0]
		}
		if port := util.GetServicePortByName(svc, name); port != nil {
			return port.NodePort
		}
		return 0
	}

	serviceNameRange := map[string]struct{}{}
	for i := 0; i < int(sen.Spec.Replicas); i++ {
		serviceName := sentinelbuilder.SentinelPodServiceName(sen.GetName(), i)
		serviceNameRange[serviceName] = struct{}{}
	}

	// the whole process is divided into 3 steps:
	// 1. delete service not in nodeport range
	// 2. create new service
	// 3. update existing service and restart pod (only one pod is restarted at a same time for each shard)

	// 1. delete service not in nodeport range
	//
	// when pod not exists and service not in nodeport range, delete service
	// NOTE: only delete service whose pod is not found
	//       let statefulset auto scale up/down for pods
	selector := sentinelbuilder.GenerateSelectorLabels(sen.Name)
	services, ret := a.fetchAllPodBindedServices(ctx, sen.Namespace, selector)
	if ret != nil {
		return ret
	}
	for _, svc := range services {
		svc := svc.DeepCopy()
		occupiedPort := getClientPort(svc)
		if _, exists := serviceNameRange[svc.Name]; !exists || !slices.Contains(configedPorts, occupiedPort) {
			_, err := a.client.GetPod(ctx, svc.Namespace, svc.Name)
			if errors.IsNotFound(err) {
				logger.Info("release nodeport service", "service", svc.Name, "port", occupiedPort)
				if err = a.client.DeleteService(ctx, svc.Namespace, svc.Name); err != nil {
					return actor.NewResultWithError(ops.CommandRequeue, err)
				}
			} else if err != nil {
				logger.Error(err, "get pods failed", "target", client.ObjectKeyFromObject(svc))
				return actor.NewResultWithError(ops.CommandRequeue, err)
			}
		}
	}

	if services, ret = a.fetchAllPodBindedServices(ctx, sen.Namespace, selector); ret != nil {
		return ret
	}

	// 2. create new service
	var (
		newPorts           []int32
		bindedNodeports    []int32
		needUpdateServices []*corev1.Service
	)
	for _, svc := range services {
		bindedNodeports = append(bindedNodeports, getClientPort(svc.DeepCopy()))
	}

	// filter used ports
	for _, port := range configedPorts {
		if !slices.Contains(bindedNodeports, port) {
			newPorts = append(newPorts, port)
		}
	}
	for i := range int(sen.Spec.Replicas) {
		serviceName := sentinelbuilder.SentinelPodServiceName(sen.Name, i)
		oldService, err := a.client.GetService(ctx, sen.Namespace, serviceName)
		if errors.IsNotFound(err) {
			if len(newPorts) == 0 {
				continue
			}
			port := newPorts[0]
			svc := sentinelbuilder.GeneratePodNodePortService(sen, i, port)
			if err = a.client.CreateService(ctx, svc.Namespace, svc); err != nil {
				a.logger.Error(err, "create nodeport service failed", "target", client.ObjectKeyFromObject(svc))
				return actor.NewResultWithValue(ops.CommandRequeue, err)
			}
			newPorts = newPorts[1:]
			continue
		} else if err != nil {
			return actor.NewResultWithError(ops.CommandRequeue, err)
		}

		svc := sentinelbuilder.GeneratePodNodePortService(sen, i, getClientPort(oldService))
		// check old service for compatibility
		if !reflect.DeepEqual(oldService.Spec.Selector, svc.Spec.Selector) ||
			len(oldService.Spec.Ports) != len(svc.Spec.Ports) ||
			!reflect.DeepEqual(oldService.Labels, svc.Labels) ||
			!reflect.DeepEqual(oldService.Annotations, svc.Annotations) {

			oldService.OwnerReferences = util.BuildOwnerReferences(sen)
			oldService.Spec = svc.Spec
			oldService.Labels = svc.Labels
			oldService.Annotations = svc.Annotations
			if err := a.client.UpdateService(ctx, oldService.Namespace, oldService); err != nil {
				a.logger.Error(err, "update nodeport service failed", "target", client.ObjectKeyFromObject(oldService))
				return actor.NewResultWithValue(ops.CommandRequeue, err)
			}
		}
		if port := getClientPort(oldService); port != 0 && !slices.Contains(configedPorts, port) {
			needUpdateServices = append(needUpdateServices, oldService)
		}
	}

	// 3. update existing service and restart pod (only one pod is restarted at a same time for each shard)
	if len(needUpdateServices) > 0 && len(newPorts) > 0 {
		port, svc := newPorts[0], needUpdateServices[0]
		if sp := util.GetServicePortByName(svc, "sentinel"); sp != nil {
			sp.NodePort = port
		}

		// NOTE: here not make sure the failover success, because the nodeport updated, the communication will be failed
		//       in k8s, the nodeport can still access for sometime after the nodeport updated
		//
		// update service
		if err = a.client.UpdateService(ctx, svc.Namespace, svc); err != nil {
			a.logger.Error(err, "update nodeport service failed", "target", client.ObjectKeyFromObject(svc), "port", port)
			return actor.NewResultWithValue(ops.CommandRequeue, err)
		}
		if pod, _ := a.client.GetPod(ctx, sen.Namespace, svc.Spec.Selector[builder.PodNameLabelKey]); pod != nil {
			if err := a.client.DeletePod(ctx, sen.Namespace, pod.Name); err != nil {
				return actor.NewResultWithError(ops.CommandRequeue, err)
			}
			return actor.NewResult(ops.CommandRequeue)
		}
	}
	return nil
}

func (a *actorEnsureResource) ensureValkeyPodService(ctx context.Context, inst types.SentinelInstance, logger logr.Logger) *actor.ActorResult {
	sen := inst.Definition()
	for i := 0; i < int(sen.Spec.Replicas); i++ {
		newSvc := sentinelbuilder.GeneratePodService(sen, i)
		if svc, err := a.client.GetService(ctx, sen.Namespace, newSvc.Name); errors.IsNotFound(err) {
			if err = a.client.CreateService(ctx, sen.Namespace, newSvc); err != nil {
				logger.Error(err, "create service failed", "target", client.ObjectKeyFromObject(newSvc))
				return actor.NewResultWithError(ops.CommandRequeue, err)
			}
		} else if err != nil {
			logger.Error(err, "get service failed", "target", client.ObjectKeyFromObject(newSvc))
			return actor.NewResult(ops.CommandRequeue)
		} else if newSvc.Spec.Type != svc.Spec.Type ||
			!reflect.DeepEqual(newSvc.Spec.Selector, svc.Spec.Selector) ||
			!reflect.DeepEqual(newSvc.Labels, svc.Labels) ||
			!reflect.DeepEqual(newSvc.Annotations, svc.Annotations) {
			svc.Spec = newSvc.Spec
			if err = a.client.UpdateService(ctx, sen.Namespace, svc); err != nil {
				logger.Error(err, "update service failed", "target", client.ObjectKeyFromObject(svc))
				return actor.NewResultWithError(ops.CommandRequeue, err)
			}
		}
	}
	return nil
}

func (a *actorEnsureResource) cleanUselessService(ctx context.Context, inst types.SentinelInstance, logger logr.Logger) *actor.ActorResult {
	sen := inst.Definition()
	services, err := a.fetchAllPodBindedServices(ctx, sen.Namespace, inst.Selector())
	if err != nil {
		return err
	}
	for _, item := range services {
		svc := item.DeepCopy()
		index, err := builder.ParsePodIndex(svc.Name)
		if err != nil {
			logger.Error(err, "parse svc name failed", "target", client.ObjectKeyFromObject(svc))
			continue
		}
		if index >= int(sen.Spec.Replicas) {
			_, err := a.client.GetPod(ctx, svc.Namespace, svc.Name)
			if errors.IsNotFound(err) {
				if err = a.client.DeleteService(ctx, svc.Namespace, svc.Name); err != nil {
					return actor.NewResultWithError(ops.CommandRequeue, err)
				}
			} else if err != nil {
				logger.Error(err, "get pods failed", "target", client.ObjectKeyFromObject(svc))
				return actor.NewResultWithError(ops.CommandRequeue, err)
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
		return nil, actor.NewResultWithError(ops.CommandRequeue, err)
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
