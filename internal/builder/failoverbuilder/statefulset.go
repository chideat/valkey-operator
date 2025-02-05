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

package failoverbuilder

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/chideat/valkey-operator/api/v1alpha1"
	v1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/aclbuilder"
	"github.com/chideat/valkey-operator/internal/builder/certbuilder"
	"github.com/chideat/valkey-operator/internal/builder/sabuilder"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/types/user"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	StorageVolumeName      = builder.ValkeyDataVolumeName
	StorageVolumeMountPath = builder.ValkeyDataVolumeDefaultMountPath

	ConfigVolumeName      = "conf"
	ConfigVolumeMountPath = "/valkey"

	ValkeyTempVolumeName      = "temp"
	ValkeyTempVolumeMountPath = "/tmp"

	TLSVolumeName      = builder.ValkeyTLSVolumeName
	TLSVolumeMountPath = builder.ValkeyTLSVolumeDefaultMountPath

	ValkeyPasswordVolumeName = "valkey-auth" //#nosec
	PasswordVolumeMountPath  = "/account"

	ValkeyOptVolumeName      = "valkey-opt"
	ValkeyOptVolumeMountPath = "/opt"

	MonitorOperatorSecretName = "MONITOR_OPERATOR_SECRET_NAME"
)

const (
	probeDelaySeconds                          = 30
	defaultTerminationGracePeriodSeconds int64 = 300
)

func FailoverStatefulSetName(sentinelName string) string {
	return fmt.Sprintf("rfr-%s", sentinelName)
}

func GenerateStatefulSet(inst types.FailoverInstance) (*appv1.StatefulSet, error) {
	var (
		rf = inst.Definition()

		users            = inst.Users()
		opUser           = users.GetOpUser()
		aclConfigMapName = aclbuilder.GenerateACLConfigMapName(inst.Arch(), rf.Name)
	)

	var (
		selectors = GenerateSelectorLabels(rf.Name)
		labels    = GenerateCommonLabels(rf.Name)

		initContainers []corev1.Container
		containers     []corev1.Container
	)

	envs := buildEnvs(inst, opUser, aclConfigMapName)
	if initContainer, err := buildValkeyDataInitContainer(rf); err != nil {
		return nil, err
	} else {
		initContainers = append(initContainers, *initContainer)
	}
	if cont, err := buildValkeyServerContainer(inst, opUser, envs); err != nil {
		return nil, err
	} else {
		containers = append(containers, *cont)
	}
	if rf.Spec.Exporter != nil {
		if exporter := builder.BuildExporterContainer(inst, rf.Spec.Exporter, opUser, rf.Spec.Access.EnableTLS); exporter != nil {
			containers = append(containers, *exporter)
		}
	}

	ss := &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            FailoverStatefulSetName(rf.Name),
			Namespace:       rf.Namespace,
			Labels:          labels,
			OwnerReferences: util.BuildOwnerReferences(rf),
		},
		Spec: appv1.StatefulSetSpec{
			ServiceName:         FailoverStatefulSetName(rf.Name),
			Replicas:            ptr.To(rf.Spec.Replicas),
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
					Annotations: rf.Spec.PodAnnotations,
				},
				Spec: corev1.PodSpec{
					HostAliases: []corev1.HostAlias{
						builder.LocalhostAlias(rf.Spec.Access.IPFamilyPrefer),
					},
					InitContainers:                initContainers,
					Containers:                    containers,
					ImagePullSecrets:              rf.Spec.ImagePullSecrets,
					Affinity:                      buildAffinity(rf.Spec.Affinity, selectors),
					Tolerations:                   rf.Spec.Tolerations,
					NodeSelector:                  rf.Spec.NodeSelector,
					ServiceAccountName:            sabuilder.ValkeyInstanceServiceAccountName,
					SecurityContext:               builder.GetPodSecurityContext(rf.Spec.SecurityContext),
					TerminationGracePeriodSeconds: ptr.To(defaultTerminationGracePeriodSeconds),
					Volumes:                       buildValkeyVolumes(inst, opUser),
				},
			},
			VolumeClaimTemplates: buildPersistentClaims(rf, labels),
		},
	}
	return ss, nil
}

func buildEnvs(inst types.FailoverInstance, opUser *user.User, aclConfigMapName string) []corev1.EnvVar {
	rf := inst.Definition()

	var monitorUri string
	if inst.Monitor().Policy() == v1.SentinelFailoverPolicy {
		var sentinelNodes []string
		for _, node := range rf.Status.Monitor.Nodes {
			sentinelNodes = append(sentinelNodes, net.JoinHostPort(node.IP, strconv.Itoa(int(node.Port))))
		}
		monitorUri = fmt.Sprintf("sentinel://%s", strings.Join(sentinelNodes, ","))
	}

	envs := []corev1.EnvVar{
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
			Name:  "ACL_CONFIGMAP_NAME",
			Value: aclConfigMapName,
		},
		{
			Name:  builder.OperatorUsername,
			Value: opUser.Name,
		},
		{
			Name:  builder.OperatorSecretName,
			Value: opUser.GetPassword().GetSecretName(),
		},
		{
			Name:  "SERVICE_TYPE",
			Value: string(rf.Spec.Access.ServiceType),
		},
		{
			Name:  "IP_FAMILY_PREFER",
			Value: string(rf.Spec.Access.IPFamilyPrefer),
		},
		{
			Name:  "MONITOR_POLICY",
			Value: string(inst.Monitor().Policy()),
		},
		{
			Name:  "MONITOR_URI",
			Value: monitorUri,
		},
		{
			Name:  MonitorOperatorSecretName,
			Value: rf.Status.Monitor.PasswordSecret,
		},
	}

	if rf.Spec.Access.EnableTLS {
		envs = append(envs, certbuilder.GenerateTLSEnvs()...)
	}
	return envs
}

func buildValkeyServerContainer(inst types.FailoverInstance, user *user.User, envs []corev1.EnvVar) (*corev1.Container, error) {
	var (
		rf       = inst.Definition()
		startCmd = []string{"sh", "/opt/run_failover.sh"}
	)
	container := corev1.Container{
		Name:            builder.ServerContainerName,
		Image:           rf.Spec.Image,
		ImagePullPolicy: builder.GetPullPolicy(rf.Spec.ImagePullPolicy),
		Env:             envs,
		Ports: []corev1.ContainerPort{
			{
				Name:          "server",
				ContainerPort: 6379,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Command: startCmd,
		StartupProbe: &corev1.Probe{
			InitialDelaySeconds: probeDelaySeconds,
			TimeoutSeconds:      5,
			FailureThreshold:    10,
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(builder.DefaultValkeyServerPort),
				},
			},
		},
		ReadinessProbe: &corev1.Probe{
			InitialDelaySeconds: 1,
			TimeoutSeconds:      5,
			FailureThreshold:    5,
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(builder.DefaultValkeyServerPort),
				},
			},
		},
		LivenessProbe: &corev1.Probe{
			InitialDelaySeconds: 10,
			TimeoutSeconds:      5,
			FailureThreshold:    5,
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(builder.DefaultValkeyServerPort),
				},
			},
		},
		Resources: rf.Spec.Resources,
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.LifecycleHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"/bin/sh", "-c", "/opt/valkey-helper failover shutdown &> /proc/1/fd/1"},
				},
			},
		},
		SecurityContext: builder.GetSecurityContext(rf.Spec.SecurityContext),
		VolumeMounts:    buildVolumeMounts(rf, user),
	}
	return &container, nil
}

func buildPersistentClaims(rf *v1alpha1.Failover, labels map[string]string) (ret []corev1.PersistentVolumeClaim) {
	if rf.Spec.Storage == nil {
		return nil
	}

	sc := rf.Spec.Storage.StorageClassName
	mode := corev1.PersistentVolumeFilesystem
	ret = append(ret, corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:   StorageVolumeName,
			Labels: labels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: rf.Spec.Storage.Capacity,
				},
			},
			StorageClassName: sc,
			VolumeMode:       &mode,
		},
	})
	if rf.Spec.Storage.RetainAfterDeleted {
		for _, vc := range ret {
			vc.OwnerReferences = util.BuildOwnerReferences(rf)
		}
	}
	return
}

func buildValkeyDataInitContainer(rf *v1alpha1.Failover) (*corev1.Container, error) {
	image := config.GetValkeyHelperImage(rf)
	if image == "" {
		return nil, fmt.Errorf("valkey-helper image is not set")
	}

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
		Name:            "init",
		Image:           image,
		ImagePullPolicy: builder.GetPullPolicy(rf.Spec.ImagePullPolicy),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      ValkeyOptVolumeName,
				MountPath: "/mnt/opt/",
			},
			{
				Name:      StorageVolumeName,
				MountPath: StorageVolumeMountPath,
			},
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
			},
			{
				Name:  "SENTINEL_ANNOUNCE_PATH",
				Value: "/data/announce.conf",
			},
			{
				Name:  "IP_FAMILY_PREFER",
				Value: string(rf.Spec.Access.IPFamilyPrefer),
			},
			{
				Name:  "SERVICE_TYPE",
				Value: string(rf.Spec.Access.ServiceType),
			},
		},
		Command:         []string{"sh", "/opt/init_failover.sh"},
		SecurityContext: builder.GetSecurityContext(rf.Spec.SecurityContext),
	}
	return &container, nil
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

func buildVolumeMounts(rf *v1alpha1.Failover, user *user.User) []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      ConfigVolumeName,
			MountPath: ConfigVolumeMountPath,
		},
		{
			Name:      StorageVolumeName,
			MountPath: StorageVolumeMountPath,
		},
		{
			Name:      ValkeyTempVolumeName,
			MountPath: ValkeyTempVolumeMountPath,
		},
		{
			Name:      ValkeyOptVolumeName,
			MountPath: ValkeyOptVolumeMountPath,
		},
	}

	if rf.Spec.Access.EnableTLS {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      TLSVolumeName,
			MountPath: TLSVolumeMountPath,
		})
	}
	if user != nil && user.GetPassword().GetSecretName() != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      ValkeyPasswordVolumeName,
			MountPath: PasswordVolumeMountPath,
		})
	}
	return volumeMounts
}

func buildValkeyVolumes(inst types.FailoverInstance, user *user.User) []corev1.Volume {
	rf := inst.Definition()

	volumes := []corev1.Volume{
		{
			Name: ConfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: ConfigMapName(rf.Name),
					},
					DefaultMode: ptr.To(int32(0400)),
				},
			},
		},
		{
			Name: ValkeyTempVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium:    corev1.StorageMediumMemory,
					SizeLimit: resource.NewQuantity(1<<20, resource.BinarySI), //1Mi
				},
			},
		},
		{
			Name: ValkeyOptVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	if secretName := user.Password.GetSecretName(); secretName != "" {
		volumes = append(volumes, corev1.Volume{
			Name: ValkeyPasswordVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		})
	}

	if rf.Spec.Access.EnableTLS {
		volumes = append(volumes, corev1.Volume{
			Name: TLSVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: certbuilder.GenerateSSLSecretName(rf.Name),
				},
			},
		})
	}
	if rf.Spec.Storage == nil {
		volumes = append(volumes, corev1.Volume{
			Name: StorageVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}
	return volumes
}
