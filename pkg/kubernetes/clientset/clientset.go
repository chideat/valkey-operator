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
	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Service is the kubernetes service entrypoint.
type ClientSet interface {
	Certificate
	ConfigMap
	CronJob
	Deployment
	Job
	Namespaces
	Pod
	PodDisruptionBudget
	PVC
	RBAC
	Secret
	Service
	ServiceAccount
	StatefulSet
	ServiceMonitor

	Failover
	Sentinel
	Cluster
	Node
	RedisUser
	Client() client.Client
}

type clientSet struct {
	Certificate
	ConfigMap
	CronJob
	Deployment
	Job
	Namespaces
	Pod
	PodDisruptionBudget
	PVC
	RBAC
	Secret
	Service
	ServiceAccount
	StatefulSet
	ServiceMonitor

	Failover
	Sentinel
	Cluster
	Node
	RedisUser
	rawClient client.Client
}

func (cs *clientSet) Client() client.Client {
	return cs.rawClient
}

// New returns a new Kubernetes client set.
func New(kubecli client.Client, logger logr.Logger) *clientSet {
	return NewWithConfig(kubecli, nil, logger)
}

// NewWithConfig returns a new Kubernetes client set.
func NewWithConfig(kubecli client.Client, restConfig *rest.Config, logger logr.Logger) *clientSet {
	return &clientSet{
		rawClient: kubecli,

		Certificate:         NewCert(kubecli, logger),
		ConfigMap:           NewConfigMap(kubecli, logger),
		CronJob:             NewCronJob(kubecli, logger),
		Deployment:          NewDeployment(kubecli, logger),
		Job:                 NewJob(kubecli, logger),
		Namespaces:          NewNamespaces(kubecli, logger),
		Pod:                 NewPod(kubecli, restConfig, logger),
		PodDisruptionBudget: NewPodDisruptionBudget(kubecli, logger),
		PVC:                 NewPVCService(kubecli, logger),
		RBAC:                NewRBAC(kubecli, logger),
		Secret:              NewSecret(kubecli, logger),
		Service:             NewService(kubecli, logger),
		ServiceAccount:      NewServiceAccount(kubecli, logger),
		StatefulSet:         NewStatefulSet(kubecli, logger),

		Failover:       NewFailoverService(kubecli, logger),
		Sentinel:       NewSentinelService(kubecli, logger),
		Cluster:        NewCluster(kubecli, logger),
		Node:           NewNode(kubecli, logger),
		ServiceMonitor: NewServiceMonitor(kubecli, logger),
		RedisUser:      NewRedisUserService(kubecli, logger),
	}
}
