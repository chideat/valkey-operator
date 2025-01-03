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
	System  AccountType = "system"
	Custom  AccountType = "custom"
	Default AccountType = "default"
)

// RedisUserSpec defines the desired state of RedisUser
type RedisUserSpec struct {
	// redis user account type
	// +kubebuilder:validation:Enum=system;custom;default
	AccountType AccountType `json:"accountType,omitempty"`
	// redis user account type
	// +kubebuilder:validation:Enum=sentinel;cluster;standalone
	Arch core.Arch `json:"arch,omitempty"`
	// Redis Username (required)
	Username string `json:"username"`
	// Redis Password secret name, key is password
	PasswordSecrets []string `json:"passwordSecrets,omitempty"`
	// redis  acl rules  string
	AclRules string `json:"aclRules,omitempty"`
	// Redis instance  Name (required)
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:MinLength=1
	RedisName string `json:"redisName"` //redisname
}

type RedisUserPhase string

const (
	UserFail    RedisUserPhase = "Fail"
	UserSuccess RedisUserPhase = "Success"
	UserPending RedisUserPhase = "Pending"
)

// RedisUserStatus defines the observed state of RedisUser
type RedisUserStatus struct {
	// Phase
	Phase Phase `json:"Phase,omitempty"`

	// Message
	Message string `json:"message,omitempty"`

	// AclRules acl rules of redis
	AclRules string `json:"aclRules,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Instance",type=string,JSONPath=`.spec.redisName`
// +kubebuilder:printcolumn:name="username",type=string,JSONPath=`.spec.username`
// +kubebuilder:printcolumn:name="phase",type=string,JSONPath=`.status.Phase`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time since creation"

// RedisUser is the Schema for the redisusers API
type RedisUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisUserSpec   `json:"spec,omitempty"`
	Status RedisUserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RedisUserList contains a list of RedisUser
type RedisUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RedisUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RedisUser{}, &RedisUserList{})
}
