/*
Copyright 2023 The RedisOperator Authors.

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
	"path"
	"strconv"
	"strings"

	"github.com/chideat/valkey-operator/api/v1alpha1"
	v1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/types/user"
	"github.com/samber/lo"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	redisStorageVolumeName       = "redis-data"
	exporterContainerName        = "redis-exporter"
	graceTime                    = 30
	PasswordENV                  = "REDIS_PASSWORD"
	redisConfigurationVolumeName = "redis-config"
	RedisTmpVolumeName           = "redis-tmp"
	RedisTLSVolumeName           = "redis-tls"
	redisAuthName                = "redis-auth"
	redisStandaloneVolumeName    = "redis-standalone"
	redisOptName                 = "redis-opt"
	OperatorUsername             = "OPERATOR_USERNAME"
	OperatorSecretName           = "OPERATOR_SECRET_NAME"
	MonitorOperatorSecretName    = "MONITOR_OPERATOR_SECRET_NAME"
	ServerContainerName          = "redis"
)

func GetRedisRWServiceName(failoverName string) string {
	return fmt.Sprintf("rfr-%s-read-write", failoverName)
}

func GetRedisROServiceName(failoverName string) string {
	return fmt.Sprintf("rfr-%s-read-only", failoverName)
}

func GenerateRedisStatefulSet(inst types.FailoverInstance, selectors map[string]string, isAllACLSupported bool) *appv1.StatefulSet {

	var (
		rf = inst.Definition()

		users            = inst.Users()
		opUser           = users.GetOpUser()
		aclConfigMapName = GenerateFailoverACLConfigMapName(rf.Name)
	)
	if opUser.Role == user.RoleOperator && !isAllACLSupported {
		opUser = users.GetDefaultUser()
	}
	if !inst.Version().IsACLSupported() {
		aclConfigMapName = ""
	}

	if len(selectors) == 0 {
		selectors = lo.Assign(GetCommonLabels(rf.Name), GenerateSelectorLabels(RedisArchRoleRedis, rf.Name))
	} else {
		selectors = lo.Assign(selectors, GenerateSelectorLabels(RedisArchRoleRedis, rf.Name))
	}
	labels := lo.Assign(GetCommonLabels(rf.Name), GenerateSelectorLabels(RedisArchRoleRedis, rf.Name), selectors)

	secretName := rf.Spec.Access.DefaultPasswordSecret
	if opUser.GetPassword() != nil {
		secretName = opUser.GetPassword().SecretName
	}
	redisCommand := getRedisCommand(rf)
	volumeMounts := getRedisVolumeMounts(rf, secretName)
	volumes := getRedisVolumes(inst, rf, secretName)

	localhost := "127.0.0.1"
	if rf.Spec.Access.IPFamilyPrefer == corev1.IPv6Protocol {
		localhost = "::1"
	}
	ss := &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            GetFailoverStatefulSetName(rf.Name),
			Namespace:       rf.Namespace,
			Labels:          labels,
			OwnerReferences: util.BuildOwnerReferences(rf),
		},
		Spec: appv1.StatefulSetSpec{
			ServiceName:         GetFailoverStatefulSetName(rf.Name),
			Replicas:            &rf.Spec.Replicas,
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
						{IP: localhost, Hostnames: []string{config.LocalInjectName}},
					},
					ServiceAccountName:            clusterbuilder.RedisInstanceServiceAccountName,
					Affinity:                      getAffinity(rf.Spec.Affinity, selectors),
					Tolerations:                   rf.Spec.Tolerations,
					NodeSelector:                  rf.Spec.NodeSelector,
					SecurityContext:               builder.GetPodSecurityContext(rf.Spec.SecurityContext),
					ImagePullSecrets:              rf.Spec.ImagePullSecrets,
					TerminationGracePeriodSeconds: ptr.To(clusterbuilder.DefaultTerminationGracePeriodSeconds),
					Containers: []corev1.Container{
						{
							Name:            "redis",
							Image:           rf.Spec.Image,
							ImagePullPolicy: builder.GetPullPolicy(rf.Spec.ImagePullPolicy),
							Env:             createRedisContainerEnvs(inst, opUser, aclConfigMapName),
							Ports: []corev1.ContainerPort{
								{
									Name:          "redis",
									ContainerPort: 6379,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: volumeMounts,
							Command:      redisCommand,
							StartupProbe: &corev1.Probe{
								InitialDelaySeconds: graceTime,
								TimeoutSeconds:      5,
								FailureThreshold:    10,
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(6379),
									},
								},
							},
							ReadinessProbe: &corev1.Probe{
								InitialDelaySeconds: 1,
								TimeoutSeconds:      5,
								FailureThreshold:    5,
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(6379),
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								InitialDelaySeconds: 10,
								TimeoutSeconds:      5,
								FailureThreshold:    5,
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(6379),
									},
								},
							},
							Resources: rf.Spec.Resources,
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.LifecycleHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"/bin/sh", "-c", "/opt/redis-tools failover shutdown &> /proc/1/fd/1"},
									},
								},
							},
							SecurityContext: builder.GetSecurityContext(rf.Spec.SecurityContext),
						},
					},
					InitContainers: []corev1.Container{
						createExposeContainer(rf),
					},
					Volumes: volumes,
				},
			},
		},
	}

	if storage := rf.Spec.Storage; storage != nil {
		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: getRedisDataVolumeName(rf),
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: rf.Spec.Storage.Capacity,
					},
				},
				StorageClassName: storage.StorageClassName,
			},
		}
		if storage.AccessMode != "" {
			pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{storage.AccessMode}
		}
		if !storage.RetainAfterDeleted {
			// Set an owner reference so the persistent volumes are deleted when the rc is
			pvc.OwnerReferences = util.BuildOwnerReferences(rf)
		}
		ss.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{pvc}
	}

	if inst.IsStandalone() && NeedStandaloneInit(rf) {
		ss.Spec.Template.Spec.InitContainers = append(ss.Spec.Template.Spec.InitContainers, createStandaloneInitContainer(rf))
	}

	if rf.Spec.Exporter.Enabled {
		defaultAnnotations := map[string]string{
			"prometheus.io/scrape": "true",
			"prometheus.io/port":   "http",
			"prometheus.io/path":   "/metrics",
		}
		ss.Spec.Template.Annotations = lo.Assign(ss.Spec.Template.Annotations, defaultAnnotations)

		exporter := createRedisExporterContainer(rf, opUser)
		ss.Spec.Template.Spec.Containers = append(ss.Spec.Template.Spec.Containers, exporter)
	}
	return ss
}

func createExposeContainer(rf *v1alpha1.Failover) corev1.Container {
	image := config.GetRedisToolsImage(rf)
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
				Name:      redisOptName,
				MountPath: "/mnt/opt/",
			},
			{
				Name:      getRedisDataVolumeName(rf),
				MountPath: "/data",
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
	return container
}

func createRedisContainerEnvs(inst types.FailoverInstance, opUser *user.User, aclConfigMapName string) []corev1.EnvVar {
	rf := inst.Definition()

	var monitorUri string
	if inst.Monitor().Policy() == v1.SentinelFailoverPolicy {
		var sentinelNodes []string
		for _, node := range rf.Status.Monitor.Nodes {
			sentinelNodes = append(sentinelNodes, net.JoinHostPort(node.IP, strconv.Itoa(int(node.Port))))
		}
		monitorUri = fmt.Sprintf("sentinel://%s", strings.Join(sentinelNodes, ","))
	}

	redisEnvs := []corev1.EnvVar{
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
			Name:  "SERVICE_NAME",
			Value: GetFailoverStatefulSetName(rf.Name),
		},
		{
			Name: "ACL_ENABLED",
			// isAllACLSupported used to make sure the account is consistent with eachother
			Value: fmt.Sprintf("%t", opUser.Role == user.RoleOperator),
		},
		{
			Name:  "ACL_CONFIGMAP_NAME",
			Value: aclConfigMapName,
		},
		{
			Name:  OperatorUsername,
			Value: opUser.Name,
		},
		{
			Name:  OperatorSecretName,
			Value: opUser.GetPassword().GetSecretName(),
		},
		{
			Name:  "TLS_ENABLED",
			Value: fmt.Sprintf("%t", rf.Spec.Access.EnableTLS),
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
	return redisEnvs
}

func createStandaloneInitContainer(rf *v1alpha1.Failover) corev1.Container {
	tmpPath := "/tmp-data"
	filepath := rf.Annotations[AnnotationStandaloneLoadFilePath]
	targetFile := ""
	if strings.HasSuffix(filepath, ".rdb") {
		targetFile = "/data/dump.rdb"
	} else {
		targetFile = "/data/appendonly.aof"
	}

	filepath = path.Join(tmpPath, filepath)
	command := fmt.Sprintf("if [ -e '%s' ]; then", targetFile) + "\n" +
		"echo 'redis storage file exist,skip' \n" +
		"else \n" +
		fmt.Sprintf("echo 'copy redis storage file' && cp %s %s ", filepath, targetFile) +
		fmt.Sprintf("&& chown 999:1000 %s ", targetFile) +
		fmt.Sprintf("&& chmod 644 %s \n", targetFile) +
		"fi"

	container := corev1.Container{
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
		Name:            "standalone-pod",
		Image:           config.GetRedisToolsImage(rf),
		ImagePullPolicy: builder.GetPullPolicy(rf.Spec.ImagePullPolicy),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      getRedisDataVolumeName(rf),
				MountPath: "/data",
			},
			{
				Name:      redisStandaloneVolumeName,
				MountPath: tmpPath,
			},
		},
		Command: []string{"sh", "-c", command},
		SecurityContext: &corev1.SecurityContext{
			Privileged: ptr.To(false),
		},
	}

	if rf.Annotations[AnnotationStandaloneInitStorage] == "hostpath" {
		container.SecurityContext = &corev1.SecurityContext{
			Privileged:   ptr.To(false),
			RunAsGroup:   ptr.To(int64(0)),
			RunAsUser:    ptr.To(int64(0)),
			RunAsNonRoot: ptr.To(false),
		}
	}
	return container
}

func createRedisExporterContainer(rf *v1alpha1.Failover, opUser *user.User) corev1.Container {
	var (
		username string
		secret   string
	)
	if opUser != nil {
		username = opUser.Name
		secret = opUser.GetPassword().GetSecretName()
		if username == "default" {
			username = ""
		}
	}

	container := corev1.Container{
		Name:            exporterContainerName,
		Command:         []string{"/redis_exporter"},
		Image:           rf.Spec.Exporter.Image,
		ImagePullPolicy: builder.GetPullPolicy(rf.Spec.Exporter.ImagePullPolicy, rf.Spec.ImagePullPolicy),
		Env: []corev1.EnvVar{
			{
				Name: "REDIS_ALIAS",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			{
				Name:  "REDIS_USER",
				Value: username,
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "metrics",
				ContainerPort: 9121,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("200Mi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
		},
		SecurityContext: builder.GetSecurityContext(rf.Spec.SecurityContext),
	}

	if secret != "" {
		container.Env = append(container.Env, corev1.EnvVar{
			Name: PasswordENV,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "password",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secret,
					},
				},
			},
		})
	}

	if rf.Spec.Access.EnableTLS {
		container.VolumeMounts = append(container.VolumeMounts,
			corev1.VolumeMount{Name: RedisTLSVolumeName, MountPath: "/tls"},
		)
		container.Env = append(container.Env, []corev1.EnvVar{
			{
				Name:  "REDIS_EXPORTER_TLS_CLIENT_KEY_FILE",
				Value: "/tls/tls.key",
			},
			{
				Name:  "REDIS_EXPORTER_TLS_CLIENT_CERT_FILE",
				Value: "/tls/tls.crt",
			},
			{
				Name:  "REDIS_EXPORTER_TLS_CA_CERT_FILE",
				Value: "/tls/ca.crt",
			},
			{
				Name:  "REDIS_EXPORTER_SKIP_TLS_VERIFICATION",
				Value: "true",
			},
			{
				Name:  "REDIS_ADDR",
				Value: fmt.Sprintf("rediss://%s:6379", config.LocalInjectName),
			},
		}...)
	} else {
		container.Env = append(container.Env, []corev1.EnvVar{
			{Name: "REDIS_ADDR",
				Value: fmt.Sprintf("redis://%s:6379", config.LocalInjectName)},
		}...)
	}
	return container
}

func getRedisCommand(rf *v1alpha1.Failover) []string {
	cmds := []string{"sh", "/opt/run_failover.sh"}
	return cmds
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

func getRedisVolumeMounts(rf *v1alpha1.Failover, secret string) []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      redisConfigurationVolumeName,
			MountPath: "/redis",
		},
		{
			Name:      getRedisDataVolumeName(rf),
			MountPath: "/data",
		},
		{
			Name:      RedisTmpVolumeName,
			MountPath: "/tmp",
		},
		{
			Name:      redisOptName,
			MountPath: "/opt",
		},
	}

	if rf.Spec.Access.EnableTLS {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      RedisTLSVolumeName,
			MountPath: "/tls",
		})
	}
	if rf.Spec.Access.DefaultPasswordSecret != "" || secret != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      redisAuthName,
			MountPath: "/account",
		})
	}
	return volumeMounts
}

func getRedisDataVolumeName(_ *v1alpha1.Failover) string {
	return redisStorageVolumeName
}

func getRedisVolumes(inst types.FailoverInstance, rf *v1alpha1.Failover, secretName string) []corev1.Volume {
	executeMode := int32(0400)
	configname := GetRedisConfigMapName(rf)
	volumes := []corev1.Volume{
		{
			Name: redisConfigurationVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configname,
					},
					DefaultMode: &executeMode,
				},
			},
		},
		{
			Name: RedisTmpVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium:    corev1.StorageMediumMemory,
					SizeLimit: resource.NewQuantity(1<<20, resource.BinarySI), //1Mi
				},
			},
		},
		{
			Name: redisOptName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	if dataVolume := getRedisDataVolume(rf); dataVolume != nil {
		volumes = append(volumes, *dataVolume)
	}
	if rf.Spec.Access.EnableTLS {
		volumes = append(volumes, corev1.Volume{
			Name: RedisTLSVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: builder.GetRedisSSLSecretName(rf.Name),
				},
			},
		})
	}
	if secretName != "" {
		volumes = append(volumes, corev1.Volume{
			Name: redisAuthName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		})
	}

	if inst.IsStandalone() && NeedStandaloneInit(rf) {
		if rf.Annotations[AnnotationStandaloneInitStorage] == "hostpath" {
			// hostpath
			if rf.Annotations[AnnotationStandaloneInitHostPath] != "" {
				hostpathType := corev1.HostPathDirectory
				volumes = append(volumes, corev1.Volume{
					Name: redisStandaloneVolumeName,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: rf.Annotations[AnnotationStandaloneInitHostPath],
							Type: &hostpathType,
						},
					},
				})
			}
		} else {
			// pvc
			if rf.Annotations[AnnotationStandaloneInitPvcName] != "" {
				volumes = append(volumes, corev1.Volume{
					Name: redisStandaloneVolumeName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: rf.Annotations[AnnotationStandaloneInitPvcName],
						},
					},
				})
			}
		}
	}
	return volumes
}

func getRedisDataVolume(rf *v1alpha1.Failover) *corev1.Volume {
	if rf.Spec.Storage != nil {
		return nil
	}
	return &corev1.Volume{
		Name: redisStorageVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}
