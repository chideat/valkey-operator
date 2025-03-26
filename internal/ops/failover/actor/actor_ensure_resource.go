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
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/actor"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/aclbuilder"
	"github.com/chideat/valkey-operator/internal/builder/certbuilder"
	"github.com/chideat/valkey-operator/internal/builder/failoverbuilder"
	"github.com/chideat/valkey-operator/internal/builder/sabuilder"
	"github.com/chideat/valkey-operator/internal/builder/sentinelbuilder"
	"github.com/chideat/valkey-operator/internal/config"
	ops "github.com/chideat/valkey-operator/internal/ops/failover"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/samber/lo"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ actor.Actor = (*actorEnsureResource)(nil)

func init() {
	actor.Register(core.ValkeyFailover, NewEnsureResourceActor)
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

	inst := val.(types.FailoverInstance)
	if (inst.Definition().Spec.PodAnnotations != nil) && inst.Definition().Spec.PodAnnotations[builder.PauseAnnotationKey] != "" {
		if ret := a.pauseStatefulSet(ctx, inst, logger); ret != nil {
			return ret
		}
		if ret := a.pauseSentinel(ctx, inst, logger); ret != nil {
			return ret
		}
		return actor.Pause()
	}

	if ret := a.ensureValkeySSL(ctx, inst, logger); ret != nil {
		return ret
	}
	if ret := a.ensureServiceAccount(ctx, inst, logger); ret != nil {
		return ret
	}
	if ret := a.ensureSentinel(ctx, inst, logger); ret != nil {
		return ret
	}
	if ret := a.ensureService(ctx, inst, logger); ret != nil {
		return ret
	}
	if ret := a.ensureConfigMap(ctx, inst, logger); ret != nil {
		return ret
	}
	if ret := a.ensureValkeyStatefulSet(ctx, inst, logger); ret != nil {
		return ret
	}
	return nil
}

func (a *actorEnsureResource) ensureValkeyStatefulSet(ctx context.Context, inst types.FailoverInstance, logger logr.Logger) *actor.ActorResult {
	var (
		err error
		cr  = inst.Definition()
	)

	// ensure inst statefulSet
	if ret := a.ensurePodDisruptionBudget(ctx, cr, logger); ret != nil {
		return ret
	}

	sts, err := failoverbuilder.GenerateStatefulSet(inst)
	if err != nil {
		return actor.RequeueWithError(err)
	}
	oldSts, err := a.client.GetStatefulSet(ctx, cr.Namespace, sts.Name)
	if errors.IsNotFound(err) {
		if err := a.client.CreateStatefulSet(ctx, cr.Namespace, sts); err != nil {
			return actor.RequeueWithError(err)
		}
		return nil
	} else if err != nil {
		logger.Error(err, "get statefulset failed", "target", client.ObjectKeyFromObject(sts))
		return actor.RequeueWithError(err)
	}

	if util.IsStatefulsetChanged(sts, oldSts, logger) {
		if *oldSts.Spec.Replicas > *sts.Spec.Replicas {
			// scale down
			oldSts.Spec.Replicas = sts.Spec.Replicas
			if err := a.client.UpdateStatefulSet(ctx, cr.Namespace, oldSts); err != nil {
				logger.Error(err, "scale down statefulset failed", "target", client.ObjectKeyFromObject(oldSts))
				return actor.RequeueWithError(err)
			}
			time.Sleep(time.Second * 3)
		}

		// patch pods with new labels in selector
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
		time.Sleep(time.Second * 3)
		if err := a.client.DeleteStatefulSet(ctx, cr.Namespace, sts.Name,
			client.PropagationPolicy(metav1.DeletePropagationOrphan)); err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "delete old statefulset failed", "target", client.ObjectKeyFromObject(sts))
			return actor.RequeueWithError(err)
		}
		if err = a.client.CreateStatefulSet(ctx, cr.Namespace, sts); err != nil {
			logger.Error(err, "update statefulset failed", "target", client.ObjectKeyFromObject(sts))
			return actor.RequeueWithError(err)
		}
	}
	return nil
}

func (a *actorEnsureResource) ensurePodDisruptionBudget(ctx context.Context, rf *v1alpha1.Failover, logger logr.Logger) *actor.ActorResult {
	pdb := failoverbuilder.NewPodDisruptionBudgetForCR(rf)

	if oldPdb, err := a.client.GetPodDisruptionBudget(ctx, rf.Namespace, pdb.Name); errors.IsNotFound(err) {
		if err := a.client.CreatePodDisruptionBudget(ctx, rf.Namespace, pdb); err != nil {
			return actor.RequeueWithError(err)
		}
	} else if err != nil {
		return actor.RequeueWithError(err)
	} else if !reflect.DeepEqual(oldPdb.Spec.Selector, pdb.Spec.Selector) {
		oldPdb.Labels = pdb.Labels
		oldPdb.Spec.Selector = pdb.Spec.Selector
		if err := a.client.UpdatePodDisruptionBudget(ctx, rf.Namespace, oldPdb); err != nil {
			return actor.RequeueWithError(err)
		}
	}
	return nil
}

func (a *actorEnsureResource) ensureConfigMap(ctx context.Context, inst types.FailoverInstance, logger logr.Logger) *actor.ActorResult {
	cr := inst.Definition()
	selector := inst.Selector()
	// ensure Valkey configMap
	if ret := a.ensureValkeyConfigMap(ctx, inst, logger, selector); ret != nil {
		return ret
	}

	if ret := aclbuilder.GenerateACLConfigMap(inst, inst.Users().Encode(true)); ret != nil {
		if err := a.client.CreateIfNotExistsConfigMap(ctx, cr.Namespace, ret); err != nil {
			return actor.RequeueWithError(err)
		}
	}
	return nil
}

func (a *actorEnsureResource) ensureValkeyConfigMap(ctx context.Context, st types.FailoverInstance, logger logr.Logger, selectors map[string]string) *actor.ActorResult {
	rf := st.Definition()
	configMap, err := failoverbuilder.GenerateConfigMap(st)
	if err != nil {
		return actor.RequeueWithError(err)
	}
	if err := a.client.CreateIfNotExistsConfigMap(ctx, rf.Namespace, configMap); err != nil {
		return actor.RequeueWithError(err)
	}
	return nil
}

func (a *actorEnsureResource) ensureValkeySSL(ctx context.Context, inst types.FailoverInstance, logger logr.Logger) *actor.ActorResult {
	rf := inst.Definition()
	if !rf.Spec.Access.EnableTLS {
		return nil
	}

	dnsNames := []string{
		certbuilder.GenerateServiceDNSName(failoverbuilder.ROServiceName(rf.Name), rf.Namespace),
		certbuilder.GenerateServiceDNSName(failoverbuilder.RWServiceName(rf.Name), rf.Namespace),
		certbuilder.GenerateServiceDNSName(sentinelbuilder.SentinelStatefulSetName(rf.Name), rf.Namespace),
	}
	for i := 0; i < int(rf.Spec.Replicas); i++ {
		name := fmt.Sprintf("%s.%s.%s",
			sentinelbuilder.SentinelPodServiceName(rf.Name, i),
			sentinelbuilder.SentinelHeadlessServiceName(rf.Name),
			rf.Namespace)
		dnsNames = append(dnsNames, name)
	}

	cc, err := certbuilder.NewCertificate(inst, dnsNames, inst.Selector())
	if err != nil {
		logger.Error(err, "create certificate failed")
		return actor.RequeueWithError(err)
	}
	if err := a.client.CreateIfNotExistsCertificate(ctx, rf.Namespace, cc); err != nil {
		return actor.RequeueWithError(err)
	}
	oldCc, err := a.client.GetCertificate(ctx, rf.Namespace, cc.GetName())
	if err != nil && !errors.IsNotFound(err) {
		return actor.RequeueWithError(err)
	}

	var (
		secretName = certbuilder.GenerateSSLSecretName(rf.Name)
		secret     *corev1.Secret
	)
	for i := 0; i < 5; i++ {
		if secret, _ = a.client.GetSecret(ctx, rf.Namespace, secretName); secret != nil {
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

func (a *actorEnsureResource) ensureServiceAccount(ctx context.Context, inst types.FailoverInstance, logger logr.Logger) *actor.ActorResult {
	cr := inst.Definition()
	sa := sabuilder.GenerateServiceAccount(cr)
	role := sabuilder.GenerateRole(cr)
	binding := sabuilder.GenerateRoleBinding(cr)
	clusterRole := sabuilder.GenerateClusterRole(cr)
	clusterRoleBinding := sabuilder.GenerateClusterRoleBinding(cr)

	if err := a.client.CreateOrUpdateServiceAccount(ctx, inst.GetNamespace(), sa); err != nil {
		logger.Error(err, "create service account failed", "target", client.ObjectKeyFromObject(sa))
		return actor.RequeueWithError(err)
	}
	if err := a.client.CreateOrUpdateRole(ctx, inst.GetNamespace(), role); err != nil {
		return actor.RequeueWithError(err)
	}
	if err := a.client.CreateOrUpdateRoleBinding(ctx, inst.GetNamespace(), binding); err != nil {
		return actor.RequeueWithError(err)
	}
	if err := a.client.CreateOrUpdateClusterRole(ctx, clusterRole); err != nil {
		return actor.RequeueWithError(err)
	}
	if oldClusterRb, err := a.client.GetClusterRoleBinding(ctx, clusterRoleBinding.Name); err != nil {
		if errors.IsNotFound(err) {
			if err := a.client.CreateClusterRoleBinding(ctx, clusterRoleBinding); err != nil {
				return actor.RequeueWithError(err)
			}
		} else {
			return actor.RequeueWithError(err)
		}
	} else {
		exists := false
		for _, sub := range oldClusterRb.Subjects {
			if sub.Namespace == inst.GetNamespace() {
				exists = true
			}
		}
		if !exists && len(oldClusterRb.Subjects) > 0 {
			oldClusterRb.Subjects = append(oldClusterRb.Subjects,
				rbacv1.Subject{Kind: "ServiceAccount",
					Name:      sabuilder.ValkeyInstanceServiceAccountName,
					Namespace: inst.GetNamespace()},
			)
			err := a.client.CreateOrUpdateClusterRoleBinding(ctx, oldClusterRb)
			if err != nil {
				return actor.RequeueWithError(err)
			}
		}
	}
	return nil
}

func (a *actorEnsureResource) ensureSentinel(ctx context.Context, inst types.FailoverInstance, logger logr.Logger) *actor.ActorResult {
	if !inst.IsBindedSentinel() {
		return nil
	}

	newSen := failoverbuilder.NewFailoverSentinel(inst)
	oldSen, err := a.client.GetSentinel(ctx, inst.GetNamespace(), inst.GetName())
	if errors.IsNotFound(err) {
		if err := a.client.Client().Create(ctx, newSen); err != nil {
			logger.Error(err, "create sentinel failed", "target", client.ObjectKeyFromObject(newSen))
			return actor.RequeueWithError(err)
		}
		return nil
	} else if err != nil {
		logger.Error(err, "get sentinel failed", "target", client.ObjectKeyFromObject(newSen))
		return actor.RequeueWithError(err)
	}
	if !reflect.DeepEqual(newSen.Spec, oldSen.Spec) ||
		!reflect.DeepEqual(newSen.Labels, oldSen.Labels) ||
		!reflect.DeepEqual(newSen.Annotations, oldSen.Annotations) {
		oldSen.Spec = newSen.Spec
		oldSen.Labels = newSen.Labels
		oldSen.Annotations = newSen.Annotations
		if err := a.client.UpdateSentinel(ctx, oldSen); err != nil {
			logger.Error(err, "update sentinel failed", "target", client.ObjectKeyFromObject(oldSen))
			return actor.RequeueWithError(err)
		}
	}
	return nil
}

func (a *actorEnsureResource) ensureService(ctx context.Context, inst types.FailoverInstance, logger logr.Logger) *actor.ActorResult {
	cr := inst.Definition()
	// read write svc
	rwSvc := failoverbuilder.GenerateReadWriteService(cr)
	roSvc := failoverbuilder.GenerateReadonlyService(cr)
	if err := a.client.CreateOrUpdateIfServiceChanged(ctx, inst.GetNamespace(), rwSvc); err != nil {
		return actor.RequeueWithError(err)
	}
	if err := a.client.CreateOrUpdateIfServiceChanged(ctx, inst.GetNamespace(), roSvc); err != nil {
		return actor.RequeueWithError(err)
	}

	selector := inst.Selector()
	exporterService := failoverbuilder.GenerateExporterService(cr)
	if err := a.client.CreateOrUpdateIfServiceChanged(ctx, inst.GetNamespace(), exporterService); err != nil {
		return actor.RequeueWithError(err)
	}

	if ret := a.cleanUselessService(ctx, cr, logger, selector); ret != nil {
		return ret
	}
	switch cr.Spec.Access.ServiceType {
	case corev1.ServiceTypeNodePort:
		if ret := a.ensureValkeySpecifiedNodePortService(ctx, inst, logger, selector); ret != nil {
			return ret
		}
	case corev1.ServiceTypeLoadBalancer:
		if ret := a.ensureValkeyPodService(ctx, cr, logger, selector); ret != nil {
			return ret
		}
	}
	return nil
}

func (a *actorEnsureResource) ensureValkeySpecifiedNodePortService(ctx context.Context,
	inst types.FailoverInstance, logger logr.Logger, selectors map[string]string) *actor.ActorResult {
	cr := inst.Definition()

	if cr.Spec.Access.Ports == "" {
		return a.ensureValkeyPodService(ctx, cr, logger, selectors)
	}

	logger.V(3).Info("ensure cluster nodeports", "namepspace", cr.Namespace, "name", cr.Name)
	configedPorts, err := helper.ParsePorts(cr.Spec.Access.Ports)
	if err != nil {
		return actor.RequeueWithError(err)
	}
	getClientPort := func(svc *corev1.Service, args ...string) int32 {
		name := "client"
		if len(args) > 0 {
			name = args[0]
		}
		if port := util.GetServicePortByName(svc, name); port != nil {
			return port.NodePort
		}
		return 0
	}

	serviceNameRange := map[string]struct{}{}
	for i := 0; i < int(cr.Spec.Replicas); i++ {
		serviceName := failoverbuilder.NodePortServiceName(cr, i)
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
	labels := failoverbuilder.GenerateSelectorLabels(cr.Name)
	services, ret := a.fetchAllPodBindedServices(ctx, cr.Namespace, labels)
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
		newPorts           []int32
		bindedNodeports    []int32
		needUpdateServices []*corev1.Service
	)
	for _, svc := range services {
		svc := svc.DeepCopy()
		bindedNodeports = append(bindedNodeports, getClientPort(svc))
	}
	// filter used ports
	for _, port := range configedPorts {
		if !slices.Contains(bindedNodeports, port) {
			newPorts = append(newPorts, port)
		}
	}
	for i := 0; i < int(cr.Spec.Replicas); i++ {
		serviceName := failoverbuilder.NodePortServiceName(cr, i)
		oldService, err := a.client.GetService(ctx, cr.Namespace, serviceName)
		if errors.IsNotFound(err) {
			if len(newPorts) == 0 {
				continue
			}
			port := newPorts[0]
			svc := failoverbuilder.GeneratePodNodePortService(cr, i, port)
			if err = a.client.CreateService(ctx, svc.Namespace, svc); err != nil {
				a.logger.Error(err, "create nodeport service failed", "target", client.ObjectKeyFromObject(svc))
				return actor.NewResultWithValue(ops.CommandRequeue, err)
			}
			newPorts = newPorts[1:]
			continue
		} else if err != nil {
			return actor.RequeueWithError(err)
		}

		svc := failoverbuilder.GeneratePodNodePortService(cr, i, getClientPort(oldService))
		// check old service for compability
		if len(oldService.OwnerReferences) == 0 ||
			oldService.OwnerReferences[0].Kind == "Pod" ||
			!reflect.DeepEqual(oldService.Spec, svc.Spec) ||
			!reflect.DeepEqual(oldService.Labels, svc.Labels) ||
			!reflect.DeepEqual(oldService.Annotations, svc.Annotations) {

			oldService.OwnerReferences = util.BuildOwnerReferences(cr)
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
		if sp := util.GetServicePortByName(svc, "client"); sp != nil {
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
		if pod, _ := a.client.GetPod(ctx, cr.Namespace, svc.Spec.Selector[builder.PodNameLabelKey]); pod != nil {
			if err := a.client.DeletePod(ctx, cr.Namespace, pod.Name); err != nil {
				return actor.RequeueWithError(err)
			}
			return actor.NewResult(ops.CommandRequeue)
		}
	}
	return nil
}

func (a *actorEnsureResource) ensureValkeyPodService(ctx context.Context, rf *v1alpha1.Failover, logger logr.Logger, selectors map[string]string) *actor.ActorResult {
	for i := 0; i < int(rf.Spec.Replicas); i++ {
		newSvc := failoverbuilder.GeneratePodService(rf, i)
		if svc, err := a.client.GetService(ctx, rf.Namespace, newSvc.Name); errors.IsNotFound(err) {
			if err = a.client.CreateService(ctx, rf.Namespace, newSvc); err != nil {
				logger.Error(err, "create service failed", "target", client.ObjectKeyFromObject(newSvc))
				return actor.RequeueWithError(err)
			}
		} else if err != nil {
			logger.Error(err, "get service failed", "target", client.ObjectKeyFromObject(newSvc))
			return actor.NewResult(ops.CommandRequeue)
		} else if newSvc.Spec.Type != svc.Spec.Type ||
			!reflect.DeepEqual(newSvc.Spec.Selector, svc.Spec.Selector) ||
			!reflect.DeepEqual(newSvc.Labels, svc.Labels) ||
			!reflect.DeepEqual(newSvc.Annotations, svc.Annotations) {
			svc.Spec = newSvc.Spec
			svc.Labels = newSvc.Labels
			svc.Annotations = newSvc.Annotations
			if err = a.client.UpdateService(ctx, rf.Namespace, svc); err != nil {
				logger.Error(err, "update service failed", "target", client.ObjectKeyFromObject(svc))
				return actor.RequeueWithError(err)
			}
		}
	}
	return nil
}

func (a *actorEnsureResource) cleanUselessService(ctx context.Context, rf *v1alpha1.Failover, logger logr.Logger, selectors map[string]string) *actor.ActorResult {
	services, err := a.fetchAllPodBindedServices(ctx, rf.Namespace, selectors)
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
		if index >= int(rf.Spec.Replicas) {
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

func (a *actorEnsureResource) pauseStatefulSet(ctx context.Context, inst types.FailoverInstance, logger logr.Logger) *actor.ActorResult {
	cr := inst.Definition()
	name := failoverbuilder.FailoverStatefulSetName(cr.Name)
	if sts, err := a.client.GetStatefulSet(ctx, cr.Namespace, name); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return actor.RequeueWithError(err)
	} else {
		if sts.Spec.Replicas == nil || *sts.Spec.Replicas == 0 {
			return nil
		}
		*sts.Spec.Replicas = 0
		if err = a.client.UpdateStatefulSet(ctx, cr.Namespace, sts); err != nil {
			return actor.RequeueWithError(err)
		}
		inst.SendEventf(corev1.EventTypeNormal, config.EventPause, "pause statefulset %s", name)
	}
	return nil
}

func (a *actorEnsureResource) pauseSentinel(ctx context.Context, inst types.FailoverInstance, logger logr.Logger) *actor.ActorResult {
	if def := inst.Definition(); def.Spec.Sentinel == nil ||
		(def.Spec.Sentinel.SentinelReference == nil && def.Spec.Sentinel.Image == "") {
		return nil
	}

	sen, err := a.client.GetSentinel(ctx, inst.GetNamespace(), inst.GetName())
	if err != nil {
		return actor.RequeueWithError(err)
	}
	if sen.Spec.Replicas == 0 {
		return nil
	}
	sen.Spec.Replicas = 0
	if err := a.client.UpdateSentinel(ctx, sen); err != nil {
		logger.Error(err, "pause sentinel failed", "target", client.ObjectKeyFromObject(sen))
		return actor.RequeueWithError(err)
	}
	inst.SendEventf(corev1.EventTypeNormal, config.EventPause, "pause instance sentinels")
	return nil
}

func (a *actorEnsureResource) fetchAllPodBindedServices(ctx context.Context, namespace string, selector map[string]string) ([]corev1.Service, *actor.ActorResult) {
	var services []corev1.Service
	if svcRes, err := a.client.GetServiceByLabels(ctx, namespace, selector); err != nil {
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
