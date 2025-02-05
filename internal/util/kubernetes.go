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
*/package util

import (
	"reflect"
	"slices"
	"time"

	"github.com/go-logr/logr"
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

// IsStatefulsetChanged
func IsStatefulsetChanged(newSts, sts *appsv1.StatefulSet, logger logr.Logger) bool {
	// statefulset check
	if !reflect.DeepEqual(newSts.GetLabels(), sts.GetLabels()) ||
		!reflect.DeepEqual(newSts.GetAnnotations(), sts.GetAnnotations()) {
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

		if !reflect.DeepEqual(oldPvc.Spec, newPvc.Spec) {
			logger.V(2).Info("pvc diff", "name", name, "old", oldPvc.Spec, "new", newPvc.Spec)
			return true
		}
	}

	return IsPodTemplasteChanged(&newSts.Spec.Template, &sts.Spec.Template, logger)
}

func IsPodTemplasteChanged(newTplSpec, oldTplSpec *corev1.PodTemplateSpec, logger logr.Logger) bool {
	if (newTplSpec == nil && oldTplSpec != nil) || (newTplSpec != nil && oldTplSpec == nil) ||
		!reflect.DeepEqual(newTplSpec.Labels, oldTplSpec.Labels) ||
		!reflect.DeepEqual(newTplSpec.Annotations, oldTplSpec.Annotations) {
		logger.V(2).Info("pod labels diff")
		return true
	}

	newSpec, oldSpec := newTplSpec.Spec, oldTplSpec.Spec
	if len(newSpec.InitContainers) != len(oldSpec.InitContainers) ||
		len(newSpec.Containers) != len(oldSpec.Containers) {
		logger.V(2).Info("pod containers length diff")
		return true
	}

	// nodeselector
	if !reflect.DeepEqual(newSpec.NodeSelector, oldSpec.NodeSelector) ||
		!reflect.DeepEqual(newSpec.Affinity, oldSpec.Affinity) ||
		!reflect.DeepEqual(newSpec.Tolerations, oldSpec.Tolerations) {
		logger.V(2).Info("pod nodeselector|affinity|tolerations diff")
		return true
	}

	if !reflect.DeepEqual(newSpec.SecurityContext, oldSpec.SecurityContext) ||
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

		// check almost all fields of container
		// should make sure that apiserver not return noset default value
		if oldCon.Image != newCon.Image || oldCon.ImagePullPolicy != newCon.ImagePullPolicy ||
			!reflect.DeepEqual(oldCon.Resources, newCon.Resources) ||
			!reflect.DeepEqual(loadEnvs(oldCon.Env), loadEnvs(newCon.Env)) ||
			!reflect.DeepEqual(oldCon.Command, newCon.Command) ||
			!reflect.DeepEqual(oldCon.Args, newCon.Args) ||
			!reflect.DeepEqual(oldCon.Ports, newCon.Ports) ||
			!reflect.DeepEqual(oldCon.Lifecycle, newCon.Lifecycle) ||
			!reflect.DeepEqual(oldCon.VolumeMounts, newCon.VolumeMounts) {

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
