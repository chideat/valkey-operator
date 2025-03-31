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

package sentinelbuilder

import (
	"fmt"
	"path"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/certbuilder"
	"github.com/chideat/valkey-operator/internal/builder/sabuilder"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/samber/lo"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	SentinelConfigVolumeName      = "sentinel-config"
	SentinelConfigVolumeMountPath = "/etc/valkey"

	ValkeyTLSVolumeName      = builder.ValkeyTLSVolumeName
	ValkeyTLSVolumeMountPath = builder.ValkeyTLSVolumeDefaultMountPath

	ValkeyDataVolumeName      = builder.ValkeyDataVolumeName
	ValkeyDataVolumeMountPath = builder.ValkeyDataVolumeDefaultMountPath

	ValkeyAuthName      = "valkey-auth"
	ValkeyAuthMountPath = "/account"

	ValkeyOptName      = "valkey-opt"
	ValkeyOptMountPath = "/opt"

	SentinelContainerName     = "sentinel"
	SentinelContainerPortName = "sentinel"

	graceTime = 30
)

func SentinelStatefulSetName(sentinelName string) string {
	return fmt.Sprintf("%s-%s", builder.ResourcePrefix(core.ValkeySentinel), sentinelName)
}

func GenerateSentinelStatefulset(inst types.SentinelInstance) (*appv1.StatefulSet, error) {
	var (
		sen            = inst.Definition()
		passwordSecret = sen.Spec.Access.DefaultPasswordSecret
		selectors      = GenerateSelectorLabels(sen.GetName())
		labels         = GenerateCommonLabels(sen.GetName())

		initContainers []corev1.Container
		containers     []corev1.Container
	)

	volumes := buildVolumes(sen, passwordSecret)
	envs := buildEnvs(sen)

	if cont, err := buildInitContainer(sen, nil); err != nil {
		return nil, err
	} else {
		initContainers = append(initContainers, *cont)
	}

	if cont, err := buildServerContainer(sen, envs); err != nil {
		return nil, err
	} else {
		containers = append(containers, *cont)
	}
	if cont, err := buildAgentContainer(sen, envs); err != nil {
		return nil, err
	} else {
		containers = append(containers, *cont)
	}

	ss := &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            SentinelStatefulSetName(sen.GetName()),
			Namespace:       sen.GetNamespace(),
			Labels:          labels,
			OwnerReferences: util.BuildOwnerReferences(sen),
		},
		Spec: appv1.StatefulSetSpec{
			ServiceName:         SentinelHeadlessServiceName(sen.GetName()),
			Replicas:            &sen.Spec.Replicas,
			PodManagementPolicy: appv1.ParallelPodManagement,
			UpdateStrategy: appv1.StatefulSetUpdateStrategy{
				Type: appv1.RollingUpdateStatefulSetStrategyType,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: selectors,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: lo.Assign(map[string]string{}, sen.Spec.PodAnnotations),
				},
				Spec: corev1.PodSpec{
					HostAliases: []corev1.HostAlias{
						builder.LocalhostAlias(sen.Spec.Access.IPFamilyPrefer),
					},
					InitContainers:                initContainers,
					Containers:                    containers,
					ImagePullSecrets:              sen.Spec.ImagePullSecrets,
					SecurityContext:               builder.GetPodSecurityContext(sen.Spec.SecurityContext),
					ServiceAccountName:            sabuilder.ValkeyInstanceServiceAccountName,
					Affinity:                      buildAffinity(sen.Spec.Affinity, selectors),
					Tolerations:                   sen.Spec.Tolerations,
					NodeSelector:                  sen.Spec.NodeSelector,
					Volumes:                       volumes,
					TerminationGracePeriodSeconds: ptr.To(int64(graceTime)),
				},
			},
		},
	}
	return ss, nil
}

func buildInitContainer(sen *v1alpha1.Sentinel, _ []corev1.EnvVar) (*corev1.Container, error) {
	image := config.GetValkeyHelperImage(sen)
	if image == "" {
		return nil, fmt.Errorf("valkey-helper image not found")
	}

	return &corev1.Container{
		Name:            "init",
		Image:           image,
		ImagePullPolicy: builder.GetPullPolicy(sen.Spec.ImagePullPolicy),
		Command:         []string{"sh", "/opt/init_sentinel.sh"},
		Env: []corev1.EnvVar{
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			{
				Name: "NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			{
				Name:  "SENTINEL_ANNOUNCE_PATH",
				Value: "/data/announce.conf",
			},
			{
				Name:  "IP_FAMILY_PREFER",
				Value: string(sen.Spec.Access.IPFamilyPrefer),
			},
			{
				Name:  "SERVICE_TYPE",
				Value: string(sen.Spec.Access.ServiceType),
			},
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: resource.MustParse("200Mi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: resource.MustParse("200Mi"),
			},
		},
		SecurityContext: builder.GetSecurityContext(sen.Spec.SecurityContext),
		VolumeMounts: []corev1.VolumeMount{
			{Name: ValkeyDataVolumeName, MountPath: ValkeyDataVolumeMountPath},
			{Name: ValkeyOptName, MountPath: path.Join("/mnt", ValkeyOptMountPath)},
		},
	}, nil
}

func buildServerContainer(sen *v1alpha1.Sentinel, envs []corev1.EnvVar) (*corev1.Container, error) {
	var (
		passwordSecret = sen.Spec.Access.DefaultPasswordSecret
		startArgs      = []string{"sh", "/opt/run_sentinel.sh"}
		shutdownArgs   = []string{"sh", "-c", "/opt/valkey-helper sentinel shutdown &> /proc/1/fd/1"}
		volumeMounts   = buildVolumeMounts(sen, passwordSecret)
	)

	return &corev1.Container{
		Name:            SentinelContainerName,
		Command:         startArgs,
		Image:           sen.Spec.Image,
		ImagePullPolicy: builder.GetPullPolicy(sen.Spec.ImagePullPolicy),
		Env:             envs,
		Ports: []corev1.ContainerPort{
			{
				Name:          SentinelContainerPortName,
				ContainerPort: builder.DefaultValkeySentinelPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		StartupProbe: &corev1.Probe{
			InitialDelaySeconds: 3,
			TimeoutSeconds:      5,
			FailureThreshold:    3,
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(builder.DefaultValkeySentinelPort),
				},
			},
		},
		LivenessProbe: &corev1.Probe{
			InitialDelaySeconds: 10,
			TimeoutSeconds:      5,
			FailureThreshold:    5,
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(builder.DefaultValkeySentinelPort),
				},
			},
		},
		Resources: sen.Spec.Resources,
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.LifecycleHandler{
				Exec: &corev1.ExecAction{
					Command: shutdownArgs,
				},
			},
		},
		SecurityContext: builder.GetSecurityContext(sen.Spec.SecurityContext),
		VolumeMounts:    volumeMounts,
	}, nil
}

func buildAgentContainer(sen *v1alpha1.Sentinel, envs []corev1.EnvVar) (*corev1.Container, error) {
	image := config.GetValkeyHelperImage(sen)
	if image == "" {
		return nil, fmt.Errorf("valkey-helper image not found")
	}
	container := corev1.Container{
		Name:            "agent",
		Image:           image,
		ImagePullPolicy: builder.GetPullPolicy(sen.Spec.ImagePullPolicy),
		Env:             envs,
		Command:         []string{"/opt/valkey-helper", "sentinel", "agent"},
		SecurityContext: builder.GetSecurityContext(sen.Spec.SecurityContext),
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      ValkeyDataVolumeName,
				MountPath: ValkeyDataVolumeMountPath,
			},
		},
	}

	if sen.Spec.Access.DefaultPasswordSecret != "" {
		vol := corev1.VolumeMount{
			Name:      ValkeyAuthName,
			MountPath: ValkeyAuthMountPath,
		}
		container.VolumeMounts = append(container.VolumeMounts, vol)
	}
	if sen.Spec.Access.EnableTLS {
		vol := corev1.VolumeMount{
			Name:      ValkeyTLSVolumeName,
			MountPath: ValkeyTLSVolumeMountPath,
		}
		container.VolumeMounts = append(container.VolumeMounts, vol)
	}
	return &container, nil
}

func buildEnvs(sen *v1alpha1.Sentinel) []corev1.EnvVar {
	valkeyEnvs := []corev1.EnvVar{
		{
			Name: "NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name: "POD_UID",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.uid",
				},
			},
		},
		{
			Name: "POD_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
		{
			Name: "POD_IPS",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.podIPs",
				},
			},
		},
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name:  builder.OperatorSecretName,
			Value: sen.Spec.Access.DefaultPasswordSecret,
		},
		{
			Name:  "TLS_ENABLED",
			Value: fmt.Sprintf("%t", sen.Spec.Access.EnableTLS),
		},
		{
			Name:  "SERVICE_TYPE",
			Value: string(sen.Spec.Access.ServiceType),
		},
		{
			Name:  "IP_FAMILY_PREFER",
			Value: string(sen.Spec.Access.IPFamilyPrefer),
		},
	}
	return valkeyEnvs
}

func buildAffinity(affinity *corev1.Affinity, labels map[string]string) *corev1.Affinity {
	if affinity != nil {
		return affinity
	}

	// Return anti-affinity
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					TopologyKey: builder.HostnameTopologyKey,
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: labels,
					},
				},
			},
		},
	}
}

func buildVolumeMounts(sen *v1alpha1.Sentinel, secretName string) []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      SentinelConfigVolumeName,
			MountPath: SentinelConfigVolumeMountPath,
		},
		{
			Name:      ValkeyDataVolumeName,
			MountPath: ValkeyDataVolumeMountPath,
		},
		{
			Name:      ValkeyOptName,
			MountPath: ValkeyOptMountPath,
		},
	}

	if sen.Spec.Access.EnableTLS {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      ValkeyTLSVolumeName,
			MountPath: ValkeyTLSVolumeMountPath,
		})
	}
	if secretName != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      ValkeyAuthName,
			MountPath: ValkeyAuthMountPath,
		})
	}
	return volumeMounts
}

func buildVolumes(sen *v1alpha1.Sentinel, secretName string) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: SentinelConfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: SentinelConfigMapName(sen.GetName()),
					},
					DefaultMode: ptr.To(int32(0400)),
				},
			},
		},
		{
			Name: ValkeyDataVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: ValkeyOptName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
	if sen.Spec.Access.EnableTLS {
		volumes = append(volumes, corev1.Volume{
			Name: ValkeyTLSVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  certbuilder.GenerateSSLSecretName(sen.Name),
					DefaultMode: ptr.To(int32(0400)),
				},
			},
		})
	}
	if secretName != "" {
		volumes = append(volumes, corev1.Volume{
			Name: ValkeyAuthName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  secretName,
					DefaultMode: ptr.To(int32(0400)),
				},
			},
		})
	}
	return volumes
}
