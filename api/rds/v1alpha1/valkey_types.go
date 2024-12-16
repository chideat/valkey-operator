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
	bufredv1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ValkeyReplicas defines the replicas of Valkey
type ValkeyReplicas struct {
	// Shards defines the number of shards for Valkey
	// for cluster arch, the default value is 3; for other arch, the value is always 1
	// +optional
	Shards int32 `json:"shards,omitempty"`

	// ReplicasOfShard defines the number of replicas for each shard
	// +optional
	ReplicasOfShard int32 `json:"replicasOfShard,omitempty"`
}

// ValkeySpec defines the desired state of Valkey
type ValkeySpec struct {
	// Version supports 7.2, 8.0
	// +kubebuilder:validation:Enum="7.2";"8.0"
	Version string `json:"version"`

	// Arch supports cluster, sentinel
	// +kubebuilder:validation:Enum="cluster";"sentinel";"standalone"
	Arch core.Arch `json:"arch"`

	// Replicas defines desired number of replicas for Valkey
	Replicas *ValkeyReplicas `json:"replicas"`

	// Resources for setting resource requirements for the Pod Resources *v1.ResourceRequirements
	Resources *corev1.ResourceRequirements `json:"resources"`

	// CustomConfigs defines the configuration settings for Valkey
	// for detailed settings, please refer to https://github.com/valkey-io/valkey/blob/unstable/valkey.conf
	CustomConfigs map[string]string `json:"customConfigs,omitempty"`

	// Storage defines the storage settings for Valkey
	Storage *core.Storage `json:"storage,omitempty"`

	//  ExternalAccess defines information for Valkey nodePorts settings
	// +optional
	Access core.InstanceAccess `json:"instanceAccess,omitempty"`

	// PodAnnotations holds Kubernetes Pod annotations PodAnnotations
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// AffinityPolicy specifies the affinity policy for the Pod
	// +optional
	// +kubebuilder:validation:Enum="SoftAntiAffinity";"AntiAffinityInSharding";"HardAntiAffinity";"CustomAffinity"
	AffinityPolicy core.AffinityPolicy `json:"affinityPolicy,omitempty"`

	// CustomAffinity specifies the custom affinity settings for the Pod
	// if AffinityPolicy is set to CustomAffinity, this field is required
	// +optional
	CustomAffinity *corev1.Affinity `json:"customAffinity,omitempty"`

	// NodeSelector specifies the node selector for the Pod
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// tolerations defines tolerations for the Pod
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// SecurityContext sets security attributes for the Pod SecurityContex
	// +optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`

	// Exporter defines Valkey exporter settings
	// +optional
	Exporter *core.Exporter `json:"exporter,omitempty"`

	// Sentinel defines Sentinel configuration settings Sentinel
	// +optional
	Sentinel *bufredv1alpha1.SentinelSettings `json:"sentinel,omitempty"`
}

// ValkeyPhase
type ValkeyPhase string

const (
	// Initializing
	Initializing ValkeyPhase = "Initializing"
	// Rebalancing
	Rebalancing ValkeyPhase = "Rebalancing"
	// Ready
	Ready ValkeyPhase = "Ready"
	// Error
	Error ValkeyPhase = "Error"
	// Paused
	Paused ValkeyPhase = "Paused"
)

// ValkeyStatus defines the observed state of Valkey
type ValkeyStatus struct {
	// Phase indicates whether all the resource for the instance is ok.
	// Values are as below:
	//   Initializing - Resource is in Initializing or Reconcile
	//   Ready        - All resources is ok. In most cases, Ready means the cluster is ok to use
	//   Error        - Error found when do resource init
	Phase ValkeyPhase `json:"phase,omitempty"`
	// This field contains an additional message for the instance's status
	Message string `json:"message,omitempty"`
	// The name of the kubernetes Secret that contains Valkey password.
	PasswordSecretName string `json:"passwordSecretName,omitempty"`
	// The name of the kubernetes Service for Valkey
	ServiceName string `json:"serviceName,omitempty"`
	// Matching labels selector for Valkey
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
	// Matching label selector for Valkey proxy.
	ProxyMatchLabels map[string]string `json:"proxyMatchLabels,omitempty"`
	// The name of the kubernetes Service for Valkey Proxy
	ProxyServiceName string `json:"proxyServiceName,omitempty"`
	// ClusterNodes valkey nodes info
	ClusterNodes []core.ValkeyNode `json:"clusterNodes,omitempty"`
	// LastShardCount indicates the last number of shards in the Valkey Cluster.
	LastShardCount int32 `json:"lastShardCount,omitempty"`
	// LastVersion indicates the last version of the Valkey instance.
	LastVersion string `json:"lastVersion,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Arch",type="string",JSONPath=".spec.arch",description="Instance arch"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version",description="Valkey version"
// +kubebuilder:printcolumn:name="Access",type="string",JSONPath=".spec.access.type",description="Instance access type"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="Instance phase"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.message",description="Instance status message"
// +kubebuilder:printcolumn:name="Bundle Version",type="string",JSONPath=".status.upgradeStatus.crVersion",description="Bundle Version"
// +kubebuilder:printcolumn:name="AutoUpgrade",type="boolean",JSONPath=".spec.upgradeOption.autoUpgrade",description="Enable instance auto upgrade"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time since creation"

// Valkey is the Schema for the valkeys API
type Valkey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ValkeySpec   `json:"spec,omitempty"`
	Status ValkeyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ValkeyList contains a list of Valkey
type ValkeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Valkey `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Valkey{}, &ValkeyList{})
}
