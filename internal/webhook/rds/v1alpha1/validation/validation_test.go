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

package validation

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestValidateClusterScalingResource(t *testing.T) {
	dss := int64(1) << 30
	memReq := int64(float64(dss)/float64(MinMaxMemoryPercentage)) + 1

	type args struct {
		shards   int32
		resource *corev1.ResourceRequirements
		datasize []int64
	}
	tests := []struct {
		name      string
		args      args
		wantErr   bool
		wantWarns admission.Warnings
	}{
		{
			name: "just match the maxmemory limit",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(memReq, resource.BinarySI),
					},
				},
				datasize: []int64{dss, dss, dss},
			},
			wantErr: false,
		},
		{
			name: "just not match the maxmemory limit",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(memReq-2, resource.BinarySI),
					},
				},
				datasize: []int64{dss, dss, dss},
			},
			wantErr: true,
		},
		{
			name: "nil resource check",
			args: args{
				shards:   3,
				resource: nil,
			},
			wantErr: false,
		},
		{
			name: "nil resource check with data",
			args: args{
				shards:   3,
				datasize: []int64{dss, dss, dss},
			},
			wantErr: false,
		},
		{
			name: "empty resource check",
			args: args{
				shards:   3,
				resource: &corev1.ResourceRequirements{},
			},
			wantErr: false,
		},
		{
			name: "empty resource check with data",
			args: args{
				shards:   3,
				resource: &corev1.ResourceRequirements{},
				datasize: []int64{dss, dss, dss},
			},
			wantErr: false,
		},
		{
			name: "min memory limit check",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(1<<24, resource.BinarySI),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "min memory limit check with warning",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(1<<25, resource.BinarySI),
					},
				},
			},
			wantErr: false,
			wantWarns: admission.Warnings{
				"memory limit it's recommended to be at least 128Mi",
			},
		},
		{
			name: "max memory limit check with warning",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(1<<36, resource.BinarySI),
					},
				},
			},
			wantErr:   false,
			wantWarns: admission.Warnings{"memory limit it's recommended to be at most 32Gi"},
		},
		{
			name: "3=>6 without change memory limit",
			args: args{
				shards: 6,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(memReq, resource.BinarySI),
					},
				},
				datasize: []int64{dss, dss, dss},
			},
			wantErr: false,
		},
		{
			name: "3=>6 without update the memory limit",
			args: args{
				shards: 6,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(memReq, resource.BinarySI),
					},
				},
				datasize: []int64{dss, dss, dss},
			},
			wantErr: false,
		},
		{
			name: "3=>6 with halve the memory limit",
			args: args{
				shards: 6,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(1+memReq/2, resource.BinarySI),
					},
				},
				datasize: []int64{dss, dss, dss},
			},
			wantErr: true,
		},
		{
			name: "4=>3 with not scaling memory",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(memReq, resource.BinarySI),
					},
				},
				datasize: []int64{dss, dss, dss, dss},
			},
			wantErr: true,
		},
		{
			name: "4=>3 with just match memory",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(int64(float64(dss+dss)/MinMaxMemoryPercentage), resource.BinarySI),
					},
				},
				datasize: []int64{dss, dss, dss, dss},
			},
			wantErr: false,
		},
		{
			name: "4=>3 with only the deleting shards have data",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(int64(float64(dss)/MinMaxMemoryPercentage), resource.BinarySI),
					},
				},
				datasize: []int64{0, 0, 0, dss},
			},
			wantErr: false,
		},
		{
			name: "4=>3 with the deleting shards is empty",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(int64(float64(dss)/MinMaxMemoryPercentage), resource.BinarySI),
					},
				},
				datasize: []int64{dss, dss, dss, 0},
			},
			wantErr: false,
		},
		{
			name: "6=>3 deleting 3 shards",
			args: args{
				shards: 3,
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(int64(float64(dss*4)/MinMaxMemoryPercentage), resource.BinarySI),
					},
				},
				datasize: []int64{dss, dss, dss, dss, dss, dss},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var warns admission.Warnings
			if err := ValidateClusterScalingResource(tt.args.shards, tt.args.resource, tt.args.datasize, &warns); (err != nil) != tt.wantErr {
				t.Errorf("ValidateClusterScalingResource() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(warns, tt.wantWarns) {
				t.Errorf("ValidateClusterScalingResource() warns = %v, want %v", warns, tt.wantWarns)
			}
		})
	}
}

func TestValidateReplicationScalingResource(t *testing.T) {
	dss := int64(1) << 30
	memReq := int64(float64(dss)/float64(MinMaxMemoryPercentage)) + 1

	type args struct {
		resource *corev1.ResourceRequirements
		datasize int64
	}
	tests := []struct {
		name      string
		args      args
		wantErr   bool
		wantWarns admission.Warnings
	}{
		{
			name: "just match the maxmemory limit",
			args: args{
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(memReq, resource.BinarySI),
					},
				},
				datasize: dss,
			},
			wantErr: false,
		},
		{
			name: "just not match the maxmemory limit",
			args: args{
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(memReq-2, resource.BinarySI),
					},
				},
				datasize: dss,
			},
			wantErr: true,
		},
		{
			name: "nil resource check",
			args: args{
				resource: nil,
			},
			wantErr: false,
		},
		{
			name: "nil resource check with data",
			args: args{
				datasize: dss,
			},
			wantErr: false,
		},
		{
			name: "empty resource check",
			args: args{
				resource: &corev1.ResourceRequirements{},
			},
			wantErr: false,
		},
		{
			name: "empty resource check with data",
			args: args{
				resource: &corev1.ResourceRequirements{},
				datasize: dss,
			},
			wantErr: false,
		},
		{
			name: "min memory limit check",
			args: args{
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(1<<24, resource.BinarySI),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "min memory limit check with warning",
			args: args{
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(1<<25, resource.BinarySI),
					},
				},
			},
			wantErr: false,
			wantWarns: admission.Warnings{
				"memory limit it's recommended to be at least 128Mi",
			},
		},
		{
			name: "max memory limit check with warning",
			args: args{
				resource: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(1<<36, resource.BinarySI),
					},
				},
			},
			wantErr:   false,
			wantWarns: admission.Warnings{"memory limit it's recommended to be at most 32Gi"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var warns admission.Warnings
			if err := ValidateReplicationScalingResource(tt.args.resource, tt.args.datasize, &warns); (err != nil) != tt.wantErr {
				t.Errorf("ValidateReplicationScalingResource() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(warns, tt.wantWarns) {
				t.Errorf("ValidateClusterScalingResource() warns = %v, want %v", warns, tt.wantWarns)
			}
		})
	}
}
