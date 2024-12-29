package builder

import (
	"reflect"

	"github.com/chideat/valkey-operator/internal/util"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

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

	for _, name := range []string{
		RedisStorageVolumeName,
	} {
		oldPvc := util.GetVolumeClaimTemplatesByName(sts.Spec.VolumeClaimTemplates, name)
		newPvc := util.GetVolumeClaimTemplatesByName(newSts.Spec.VolumeClaimTemplates, name)
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

func IsPodTemplasteChanged(newTplSpec, oldTplSpec *v1.PodTemplateSpec, logger logr.Logger) bool {
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
		oldVol := util.GetVolumeByName(oldSpec.Volumes, newVol.Name)
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
		oldCon, newCon := util.GetContainerByName(&oldSpec, name), util.GetContainerByName(&newSpec, name)
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

func loadEnvs(envs []v1.EnvVar) map[string]string {
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
