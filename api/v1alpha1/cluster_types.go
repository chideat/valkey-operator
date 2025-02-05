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

package v1alpha1

import (
	"github.com/chideat/valkey-operator/api/core"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ShardConfig struct {
	// Slots is the slot range for the shard, eg: 0-1000,1002,1005-1100
	//+kubebuilder:validation:Pattern:=`^(\d{1,5}|(\d{1,5}-\d{1,5}))(,(\d{1,5}|(\d{1,5}-\d{1,5})))*$`
	Slots string `json:"slots,omitempty"`
}

// ClusterReplicas
type ClusterReplicas struct {
	// Shards is the number of cluster shards
	// +kubebuilder:validation:Minimum=3
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Required
	// +kubebuilder:default=3
	Shards int32 `json:"masterSize"`

	// ShardsConfig is the configuration of each shard
	// +kubebuilder:validation:MinItems=3
	// +optional
	ShardsConfig []*ShardConfig `json:"shardsConfig,omitempty"`

	// ReplicasOfShard is the number of replicas for each master node
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=5
	ReplicasOfShard int32 `json:"replicasOfShard"`
}

// ClusterSpec defines the desired state of Cluster
type ClusterSpec struct {
	// Image valkey image
	Image string `json:"image,omitempty"`
	// ImagePullPolicy is the image pull policy
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// ImagePullSecrets
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// Replicas is the number of cluster replicas
	Replicas ClusterReplicas `json:"replicas"`

	// CustomConfigs is the custom configuration
	//
	// Most of the settings is key-value format.
	CustomConfigs map[string]string `json:"customConfigs,omitempty"`

	// Resources
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Access defines the access for valkey
	Access core.InstanceAccess `json:"access,omitempty"`
	// Storage
	Storage *core.Storage `json:"storage,omitempty"`

	// Exporter
	Exporter *core.Exporter `json:"exporter,omitempty"`

	// PodAnnotations
	PodAnnotations map[string]string `json:"annotations,omitempty"`
	// AffinityPolicy
	// +kubebuilder:validation:Enum=SoftAntiAffinity;AntiAffinityInSharding;AntiAffinity
	AffinityPolicy *core.AffinityPolicy `json:"affinityPolicy,omitempty"`
	// Affinity
	CustomAffinity *corev1.Affinity `json:"CustomAffinity,omitempty"`
	// NodeSelector
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Tolerations
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// SecurityContext
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`
}

// ClusterPhase Valkey Cluster status
type ClusterPhase string

const (
	ClusterPhaseOK ClusterPhase = "Healthy"
	// ClusterStatusKO ClusterPhase KO
	ClusterPhaseKO ClusterPhase = "Failed"
	// ClusterStatusCreating ClusterPhase Creating
	ClusterPhaseCreating ClusterPhase = "Creating"
	// ClusterStatusRollingUpdate ClusterPhase RollingUpdate
	ClusterPhaseRollingUpdate ClusterPhase = "RollingUpdate"
	// ClusterStatusRebalancing ClusterPhase rebalancing
	ClusterPhaseRebalancing ClusterPhase = "Rebalancing"
	// clusterStatusPaused cluster status paused
	ClusterPhasePaused ClusterPhase = "Paused"
)

// ClusterServiceStatus
type ClusterServiceStatus string

const (
	ClusterInService    ClusterServiceStatus = "InService"
	ClusterOutOfService ClusterServiceStatus = "OutOfService"
)

// ClusterShardsSlotStatus
type ClusterShardsSlotStatus struct {
	// Slots slots this shard holds or will holds
	Slots string `json:"slots,omitempty"`
	// Status the status of this status
	Status string `json:"status,omitempty"`
	// ShardIndex indicates the slots importing from or migrate to
	ShardIndex *int32 `json:"shardId"`
}

// ClusterShards
type ClusterShards struct {
	// ID match the shard-id in cluster shard
	Id string `json:"id,omitempty"`
	// Index the shard index
	Index int32 `json:"index"`
	// Slots records the slots status of this shard
	Slots []*ClusterShardsSlotStatus `json:"slots"`
}

// ClusterStatus defines the observed state of Cluster
type ClusterStatus struct {
	// Status the status of the cluster
	Phase ClusterPhase `json:"status"`
	// Message the message of the status
	Message string `json:"message,omitempty"`
	// Nodes the cluster nodes
	Nodes []core.ValkeyNode `json:"nodes,omitempty"`
	// ServiceStatus the cluster service status
	ServiceStatus ClusterServiceStatus `json:"clusterStatus,omitempty"`
	// Shards the cluster shards
	Shards []*ClusterShards `json:"shards,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Shards",type="integer",JSONPath=".status.numberOfMaster",description="Current Shards"
// +kubebuilder:printcolumn:name="Service Status",type="string",JSONPath=".status.serviceStatus",description="Service status"
// +kubebuilder:printcolumn:name="Access",type="string",JSONPath=".spec.access.type",description="Instance access type"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="Instance phase"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.reason",description="Status message"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time since creation"

// Cluster is the Schema for the clusters API
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}
