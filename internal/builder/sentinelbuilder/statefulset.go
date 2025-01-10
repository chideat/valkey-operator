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

	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
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
	SentinelConfigVolumeMountPath = "/conf"
	ValkeyTLSVolumeName           = "valkey-tls"
	ValkeyTLSVolumeMountPath      = "/tls"
	ValkeyDataVolumeName          = "data"
	ValkeyDataVolumeMountPath     = "/data"
	ValkeyAuthName                = "valkey-auth"
	ValkeyAuthMountPath           = "/account"
	ValkeyOptName                 = "valkey-opt"
	ValkeyOptMountPath            = "/opt"
	OperatorUsername              = "OPERATOR_USERNAME"
	OperatorSecretName            = "OPERATOR_SECRET_NAME"
	SentinelContainerName         = "sentinel"
	SentinelContainerPortName     = "sentinel"
	graceTime                     = 30
)

func NewSentinelStatefulset(sen types.SentinelInstance, selectors map[string]string) *appv1.StatefulSet {
	var (
		inst           = sen.Definition()
		passwordSecret = inst.Spec.Access.DefaultPasswordSecret
	)
	if len(selectors) == 0 {
		selectors = GenerateSelectorLabels(ValkeyArchRoleSEN, inst.GetName())
	} else {
		selectors = lo.Assign(selectors, GenerateSelectorLabels(ValkeyArchRoleSEN, inst.GetName()))
	}
	labels := lo.Assign(GetCommonLabels(inst.GetName()), selectors)

	startArgs := []string{"sh", "/opt/run_sentinel.sh"}
	shutdownArgs := []string{"sh", "-c", "/opt/valkey-tools sentinel shutdown &> /proc/1/fd/1"}
	volumes := getVolumes(inst, passwordSecret)
	volumeMounts := getValkeyVolumeMounts(inst, passwordSecret)
	envs := createValkeyContainerEnvs(inst)

	localhost := "127.0.0.1"
	if inst.Spec.Access.IPFamilyPrefer == corev1.IPv6Protocol {
		localhost = "::1"
	}
	ss := &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            GetSentinelStatefulSetName(inst.GetName()),
			Namespace:       inst.GetNamespace(),
			Labels:          labels,
			OwnerReferences: util.BuildOwnerReferences(inst),
		},
		Spec: appv1.StatefulSetSpec{
			ServiceName:         GetSentinelHeadlessServiceName(inst.GetName()),
			Replicas:            &inst.Spec.Replicas,
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
					Annotations: inst.Spec.PodAnnotations,
				},
				Spec: corev1.PodSpec{
					HostAliases: []corev1.HostAlias{
						{IP: localhost, Hostnames: []string{config.LocalInjectName}},
					},
					Containers: []corev1.Container{
						{
							Name:            SentinelContainerName,
							Command:         startArgs,
							Image:           inst.Spec.Image,
							ImagePullPolicy: builder.GetPullPolicy(inst.Spec.ImagePullPolicy),
							Env:             envs,
							Ports: []corev1.ContainerPort{
								{
									Name:          SentinelContainerPortName,
									ContainerPort: 26379,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							StartupProbe: &corev1.Probe{
								InitialDelaySeconds: 3,
								TimeoutSeconds:      5,
								FailureThreshold:    3,
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(26379),
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								InitialDelaySeconds: 10,
								TimeoutSeconds:      5,
								FailureThreshold:    5,
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(26379),
									},
								},
							},
							Resources: inst.Spec.Resources,
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.LifecycleHandler{
									Exec: &corev1.ExecAction{
										Command: shutdownArgs,
									},
								},
							},
							SecurityContext: builder.GetSecurityContext(inst.Spec.SecurityContext),
							VolumeMounts:    volumeMounts,
						},
					},
					ImagePullSecrets:              inst.Spec.ImagePullSecrets,
					SecurityContext:               builder.GetPodSecurityContext(inst.Spec.SecurityContext),
					ServiceAccountName:            clusterbuilder.ValkeyInstanceServiceAccountName,
					Affinity:                      getAffinity(inst.Spec.Affinity, selectors),
					Tolerations:                   inst.Spec.Tolerations,
					NodeSelector:                  inst.Spec.NodeSelector,
					Volumes:                       volumes,
					TerminationGracePeriodSeconds: ptr.To(int64(graceTime)),
				},
			},
		},
	}

	ss.Spec.Template.Spec.InitContainers = append(ss.Spec.Template.Spec.InitContainers, *buildInitContainer(inst, nil))
	ss.Spec.Template.Spec.Containers = append(ss.Spec.Template.Spec.Containers, *buildAgentContainer(inst, envs))

	if inst.Spec.Access.ServiceType == corev1.ServiceTypeNodePort ||
		inst.Spec.Access.ServiceType == corev1.ServiceTypeLoadBalancer {

		ss.Spec.Template.Spec.InitContainers = append(ss.Spec.Template.Spec.InitContainers, buildExposeContainer(inst))
	}
	return ss
}

func buildExposeContainer(inst *v1alpha1.Sentinel) corev1.Container {
	container := corev1.Container{
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
		Name:            "expose",
		Image:           config.GetValkeyToolsImage(inst),
		ImagePullPolicy: builder.GetPullPolicy(inst.Spec.ImagePullPolicy),
		VolumeMounts: []corev1.VolumeMount{
			{Name: ValkeyDataVolumeName, MountPath: ValkeyDataVolumeMountPath},
		},
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
			}, {
				Name:  "SENTINEL_ANNOUNCE_PATH",
				Value: "/data/announce.conf",
			},
			{
				Name:  "IP_FAMILY_PREFER",
				Value: string(inst.Spec.Access.IPFamilyPrefer),
			},
			{
				Name:  "SERVICE_TYPE",
				Value: string(inst.Spec.Access.ServiceType),
			},
		},
		Command:         []string{"/opt/valkey-tools", "sentinel", "expose"},
		SecurityContext: builder.GetSecurityContext(inst.Spec.SecurityContext),
	}
	return container
}

func buildInitContainer(inst *v1alpha1.Sentinel, _ []corev1.EnvVar) *corev1.Container {
	image := config.GetValkeyToolsImage(inst)
	if image == "" {
		return nil
	}

	return &corev1.Container{
		Name:            "init",
		Image:           image,
		ImagePullPolicy: builder.GetPullPolicy(inst.Spec.ImagePullPolicy),
		Command:         []string{"sh", "/opt/init_sentinel.sh"},
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
		VolumeMounts: []corev1.VolumeMount{
			{Name: ValkeyOptName, MountPath: path.Join("/mnt", ValkeyOptMountPath)},
		},
	}
}

func buildAgentContainer(inst *v1alpha1.Sentinel, envs []corev1.EnvVar) *corev1.Container {
	image := config.GetValkeyToolsImage(inst)
	if image == "" {
		return nil
	}
	container := corev1.Container{
		Name:            "agent",
		Image:           image,
		ImagePullPolicy: builder.GetPullPolicy(inst.Spec.ImagePullPolicy),
		Env:             envs,
		Command:         []string{"/opt/valkey-tools", "sentinel", "agent"},
		SecurityContext: builder.GetSecurityContext(inst.Spec.SecurityContext),
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

	if inst.Spec.Access.DefaultPasswordSecret != "" {
		vol := corev1.VolumeMount{
			Name:      ValkeyAuthName,
			MountPath: ValkeyAuthMountPath,
		}
		container.VolumeMounts = append(container.VolumeMounts, vol)
	}
	if inst.Spec.Access.EnableTLS {
		vol := corev1.VolumeMount{
			Name:      ValkeyTLSVolumeName,
			MountPath: ValkeyTLSVolumeMountPath,
		}
		container.VolumeMounts = append(container.VolumeMounts, vol)
	}
	return &container
}

func createValkeyContainerEnvs(inst *v1alpha1.Sentinel) []corev1.EnvVar {
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
			Name:  OperatorSecretName,
			Value: inst.Spec.Access.DefaultPasswordSecret,
		},
		{
			Name:  "TLS_ENABLED",
			Value: fmt.Sprintf("%t", inst.Spec.Access.EnableTLS),
		},
		{
			Name:  "SERVICE_TYPE",
			Value: string(inst.Spec.Access.ServiceType),
		},
		{
			Name:  "IP_FAMILY_PREFER",
			Value: string(inst.Spec.Access.IPFamilyPrefer),
		},
	}
	return valkeyEnvs
}

func getAffinity(affinity *corev1.Affinity, labels map[string]string) *corev1.Affinity {
	if affinity != nil {
		return affinity
	}

	// Return a SOFT anti-affinity
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						TopologyKey: builder.HostnameTopologyKey,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
					},
				},
			},
		},
	}
}

func getValkeyVolumeMounts(inst *v1alpha1.Sentinel, secretName string) []corev1.VolumeMount {
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

	if inst.Spec.Access.EnableTLS {
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

func getVolumes(inst *v1alpha1.Sentinel, secretName string) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: SentinelConfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: GetSentinelConfigMapName(inst.GetName()),
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
	if inst.Spec.Access.EnableTLS {
		volumes = append(volumes, corev1.Volume{
			Name: ValkeyTLSVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  builder.GetValkeySSLSecretName(inst.Name),
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
