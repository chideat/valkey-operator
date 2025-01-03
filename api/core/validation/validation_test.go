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
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	v1 "k8s.io/api/core/v1"
)

func TestValidateInstanceAccess(t *testing.T) {
	type args struct {
		acc *core.InstanceAccess
		nc  int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "service type not specified",
			args: args{
				acc: &core.InstanceAccess{},
			},
		},
		{
			name: "service type is ClusterIP",
			args: args{
				acc: &core.InstanceAccess{ServiceType: v1.ServiceTypeClusterIP},
			},
		},
		{
			name: "service type is LoadBalancer",
			args: args{
				acc: &core.InstanceAccess{ServiceType: v1.ServiceTypeLoadBalancer},
			},
		},
		{
			name: "service type is ExternalName",
			args: args{
				acc: &core.InstanceAccess{ServiceType: v1.ServiceTypeExternalName},
			},
			wantErr: true,
		},
		{
			name: "nodeport without specified ports",
			args: args{
				acc: &core.InstanceAccess{ServiceType: v1.ServiceTypeNodePort},
			},
			wantErr: false,
		},
		{
			name: "nodeport without specified sequence ports",
			args: args{
				acc: &core.InstanceAccess{
					ServiceType: v1.ServiceTypeNodePort,
					Ports:       "6379,6380,6381",
				},
			},
			wantErr: true,
		},
		{
			name: "nodeport with matched sequence ports",
			args: args{
				acc: &core.InstanceAccess{
					ServiceType: v1.ServiceTypeNodePort,
					Ports:       "6379,6380,6381",
				},
				nc: 3,
			},
			wantErr: false,
		},
		{
			name: "nodeport with not matched sequence ports",
			args: args{
				acc: &core.InstanceAccess{
					ServiceType: v1.ServiceTypeNodePort,
					Ports:       "6379,6380,6381",
				},
				nc: 5,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateInstanceAccess(tt.args.acc, tt.args.nc, nil); (err != nil) != tt.wantErr {
				t.Errorf("ValidateInstanceAccess() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
