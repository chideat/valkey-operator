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

package clusterbuilder

import (
	"crypto/sha1" // #nosec
	"fmt"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/aclbuilder"
	"github.com/chideat/valkey-operator/internal/builder/certbuilder"
	"github.com/chideat/valkey-operator/internal/builder/sabuilder"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/types/user"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	TLSVolumeName      = builder.ValkeyTLSVolumeName
	TLSVolumeMountPath = builder.ValkeyTLSVolumeDefaultMountPath

	StorageVolumeName      = builder.ValkeyDataVolumeName
	StorageVolumeMountPath = builder.ValkeyDataVolumeDefaultMountPath

	ValkeyTempVolumeName      = "temp"
	ValkeyTempVolumeMountPath = "/tmp"

	ValkeyPasswordVolumeName = "valkey-auth" //#nosec
	PasswordVolumeMountPath  = "/account"

	ConfigVolumeName      = "conf"
	ConfigVolumeMountPath = "/etc/valkey"

	ValkeyOptVolumeName      = "valkey-opt"
	ValkeyOptVolumeMountPath = "/opt"
)

const (
	probeDelaySeconds                          = 30
	defaultTerminationGracePeriodSeconds int64 = 300
)

func ClusterStatefulSetName(name string, i int) string {
	return fmt.Sprintf("%s-%s-%d", builder.ResourcePrefix(core.ValkeyCluster), name, i)
}

func buildEnvs(inst types.ClusterInstance, headlessSvcName string) []corev1.EnvVar {
	var (
		cluster          = inst.Definition()
		users            = inst.Users()
		opUser           = users.GetOpUser()
		aclConfigMapName = aclbuilder.GenerateACLConfigMapName(inst.Arch(), cluster.GetName())
	)

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
			Name:  "IP_FAMILY_PREFER",
			Value: string(cluster.Spec.Access.IPFamilyPrefer),
		},
		{
			Name:  "SERVICE_TYPE",
			Value: string(cluster.Spec.Access.ServiceType),
		},
		{
			Name:  "SERVICE_NAME",
			Value: string(headlessSvcName),
		},
	}

	if cluster.Spec.Access.EnableTLS {
		envs = append(envs, certbuilder.GenerateTLSEnvs()...)
	}
	return envs
}

// GenerateStatefulSet creates a new StatefulSet for the given Cluster.
func GenerateStatefulSet(inst types.ClusterInstance, index int) (*appsv1.StatefulSet, error) {
	var (
		cluster = inst.Definition()
		spec    = cluster.Spec
		users   = inst.Users()
		opUser  = users.GetOpUser()
	)

	var (
		stsName         = ClusterStatefulSetName(cluster.Name, index)
		selectors       = GenerateClusterStatefulSetSelectors(cluster.Name, index)
		labels          = GenerateClusterStatefulSetLabels(cluster.Name, index)
		headlessSvcName = ClusterHeadlessSvcName(cluster.Name, index)
		initContainers  []corev1.Container
		containers      []corev1.Container
	)

	envs := buildEnvs(inst, headlessSvcName)

	// persistent shard id
	data := sha1.Sum(fmt.Appendf([]byte{}, "%s/%s", cluster.Namespace, stsName)) // #nosec
	shardId := fmt.Sprintf("%x", data)
	envs = append(envs, corev1.EnvVar{
		Name:  "SHARD_ID",
		Value: shardId,
	})
	if container := buildValkeyDataInitContainer(cluster, opUser, envs); container != nil {
		initContainers = append(initContainers, *container)
	}
	containers = append(containers, buildValkeyServerContainer(cluster, opUser, envs, index))

	if exporter := builder.BuildExporterContainer(inst,
		cluster.Spec.Exporter, opUser, cluster.Spec.Access.EnableTLS); exporter != nil {
		containers = append(containers, *exporter)
	}
	if container := buildValkeyAgentContainer(cluster, opUser, envs); container != nil {
		containers = append(containers, *container)
	}

	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            stsName,
			Namespace:       cluster.Namespace,
			Labels:          labels,
			OwnerReferences: util.BuildOwnerReferences(cluster),
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName:         headlessSvcName,
			Replicas:            ptr.To(spec.Replicas.ReplicasOfShard),
			PodManagementPolicy: appsv1.ParallelPodManagement,
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			Selector: &metav1.LabelSelector{MatchLabels: selectors},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: cluster.Spec.PodAnnotations,
				},
				Spec: corev1.PodSpec{
					InitContainers: initContainers,
					Containers:     containers,
					HostAliases: []corev1.HostAlias{
						builder.LocalhostAlias(cluster.Spec.Access.IPFamilyPrefer),
					},
					ImagePullSecrets:              spec.ImagePullSecrets,
					SecurityContext:               builder.GetPodSecurityContext(spec.SecurityContext),
					ServiceAccountName:            sabuilder.ValkeyInstanceServiceAccountName,
					Tolerations:                   spec.Tolerations,
					NodeSelector:                  spec.NodeSelector,
					Affinity:                      buildAffinity(cluster, selectors, stsName),
					TerminationGracePeriodSeconds: ptr.To(defaultTerminationGracePeriodSeconds),
					Volumes:                       buildValkeyVolumes(cluster, opUser),
				},
			},
			VolumeClaimTemplates: buildPersistentClaims(cluster, selectors),
		},
	}
	return ss, nil
}

func buildPersistentClaims(cluster *v1alpha1.Cluster, labels map[string]string) (ret []corev1.PersistentVolumeClaim) {
	if cluster.Spec.Storage == nil {
		return nil
	}

	sc := cluster.Spec.Storage.StorageClassName
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
					corev1.ResourceStorage: *cluster.Spec.Storage.Capacity,
				},
			},
			StorageClassName: sc,
			VolumeMode:       &mode,
		},
	})
	if cluster.Spec.Storage.RetainAfterDeleted {
		for _, vc := range ret {
			vc.OwnerReferences = util.BuildOwnerReferences(cluster)
		}
	}
	return
}

func buildValkeyServerContainer(cluster *v1alpha1.Cluster, u *user.User, envs []corev1.EnvVar, index int) corev1.Container {
	startArgs := []string{"sh", "/opt/run_cluster.sh"}

	container := corev1.Container{
		Name:            builder.ServerContainerName,
		Env:             envs,
		Image:           cluster.Spec.Image,
		ImagePullPolicy: builder.GetPullPolicy(cluster.Spec.ImagePullPolicy),
		Command:         startArgs,
		Ports: []corev1.ContainerPort{
			{
				Name:          "server",
				ContainerPort: builder.DefaultValkeyServerPort,
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          "gossip",
				ContainerPort: builder.DefaultValkeyServerBusPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		StartupProbe: &corev1.Probe{
			InitialDelaySeconds: probeDelaySeconds,
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
			PeriodSeconds:       10,
			TimeoutSeconds:      10,
			SuccessThreshold:    1,
			FailureThreshold:    3,
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(builder.DefaultValkeyServerPort),
				},
			},
		},
		ReadinessProbe: &corev1.Probe{
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
			TimeoutSeconds:      5,
			FailureThreshold:    3,
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(builder.DefaultValkeyServerPort),
				},
			},
		},
		Resources: cluster.Spec.Resources,
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.LifecycleHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"sh", "-inst", "/opt/valkey-helper cluster shutdown  &> /proc/1/fd/1"},
				},
			},
		},
		SecurityContext: builder.GetSecurityContext(cluster.Spec.SecurityContext),
		VolumeMounts:    buildVolumeMounts(cluster, u),
	}
	return container
}

func buildValkeyDataInitContainer(cluster *v1alpha1.Cluster, user *user.User, envs []corev1.EnvVar) *corev1.Container {
	image := config.GetValkeyHelperImage(cluster)
	if image == "" {
		return nil
	}

	initContainer := corev1.Container{
		Name:            "init",
		Image:           image,
		ImagePullPolicy: builder.GetPullPolicy(cluster.Spec.ImagePullPolicy),
		Env:             envs,
		Command:         []string{"sh", "-c", "/opt/init_cluster.sh"},
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
			{Name: StorageVolumeName, MountPath: StorageVolumeMountPath},
			{Name: ValkeyOptVolumeName, MountPath: "/mnt/opt"},
			{Name: ValkeyTempVolumeName, MountPath: ValkeyTempVolumeMountPath},
		},
	}

	if user.Password.GetSecretName() != "" {
		mount := corev1.VolumeMount{
			Name:      ValkeyPasswordVolumeName,
			MountPath: PasswordVolumeMountPath,
		}
		initContainer.VolumeMounts = append(initContainer.VolumeMounts, mount)
	}

	if cluster.Spec.Access.EnableTLS {
		mount := corev1.VolumeMount{
			Name:      TLSVolumeName,
			MountPath: TLSVolumeMountPath,
		}
		initContainer.VolumeMounts = append(initContainer.VolumeMounts, mount)
	}
	return &initContainer
}

func buildValkeyAgentContainer(cluster *v1alpha1.Cluster, user *user.User, envs []corev1.EnvVar) *corev1.Container {
	image := config.GetValkeyHelperImage(cluster)
	if image == "" {
		return nil
	}

	container := corev1.Container{
		Name:            "agent",
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Env:             envs,
		Command:         []string{"/opt/valkey-helper", "runner", "cluster", "--sync-l2c"},
		SecurityContext: builder.GetSecurityContext(cluster.Spec.SecurityContext),
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
			{Name: StorageVolumeName, MountPath: StorageVolumeMountPath, ReadOnly: true},
		},
	}

	if user.Password.GetSecretName() != "" {
		mount := corev1.VolumeMount{
			Name:      ValkeyPasswordVolumeName,
			MountPath: PasswordVolumeMountPath,
		}
		container.VolumeMounts = append(container.VolumeMounts, mount)
	}

	if cluster.Spec.Access.EnableTLS {
		mount := corev1.VolumeMount{
			Name:      TLSVolumeName,
			MountPath: TLSVolumeMountPath,
		}
		container.VolumeMounts = append(container.VolumeMounts, mount)
	}
	return &container
}

func buildVolumeMounts(cluster *v1alpha1.Cluster, user *user.User) []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		{Name: ConfigVolumeName, MountPath: ConfigVolumeMountPath},
		{Name: StorageVolumeName, MountPath: StorageVolumeMountPath},
		{Name: ValkeyOptVolumeName, MountPath: ValkeyOptVolumeMountPath},
		{Name: ValkeyTempVolumeName, MountPath: ValkeyTempVolumeMountPath},
	}
	if user.Password.GetSecretName() != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      ValkeyPasswordVolumeName,
			MountPath: PasswordVolumeMountPath,
		})
	}
	if cluster.Spec.Access.EnableTLS {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      TLSVolumeName,
			MountPath: TLSVolumeMountPath,
		})
	}
	return volumeMounts
}

func buildValkeyVolumes(cluster *v1alpha1.Cluster, user *user.User) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: ConfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: ValkeyConfigMapName(cluster.Name),
					},
					DefaultMode: ptr.To(int32(0400)),
				},
			},
		},
		{
			Name: ValkeyOptVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
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
	}

	if user != nil && user.Password.GetSecretName() != "" {
		volumes = append(volumes, corev1.Volume{
			Name: ValkeyPasswordVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: user.Password.GetSecretName(),
				},
			},
		})
	}

	if cluster.Spec.Access.EnableTLS {
		volumes = append(volumes, corev1.Volume{
			Name: TLSVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: certbuilder.GenerateSSLSecretName(cluster.Name),
				},
			},
		})
	}

	if cluster.Spec.Storage == nil {
		volumes = append(volumes, corev1.Volume{
			Name: StorageVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}
	return volumes
}

func buildAffinity(cluster *v1alpha1.Cluster, selectors map[string]string, ssName string) *corev1.Affinity {
	if cluster.Spec.AffinityPolicy == nil {
		cluster.Spec.AffinityPolicy = ptr.To(core.AntiAffinityInShard)
	}

	switch *cluster.Spec.AffinityPolicy {
	case core.CustomAffinity:
		return cluster.Spec.CustomAffinity
	case core.AntiAffinity:
		return &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						TopologyKey: builder.HostnameTopologyKey,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: selectors,
						},
					},
				},
			},
		}
	case core.SoftAntiAffinity:
		// return a SOFT anti-affinity by default
		return &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 80,
						PodAffinityTerm: corev1.PodAffinityTerm{
							TopologyKey: builder.HostnameTopologyKey,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: selectors,
							},
						},
					},
					{
						Weight: 20,
						PodAffinityTerm: corev1.PodAffinityTerm{
							TopologyKey: builder.HostnameTopologyKey,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: selectors,
							},
						},
					},
				},
			},
		}
	default:
		return &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						TopologyKey: builder.HostnameTopologyKey,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: selectors,
						},
					},
				},
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 100,
						PodAffinityTerm: corev1.PodAffinityTerm{
							TopologyKey: builder.HostnameTopologyKey,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: selectors,
							},
						},
					},
				},
			},
		}
	}
}
