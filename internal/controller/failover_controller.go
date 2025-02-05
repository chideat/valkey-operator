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
	"fmt"
	"reflect"
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
	"github.com/chideat/valkey-operator/internal/builder/certbuilder"
	"github.com/chideat/valkey-operator/internal/builder/sentinelbuilder"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/chideat/valkey-operator/internal/ops"
)

// FailoverReconciler reconciles a Failover object
type FailoverReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder
	Engine        *ops.OpEngine
}

// +kubebuilder:rbac:groups=valkey.buf.red,resources=failovers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=valkey.buf.red,resources=failovers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=valkey.buf.red,resources=failovers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *FailoverReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("target", req.String())

	var instance v1alpha1.Failover
	if err := r.Get(ctx, req.NamespacedName, &instance); errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		logger.Error(err, "get resource failed")
		return ctrl.Result{}, err
	}

	// err := instance.Validate()
	// if err != nil {
	// 	instance.Status.Message = err.Error()
	// 	instance.Status.Phase = v1alpha1.Fail
	// 	if err := r.Status().Update(ctx, &instance); err != nil {
	// 		logger.Error(err, "update status failed")
	// 		return ctrl.Result{}, err
	// 	}
	// 	return ctrl.Result{}, err
	// }

	if crVersion := instance.Annotations[config.CRVersionKey]; crVersion == "" {
		if config.GetOperatorVersion() != "" {
			if instance.Annotations == nil {
				instance.Annotations = make(map[string]string)
			}
			instance.Annotations[config.CRVersionKey] = config.GetOperatorVersion()
			if err := r.Client.Update(ctx, &instance); err != nil {
				logger.Error(err, "update instance actor version failed")
			}
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
	}

	{
		var (
			nodes       []v1alpha1.SentinelMonitorNode
			serviceName = sentinelbuilder.SentinelHeadlessServiceName(instance.GetName())
			status      = &instance.Status
			oldStatus   = instance.Status.DeepCopy()
		)
		if instance.Spec.Sentinel == nil {
			status.Monitor = v1alpha1.MonitorStatus{
				Policy: v1alpha1.ManualFailoverPolicy,
			}
		} else {
			// TODO: use DNS SRV replace config all sentinel node address, which will cause data pods restart
			for i := 0; i < int(instance.Spec.Sentinel.Replicas); i++ {
				podName := sentinelbuilder.SentinelPodServiceName(instance.GetName(), i)
				srv := fmt.Sprintf("%s.%s.%s", podName, serviceName, instance.GetNamespace())
				nodes = append(nodes, v1alpha1.SentinelMonitorNode{IP: srv, Port: 26379})
			}

			status.Monitor.Policy = v1alpha1.SentinelFailoverPolicy
			if instance.Spec.Sentinel.SentinelReference == nil {
				// HARDCODE: use mymaster as sentinel monitor name
				status.Monitor.Name = "mymaster"

				// append history password secrets
				// NOTE: here recorded empty password
				passwordSecret := instance.Spec.Sentinel.Access.DefaultPasswordSecret
				if status.Monitor.PasswordSecret != passwordSecret {
					status.Monitor.OldPasswordSecret = status.Monitor.PasswordSecret
					status.Monitor.PasswordSecret = passwordSecret
				}

				if instance.Spec.Sentinel.Access.EnableTLS {
					if instance.Spec.Sentinel.Access.ExternalTLSSecret != "" {
						status.Monitor.TLSSecret = instance.Spec.Sentinel.Access.ExternalTLSSecret
					} else {
						status.Monitor.TLSSecret = certbuilder.GenerateSSLSecretName(instance.GetName())
					}
				}
				status.Monitor.Nodes = nodes
			} else {
				status.Monitor.Name = fmt.Sprintf("%s.%s", instance.GetNamespace(), instance.GetName())
				status.Monitor.OldPasswordSecret = status.Monitor.PasswordSecret
				status.Monitor.PasswordSecret = instance.Spec.Sentinel.SentinelReference.Auth.PasswordSecret
				status.Monitor.TLSSecret = instance.Spec.Sentinel.SentinelReference.Auth.TLSSecret
				status.Monitor.Nodes = instance.Spec.Sentinel.SentinelReference.Nodes
			}
		}
		if !reflect.DeepEqual(status, oldStatus) {
			if err := r.Status().Update(ctx, &instance); err != nil {
				logger.Error(err, "update status failed")
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
	}
	return r.Engine.Run(ctx, &instance)
}

// SetupWithManager sets up the controller with the Manager.
func (r *FailoverReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Failover{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 8}).
		Owns(&v1alpha1.Sentinel{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
