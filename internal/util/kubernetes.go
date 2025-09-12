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

package util

import (
	"reflect"
	"slices"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func BuildOwnerReferences(obj client.Object) (refs []metav1.OwnerReference) {
	if obj != nil {
		if obj.GetObjectKind().GroupVersionKind().Kind != "" &&
			obj.GetName() != "" && obj.GetUID() != "" {
			refs = append(refs, metav1.OwnerReference{
				APIVersion:         obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
				Kind:               obj.GetObjectKind().GroupVersionKind().Kind,
				Name:               obj.GetName(),
				UID:                obj.GetUID(),
				Controller:         ptr.To(true),
				BlockOwnerDeletion: ptr.To(true),
			})
		} else {
			refs = append(refs, obj.GetOwnerReferences()...)
		}
	}
	return
}

func ObjectKey(namespace, name string) client.ObjectKey {
	return client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}
}

func BuildOwnerReferencesWithParents(obj client.Object) (refs []metav1.OwnerReference) {
	if obj != nil {
		if obj.GetObjectKind().GroupVersionKind().Kind != "" &&
			obj.GetName() != "" && obj.GetUID() != "" {
			refs = append(refs, metav1.OwnerReference{
				APIVersion:         obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
				Kind:               obj.GetObjectKind().GroupVersionKind().Kind,
				Name:               obj.GetName(),
				UID:                obj.GetUID(),
				BlockOwnerDeletion: ptr.To(true),
				Controller:         ptr.To(true),
			})
		}
		for _, ref := range obj.GetOwnerReferences() {
			ref.Controller = nil
			refs = append(refs, ref)
		}
	}
	return
}

// GetContainerByName
func GetContainerByName(pod *corev1.PodSpec, name string) *corev1.Container {
	if pod == nil || name == "" {
		return nil
	}
	for _, c := range pod.InitContainers {
		if c.Name == name {
			return &c
		}
	}
	for _, c := range pod.Containers {
		if c.Name == name {
			return &c
		}
	}
	return nil
}

func GetServicePortByName(svc *corev1.Service, name string) *corev1.ServicePort {
	if svc == nil {
		return nil
	}
	for i := range svc.Spec.Ports {
		if svc.Spec.Ports[i].Name == name {
			return &svc.Spec.Ports[i]
		}
	}
	return nil
}

// GetVolumeClaimTemplatesByName
func GetVolumeClaimTemplatesByName(vols []corev1.PersistentVolumeClaim, name string) *corev1.PersistentVolumeClaim {
	for _, vol := range vols {
		if vol.GetName() == name {
			return &vol
		}
	}
	return nil
}

// GetVolumes
func GetVolumeByName(vols []corev1.Volume, name string) *corev1.Volume {
	for _, vol := range vols {
		if vol.Name == name {
			return &vol
		}
	}
	return nil
}

// isSubmap checks if map 'a' is a submap of map 'b'.
// It returns true if every key-value pair in 'a' is also present in 'b'.
func isSubmap[K, V comparable](a, b map[K]V) bool {
	if len(a) == 0 {
		return true
	}
	if len(b) == 0 {
		return false
	}
	for keyA, valA := range a {
		valB, ok := b[keyA]
		if !ok || valA != valB {
			return false
		}
	}
	return true
}

// IsStatefulsetChanged
func IsStatefulsetChanged(newSts, sts *appsv1.StatefulSet, logger logr.Logger) bool {
	// statefulset check
	if !cmp.Equal(newSts.GetLabels(), sts.GetLabels(), cmpopts.EquateEmpty()) ||
		!cmp.Equal(newSts.GetAnnotations(), sts.GetAnnotations(), cmpopts.EquateEmpty()) {
		logger.V(2).Info("labels or annotations diff")
		return true
	}

	if *newSts.Spec.Replicas != *sts.Spec.Replicas ||
		newSts.Spec.ServiceName != sts.Spec.ServiceName {
		logger.V(2).Info("replicas diff")
		return true
	}

	var vcNames []string
	for _, vc := range newSts.Spec.VolumeClaimTemplates {
		vcNames = append(vcNames, vc.Name)
	}
	for _, vc := range sts.Spec.VolumeClaimTemplates {
		if !slices.Contains(vcNames, vc.Name) {
			logger.V(2).Info("volumeclaimtemplates diff")
			return true
		}
	}

	for _, name := range vcNames {
		oldPvc := GetVolumeClaimTemplatesByName(sts.Spec.VolumeClaimTemplates, name)
		newPvc := GetVolumeClaimTemplatesByName(newSts.Spec.VolumeClaimTemplates, name)
		if oldPvc == nil && newPvc == nil {
			continue
		}
		if (oldPvc == nil && newPvc != nil) || (oldPvc != nil && newPvc == nil) {
			logger.V(2).Info("pvc diff", "name", name, "old", oldPvc, "new", newPvc)
			return true
		}

		if !cmp.Equal(oldPvc.Spec, newPvc.Spec, cmpopts.EquateEmpty()) {
			logger.V(2).Info("pvc diff", "name", name, "old", oldPvc.Spec, "new", newPvc.Spec)
			return true
		}
	}

	return IsPodTemplasteChanged(&newSts.Spec.Template, &sts.Spec.Template, logger)
}

func IsPodTemplasteChanged(newTplSpec, oldTplSpec *corev1.PodTemplateSpec, logger logr.Logger) bool {
	if (newTplSpec == nil && oldTplSpec != nil) || (newTplSpec != nil && oldTplSpec == nil) ||
		!cmp.Equal(newTplSpec.Labels, oldTplSpec.Labels, cmpopts.EquateEmpty()) ||
		!isSubmap(newTplSpec.Annotations, oldTplSpec.Annotations) {

		logger.V(2).Info("pod labels diff",
			"newLabels", newTplSpec.Labels, "oldLabels", oldTplSpec.Labels,
			"newAnnotations", newTplSpec.Annotations, "oldAnnotations", oldTplSpec.Annotations,
		)
		return true
	}

	newSpec, oldSpec := newTplSpec.Spec, oldTplSpec.Spec
	if len(newSpec.InitContainers) != len(oldSpec.InitContainers) ||
		len(newSpec.Containers) != len(oldSpec.Containers) {
		logger.V(2).Info("pod containers length diff")
		return true
	}

	// nodeselector
	if !cmp.Equal(newSpec.NodeSelector, oldSpec.NodeSelector, cmpopts.EquateEmpty()) ||
		!cmp.Equal(newSpec.Affinity, oldSpec.Affinity, cmpopts.EquateEmpty()) ||
		!cmp.Equal(newSpec.Tolerations, oldSpec.Tolerations, cmpopts.EquateEmpty()) {
		logger.V(2).Info("pod nodeselector|affinity|tolerations diff")
		return true
	}

	if !cmp.Equal(newSpec.SecurityContext, oldSpec.SecurityContext, cmpopts.EquateEmpty()) ||
		newSpec.HostNetwork != oldSpec.HostNetwork ||
		newSpec.ServiceAccountName != oldSpec.ServiceAccountName {
		logger.V(2).Info("pod securityContext or hostnetwork or serviceaccount diff",
			"securityContext", !reflect.DeepEqual(newSpec.SecurityContext, oldSpec.SecurityContext),
			"new", newSpec.SecurityContext, "old", oldSpec.SecurityContext)
		return true
	}

	if len(newSpec.Volumes) != len(oldSpec.Volumes) {
		return true
	}
	for _, newVol := range newSpec.Volumes {
		oldVol := GetVolumeByName(oldSpec.Volumes, newVol.Name)
		if oldVol == nil {
			return true
		}
		if newVol.Secret != nil && (oldVol.Secret == nil || oldVol.Secret.SecretName != newVol.Secret.SecretName) {
			return true
		}
		if newVol.ConfigMap != nil && (oldVol.ConfigMap == nil || oldVol.ConfigMap.Name != newVol.ConfigMap.Name) {
			return true
		}
		if newVol.PersistentVolumeClaim != nil &&
			(oldVol.PersistentVolumeClaim == nil || oldVol.PersistentVolumeClaim.ClaimName != newVol.PersistentVolumeClaim.ClaimName) {
			return true
		}
		if newVol.HostPath != nil && (oldVol.HostPath == nil || oldVol.HostPath.Path != newVol.HostPath.Path) {
			return true
		}
		if newVol.EmptyDir != nil && (oldVol.EmptyDir == nil || oldVol.EmptyDir.Medium != newVol.EmptyDir.Medium) {
			return true
		}
	}

	containerNames := map[string]struct{}{}
	for _, c := range newSpec.InitContainers {
		containerNames[c.Name] = struct{}{}
	}
	for _, c := range newSpec.Containers {
		containerNames[c.Name] = struct{}{}
	}

	for name := range containerNames {
		oldCon, newCon := GetContainerByName(&oldSpec, name), GetContainerByName(&newSpec, name)
		if (oldCon == nil && newCon != nil) || (oldCon != nil && newCon == nil) {
			logger.V(2).Info("pod containers not match diff")
			return true
		}

		nLimits, nReqs := newCon.Resources.Limits, newCon.Resources.Requests
		oLimits, oReqs := oldCon.Resources.Limits, oldCon.Resources.Requests
		if oLimits.Cpu().Cmp(*nLimits.Cpu()) != 0 || oLimits.Memory().Cmp(*nLimits.Memory()) != 0 ||
			oReqs.Cpu().Cmp(*nReqs.Cpu()) != 0 || oReqs.Memory().Cmp(*nReqs.Memory()) != 0 ||
			(!nLimits.StorageEphemeral().IsZero() && oLimits.StorageEphemeral().Cmp(*nLimits.StorageEphemeral()) != 0) ||
			(!nReqs.StorageEphemeral().IsZero() && oReqs.StorageEphemeral().Cmp(*nReqs.StorageEphemeral()) != 0) {
			logger.V(2).Info("pod containers resources diff",
				"CpuLimit", oLimits.Cpu().Cmp(*nLimits.Cpu()),
				"MemLimit", oLimits.Memory().Cmp(*nLimits.Memory()),
				"CpuRequest", oReqs.Cpu().Cmp(*nReqs.Cpu()),
				"MemRequest", oReqs.Memory().Cmp(*nReqs.Memory()),
				"StorageEphemeralLimit", oLimits.StorageEphemeral().Cmp(*nLimits.StorageEphemeral()),
				"StorageEphemeralRequest", oReqs.StorageEphemeral().Cmp(*nReqs.StorageEphemeral()),
			)
			return true
		}

		// check almost all fields of container
		// should make sure that apiserver not return noset default value
		if oldCon.Image != newCon.Image || oldCon.ImagePullPolicy != newCon.ImagePullPolicy ||
			!reflect.DeepEqual(loadEnvs(oldCon.Env), loadEnvs(newCon.Env)) ||
			!reflect.DeepEqual(oldCon.Command, newCon.Command) ||
			!reflect.DeepEqual(oldCon.Args, newCon.Args) ||
			!reflect.DeepEqual(oldCon.Ports, newCon.Ports) ||
			!cmp.Equal(oldCon.Lifecycle, newCon.Lifecycle, cmpopts.EquateEmpty()) ||
			!cmp.Equal(oldCon.VolumeMounts, newCon.VolumeMounts, cmpopts.EquateEmpty()) {

			logger.V(2).Info("pod containers config diff",
				"image", oldCon.Image != newCon.Image,
				"imagepullpolicy", oldCon.ImagePullPolicy != newCon.ImagePullPolicy,
				"resources", !reflect.DeepEqual(oldCon.Resources, newCon.Resources),
				"env", !reflect.DeepEqual(loadEnvs(oldCon.Env), loadEnvs(newCon.Env)),
				"command", !reflect.DeepEqual(oldCon.Command, newCon.Command),
				"args", !reflect.DeepEqual(oldCon.Args, newCon.Args),
				"ports", !reflect.DeepEqual(oldCon.Ports, newCon.Ports),
				"lifecycle", !reflect.DeepEqual(oldCon.Lifecycle, newCon.Lifecycle),
				"volumemounts", !reflect.DeepEqual(oldCon.VolumeMounts, newCon.VolumeMounts),
			)
			return true
		}
	}
	return false
}

func IsServiceChanged(ns, os *corev1.Service, logger logr.Logger) bool {
	if (ns == nil && os != nil) || (ns != nil && os == nil) {
		return true
	}
	newSvc, oldSvc := ns.DeepCopy(), os.DeepCopy()

	isSubset := func(n, o map[string]string) bool {
		if len(n) > len(o) {
			return false
		}
		for k, v := range n {
			if val, ok := o[k]; !ok || val != v {
				return false
			}
		}
		return true
	}

	if !isSubset(newSvc.Labels, oldSvc.Labels) ||
		!isSubset(newSvc.Annotations, oldSvc.Annotations) {
		logger.V(1).Info("Service labels or annotations changed",
			"newLabels", newSvc.Labels,
			"oldLabels", oldSvc.Labels,
			"newAnnotations", newSvc.Annotations,
			"oldAnnotations", oldSvc.Annotations,
		)
		return true
	}

	if newSvc.Spec.Type == "" {
		newSvc.Spec.Type = corev1.ServiceTypeClusterIP
	}
	if oldSvc.Spec.Type == "" {
		oldSvc.Spec.Type = corev1.ServiceTypeClusterIP
	}

	if newSvc.Spec.Type != oldSvc.Spec.Type {
		logger.V(1).Info("Service type changed")
		return true
	}
	if newSvc.Spec.Type == corev1.ServiceTypeLoadBalancer {
		if newSvc.Spec.AllocateLoadBalancerNodePorts == nil {
			newSvc.Spec.AllocateLoadBalancerNodePorts = ptr.To(true)
		}
		if oldSvc.Spec.AllocateLoadBalancerNodePorts == nil {
			oldSvc.Spec.AllocateLoadBalancerNodePorts = ptr.To(true)
		}
	}
	if newSvc.Spec.SessionAffinity == "" {
		newSvc.Spec.SessionAffinity = corev1.ServiceAffinityNone
	}
	if oldSvc.Spec.SessionAffinity == "" {
		oldSvc.Spec.SessionAffinity = corev1.ServiceAffinityNone
	}
	if newSvc.Spec.InternalTrafficPolicy == nil {
		newSvc.Spec.InternalTrafficPolicy = ptr.To(corev1.ServiceInternalTrafficPolicyCluster)
	}
	if oldSvc.Spec.InternalTrafficPolicy == nil {
		oldSvc.Spec.InternalTrafficPolicy = ptr.To(corev1.ServiceInternalTrafficPolicyCluster)
	}
	if newSvc.Spec.ExternalTrafficPolicy == "" {
		newSvc.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeCluster
	}
	if oldSvc.Spec.ExternalTrafficPolicy == "" {
		oldSvc.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeCluster
	}

	if !cmp.Equal(newSvc.Spec.Selector, oldSvc.Spec.Selector, cmpopts.EquateEmpty()) ||
		!cmp.Equal(newSvc.Spec.IPFamilyPolicy, oldSvc.Spec.IPFamilyPolicy, cmpopts.EquateEmpty()) ||
		!cmp.Equal(newSvc.Spec.IPFamilies, oldSvc.Spec.IPFamilies, cmpopts.EquateEmpty()) ||
		newSvc.Spec.HealthCheckNodePort != oldSvc.Spec.HealthCheckNodePort ||
		newSvc.Spec.PublishNotReadyAddresses != oldSvc.Spec.PublishNotReadyAddresses ||
		newSvc.Spec.SessionAffinity != oldSvc.Spec.SessionAffinity ||
		!cmp.Equal(newSvc.Spec.InternalTrafficPolicy, oldSvc.Spec.InternalTrafficPolicy, cmpopts.EquateEmpty()) ||
		newSvc.Spec.ExternalTrafficPolicy != oldSvc.Spec.ExternalTrafficPolicy ||
		!cmp.Equal(newSvc.Spec.TrafficDistribution, oldSvc.Spec.TrafficDistribution, cmpopts.EquateEmpty()) ||

		(newSvc.Spec.Type == corev1.ServiceTypeLoadBalancer &&
			(newSvc.Spec.LoadBalancerIP != oldSvc.Spec.LoadBalancerIP ||
				!cmp.Equal(newSvc.Spec.LoadBalancerSourceRanges, oldSvc.Spec.LoadBalancerSourceRanges, cmpopts.EquateEmpty()) ||
				!cmp.Equal(newSvc.Spec.AllocateLoadBalancerNodePorts, oldSvc.Spec.AllocateLoadBalancerNodePorts, cmpopts.EquateEmpty()))) {

		logger.V(1).Info("Service spec changed",
			"selector", !cmp.Equal(newSvc.Spec.Selector, oldSvc.Spec.Selector, cmpopts.EquateEmpty()),
			"familypolicy", !cmp.Equal(newSvc.Spec.IPFamilyPolicy, oldSvc.Spec.IPFamilyPolicy, cmpopts.EquateEmpty()),
			"IPFamilies", !cmp.Equal(newSvc.Spec.IPFamilies, oldSvc.Spec.IPFamilies, cmpopts.EquateEmpty()),
			"allocatelbport", !cmp.Equal(newSvc.Spec.AllocateLoadBalancerNodePorts, oldSvc.Spec.AllocateLoadBalancerNodePorts, cmpopts.EquateEmpty()),
			"HealthCheckNodePort", newSvc.Spec.HealthCheckNodePort != oldSvc.Spec.HealthCheckNodePort,
			"LoadBalancerIP", newSvc.Spec.LoadBalancerIP != oldSvc.Spec.LoadBalancerIP,
			"LoadBalancerSourceRanges", !cmp.Equal(newSvc.Spec.LoadBalancerSourceRanges, oldSvc.Spec.LoadBalancerSourceRanges, cmpopts.EquateEmpty()),
			"PublishNotReadyAddresses", newSvc.Spec.PublishNotReadyAddresses != oldSvc.Spec.PublishNotReadyAddresses,
			"TrafficDistribution", !cmp.Equal(newSvc.Spec.TrafficDistribution, oldSvc.Spec.TrafficDistribution, cmpopts.EquateEmpty()),
		)
		return true
	}

	if len(newSvc.Spec.Ports) != len(oldSvc.Spec.Ports) {
		logger.V(1).Info("Service ports length changed")
		return true
	}
	for i, port := range newSvc.Spec.Ports {
		oldPort := oldSvc.Spec.Ports[i]
		if port.Protocol == "" {
			port.Protocol = corev1.ProtocolTCP
		}
		if oldPort.Protocol == "" {
			oldPort.Protocol = corev1.ProtocolTCP
		}

		if port.Name != oldPort.Name ||
			port.Protocol != oldPort.Protocol ||
			port.Port != oldPort.Port ||
			(newSvc.Spec.Type == corev1.ServiceTypeNodePort && port.NodePort != 0 && port.NodePort != oldPort.NodePort) {

			logger.V(1).Info("Service port changed",
				"portName", port.Name,
				"portProtocol", port.Protocol,
				"portNumber", port.Port,
				"nodePort", port.NodePort,
				"oldPortName", oldPort.Name,
				"oldPortProtocol", oldPort.Protocol,
				"oldPortNumber", oldPort.Port,
				"oldNodePort", oldPort.NodePort,
			)
			return true
		}
	}
	return false
}

func loadEnvs(envs []corev1.EnvVar) map[string]string {
	kvs := map[string]string{}
	for _, item := range envs {
		if item.ValueFrom != nil {
			switch {
			case item.ValueFrom.FieldRef != nil:
				kvs[item.Name] = item.ValueFrom.FieldRef.FieldPath
			case item.ValueFrom.SecretKeyRef != nil:
				kvs[item.Name] = item.ValueFrom.SecretKeyRef.Name
			case item.ValueFrom.ConfigMapKeyRef != nil:
				kvs[item.Name] = item.ValueFrom.ConfigMapKeyRef.Name
			case item.ValueFrom.ResourceFieldRef != nil:
				kvs[item.Name] = item.ValueFrom.ResourceFieldRef.Resource
			}
		} else {
			kvs[item.Name] = item.Value
		}
	}
	return kvs
}

func RetryOnTimeout(f func() error, step int) error {
	return retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: time.Second,
		Factor:   1.0,
		Jitter:   0.2,
	}, func(err error) bool {
		return errors.IsTimeout(err) || errors.IsServerTimeout(err) ||
			errors.IsTooManyRequests(err) || errors.IsServiceUnavailable(err)
	}, f)
}
