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
*/package clientset

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// Pod the client that knows how to interact with kubernetes to manage them
type Pod interface {
	// GetPod get pod from kubernetes with namespace and name
	GetPod(ctx context.Context, namespace string, name string) (*corev1.Pod, error)
	// CreatePod will create the given pod
	CreatePod(ctx context.Context, namespace string, pod *corev1.Pod) error
	// UpdatePod will update the given pod
	UpdatePod(ctx context.Context, namespace string, pod *corev1.Pod) error
	// CreateOrUpdatePod will update the given pod or create it if does not exist
	CreateOrUpdatePod(ctx context.Context, namespace string, pod *corev1.Pod) error
	// DeletePod will delete the given pod
	DeletePod(ctx context.Context, namespace string, name string, opts ...client.DeleteOption) error
	// ListPodByLabels
	ListPodByLabels(ctx context.Context, namespace string, label_map map[string]string) (*corev1.PodList, error)
	// ListPods get set of pod on a given namespace
	ListPods(ctx context.Context, namespace string) (*corev1.PodList, error)
	// PatchPodLabel
	PatchPodLabel(ctx context.Context, pod *corev1.Pod, labelkey string, labelValue string) error
	// Exec exec command remotely
	Exec(ctx context.Context, namespace, name, containerName string, cmd []string) (io.Reader, io.Reader, error)
}

// PodOption is the pod client interface implementation using API calls to kubernetes.
type PodOption struct {
	restClient rest.Interface
	restConfig *rest.Config
	client     client.Client
	logger     logr.Logger
}

// NewPod returns a new Pod client.
func NewPod(kubeClient client.Client, restConfig *rest.Config, logger logr.Logger) Pod {
	logger = logger.WithValues("service", "k8s.pod")

	podOpt := &PodOption{
		restConfig: restConfig,
		client:     kubeClient,
		logger:     logger,
	}

	if restConfig != nil {
		restClient, err := apiutil.RESTClientForGVK(corev1.SchemeGroupVersion.WithKind("Pod"), false, restConfig, scheme.Codecs, http.DefaultClient)
		if err != nil {
			panic(err)
		}
		podOpt.restClient = restClient
	}
	return podOpt
}

// GetPod implement the Pod.Interface
func (p *PodOption) GetPod(ctx context.Context, namespace string, name string) (*corev1.Pod, error) {
	pod := &corev1.Pod{}
	err := p.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, pod)
	if err != nil {
		return nil, err
	}
	return pod, err
}

// CreatePod implement the Pod.Interface
func (p *PodOption) CreatePod(ctx context.Context, namespace string, pod *corev1.Pod) error {
	err := p.client.Create(ctx, pod)
	if err != nil {
		return err
	}

	p.logger.WithValues("namespace", namespace, "pod", pod.Name).V(3).Info("pod created")
	return nil
}

// UpdatePod only overwrite labels/annotations of the pod
func (p *PodOption) UpdatePod(ctx context.Context, namespace string, pod *corev1.Pod) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		oldPod, err := p.GetPod(ctx, namespace, pod.Name)
		if errors.IsNotFound(err) {
			return nil
		} else if err != nil {
			return err
		}
		oldPod.Labels = lo.Assign(oldPod.Labels, pod.Labels)
		oldPod.Annotations = lo.Assign(oldPod.Annotations, pod.Annotations)

		return p.client.Update(ctx, pod)
	})
}

// CreateOrUpdatePod implement the Pod.Interface
func (p *PodOption) CreateOrUpdatePod(ctx context.Context, namespace string, pod *corev1.Pod) error {
	storedPod, err := p.GetPod(ctx, namespace, pod.Name)
	if err != nil {
		// If no resource we need to create.
		if errors.IsNotFound(err) {
			return p.CreatePod(ctx, namespace, pod)
		}
		return err
	}

	// Already exists, need to Update.
	// Set the correct resource version to ensure we are on the latest version. This way the only valid
	// namespace is our spec(https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#concurrency-control-and-consistency),
	// we will replace the current namespace state.
	pod.ResourceVersion = storedPod.ResourceVersion
	return p.UpdatePod(ctx, namespace, pod)
}

// DeletePod implement the Pod.Interface
func (p *PodOption) DeletePod(ctx context.Context, namespace string, name string, opts ...client.DeleteOption) error {
	pod := &corev1.Pod{}
	if err := p.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, pod); err != nil {
		return err
	}
	return p.client.Delete(ctx, pod, opts...)
}

// ListPods implement the Pod.Interface
func (p *PodOption) ListPods(ctx context.Context, namespace string) (*corev1.PodList, error) {
	ps := &corev1.PodList{}
	listOps := &client.ListOptions{
		Namespace: namespace,
	}
	err := p.client.List(ctx, ps, listOps)
	return ps, err
}

func (p *PodOption) PatchPodLabel(ctx context.Context, pod *corev1.Pod, labelkey string, labelValue string) error {
	labelPatch := fmt.Sprintf(`[{"op":"add","path":"/metadata/labels/%s","value":"%s" }]`, strings.Replace(labelkey, "/", "~1", -1), labelValue)

	return p.client.Patch(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: pod.Namespace,
			Name:      pod.Name,
		},
	}, client.RawPatch(types.JSONPatchType, []byte(labelPatch)), &client.PatchOptions{})
}

func (p *PodOption) ListPodByLabels(ctx context.Context, namespace string, label_map map[string]string) (*corev1.PodList, error) {
	listOps := &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labels.SelectorFromSet(label_map),
	}
	podlist := &corev1.PodList{}
	err := p.client.List(ctx, podlist, listOps)
	return podlist, err
}

// Exec
func (p *PodOption) Exec(ctx context.Context, namespace, name, containerName string, cmd []string) (io.Reader, io.Reader, error) {
	if p.restClient == nil {
		return nil, nil, fmt.Errorf("restconfig not set")
	}

	req := p.restClient.Post().Resource("pods").
		Namespace(namespace).Name(name).SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   cmd,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	var stdout, stderr bytes.Buffer
	if exec, err := remotecommand.NewSPDYExecutor(p.restConfig, "POST", req.URL()); err != nil {
		return nil, nil, err
	} else if err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: &stdout, Stderr: &stderr, Tty: true}); err != nil {
		return nil, nil, err
	}
	return &stdout, &stderr, nil
}
