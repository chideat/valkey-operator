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

// Authorization defines the authorization settings for valkey
type Authorization struct {
	// Username the username for valkey
	Username string `json:"username,omitempty"`
	// PasswordSecret the password secret for valkey
	PasswordSecret string `json:"passwordSecret,omitempty"`
	// TLSSecret the tls secret
	TLSSecret string `json:"tlsSecret,omitempty"`
}

type SentinelMonitorNode struct {
	// IP the sentinel node ip
	IP string `json:"ip,omitempty"`
	// Port the sentinel node port
	Port int32 `json:"port,omitempty"`
	// Flags
	Flags string `json:"flags,omitempty"`
}

// SentinelReference defines the sentinel reference
type SentinelReference struct {
	// Addresses the sentinel addresses
	// +kubebuilder:validation:MinItems=3
	Nodes []SentinelMonitorNode `json:"nodes,omitempty"`
	// Auth the sentinel auth
	Auth Authorization `json:"auth,omitempty"`
}

// SentinelSettings defines the specification of the sentinel cluster
type SentinelSettings struct {
	SentinelSpec `json:",inline"`

	// SentinelReference the sentinel reference
	SentinelReference *SentinelReference `json:"sentinelReference,omitempty"`

	// MonitorConfig configs for sentinel to monitor this replication, including:
	// - down-after-milliseconds
	// - failover-timeout
	// - parallel-syncs
	MonitorConfig map[string]string `json:"monitorConfig,omitempty"`

	// Quorum the number of Sentinels that need to agree about the fact the master is not reachable,
	// in order to really mark the master as failing, and eventually start a failover procedure if possible.
	// If not specified, the default value is the majority of the Sentinels.
	Quorum *int32 `json:"quorum,omitempty"`
}

// FailoverSpec defines the desired state of Failover
type FailoverSpec struct {
	Image            string                        `json:"image,omitempty"`
	ImagePullPolicy  corev1.PullPolicy             `json:"imagePullPolicy,omitempty"`
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	Replicas  int32                       `json:"replicas,omitempty"`
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// CustomConfig custom valkey configuration
	CustomConfigs map[string]string `json:"customConfigs,omitempty"`
	// Storage
	Storage *core.Storage `json:"storage,omitempty"`

	// Exporter
	Exporter *core.Exporter `json:"exporter,omitempty"`
	// Access
	Access core.InstanceAccess `json:"access,omitempty"`

	PodAnnotations  map[string]string          `json:"podAnnotations,omitempty"`
	Affinity        *corev1.Affinity           `json:"affinity,omitempty"`
	Tolerations     []corev1.Toleration        `json:"tolerations,omitempty"`
	NodeSelector    map[string]string          `json:"nodeSelector,omitempty"`
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`

	// Sentinel
	Sentinel *SentinelSettings `json:"sentinel,omitempty"`
}

type FailoverPhase string

const (
	FailoverPhaseReady    FailoverPhase = "Ready"
	FailoverPhaseFailed   FailoverPhase = "Failed"
	FailoverPhasePaused   FailoverPhase = "Paused"
	FailoverPhaseCreating FailoverPhase = "Creating"
)

type FailoverPolicy string

const (
	SentinelFailoverPolicy FailoverPolicy = "sentinel"
	ManualFailoverPolicy   FailoverPolicy = "manual"
)

type MonitorStatus struct {
	// Policy the failover policy
	Policy FailoverPolicy `json:"policy,omitempty"`
	// Name monitor name
	Name string `json:"name,omitempty"`
	// Username sentinel username
	Username string `json:"username,omitempty"`
	// PasswordSecret
	PasswordSecret string `json:"passwordSecret,omitempty"`
	// OldPasswordSecret
	OldPasswordSecret string `json:"oldPasswordSecret,omitempty"`
	// TLSSecret the tls secret
	TLSSecret string `json:"tlsSecret,omitempty"`
	// Nodes the sentinel monitor nodes
	Nodes []SentinelMonitorNode `json:"nodes,omitempty"`
}

// FailoverStatus defines the observed state of Failover
type FailoverStatus struct {
	// Phase
	Phase FailoverPhase `json:"phase,omitempty"`
	// Message the status message
	Message string `json:"message,omitempty"`
	// Nodes the valkey cluster nodes
	Nodes []core.ValkeyNode `json:"nodes,omitempty"`
	// TLSSecret the tls secret
	TLSSecret string `json:"tlsSecret,omitempty"`
	// Monitor the monitor status
	Monitor MonitorStatus `json:"monitor,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.valkey.replicas",description="Valkey replicas"
// +kubebuilder:printcolumn:name="Sentinels",type="integer",JSONPath=".spec.sentinel.replicas",description="Valkey failover replicas"
// +kubebuilder:printcolumn:name="Access",type="string",JSONPath=".spec.valkey.access.type",description="Instance access type"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="Instance phase"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.message",description="Instance status message"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time since creation"

// Failover is the Schema for the failovers API
type Failover struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FailoverSpec   `json:"spec,omitempty"`
	Status FailoverStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FailoverList contains a list of Failover
type FailoverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Failover `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Failover{}, &FailoverList{})
}
