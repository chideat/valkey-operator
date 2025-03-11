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

package rds

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/chideat/valkey-operator/api/core"
	rdsv1alpha1 "github.com/chideat/valkey-operator/api/rds/v1alpha1"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/clusterbuilder"
	"github.com/chideat/valkey-operator/internal/builder/failoverbuilder"
	"github.com/chideat/valkey-operator/internal/config"
	vkHandler "github.com/chideat/valkey-operator/internal/controller/rds/valkey"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ValkeyReconciler reconciles a Valkey object
type ValkeyReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder
}

const (
	pvcFinalizer  = "delete-pvc"
	requeueSecond = 10 * time.Second
)

// +kubebuilder:rbac:groups=rds.valkey.buf.red,resources=valkeys,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rds.valkey.buf.red,resources=valkeys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rds.valkey.buf.red,resources=valkeys/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ValkeyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(context.TODO()).WithValues("target", req.String()).WithName("RDS")

	inst := &rdsv1alpha1.Valkey{}
	if err := r.Get(ctx, req.NamespacedName, inst); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		logger.Error(err, "Fail to get valkey instance")
		return reconcile.Result{}, err
	}
	if inst.GetDeletionTimestamp() != nil {
		if err := r.processFinalizer(inst); err != nil {
			logger.Error(err, "fail to process finalizer")
			return r.updateInstanceStatus(ctx, inst, err, logger)
		}
		return ctrl.Result{}, nil
	}

	oldInst := inst.DeepCopy()
	// inst.Default()
	if !reflect.DeepEqual(oldInst, inst) {
		if err := r.Update(ctx, inst); err != nil {
			logger.Error(err, "fail to update valkey instance")
			return ctrl.Result{}, err
		}
	}

	var (
		err             error
		operatorVersion = config.GetOperatorVersion()
	)

	if operatorVersion != "" && operatorVersion != inst.Annotations[config.OperatorVersionAnnotation] {
		logger.V(3).Info("instance operatorVersion is not match")
		if inst.Annotations == nil {
			inst.Annotations = make(map[string]string)
		}
		inst.Annotations[config.OperatorVersionAnnotation] = operatorVersion
		return r.updateInstance(ctx, inst, logger)
	}

	// ensure instance
	switch inst.Spec.Arch {
	case core.ValkeyCluster:
		if err := r.reconcileCluster(ctx, inst, logger); err != nil {
			logger.Error(err, "fail to reconcile cluster instance")
			return r.updateInstanceStatus(ctx, inst, err, logger)
		}
	case core.ValkeyFailover, core.ValkeyReplica:
		if shards := inst.Spec.Replicas.Shards; shards == 0 {
			inst.Spec.Replicas.Shards = 1
		}
		if err := r.reconcileFailover(ctx, inst, logger); err != nil {
			logger.Error(err, fmt.Sprintf("fail to reconcile %s instance", inst.Spec.Arch))
			return r.updateInstanceStatus(ctx, inst, err, logger)
		}
	default:
		err = fmt.Errorf("this arch isn't valid, must be cluster, sentinel or standalone")
		return ctrl.Result{}, err
	}

	return r.updateInstanceStatus(ctx, inst, err, logger)
}

func (r *ValkeyReconciler) reconcileFailover(ctx context.Context, inst *rdsv1alpha1.Valkey, logger logr.Logger) error {
	if inst.Spec.CustomConfigs == nil {
		inst.Spec.CustomConfigs = map[string]string{}
	}

	failover := &v1alpha1.Failover{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      inst.Name,
		Namespace: inst.Namespace,
	}, failover); errors.IsNotFound(err) {
		failover, err = vkHandler.GenerateFailover(inst)
		if err != nil {
			return err
		}

		if err := r.Create(ctx, failover); err != nil {
			inst.Status.Phase = rdsv1alpha1.Failed
			inst.Status.Message = err.Error()
			logger.Error(err, "fail to create failover instance")
			return err
		}
		inst.Status.MatchLabels = failoverbuilder.GenerateSelectorLabels(inst.Name)
		return nil
	} else if err != nil {
		return err
	}

	if len(inst.Status.MatchLabels) == 0 {
		inst.Status.MatchLabels = failoverbuilder.GenerateSelectorLabels(inst.Name)
	}
	for key := range vkHandler.GetValkeyConfigsApplyPolicyByVersion(inst.Spec.Version) {
		if inst.Spec.CustomConfigs[key] != failover.Spec.CustomConfigs[key] {
			if inst.Spec.PodAnnotations == nil {
				inst.Spec.PodAnnotations = map[string]string{}
			}
			inst.Spec.PodAnnotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339Nano)
			break
		}
	}

	newFailover, err := vkHandler.GenerateFailover(inst)
	if err != nil {
		logger.Error(err, "fail to generate failover instance")
		return err
	}
	// ensure inst should update
	if vkHandler.ShouldUpdateFailover(failover, newFailover, logger) {
		newFailover.ResourceVersion = failover.ResourceVersion
		newFailover.Status = failover.Status
		if err := r.updateFailoverInstance(ctx, newFailover); err != nil {
			inst.Status.Phase = rdsv1alpha1.Failed
			inst.Status.Message = err.Error()
			logger.Error(err, "fail to update failover inst")
			return err
		}
		failover = newFailover
	}

	inst.Status.LastShardCount = 1
	inst.Status.LastVersion = inst.Spec.Version
	inst.Status.ClusterNodes = failover.Status.Nodes
	inst.Status.Message = failover.Status.Message
	if failover.Status.Phase == v1alpha1.FailoverPhaseFailed {
		logger.V(3).Info("instance is fail")
		inst.Status.Phase = rdsv1alpha1.Failed
		inst.Status.Message = failover.Status.Message
	} else if failover.Status.Phase == v1alpha1.FailoverPhaseReady {
		logger.V(3).Info("instance is ready")
		inst.Status.Phase = rdsv1alpha1.Ready
	} else if failover.Status.Phase == v1alpha1.FailoverPhasePaused {
		logger.V(3).Info("instance is paused")
		inst.Status.Phase = rdsv1alpha1.Paused
	} else {
		logger.V(3).Info("instance is unhealthy, waiting failover to up", "phase", failover.Status.Phase)
		inst.Status.Phase = rdsv1alpha1.Initializing
	}
	return nil
}

func (r *ValkeyReconciler) reconcileCluster(ctx context.Context, inst *rdsv1alpha1.Valkey, logger logr.Logger) error {
	cluster := &v1alpha1.Cluster{}
	if inst.Spec.PodAnnotations == nil {
		inst.Spec.PodAnnotations = make(map[string]string)
	}

	if err := r.Get(ctx, types.NamespacedName{
		Name:      inst.Name,
		Namespace: inst.Namespace,
	}, cluster); errors.IsNotFound(err) {
		cluster, err = vkHandler.GenerateRedisCluster(inst)
		if err != nil {
			return err
		}
		// Record actor versions too keep actions consistent
		cluster.Annotations[config.CRVersionKey] = config.GetOperatorVersion()

		if err := r.Create(ctx, cluster); err != nil {
			logger.Error(err, "fail to create cluster instance")
			inst.Status.Phase = rdsv1alpha1.Failed
			inst.Status.Message = err.Error()
			return err
		}
		inst.Status.MatchLabels = clusterbuilder.GenerateClusterLabels(inst.Name, nil)
		return nil
	} else if err != nil {
		return err
	}

	if len(inst.Status.MatchLabels) == 0 {
		inst.Status.MatchLabels = clusterbuilder.GenerateClusterLabels(inst.Name, nil)
	}
	for key := range vkHandler.GetValkeyConfigsApplyPolicyByVersion(inst.Spec.Version) {
		if inst.Spec.CustomConfigs[key] != cluster.Spec.CustomConfigs[key] {
			if inst.Spec.PodAnnotations == nil {
				inst.Spec.PodAnnotations = map[string]string{}
			}
			inst.Spec.PodAnnotations[builder.RestartAnnotationKey] = time.Now().Format(time.RFC3339Nano)
			break
		}
	}
	newCluster, err := vkHandler.GenerateRedisCluster(inst)
	if err != nil {
		return err
	}

	// ensure inst should update
	if vkHandler.ShouldUpdateCluster(cluster, newCluster, logger) {
		newCluster.ResourceVersion = cluster.ResourceVersion
		newCluster.Status = cluster.Status
		if err := r.updateClusterInstance(ctx, newCluster); err != nil {
			logger.Error(err, "fail to update cluster instance")
			inst.Status.Phase = rdsv1alpha1.Failed
			inst.Status.Message = err.Error()
			return err
		}
		cluster = newCluster
	}

	inst.Status.LastShardCount = cluster.Spec.Replicas.Shards
	inst.Status.LastVersion = inst.Spec.Version
	inst.Status.ClusterNodes = cluster.Status.Nodes
	inst.Status.Message = cluster.Status.Message
	if vkHandler.ClusterIsUp(cluster) {
		logger.V(3).Info("instance is ready")
		inst.Status.Phase = rdsv1alpha1.Ready
	} else if cluster.Status.Phase == v1alpha1.ClusterPhasePaused {
		inst.Status.Phase = rdsv1alpha1.Paused
	} else if cluster.Status.Phase == v1alpha1.ClusterPhaseRebalancing {
		inst.Status.Phase = rdsv1alpha1.Rebalancing
		inst.Status.Message = cluster.Status.Message
	} else if cluster.Status.Phase == v1alpha1.ClusterPhaseFailed {
		inst.Status.Phase = rdsv1alpha1.Failed
		inst.Status.Message = cluster.Status.Message
	} else {
		logger.V(3).Info("instance is unhealthy, waiting cluster to up")
		inst.Status.Phase = rdsv1alpha1.Initializing
	}
	return nil
}

func (r *ValkeyReconciler) updateClusterInstance(ctx context.Context, inst *v1alpha1.Cluster) error {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var oldInst v1alpha1.Cluster
		if err := r.Get(ctx, client.ObjectKeyFromObject(inst), &oldInst); err != nil {
			return err
		}
		inst.ResourceVersion = oldInst.ResourceVersion
		return r.Update(ctx, inst)
	}); err != nil {
		return err
	}
	return nil
}

func (r *ValkeyReconciler) updateFailoverInstance(ctx context.Context, inst *v1alpha1.Failover) error {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var oldInst v1alpha1.Failover
		if err := r.Get(ctx, client.ObjectKeyFromObject(inst), &oldInst); err != nil {
			return err
		}
		inst.ResourceVersion = oldInst.ResourceVersion
		return r.Update(ctx, inst)
	}); err != nil {
		return err
	}
	return nil
}

func (r *ValkeyReconciler) updateInstanceStatus(ctx context.Context, inst *rdsv1alpha1.Valkey, err error, logger logr.Logger) (ctrl.Result, error) {
	logger.V(3).Info("updating inst state")

	if inst.Status.Phase == rdsv1alpha1.Failed {
		inst.Status.Phase = rdsv1alpha1.Initializing
	}
	if err != nil {
		inst.Status.Phase = rdsv1alpha1.Failed
		inst.Status.Message = err.Error()
	}

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var oldInst rdsv1alpha1.Valkey
		if err := r.Get(ctx, client.ObjectKeyFromObject(inst), &oldInst); err != nil {
			return err
		}
		if reflect.DeepEqual(oldInst.Status, inst.Status) {
			return nil
		}
		inst.ResourceVersion = oldInst.ResourceVersion
		return r.Status().Update(ctx, inst)
	}); errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else {
		return ctrl.Result{RequeueAfter: requeueSecond}, err
	}
}

func (r *ValkeyReconciler) updateInstance(ctx context.Context, inst *rdsv1alpha1.Valkey, logger logr.Logger) (ctrl.Result, error) {
	logger.V(3).Info("updating instance")
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var oldInst rdsv1alpha1.Valkey
		if err := r.Get(ctx, client.ObjectKeyFromObject(inst), &oldInst); err != nil {
			return err
		}
		inst.ResourceVersion = oldInst.ResourceVersion
		return r.Update(ctx, inst)
	}); errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else {
		return ctrl.Result{RequeueAfter: requeueSecond}, err
	}
}

func (r *ValkeyReconciler) processFinalizer(inst *rdsv1alpha1.Valkey) error {
	for _, v := range inst.GetFinalizers() {
		if v == pvcFinalizer {
			// delete pvcs
			if inst.Status.MatchLabels == nil {
				return fmt.Errorf("can't delete inst pvcs, status.matchLabels is empty")
			}
			err := r.DeleteAllOf(context.TODO(), &corev1.PersistentVolumeClaim{}, client.InNamespace(inst.Namespace),
				client.MatchingLabels(inst.Status.MatchLabels))
			if err != nil {
				return err
			}
			controllerutil.RemoveFinalizer(inst, v)
			err = r.Update(context.TODO(), inst)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ValkeyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rdsv1alpha1.Valkey{}).
		Owns(&v1alpha1.Cluster{}).
		Owns(&v1alpha1.Failover{}).
		Complete(r)
}
