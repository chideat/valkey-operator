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

package main

import (
	"fmt"
	"testing"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	rdsv1alpha1 "github.com/chideat/valkey-operator/api/rds/v1alpha1"
	valkeybufredv1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TestSchemeCoversEveryTypeTheOperatorWrites pins the manager scheme against the
// objects the reconcilers construct.
//
// cert-manager was missing, so any instance with access.enableTLS stopped at
// Initializing with "no kind is registered for the type v1.Certificate in
// scheme". The RBAC marker granting certificates create/update was already
// there, so the omission was the registration alone, and it made enableTLS
// unusable rather than degraded.
func TestSchemeCoversEveryTypeTheOperatorWrites(t *testing.T) {
	for _, obj := range []runtime.Object{
		&certv1.Certificate{},
		&valkeybufredv1alpha1.Cluster{},
		&valkeybufredv1alpha1.Failover{},
		&valkeybufredv1alpha1.Sentinel{},
		&valkeybufredv1alpha1.User{},
		&rdsv1alpha1.Valkey{},
	} {
		t.Run(fmt.Sprintf("%T", obj), func(t *testing.T) {
			kinds, _, err := scheme.ObjectKinds(obj)
			if err != nil {
				t.Fatalf("%T is not registered in the manager scheme: %v", obj, err)
			}
			if len(kinds) == 0 {
				t.Fatalf("%T resolved to no kind", obj)
			}
		})
	}
}
