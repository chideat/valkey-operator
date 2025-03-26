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

package validation

import (
	"context"
	"fmt"

	security "github.com/chideat/valkey-operator/pkg/security/password"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	MinMemoryLimits        = resource.NewQuantity(1<<25, resource.BinarySI)
	WarningMinMemoryLimits = resource.NewQuantity(1<<27, resource.BinarySI)
	WarningMaxMemoryLimits = resource.NewQuantity(1<<35, resource.BinarySI)
)

const (
	MinMaxMemoryPercentage = 0.8
)

// ValidateClusterScaling validates the scaling of a cluster
func ValidateClusterScalingResource(shards int32, resReq *corev1.ResourceRequirements, datasets []int64, warns *admission.Warnings) (err error) {
	if resReq == nil || resReq.Limits.Memory().Value() == 0 {
		return nil
	}

	memoryLimit := resReq.Limits.Memory()
	if memoryLimit.Cmp(*MinMemoryLimits) < 0 {
		return fmt.Errorf("memory limit is too low, must be at least %s", MinMemoryLimits.String())
	}
	if memoryLimit.Cmp(*WarningMinMemoryLimits) < 0 {
		*warns = append(*warns, fmt.Sprintf("memory limit it's recommended to be at least %s", WarningMinMemoryLimits.String()))
	}
	if memoryLimit.Cmp(*WarningMaxMemoryLimits) > 0 {
		*warns = append(*warns, fmt.Sprintf("memory limit it's recommended to be at most %s", WarningMaxMemoryLimits.String()))
	}

	if len(datasets) >= 3 {
		maxdatasets := float64(lo.Max(datasets))
		memoryLimitVal := float64(memoryLimit.Value())

		if int(shards) >= len(datasets) {
			if wanted := maxdatasets / MinMaxMemoryPercentage; memoryLimitVal < wanted {
				res := resource.NewQuantity(int64(wanted), resource.BinarySI)
				return fmt.Errorf("memory limit may can't serve current dataset, should be at least %s", res.String())
			}
		} else {
			// NOTE: every shard should be able to hold all the deleting shards
			mergedDataSize := float64(lo.Sum(datasets[shards:]))
			maxdatasets = float64(lo.Max(datasets[0:shards])) + mergedDataSize
			if wanted := maxdatasets / MinMaxMemoryPercentage; memoryLimitVal < wanted {
				res := resource.NewQuantity(int64(wanted), resource.BinarySI)
				return fmt.Errorf("memory limit may can't serve current dataset, shoud be at least %s", res.String())
			}
		}
	}
	return
}

// ValidateReplicationScaling validates the scaling of a single replication
func ValidateReplicationScalingResource(resReq *corev1.ResourceRequirements, datasets int64, warns *admission.Warnings) (err error) {
	if resReq == nil || resReq.Limits.Memory().Value() == 0 {
		return nil
	}

	memoryLimit := resReq.Limits.Memory()
	if memoryLimit.Cmp(*MinMemoryLimits) < 0 {
		return fmt.Errorf("memory limit is too low, must be at least %s", MinMemoryLimits.String())
	}
	if memoryLimit.Cmp(*WarningMinMemoryLimits) < 0 {
		*warns = append(*warns, fmt.Sprintf("memory limit it's recommended to be at least %s", WarningMinMemoryLimits.String()))
	}
	if memoryLimit.Cmp(*WarningMaxMemoryLimits) > 0 {
		*warns = append(*warns, fmt.Sprintf("memory limit it's recommended to be at most %s", WarningMaxMemoryLimits.String()))
	}

	if datasets > 0 {
		maxdatasets := float64(datasets)
		memoryLimitVal := float64(memoryLimit.Value())
		if wanted := maxdatasets / MinMaxMemoryPercentage; memoryLimitVal < wanted {
			res := resource.NewQuantity(int64(wanted), resource.BinarySI)
			return fmt.Errorf("memory limit may can't serve current dataset, should be at least %s", res.String())
		}
	}
	return
}

func ValidateActiveRedisService(f bool, serviceID *int32, warns *admission.Warnings) (err error) {
	if !f {
		return nil
	}

	if serviceID == nil || *serviceID < 0 || *serviceID > 15 {
		return fmt.Errorf("activeredis is enabled but serviceID is not valid")
	}
	return
}

func ValidatePasswordSecret(namespace, secretName string, mgrClient client.Client, warns *admission.Warnings) error {
	if mgrClient == nil {
		return nil
	}
	if secretName != "" {
		secret := &corev1.Secret{}
		if err := mgrClient.Get(context.Background(), types.NamespacedName{
			Namespace: namespace,
			Name:      secretName,
		}, secret); err != nil {
			return err
		}
		return security.PasswordValidate(string(secret.Data["password"]), 8, 32)
	}
	return nil
}
