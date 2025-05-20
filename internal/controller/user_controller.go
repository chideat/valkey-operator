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
	"reflect"
	"strings"
	"time"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/chideat/valkey-operator/internal/controller/user"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	security "github.com/chideat/valkey-operator/pkg/security/password"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	UserFinalizer = "buf.red/user-finalizer"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	K8sClient     kubernetes.ClientSet
	EventRecorder record.EventRecorder
	Handler       *user.UserHandler
}

// +kubebuilder:rbac:groups=valkey.buf.red,resources=users,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=valkey.buf.red,resources=users/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=valkey.buf.red,resources=users/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("User").WithValues("target", req.String())

	instance := v1alpha1.User{}
	err := r.Client.Get(ctx, req.NamespacedName, &instance)
	if err != nil {
		logger.Error(err, "get valkey user failed")
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	isMarkedToBeDeleted := instance.GetDeletionTimestamp() != nil
	if isMarkedToBeDeleted {
		logger.Info("remove finalizer")
		controllerutil.RemoveFinalizer(&instance, UserFinalizer)
		if err := r.Update(ctx, &instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	vkName := instance.Spec.InstanceName
	switch instance.Spec.Arch {
	case core.ValkeySentinel, core.ValkeyReplica:
		rf := &v1alpha1.Failover{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: instance.Namespace, Name: vkName}, rf); err != nil {
			if errors.IsNotFound(err) {
				logger.Error(err, "instance not found", "name", vkName)
				return ctrl.Result{Requeue: true}, r.Delete(ctx, &instance)
			}
			logger.Error(err, "get failover failed", "name", instance.Name)
			return ctrl.Result{}, err
		}
	case core.ValkeyCluster:
		cluster := &v1alpha1.Cluster{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: instance.Namespace, Name: vkName}, cluster); err != nil {
			if errors.IsNotFound(err) {
				logger.Error(err, "instance not found", "name", vkName)
				return ctrl.Result{Requeue: true}, r.Delete(ctx, &instance)
			}
			logger.Error(err, "get cluster instance failed", "name", instance.Name)
			return ctrl.Result{}, err
		}
	}

	// verify user password
	for _, name := range instance.Spec.PasswordSecrets {
		if name == "" {
			continue
		}
		secret := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{
			Namespace: instance.Namespace,
			Name:      name,
		}, secret); err != nil {
			logger.Error(err, "get secret failed", "secret name", name)
			instance.Status.Message = err.Error()
			instance.Status.Phase = v1alpha1.UserFail
			if e := r.Client.Status().Update(ctx, &instance); e != nil {
				logger.Error(e, "update User status to Fail failed")
			}
			return ctrl.Result{}, err
		} else if err := security.PasswordValidate(string(secret.Data["password"]), 8, 32); err != nil {
			if instance.Spec.AccountType != v1alpha1.SystemAccount {
				instance.Status.Message = err.Error()
				instance.Status.Phase = v1alpha1.UserFail
				if e := r.Client.Status().Update(ctx, &instance); e != nil {
					logger.Error(e, "update User status to Fail failed")
				}
				return ctrl.Result{}, err
			}
		}

		if secret.GetLabels() == nil {
			secret.SetLabels(map[string]string{})
		}
		if secret.Labels[builder.InstanceNameLabelKey] != vkName ||
			len(secret.GetOwnerReferences()) == 0 || secret.OwnerReferences[0].UID != instance.GetUID() {

			secret.Labels[builder.ManagedByLabelKey] = config.AppName
			secret.Labels[builder.InstanceNameLabelKey] = vkName
			secret.OwnerReferences = util.BuildOwnerReferences(&instance)
			if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				return r.K8sClient.UpdateSecret(ctx, instance.Namespace, secret)
			}); err != nil {
				logger.Error(err, "update secret owner failed", "secret", secret.Name)
				instance.Status.Message = err.Error()
				instance.Status.Phase = v1alpha1.UserFail
				return ctrl.Result{RequeueAfter: time.Second * 5}, r.Client.Status().Update(ctx, &instance)
			}
		}
	}

	if err := r.Handler.Do(ctx, instance, logger); err != nil {
		if strings.Contains(err.Error(), "instance is not ready") ||
			strings.Contains(err.Error(), "node not ready") ||
			strings.Contains(err.Error(), "user not operator") ||
			strings.Contains(err.Error(), "ERR unknown command `ACL`") {
			logger.V(3).Info("instance is not ready", "instance", vkName)
			instance.Status.Message = err.Error()
			instance.Status.Phase = v1alpha1.UserPending
			if err := r.updateUserStatus(ctx, &instance); err != nil {
				logger.Error(err, "update User status to Pending failed")
			}
			return ctrl.Result{RequeueAfter: time.Second * 15}, nil
		}

		instance.Status.Message = err.Error()
		instance.Status.Phase = v1alpha1.UserFail
		logger.Error(err, "user reconcile failed")
		if err := r.updateUserStatus(ctx, &instance); err != nil {
			logger.Error(err, "update User status to Fail failed")
		}
		return reconcile.Result{RequeueAfter: time.Second * 10}, nil
	}
	instance.Status.Phase = v1alpha1.UserReady
	instance.Status.Message = ""
	logger.V(3).Info("user reconcile success")
	if err := r.updateUserStatus(ctx, &instance); err != nil {
		logger.Error(err, "update User status to Success failed")
		return reconcile.Result{RequeueAfter: time.Second * 10}, err
	}
	if !controllerutil.ContainsFinalizer(&instance, UserFinalizer) {
		controllerutil.AddFinalizer(&instance, UserFinalizer)
		if err := r.updateUser(ctx, &instance); err != nil {
			logger.Error(err, "update finalizer user failed")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *UserReconciler) updateUserStatus(ctx context.Context, inst *v1alpha1.User) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var oldUser v1alpha1.User
		if err := r.Get(ctx, types.NamespacedName{Namespace: inst.Namespace, Name: inst.Name}, &oldUser); err != nil {
			return err
		}
		inst.ResourceVersion = oldUser.ResourceVersion
		return r.Status().Update(ctx, inst)
	})
}

func (r *UserReconciler) updateUser(ctx context.Context, inst *v1alpha1.User) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var oldUser v1alpha1.User
		if err := r.Get(ctx, types.NamespacedName{Namespace: inst.Namespace, Name: inst.Name}, &oldUser); err != nil {
			return err
		}
		inst.ResourceVersion = oldUser.ResourceVersion
		return r.Update(ctx, inst)
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.User{}).
		Owns(&corev1.Secret{}).
		WithEventFilter(generationChangedFilter()).
		WithOptions(controller.Options{MaxConcurrentReconciles: 8}).
		Complete(r)
}

func generationChangedFilter() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld == nil {
				return false
			}
			if e.ObjectNew == nil {
				return false
			}
			if reflect.TypeOf(e.ObjectNew).String() == reflect.TypeOf(&corev1.Secret{}).String() {
				return true
			}
			// Ignore updates to CR status in which case metadata.Generation does not change
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
		},
	}
}
