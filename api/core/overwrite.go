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

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// OverwriteKind is the kind of generated object an Overwrite patches.
// +kubebuilder:validation:Enum=StatefulSet;PodDisruptionBudget;Service
type OverwriteKind string

const (
	// OverwriteKindStatefulSet patches the StatefulSets the operator generates,
	// their pod template included.
	OverwriteKindStatefulSet OverwriteKind = "StatefulSet"
	// OverwriteKindPodDisruptionBudget patches the PodDisruptionBudgets the
	// operator generates, one for each StatefulSet.
	OverwriteKindPodDisruptionBudget OverwriteKind = "PodDisruptionBudget"
	// OverwriteKindService patches the Services the operator generates, those
	// of one target.
	OverwriteKindService OverwriteKind = "Service"
)

// OverwriteTarget names a group of the Services the operator generates.
// +kubebuilder:validation:Enum=headless;instance;readwrite;readonly;exporter;pod
type OverwriteTarget string

const (
	// OverwriteTargetHeadless is the headless Service of each cluster shard,
	// or of the sentinel nodes.
	OverwriteTargetHeadless OverwriteTarget = "headless"
	// OverwriteTargetInstance is the Service in front of all the nodes of a
	// cluster.
	OverwriteTargetInstance OverwriteTarget = "instance"
	// OverwriteTargetReadWrite is the Service in front of the primary of a
	// failover or replica instance.
	OverwriteTargetReadWrite OverwriteTarget = "readwrite"
	// OverwriteTargetReadOnly is the Service in front of the replicas of a
	// failover or replica instance.
	OverwriteTargetReadOnly OverwriteTarget = "readonly"
	// OverwriteTargetExporter is the headless Service of the Valkey nodes of a
	// failover or replica instance, which also serves the exporter's port.
	OverwriteTargetExporter OverwriteTarget = "exporter"
	// OverwriteTargetPod is the Service of each pod, through which the pod
	// announces its address.
	OverwriteTargetPod OverwriteTarget = "pod"
)

// Overwrite patches the objects of one kind that the operator generates; for
// Services, those of one target.
//
// Values in the patch win over the generated ones, except for the fields the
// operator protects: the ones its own logic reads or depends on, such as the
// valkey container's command, probes and resources, and the ones that already
// have a typed field in the spec, such as affinity and tolerations. A patch
// that sets a protected field is rejected at admission; one that reaches the
// operator without admission has those fields restored, and a Warning event
// names them.
type Overwrite struct {
	// Kind of the generated objects to patch.
	Kind OverwriteKind `json:"kind"`

	// Target names the Services to patch. It is required for kind Service and
	// not allowed for the other kinds: headless, instance or pod on the
	// cluster architecture; readwrite, readonly, exporter or pod on the
	// failover and replica architectures; headless or pod for the sentinel
	// nodes.
	// +optional
	Target OverwriteTarget `json:"target,omitempty"`

	// Patch is a strategic merge patch applied to every generated object of
	// that kind and target, metadata and spec, kept exactly as written. It is
	// an object, or a string that holds one in YAML or JSON. A null in it
	// deletes a field; client-side kubectl apply and merge patches drop nulls
	// from an object, but not from a string.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	Patch apiextensionsv1.JSON `json:"patch"`
}
