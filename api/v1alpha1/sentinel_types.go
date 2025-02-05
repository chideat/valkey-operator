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

// SentinelInstanceAccess
type SentinelInstanceAccess struct {
	core.InstanceAccess `json:",inline"`

	// ExternalTLSSecret the external TLS secret to use, if not provided, the operator will issue one
	ExternalTLSSecret string `json:"externalTLSSecret,omitempty"`
}

// SentinelSpec defines the desired state of Sentinel
type SentinelSpec struct {
	// Image the valkey sentinel image
	Image string `json:"image,omitempty"`

	// ImagePullPolicy the image pull policy
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// ImagePullSecrets the image pull secrets
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// Replicas the number of sentinel replicas
	//
	// +kubebuilder:validation:Minimum=3
	Replicas int32 `json:"replicas,omitempty"`

	// Resources the resources for sentinel
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// CustomConfigs the config for sentinel
	CustomConfigs map[string]string `json:"customConfigs,omitempty"`

	// Exporter defines the specification for the sentinel exporter
	Exporter *core.Exporter `json:"exporter,omitempty"`

	// Access the access for sentinel
	Access SentinelInstanceAccess `json:"access,omitempty"`

	// Affinity
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// SecurityContext
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`
	// Tolerations
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// NodeSelector
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// PodAnnotations
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
}

// SentinelPhase
type SentinelPhase string

const (
	// SentinelCreating the sentinel creating phase
	SentinelCreating SentinelPhase = "Creating"
	// SentinelPaused the sentinel paused phase
	SentinelPaused SentinelPhase = "Paused"
	// SentinelReady the sentinel ready phase
	SentinelReady SentinelPhase = "Ready"
	// SentinelFail the sentinel fail phase
	SentinelFail SentinelPhase = "Fail"
)

// SentinelStatus defines the observed state of Sentinel
type SentinelStatus struct {
	// Phase the status phase
	Phase SentinelPhase `json:"phase,omitempty"`
	// Message the status message
	Message string `json:"message,omitempty"`
	// Nodes the valkey node details
	Nodes []core.ValkeyNode `json:"nodes,omitempty"`
	// TLSSecret the tls secret
	TLSSecret string `json:"tlsSecret,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas",description="Sentinel replicas"
// +kubebuilder:printcolumn:name="Access",type="string",JSONPath=".spec.access.type",description="Instance access type"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="Instance phase"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.message",description="Instance status message"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time since creation"

// Sentinel is the Schema for the sentinels API
type Sentinel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SentinelSpec   `json:"spec,omitempty"`
	Status SentinelStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SentinelList contains a list of Sentinel
type SentinelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Sentinel `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Sentinel{}, &SentinelList{})
}
