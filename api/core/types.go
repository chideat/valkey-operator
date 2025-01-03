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
package core

// +kubebuilder:object:generate=true

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Arch string

const (
	// ValkeyCluster is the Valkey Cluster arch
	ValkeyCluster Arch = "cluster"
	// ValkeyFailover is the Valkey Sentinel arch, who relies on the sentinel to support high availability.
	ValkeyFailover Arch = "failover"
	// ValkeyReplica is the Valkey primary-replica arch, who use etcd to make sure the primary node.
	ValkeyReplica Arch = "replica"

	// ValkeySentinel is the Sentinel arch
	ValkeySentinel Arch = "sentinel"
)

// NodeRole valkey node role type
type NodeRole string

const (
	// NodeRoleNone None node role
	NodeRoleNone NodeRole = "none"
	// NodeRoleMaster Master node role
	// TODO: rename to Primary
	NodeRoleMaster NodeRole = "master"
	// NodeRoleReplica Master node role
	NodeRoleReplica NodeRole = "replica"
	// NodeRoleSentinel Master node role
	NodeRoleSentinel NodeRole = "sentinel"
)

// AffinityPolicy defines the affinity policy of the Valkey
type AffinityPolicy string

const (
	// SoftAntiAffinity defines the soft anti-affinity policy
	// all pods will try to be scheduled on different nodes,
	// pods of the same shard may be scheduled on the same node
	SoftAntiAffinity AffinityPolicy = "SoftAntiAffinity"

	// AntiAffinityInShard defines the anti-affinity policy in shard
	// pods of the same shard will be scheduled on different nodes,
	// but pods of different shards can be scheduled on the same node
	AntiAffinityInShard AffinityPolicy = "AntiAffinityInShard"

	// AntiAffinity defines the anti-affinity policy
	// all pods will be scheduled on different nodes
	AntiAffinity AffinityPolicy = "AntiAffinity"

	// CustomAffinity defines the custom affinity policy
	CustomAffinity AffinityPolicy = ""
)

type Storage struct {
	// storageClassName is the name of the StorageClass required by the claim.
	// if not set, the default StorageClass will be used
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`

	// Capacity is the cap of the volume to request.
	// if not set and StorageClassName is set, the default StorageClass size will be the double size of memory limit.
	Capacity resource.Quantity `json:"capacity,omitempty"`

	// AccessMode is the access mode of the volume.
	// +kubebuilder:default:=ReadWriteOnce
	AccessMode corev1.PersistentVolumeAccessMode `json:"accessMode,omitempty"`

	// RetainAfterDeleted defines whether the storage should be retained after the ValkeyCluster is deleted
	RetainAfterDeleted bool `json:"retainAfterDeleted,omitempty"`
}

// Exporter
type Exporter struct {
	// Enabled enable the exporter
	Enabled bool `json:"enabled,omitempty"`
	// Image the exporter image
	Image string `json:"image,omitempty"`
	// ImagePullPolicy
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// Resources for setting resource requirements for the Pod Resources *v1.ResourceRequirements
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// InstanceAccess
type InstanceAccess struct {
	// DefaultPasswordSecret referered to the secret which defined the password for default user
	// The referered secret must have `password` key whose value matching regex: ^[a-zA-Z0-9_!@#$%^&*()-_=+?]{8,128}$
	// +optional
	DefaultPasswordSecret string `json:"defaultPasswordSecret,omitempty"`

	// ServiceType defines the type of the all related services
	// +kubebuilder:default:=ClusterIP
	// +kubebuilder:validation:Enum=NodePort;LoadBalancer;ClusterIP
	ServiceType corev1.ServiceType `json:"type,omitempty"`

	// The annnotations of the service which will be attached to services
	Annotations map[string]string `json:"annotations,omitempty"`

	// IPFamily represents the IP Family (IPv4 or IPv6).
	// This type is used to express the family of an IP expressed by a type (e.g. service.spec.ipFamilies).
	// +kubebuilder:validation:Enum=IPv4;IPv6
	IPFamilyPrefer corev1.IPFamily `json:"ipFamilyPrefer,omitempty"`

	// Ports defines the nodeports of NodePort service
	// +kubebuilder:validation:Pattern="^([0-9]+:[0-9]+)(,[0-9]+:[0-9]+)*$"
	Ports string `json:"ports,omitempty"`

	// EnableTLS enable TLS for external access
	EnableTLS bool `json:"enableTLS,omitempty"`

	// Cert Issuer for external access TLS certificate
	CertIssuer string `json:"certIssuer,omitempty"`

	// Cert Issuer Type for external access TLS certificate
	// +kubebuilder:default:="ClusterIssuer"
	// +kubebuilder:validation:Enum=ClusterIssuer;Issuer
	CertIssuerType string `json:"certIssuerType,omitempty"`
}

// ValkeyNode represent a ValkeyCluster Node
type ValkeyNode struct {
	// ID is the valkey cluster node id, not runid
	ID string `json:"id,omitempty"`
	// ShardID cluster shard id of the node
	ShardID string `json:"shardId,omitempty"`
	// Role is the role of the node, master or slave
	Role NodeRole `json:"role"`
	// IP is the ip of the node. if access announce is enabled, it will be the access ip
	IP string `json:"ip"`
	// Port is the port of the node. if access announce is enabled, it will be the access port
	Port string `json:"port"`
	// Slots is the slot range for the shard, eg: 0-1000,1002,1005-1100
	Slots string `json:"slots,omitempty"`
	// MasterRef is the master node id of this node
	MasterRef string `json:"masterRef,omitempty"`
	// StatefulSet is the statefulset name of this pod
	StatefulSet string `json:"statefulSet"`
	// PodName current pod name
	PodName string `json:"podName"`
	// NodeName is the node name of the node where holds the pod
	NodeName string `json:"nodeName"`
}
