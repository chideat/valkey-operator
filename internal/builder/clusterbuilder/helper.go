/*
Copyright 2023 The RedisOperator Authors.

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

package clusterbuilder

import (
	"context"
	"fmt"

	v1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	PasswordENV = "REDIS_PASSWORD"
	passwordKey = "password" //NOSONAR
)

// getPublicLabels
// NOTE: this labels are const, take care of edit this
func getPublicLabels(name string) map[string]string {
	return map[string]string{
		"redis.kun/name":           name,
		"middleware.instance/type": "distributed-redis-cluster",
		builder.InstanceNameLabel:  name,
		builder.ManagedByLabel:     "redis-cluster-operator",
	}
}

// GetClusterStatefulsetSelectorLabels
// this labels are const, take care of update this
func GetClusterStatefulsetSelectorLabels(name string, index int) map[string]string {
	labels := getPublicLabels(name)
	if index >= 0 {
		labels["statefulSet"] = ClusterStatefulSetName(name, index)
	}
	return labels
}

// GetClusterStatefulsetLabels
// this labels are const, take care of update this
func GetClusterStatefulsetLabels(name string, index int) map[string]string {
	labels := getPublicLabels(name)
	labels["statefulSet"] = ClusterStatefulSetName(name, index)
	return labels
}

// GetClusterStaticLabels
// this labels are const, take care of modify this.
func GetClusterLabels(name string, extra map[string]string) map[string]string {
	labels := getPublicLabels(name)
	for k, v := range extra {
		labels[k] = v
	}
	return labels
}

// IsPasswordChanged determine whether the password is changed.
func IsPasswordChanged(cluster *v1alpha1.Cluster, sts *appsv1.StatefulSet) bool {
	envSet := sts.Spec.Template.Spec.Containers[0].Env
	secretName := getSecretKeyRefByKey(PasswordENV, envSet)

	return cluster.Spec.Access.DefaultPasswordSecret != secretName
}

func getSecretKeyRefByKey(key string, envSet []corev1.EnvVar) string {
	for _, value := range envSet {
		if key == value.Name {
			if value.ValueFrom != nil && value.ValueFrom.SecretKeyRef != nil {
				return value.ValueFrom.SecretKeyRef.Name
			}
		}
	}
	return ""
}

// GetOldRedisClusterPassword return old redis cluster's password.
func GetOldRedisClusterPassword(client client.Client, sts *appsv1.StatefulSet) (string, error) {
	envSet := sts.Spec.Template.Spec.Containers[0].Env
	secretName := getSecretKeyRefByKey(PasswordENV, envSet)
	if secretName == "" {
		return "", nil
	}
	secret := &corev1.Secret{}
	err := client.Get(context.TODO(), types.NamespacedName{
		Name:      secretName,
		Namespace: sts.Namespace,
	}, secret)
	if err != nil {
		return "", err
	}
	return string(secret.Data[passwordKey]), nil
}

// GetClusterPassword return current redis cluster's password.
func GetClusterPassword(client client.Client, cluster *v1alpha1.Cluster) (string, error) {
	if cluster.Spec.Access.DefaultPasswordSecret == "" {
		return "", nil
	}
	secret := &corev1.Secret{}
	err := client.Get(context.TODO(), types.NamespacedName{
		Name:      cluster.Spec.Access.DefaultPasswordSecret,
		Namespace: cluster.Namespace,
	}, secret)
	if err != nil {
		return "", err
	}
	return string(secret.Data[passwordKey]), nil
}

func GenerateClusterTLSSecretName(name string) string {
	return fmt.Sprintf("%s-tls", name)
}
