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

package actor

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/actor"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/chideat/valkey-operator/internal/ops/cluster"
	cops "github.com/chideat/valkey-operator/internal/ops/cluster"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/types"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
)

var _ actor.Actor = (*actorHealPod)(nil)

func init() {
	actor.Register(core.ValkeyCluster, NewHealPodActor)
}

func NewHealPodActor(client kubernetes.ClientSet, logger logr.Logger) actor.Actor {
	return &actorHealPod{
		client: client,
		logger: logger,
	}
}

type actorHealPod struct {
	client kubernetes.ClientSet
	logger logr.Logger
}

func (a *actorHealPod) SupportedCommands() []actor.Command {
	return []actor.Command{cluster.CommandHealPod}
}

func (a *actorHealPod) Version() *semver.Version {
	return semver.MustParse("0.1.0")
}

// Do
func (a *actorHealPod) Do(ctx context.Context, val types.Instance) *actor.ActorResult {
	logger := val.Logger().WithValues("actor", cops.CommandHealPod.String())

	// clean terminating pods
	var (
		cluster = val.(types.ClusterInstance)
		now     = time.Now()
	)
	pods, err := cluster.RawNodes(ctx)
	if err != nil {
		logger.Error(err, "get pods failed")
		return actor.RequeueWithError(err)
	}

	for _, pod := range pods {
		timestamp := pod.GetDeletionTimestamp()
		if timestamp == nil {
			continue
		}
		grace := time.Second * 30
		if val := pod.GetDeletionGracePeriodSeconds(); val != nil {
			grace = time.Duration(*val) * time.Second
		}
		if now.Sub(timestamp.Time) <= grace {
			continue
		}

		objKey := client.ObjectKey{Namespace: pod.GetNamespace(), Name: pod.GetName()}
		logger.V(2).Info("for delete pod", "name", pod.GetName())
		// force delete the terminating pods
		if err := a.client.DeletePod(ctx, cluster.GetNamespace(), pod.GetName(), client.GracePeriodSeconds(0)); err != nil {
			logger.Error(err, "force delete pod failed", "target", objKey)
		} else {
			cluster.SendEventf(corev1.EventTypeWarning, config.EventCleanResource, "force delete blocked terminating pod %s", objKey.Name)

			logger.Info("force delete blocked terminating pod", "target", objKey)
			return actor.Requeue()
		}
	}

	if typ := cluster.Definition().Spec.Access.ServiceType; typ == corev1.ServiceTypeNodePort ||
		typ == corev1.ServiceTypeLoadBalancer {
		for _, node := range cluster.Nodes() {
			if !node.IsReady() {
				continue
			}
			announceIP := node.DefaultIP().String()
			announcePort := node.Port()

			svc, err := a.client.GetService(ctx, cluster.GetNamespace(), node.GetName())
			if errors.IsNotFound(err) {
				logger.Info("service not found", "name", node.GetName())
				return actor.NewResult(cops.CommandEnsureResource)
			} else if err != nil {
				logger.Error(err, "get service failed", "name", node.GetName())
				return actor.RequeueWithError(err)
			}
			switch typ {
			case corev1.ServiceTypeNodePort:
				port := util.GetServicePortByName(svc, "client")
				if port != nil {
					if int(port.NodePort) != announcePort {
						logger.V(3).Info("node port not match", "name", node.GetName(), "announcePort", announcePort, "nodePort", port.NodePort)
						if err := a.client.DeletePod(ctx, cluster.GetNamespace(), node.GetName()); err != nil {
							logger.Error(err, "delete pod failed", "name", node.GetName())
							return actor.RequeueWithError(err)
						} else {
							cluster.SendEventf(corev1.EventTypeWarning, config.EventCleanResource,
								"force delete pod with inconsist announce %s", node.GetName())
							return actor.Requeue()
						}
					}
				} else {
					logger.Error(fmt.Errorf("service port not found"), "service port not found", "name", node.GetName(), "port", "client")
				}
			case corev1.ServiceTypeLoadBalancer:
				if index := slices.IndexFunc(svc.Status.LoadBalancer.Ingress, func(ing corev1.LoadBalancerIngress) bool {
					return ing.IP == announceIP || ing.Hostname == announceIP
				}); index < 0 {
					logger.V(3).Info("lb ip not match", "name", node.GetName(), "lbip", announceIP)
					if err := a.client.DeletePod(ctx, cluster.GetNamespace(), node.GetName()); err != nil {
						logger.Error(err, "delete pod failed", "name", node.GetName())
						return actor.RequeueWithError(err)
					} else {
						cluster.SendEventf(corev1.EventTypeWarning, config.EventCleanResource,
							"force delete pod with inconsist announce %s", node.GetName())
						return actor.Requeue()
					}
				}
			}
		}
	}

	// A Pending pod on a stale StatefulSet revision may never be rolled: the RollingUpdate
	// strategy advances in reverse-ordinal order and waits for each pod to be Running+Ready
	// before rolling the next, so a wedged upper-ordinal Pending pod can stall the roll of the
	// rest. After a spec change (e.g. a resource downscale of an unschedulable instance) the
	// desired resources reach the StatefulSet template but never reach such a pod. Delete
	// Pending pods that are on a stale StatefulSet revision so the StatefulSet recreates them
	// from the updated template; it may recreate on the current (old) revision once, so this
	// converges over reconciles.
	for i := range pods {
		pod := &pods[i]
		if pod.Status.Phase != corev1.PodPending || pod.GetDeletionTimestamp() != nil {
			continue
		}
		idx := strings.LastIndex(pod.Name, "-")
		if idx <= 0 {
			continue
		}
		sts, err := a.client.GetStatefulSet(ctx, cluster.GetNamespace(), pod.Name[:idx])
		if err != nil {
			continue
		}
		if rev := sts.Status.UpdateRevision; rev != "" && pod.Labels[appsv1.StatefulSetRevisionLabel] != rev {
			if err := a.client.DeletePod(ctx, cluster.GetNamespace(), pod.Name); err != nil {
				logger.Error(err, "delete stale pending pod failed", "pod", pod.Name)
				return actor.RequeueWithError(err)
			}
			cluster.SendEventf(corev1.EventTypeWarning, config.EventCleanResource,
				"recreate stale Pending pod %s to apply updated spec", pod.Name)
			return actor.Requeue()
		}
	}

	// Always ensure resources so a pending spec change (resources/image) reaches the
	// StatefulSet template even while pods are Pending — IsResourceFullfilled is existence-only,
	// so the heal loop would otherwise never run EnsureResource. EnsureResource is idempotent.
	return actor.NewResult(cops.CommandEnsureResource)
}
