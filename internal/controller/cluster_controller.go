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

package controller

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/chideat/valkey-operator/internal/ops"
)

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder
	Engine        *ops.OpEngine
}

// +kubebuilder:rbac:groups=valkey.buf.red,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=valkey.buf.red,resources=clusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=valkey.buf.red,resources=clusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("target", req.String())

	var instance v1alpha1.Cluster
	if err := r.Get(ctx, req.NamespacedName, &instance); errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		logger.Error(err, "get resource failed")
		return ctrl.Result{}, err
	}

	// update default status
	if instance.Status.Phase == "" {
		// update status to creating
		r.EventRecorder.Eventf(&instance, corev1.EventTypeNormal, "Creating", "new instance request")

		instance.Status.Phase = v1alpha1.ClusterPhaseCreating
		if err := r.Status().Update(ctx, &instance); err != nil {
			logger.Error(err, "update instance status failed")
			if errors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}
	}

	crVersion := instance.Annotations[config.CRVersionKey]
	if crVersion == "" {
		if config.GetOperatorVersion() != "" {
			if instance.Annotations == nil {
				instance.Annotations = make(map[string]string)
			}
			// update crVersion to instance
			instance.Annotations[config.CRVersionKey] = config.GetOperatorVersion()
			if err := r.Update(ctx, &instance); err != nil {
				logger.Error(err, "update instance actor version failed")
			}
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
	}

	// ================ setup default ===================

	// _ = instance.Default()

	// ================ setup default end ===================

	return r.Engine.Run(ctx, &instance)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Cluster{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 8}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
