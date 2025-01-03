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
package clientset

import (
	"context"
	"reflect"

	v1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Cluster interface {
	GetCluster(ctx context.Context, namespace, name string) (*v1alpha1.Cluster, error)
	// UpdateRedisFailover update the redisfailover on a cluster.
	UpdateCluster(ctx context.Context, inst *v1alpha1.Cluster) error
	// UpdateRedisFailoverStatus
	UpdateClusterStatus(ctx context.Context, inst *v1alpha1.Cluster) error
}

type ClusterOption struct {
	client client.Client
	logger logr.Logger
}

func NewCluster(kubeClient client.Client, logger logr.Logger) Cluster {
	logger = logger.WithName("Cluster")
	return &ClusterOption{
		client: kubeClient,
		logger: logger,
	}
}

func (r *ClusterOption) GetCluster(ctx context.Context, namespace, name string) (*v1alpha1.Cluster, error) {
	ret := v1alpha1.Cluster{}
	err := r.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &ret)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

// UpdateRedisFailover
func (r *ClusterOption) UpdateCluster(ctx context.Context, inst *v1alpha1.Cluster) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var oldInst v1alpha1.Cluster
		if err := r.client.Get(ctx, client.ObjectKeyFromObject(inst), &oldInst); err != nil {
			r.logger.Error(err, "get Cluster failed")
			return err
		}
		inst.ResourceVersion = oldInst.ResourceVersion
		return r.client.Update(ctx, inst)
	})
}

func (r *ClusterOption) UpdateClusterStatus(ctx context.Context, inst *v1alpha1.Cluster) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var oldInst v1alpha1.Cluster
		if err := r.client.Get(ctx, client.ObjectKeyFromObject(inst), &oldInst); err != nil {
			r.logger.Error(err, "get Cluster failed")
			return err
		}

		if !reflect.DeepEqual(oldInst.Status, inst.Status) {
			inst.ResourceVersion = oldInst.ResourceVersion
			return r.client.Status().Update(ctx, inst)
		}
		return nil
	})
}
