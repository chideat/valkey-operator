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
	"errors"
	"fmt"
	"slices"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	valkeybufredv1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var clusterlog = logf.Log.WithName("cluster-resource")

// SetupClusterWebhookWithManager registers the webhook for Cluster in the manager.
func SetupClusterWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&valkeybufredv1alpha1.Cluster{}).
		WithValidator(&ClusterCustomValidator{}).
		Complete()
}

// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-valkey-buf-red-v1alpha1-cluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=valkey.buf.red,resources=clusters,verbs=create,versions=v1alpha1,name=vcluster-v1alpha1.kb.io,admissionReviewVersions=v1

// ClusterCustomValidator struct is responsible for validating the Cluster resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type ClusterCustomValidator struct{}

var _ webhook.CustomValidator = &ClusterCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Cluster.
func (v *ClusterCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	cluster, ok := obj.(*valkeybufredv1alpha1.Cluster)
	if !ok {
		return nil, fmt.Errorf("expected a Cluster object but got %T", obj)
	}

	if index := slices.IndexFunc(cluster.GetOwnerReferences(), func(ownerRef v1.OwnerReference) bool {
		return ownerRef.Kind == "Valkey"
	}); index < 0 {
		return nil, errors.New("please use Valkey API to create Cluster resources")
	}
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Cluster.
func (v *ClusterCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Cluster.
func (v *ClusterCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
