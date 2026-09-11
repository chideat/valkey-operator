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
		logger.Error(err, "Fail to get valkey instance")
		return reconcile.Result{}, client.IgnoreNotFound(err)
	} else if inst.GetDeletionTimestamp() != nil {
		if err := r.processFinalizer(ctx, inst); err != nil {
			logger.Error(err, "fail to process finalizer")
			return r.updateInstanceStatus(ctx, inst, err, logger)
		}
		return ctrl.Result{}, nil
	}

	var (
		err             error
		operatorVersion = config.GetOperatorVersion()
	)
	if operatorVersion != "" && operatorVersion != inst.Annotations[builder.OperatorVersionAnnotation] {
		logger.V(3).Info("instance operatorVersion is not match")
		if inst.Annotations == nil {
			inst.Annotations = make(map[string]string)
		}
		inst.Annotations[builder.OperatorVersionAnnotation] = operatorVersion
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
		if err := r.reconcileFailover(ctx, inst, logger); err != nil {
			logger.Error(err, fmt.Sprintf("fail to reconcile %s instance", inst.Spec.Arch))
			return r.updateInstanceStatus(ctx, inst, err, logger)
		}
	default:
		err = fmt.Errorf("this arch isn't valid, must be cluster, failover or replica")
		return ctrl.Result{}, err
	}
	return r.updateInstanceStatus(ctx, inst, err, logger)
}

func (r *ValkeyReconciler) reconcileFailover(ctx context.Context, inst *rdsv1alpha1.Valkey, logger logr.Logger) error {
	if inst.Spec.CustomConfigs == nil {
		inst.Spec.CustomConfigs = map[string]string{}
	}

	// Record the PVC selector before anything that can fail, so an instance that never
	// becomes ready can still have its PVCs cleaned up on delete.
	if len(inst.Status.MatchLabels) == 0 {
		inst.Status.MatchLabels = failoverbuilder.GenerateSelectorLabels(inst.Name)
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
		return nil
	} else if err != nil {
		return err
	} else if failover.GetDeletionTimestamp() != nil {
		return fmt.Errorf("redis failover %s is deleting, waiting for it to be deleted", failover.Name)
	}

	if inst.Spec.PodAnnotations == nil {
		inst.Spec.PodAnnotations = make(map[string]string)
	}
	for key := range vkHandler.GetValkeyConfigsApplyPolicyByVersion(inst.Spec.Version) {
		if inst.Spec.CustomConfigs[key] != failover.Spec.CustomConfigs[key] {
			inst.Spec.PodAnnotations[builder.RestartAnnotationKey] = time.Now().Format(time.RFC3339Nano)
			break
		}
	}
	inst.Spec.PodAnnotations = builder.MergeRestartAnnotation(inst.Spec.PodAnnotations, failover.Spec.PodAnnotations)

	newFailover, err := vkHandler.GenerateFailover(inst)
	if err != nil {
		logger.Error(err, "fail to generate failover instance")
		return err
	}
	// ensure inst should update
	if vkHandler.ShouldUpdateFailover(failover, newFailover, logger) {
		newFailover.ResourceVersion = failover.ResourceVersion
		newFailover.Status = failover.Status
		if err := r.updateFailoverInstance(ctx, newFailover, logger); err != nil {
			inst.Status.Phase = rdsv1alpha1.Failed
			inst.Status.Message = err.Error()
			logger.Error(err, "fail to update failover inst")
			return err
		}
		failover = newFailover
	}

	inst.Status.LastShardCount = 1
	inst.Status.LastVersion = inst.Spec.Version
	inst.Status.Nodes = failover.Status.Nodes
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
	// Record the PVC selector before anything that can fail, so an instance that never
	// becomes ready can still have its PVCs cleaned up on delete.
	if len(inst.Status.MatchLabels) == 0 {
		inst.Status.MatchLabels = clusterbuilder.GenerateClusterLabels(inst.Name, nil)
	}

	if err := r.Get(ctx, types.NamespacedName{
		Name:      inst.Name,
		Namespace: inst.Namespace,
	}, cluster); errors.IsNotFound(err) {
		cluster, err = vkHandler.GenerateValkeyCluster(inst)
		if err != nil {
			return err
		}
		// Record actor versions too keep actions consistent
		cluster.Annotations[builder.CRVersionKey] = config.GetOperatorVersion()

		if err := r.Create(ctx, cluster); err != nil {
			logger.Error(err, "fail to create cluster instance")
			inst.Status.Phase = rdsv1alpha1.Failed
			inst.Status.Message = err.Error()
			return err
		}
		return nil
	} else if err != nil {
		return err
	} else if cluster.GetDeletionTimestamp() != nil {
		// wait old resource deleted
		logger.V(3).Info("redis cluster is deleting, waiting for it to be deleted")
		return fmt.Errorf("redis cluster %s is deleting, waiting for it to be deleted", cluster.Name)
	}

	for key := range vkHandler.GetValkeyConfigsApplyPolicyByVersion(inst.Spec.Version) {
		if inst.Spec.CustomConfigs[key] != cluster.Spec.CustomConfigs[key] {
			inst.Spec.PodAnnotations[builder.RestartAnnotationKey] = time.Now().Format(time.RFC3339Nano)
			break
		}
	}
	inst.Spec.PodAnnotations = builder.MergeRestartAnnotation(inst.Spec.PodAnnotations, cluster.Spec.PodAnnotations)

	newCluster, err := vkHandler.GenerateValkeyCluster(inst)
	if err != nil {
		return err
	}

	// ensure inst should update
	if vkHandler.ShouldUpdateCluster(cluster, newCluster, logger) {
		newCluster.ResourceVersion = cluster.ResourceVersion
		newCluster.Status = cluster.Status
		if err := r.updateClusterInstance(ctx, newCluster, logger); err != nil {
			logger.Error(err, "fail to update cluster instance")
			inst.Status.Phase = rdsv1alpha1.Failed
			inst.Status.Message = err.Error()
			return err
		}
		cluster = newCluster
	}

	inst.Status.LastShardCount = cluster.Spec.Replicas.Shards
	inst.Status.LastVersion = inst.Spec.Version
	inst.Status.Nodes = cluster.Status.Nodes
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

func (r *ValkeyReconciler) updateClusterInstance(ctx context.Context, inst *v1alpha1.Cluster, logger logr.Logger) error {
	logger.V(3).Info("updating cluster instance")
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

func (r *ValkeyReconciler) updateFailoverInstance(ctx context.Context, inst *v1alpha1.Failover, logger logr.Logger) error {
	logger.V(3).Info("updating failover instance")
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
	logger.V(3).Info("updating instance state")

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

// pvcSelectors returns the label selectors that identify the instance's PVCs.
//
// status.matchLabels is the recorded selector, but it is only persisted once the instance
// reconciled far enough to reach reconcileCluster/reconcileFailover. An instance that failed
// earlier — an unresolvable image version, a rejected child CR, an invalid arch — is deleted
// with an empty status, and the selector must still be derivable, or the finalizer blocks
// deletion forever.
//
// Deriving is always possible: the child CR carries the instance's own name, so both
// selectors are pure functions of it, and they are exactly the labels the builders stamp
// onto the PVCs through the StatefulSet's volumeClaimTemplates.
func pvcSelectors(inst *rdsv1alpha1.Valkey) []map[string]string {
	if len(inst.Status.MatchLabels) > 0 {
		return []map[string]string{inst.Status.MatchLabels}
	}

	clusterLabels := clusterbuilder.GenerateClusterLabels(inst.Name, nil)
	failoverLabels := failoverbuilder.GenerateSelectorLabels(inst.Name)
	switch inst.Spec.Arch {
	case core.ValkeyCluster:
		return []map[string]string{clusterLabels}
	case core.ValkeyFailover, core.ValkeyReplica:
		return []map[string]string{failoverLabels}
	default:
		// The arch was never defaulted or is invalid, so the topology is unknown. Try both:
		// each selector carries InstanceNameLabelKey, which scopes it to this instance, so
		// neither can match another instance's PVCs.
		return []map[string]string{clusterLabels, failoverLabels}
	}
}

// processFinalizer reclaims the instance's PVCs and then removes the finalizer that asked
// for it. The data PVCs carry no ownerReference, so this is the only thing that takes the
// storage with the instance — and a failure here leaves the CR in Terminating for good,
// recoverable only by hand-patching finalizers.
func (r *ValkeyReconciler) processFinalizer(ctx context.Context, inst *rdsv1alpha1.Valkey) error {
	for _, v := range inst.GetFinalizers() {
		if v == pvcFinalizer {
			for _, selector := range pvcSelectors(inst) {
				if err := r.DeleteAllOf(ctx, &corev1.PersistentVolumeClaim{}, client.InNamespace(inst.Namespace),
					client.MatchingLabels(selector)); err != nil {
					return err
				}
			}
			controllerutil.RemoveFinalizer(inst, v)
			if err := r.Update(ctx, inst); err != nil {
				if errors.IsNotFound(err) {
					continue
				}
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
