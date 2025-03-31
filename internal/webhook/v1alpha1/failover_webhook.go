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
var failoverlog = logf.Log.WithName("failover-resource")

// SetupFailoverWebhookWithManager registers the webhook for Failover in the manager.
func SetupFailoverWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&valkeybufredv1alpha1.Failover{}).
		WithValidator(&FailoverCustomValidator{}).
		Complete()
}

// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-valkey-buf-red-v1alpha1-failover,mutating=false,failurePolicy=fail,sideEffects=None,groups=valkey.buf.red,resources=failovers,verbs=create,versions=v1alpha1,name=vfailover-v1alpha1.kb.io,admissionReviewVersions=v1

// FailoverCustomValidator struct is responsible for validating the Failover resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type FailoverCustomValidator struct{}

var _ webhook.CustomValidator = &FailoverCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Failover.
func (v *FailoverCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	failover, ok := obj.(*valkeybufredv1alpha1.Failover)
	if !ok {
		return nil, fmt.Errorf("expected a Failover object but got %T", obj)
	}
	failoverlog.Info("Validation for Failover upon creation", "name", failover.GetName())

	if index := slices.IndexFunc(failover.GetOwnerReferences(), func(ownerRef v1.OwnerReference) bool {
		return ownerRef.Kind == "Valkey"
	}); index < 0 {
		return nil, errors.New("please use Valkey API to create Failover resources")
	}
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Failover.
func (v *FailoverCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Failover.
func (v *FailoverCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
