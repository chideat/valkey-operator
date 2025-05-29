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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AccountType string

const (
	SystemAccount AccountType = "system"
	CustomAccount AccountType = "custom"
)

// UserSpec defines the desired state of User
type UserSpec struct {
	// user account type
	// +kubebuilder:validation:Enum=system;custom
	AccountType AccountType `json:"accountType,omitempty"`
	// user account type
	// +kubebuilder:validation:Enum=failover;cluster;replica
	Arch core.Arch `json:"arch,omitempty"`
	// Username (required)
	Username string `json:"username"`
	// PasswordSecrets Password secret name, key is password
	PasswordSecrets []string `json:"passwordSecrets,omitempty"`
	// AclRules acl rules  string
	AclRules string `json:"aclRules,omitempty"`
	// InstanceName instance  Name (required)
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:MinLength=1
	InstanceName string `json:"instanceName"`
}

type UserPhase string

const (
	UserFail    UserPhase = "Fail"
	UserReady   UserPhase = "Ready"
	UserPending UserPhase = "Pending"
)

// UserStatus defines the observed state of User
type UserStatus struct {
	// Phase
	Phase UserPhase `json:"Phase,omitempty"`

	// Message
	Message string `json:"message,omitempty"`

	// AclRules acl rules of valkey
	AclRules string `json:"aclRules,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Instance",type=string,JSONPath=`.spec.instanceName`
// +kubebuilder:printcolumn:name="username",type=string,JSONPath=`.spec.username`
// +kubebuilder:printcolumn:name="phase",type=string,JSONPath=`.status.Phase`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time since creation"

// User is the Schema for the users API
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec,omitempty"`
	Status UserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UserList contains a list of User
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

func init() {
	SchemeBuilder.Register(&User{}, &UserList{})
}
