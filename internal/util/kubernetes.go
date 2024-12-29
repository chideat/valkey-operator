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

package util

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func BuildOwnerReferences(obj client.Object) (refs []metav1.OwnerReference) {
	if obj != nil {
		if obj.GetObjectKind().GroupVersionKind().Kind != "" &&
			obj.GetName() != "" && obj.GetUID() != "" {
			refs = append(refs, metav1.OwnerReference{
				APIVersion:         obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
				Kind:               obj.GetObjectKind().GroupVersionKind().Kind,
				Name:               obj.GetName(),
				UID:                obj.GetUID(),
				Controller:         ptr.To(true),
				BlockOwnerDeletion: ptr.To(true),
			})
		} else {
			refs = append(refs, obj.GetOwnerReferences()...)
		}
	}
	return
}

func ObjectKey(namespace, name string) client.ObjectKey {
	return client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}
}

func BuildOwnerReferencesWithParents(obj client.Object) (refs []metav1.OwnerReference) {
	if obj != nil {
		if obj.GetObjectKind().GroupVersionKind().Kind != "" &&
			obj.GetName() != "" && obj.GetUID() != "" {
			refs = append(refs, metav1.OwnerReference{
				APIVersion:         obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
				Kind:               obj.GetObjectKind().GroupVersionKind().Kind,
				Name:               obj.GetName(),
				UID:                obj.GetUID(),
				BlockOwnerDeletion: ptr.To(true),
				Controller:         ptr.To(true),
			})
		}
		for _, ref := range obj.GetOwnerReferences() {
			ref.Controller = nil
			refs = append(refs, ref)
		}
	}
	return
}

// GetContainerByName
func GetContainerByName(pod *corev1.PodSpec, name string) *corev1.Container {
	if pod == nil || name == "" {
		return nil
	}
	for _, c := range pod.InitContainers {
		if c.Name == name {
			return &c
		}
	}
	for _, c := range pod.Containers {
		if c.Name == name {
			return &c
		}
	}
	return nil
}

func GetServicePortByName(svc *corev1.Service, name string) *corev1.ServicePort {
	if svc == nil {
		return nil
	}
	for i := range svc.Spec.Ports {
		if svc.Spec.Ports[i].Name == name {
			return &svc.Spec.Ports[i]
		}
	}
	return nil
}

// GetVolumeClaimTemplatesByName
func GetVolumeClaimTemplatesByName(vols []corev1.PersistentVolumeClaim, name string) *corev1.PersistentVolumeClaim {
	for _, vol := range vols {
		if vol.GetName() == name {
			return &vol
		}
	}
	return nil
}

// GetVolumes
func GetVolumeByName(vols []corev1.Volume, name string) *corev1.Volume {
	for _, vol := range vols {
		if vol.Name == name {
			return &vol
		}
	}
	return nil
}

func RetryOnTimeout(f func() error, step int) error {
	return retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: time.Second,
		Factor:   1.0,
		Jitter:   0.2,
	}, func(err error) bool {
		return errors.IsTimeout(err) || errors.IsServerTimeout(err) ||
			errors.IsTooManyRequests(err) || errors.IsServiceUnavailable(err)
	}, f)
}

func ParsePodIndex(name string) (index int, err error) {
	fields := strings.Split(name, "-")
	if len(fields) < 2 {
		return -1, fmt.Errorf("invalid pod name %s", name)
	}
	if index, err = strconv.Atoi(fields[len(fields)-1]); err != nil {
		return -1, fmt.Errorf("invalid pod name %s", name)
	}
	return index, nil
}
