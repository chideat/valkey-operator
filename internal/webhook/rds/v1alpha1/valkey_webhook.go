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
	"context"
	"fmt"

	"github.com/chideat/valkey-operator/api/core"
	corehelper "github.com/chideat/valkey-operator/api/core/helper"
	coreVal "github.com/chideat/valkey-operator/api/core/validation"
	rdsv1alpha1 "github.com/chideat/valkey-operator/api/rds/v1alpha1"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/webhook/rds/v1alpha1/helper"
	"github.com/chideat/valkey-operator/internal/webhook/rds/v1alpha1/validation"
	"github.com/chideat/valkey-operator/pkg/slot"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	ConfigReplBacklogSizeKey = "repl-backlog-size"
)

// nolint:unused
// log is for logging in this package.
var logger = logf.Log.WithName("inst-resource")

// SetupValkeyWebhookWithManager registers the webhook for inst in the manager.
func SetupValkeyWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&rdsv1alpha1.Valkey{}).
		WithValidator(&ValkeyCustomValidator{}).
		WithDefaulter(&ValkeyCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-rds-inst-buf-red-v1alpha1-inst,mutating=true,failurePolicy=fail,sideEffects=None,groups=rds.inst.buf.red,resources=valkeys,verbs=create;update,versions=v1alpha1,name=mvalkey-v1alpha1.kb.io,admissionReviewVersions=v1

// ValkeyCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind inst when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type ValkeyCustomDefaulter struct {
}

var _ webhook.CustomDefaulter = &ValkeyCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind inst.
func (d *ValkeyCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	inst, ok := obj.(*rdsv1alpha1.Valkey)
	if !ok {
		return fmt.Errorf("expected an inst object but got %T", obj)
	}
	if inst.Annotations == nil {
		inst.Annotations = make(map[string]string)
	}
	if inst.Spec.CustomConfigs == nil {
		inst.Spec.CustomConfigs = make(map[string]string)
	}
	if inst.Spec.PodAnnotations == nil {
		inst.Spec.PodAnnotations = make(map[string]string)
	}

	if inst.Spec.CustomConfigs[ConfigReplBacklogSizeKey] == "" {
		// https://raw.githubusercontent.com/antirez/redis/7.0/redis.conf
		// https://docs.redis.com/latest/rs/databases/active-active/manage/#replication-backlog
		if inst.Spec.Resources != nil {
			for _, resource := range []*resource.Quantity{inst.Spec.Resources.Limits.Memory(), inst.Spec.Resources.Requests.Memory()} {
				if resource == nil {
					continue
				}
				if val, ok := resource.AsInt64(); ok {
					val = int64(0.01 * float64(val))
					if val > 256*1024*1024 {
						val = 256 * 1024 * 1024
					} else if val < 1024*1024 {
						val = 1024 * 1024
					}
					inst.Spec.CustomConfigs[ConfigReplBacklogSizeKey] = fmt.Sprintf("%d", val)
				}
			}
		}
	}

	// init exporter settings
	if inst.Spec.Exporter == nil {
		inst.Spec.Exporter = &rdsv1alpha1.ValkeyExporter{}
	}
	if inst.Spec.Exporter.Resources.Limits.Cpu().IsZero() ||
		inst.Spec.Exporter.Resources.Limits.Memory().IsZero() {
		inst.Spec.Exporter.Resources = &corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("384Mi"),
			},
		}
	}

	if inst.Spec.Storage != nil {
		if inst.Spec.Storage.StorageClassName != nil && inst.Spec.Storage.Capacity == nil {
			size := resource.NewQuantity(inst.Spec.Resources.Limits.Memory().Value()*2, resource.BinarySI)
			inst.Spec.Storage.Capacity = size
		}
	}

	switch inst.Spec.Arch {
	case core.ValkeyFailover:
		if inst.Spec.Replicas == nil {
			inst.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards:          1,
				ReplicasOfShard: 2,
			}
		}
		if inst.Spec.Sentinel == nil {
			inst.Spec.Sentinel = &v1alpha1.SentinelSettings{}
		}
		if len(inst.Spec.Sentinel.MonitorConfig) == 0 {
			inst.Spec.Sentinel.MonitorConfig = builder.SentinelMonitorDefaultConfigs
		}

		if inst.Spec.Sentinel.SentinelReference == nil {
			if inst.Spec.Sentinel.Replicas <= 0 {
				inst.Spec.Sentinel.Replicas = 3
			}
			inst.Spec.Sentinel.Access.ServiceType = inst.Spec.Access.ServiceType

			sentinel := inst.Spec.Sentinel
			if sentinel.Resources.Limits.Cpu().IsZero() && sentinel.Resources.Limits.Memory().IsZero() {
				sentinel.Resources = corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Limits: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				}
			}
		}
	case core.ValkeyReplica:
		inst.Spec.Sentinel = nil
		if inst.Spec.Replicas == nil {
			inst.Spec.Replicas = &rdsv1alpha1.ValkeyReplicas{
				Shards: 1,
			}
		}
		inst.Spec.Replicas.Shards = 1
		inst.Spec.Replicas.ShardsConfig = nil
	}
	return nil
}

// +kubebuilder:webhook:path=/validate-rds-inst-buf-red-v1alpha1-inst,mutating=false,failurePolicy=fail,sideEffects=None,groups=rds.inst.buf.red,resources=valkeys,verbs=create;update,versions=v1alpha1,name=vvalkey-v1alpha1.kb.io,admissionReviewVersions=v1

// ValkeyCustomValidator struct is responsible for validating the inst resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type ValkeyCustomValidator struct {
	mgrClient client.Client
}

var _ webhook.CustomValidator = &ValkeyCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type inst.
func (v *ValkeyCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (warns admission.Warnings, err error) {
	inst, ok := obj.(*rdsv1alpha1.Valkey)
	if !ok {
		return nil, fmt.Errorf("expected a inst object but got %T", obj)
	}
	logger.Info("Validation for inst upon creation", "name", inst.GetName())

	if err := validation.ValidatePasswordSecret(inst.Namespace, inst.Spec.Access.DefaultPasswordSecret, v.mgrClient, &warns); err != nil {
		return warns, err
	}
	if err := coreVal.ValidateInstanceAccess(&inst.Spec.Access,
		helper.CalculateNodeCount(inst.Spec.Arch, inst.Spec.Replicas.Shards, inst.Spec.Replicas.ReplicasOfShard),
		&warns); err != nil {
		return warns, err
	}

	switch inst.Spec.Arch {
	case core.ValkeyCluster:
		if inst.Spec.Replicas == nil {
			return nil, fmt.Errorf("instance replicas not specified")
		}
		if inst.Spec.Replicas.Shards < 3 {
			return nil, fmt.Errorf("spec.replicas.shards must >= 3")
		}

		// validate shard configs
		shards := int32(len(inst.Spec.Replicas.ShardsConfig))
		if shards > 0 {
			if shards != inst.Spec.Replicas.Shards {
				return nil, fmt.Errorf("specified shard slots list length not not match shards count")
			}

			var (
				fullSlots *slot.Slots
				total     int
			)
			for _, shard := range inst.Spec.Replicas.ShardsConfig {
				if shardSlots, err := slot.LoadSlots(shard.Slots); err != nil {
					return nil, fmt.Errorf("failed to load shard slots: %v", err)
				} else {
					fullSlots = fullSlots.Union(shardSlots)
					total += shardSlots.Count(slot.SlotAssigned)
				}
			}
			if !fullSlots.IsFullfilled() {
				return nil, fmt.Errorf("specified shard slots not fullfilled")
			}
			if total > slot.ValkeyMaxSlots {
				return nil, fmt.Errorf("specified shard slots duplicated")
			}
		}
	case core.ValkeyFailover:
		if inst.Spec.Replicas == nil {
			return nil, fmt.Errorf("instance replicas not specified")
		}
		if inst.Spec.Replicas.Shards != 1 {
			return nil, fmt.Errorf("spec.replicas.shards must be 1")
		}

		if inst.Spec.Sentinel == nil {
			return nil, fmt.Errorf("spec.sentinel not specified")
		}
		if inst.Spec.Sentinel.Replicas%2 == 0 || inst.Spec.Sentinel.Replicas < 3 {
			return nil, fmt.Errorf("sentinel replicas must be odd and greater >= 3")
		}
		if inst.Spec.Sentinel.Access.DefaultPasswordSecret != "" {
			if err := validation.ValidatePasswordSecret(inst.Namespace, inst.Spec.Sentinel.Access.DefaultPasswordSecret, v.mgrClient, &warns); err != nil {
				return warns, fmt.Errorf("sentinel password secret: %v", err)
			}
		}

		if err := coreVal.ValidateInstanceAccess(&inst.Spec.Sentinel.Access.InstanceAccess, int(inst.Spec.Sentinel.Replicas), &warns); err != nil {
			return warns, err
		}

		if inst.Spec.Access.ServiceType == v1.ServiceTypeNodePort {
			portMap := map[int32]struct{}{}
			ports, _ := corehelper.ParsePorts(inst.Spec.Access.Ports)
			for _, port := range ports {
				if _, ok := portMap[port]; ok {
					return nil, fmt.Errorf("port %d has been assigned", port)
				}
				portMap[port] = struct{}{}
			}
			ports, _ = corehelper.ParsePorts(inst.Spec.Sentinel.Access.Ports)
			for _, port := range ports {
				if _, ok := portMap[port]; ok {
					return nil, fmt.Errorf("port %d has been assigned", port)
				}
			}
		}
	case core.ValkeyReplica:
		if inst.Spec.Replicas == nil {
			return nil, fmt.Errorf("instance replicas not specified")
		}
		if inst.Spec.Replicas.Shards != 1 {
			return nil, fmt.Errorf("spec.replicas.shards must be 1")
		}
	}
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type inst.
func (v *ValkeyCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warns admission.Warnings, err error) {
	inst, ok := newObj.(*rdsv1alpha1.Valkey)
	if !ok {
		return nil, fmt.Errorf("expected a inst object for the newObj but got %T", newObj)
	}
	logger.Info("Validation for inst upon update", "name", inst.GetName())

	if err := validation.ValidatePasswordSecret(inst.Namespace, inst.Spec.Access.DefaultPasswordSecret, v.mgrClient, &warns); err != nil {
		return warns, err
	}

	switch inst.Spec.Arch {
	case core.ValkeyCluster:
		if inst.Spec.Replicas == nil {
			return nil, fmt.Errorf("instance replicas not specified")
		}

		if err := coreVal.ValidateInstanceAccess(&inst.Spec.Access,
			helper.CalculateNodeCount(inst.Spec.Arch, inst.Spec.Replicas.Shards, inst.Spec.Replicas.ReplicasOfShard),
			&warns); err != nil {
			return warns, err
		}

		shards := int32(len(inst.Spec.Replicas.ShardsConfig))
		if shards > 0 && shards == inst.Spec.Replicas.Shards {
			// for update validator, only check slots fullfilled
			var (
				fullSlots *slot.Slots
				total     int
			)
			for _, shard := range inst.Spec.Replicas.ShardsConfig {
				if shardSlots, err := slot.LoadSlots(shard.Slots); err != nil {
					return nil, fmt.Errorf("failed to load shard slots: %v", err)
				} else {
					fullSlots = fullSlots.Union(shardSlots)
					total += shardSlots.Count(slot.SlotAssigned)
				}
			}
			if !fullSlots.IsFullfilled() {
				return nil, fmt.Errorf("specified shard slots not fullfilled")
			}
			if total > slot.ValkeyMaxSlots {
				return nil, fmt.Errorf("specified shard slots duplicated")
			}
		}
	case core.ValkeyFailover:
		if inst.Spec.Replicas == nil {
			return nil, fmt.Errorf("instance replicas not specified")
		}
		if inst.Spec.Replicas.Shards != 1 {
			return nil, fmt.Errorf("spec.replicas.shards must be 1")
		}

		if inst.Spec.Sentinel.Replicas%2 == 0 || inst.Spec.Sentinel.Replicas < 3 {
			return nil, fmt.Errorf("sentinel replicas must be odd and greater >= 3")
		}

		if err := coreVal.ValidateInstanceAccess(&inst.Spec.Access,
			helper.CalculateNodeCount(inst.Spec.Arch, inst.Spec.Replicas.Shards, inst.Spec.Replicas.ReplicasOfShard),
			&warns); err != nil {
			return warns, err
		}
		if err := coreVal.ValidateInstanceAccess(&inst.Spec.Sentinel.Access.InstanceAccess, int(inst.Spec.Sentinel.Replicas), &warns); err != nil {
			return warns, err
		}

		if inst.Spec.Access.ServiceType == v1.ServiceTypeNodePort {
			portMap := map[int32]struct{}{}
			ports, _ := corehelper.ParsePorts(inst.Spec.Access.Ports)
			for _, port := range ports {
				if _, ok := portMap[port]; ok {
					return nil, fmt.Errorf("port %d has assigned", port)
				}
				portMap[port] = struct{}{}
			}
			ports, _ = corehelper.ParsePorts(inst.Spec.Sentinel.Access.Ports)
			for _, port := range ports {
				if _, ok := portMap[port]; ok {
					return nil, fmt.Errorf("port %d has assigned", port)
				}
			}
		}
	case core.ValkeyReplica:
		if inst.Spec.Replicas == nil {
			return nil, fmt.Errorf("instance replicas not specified")
		}
		if inst.Spec.Replicas.Shards != 1 {
			return nil, fmt.Errorf("spec.replicas.shards must be 1")
		}

		if err := coreVal.ValidateInstanceAccess(&inst.Spec.Access,
			helper.CalculateNodeCount(inst.Spec.Arch, inst.Spec.Replicas.Shards, inst.Spec.Replicas.ReplicasOfShard),
			&warns); err != nil {
			return warns, err
		}
	}
	return
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type inst.
func (v *ValkeyCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (warns admission.Warnings, err error) {
	return
}
