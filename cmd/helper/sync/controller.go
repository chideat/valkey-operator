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

package sync

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

type PersistentObject struct {
	data map[string][]byte
}

func LoadPersistentObject(ctx context.Context, client *kubernetes.Clientset, kind, namespace, name string) (*PersistentObject, error) {
	switch strings.ToLower(kind) {
	case "secret":
		secret, err := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return &PersistentObject{data: map[string][]byte{}}, nil
		} else if err != nil {
			return nil, err
		}
		return &PersistentObject{data: secret.Data}, nil
	default:
		cm, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return &PersistentObject{data: map[string][]byte{}}, nil
		} else if err != nil {
			return nil, err
		}
		obj := &PersistentObject{data: map[string][]byte{}}
		for k, v := range cm.Data {
			obj.data[k] = []byte(v)
		}
		return obj, nil
	}
}

func NewObject() *PersistentObject {
	return &PersistentObject{data: map[string][]byte{}}
}

func (obj *PersistentObject) Get(key string) []byte {
	if obj == nil {
		return nil
	}
	return obj.data[key]
}

func (obj *PersistentObject) Set(key string, val []byte) {
	if obj == nil {
		return
	}
	if obj.data == nil {
		obj.data = map[string][]byte{}
	}
	obj.data[key] = val
}

func (obj *PersistentObject) Save(ctx context.Context, client *kubernetes.Clientset, kind, namespace, name string,
	owners []metav1.OwnerReference, logger logr.Logger) error {

	switch strings.ToLower(kind) {
	case "secret":
		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       namespace,
				Name:            name,
				OwnerReferences: owners,
			},
			Data: obj.data,
		}

		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			oldSecret, err := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				if _, err := client.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
					logger.Error(err, "create secret failed")
					return err
				}
			} else if err != nil {
				logger.Error(err, "get secret failed", "target", name)
				return err
			}
			secret.ResourceVersion = oldSecret.ResourceVersion
			if _, err := client.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
				logger.Error(err, "update secret failed", "target", name)
				return err
			}
			return nil
		})
	default:
		cm := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       namespace,
				Name:            name,
				OwnerReferences: owners,
			},
			Data: map[string]string{},
		}
		for k, v := range obj.data {
			cm.Data[k] = string(v)
		}

		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			oldCm, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				if _, err := client.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{}); err != nil {
					logger.Error(err, "create configmap failed")
					return err
				}
			} else if err != nil {
				logger.Error(err, "get configmap failed", "target", name)
				return err
			}
			cm.ResourceVersion = oldCm.ResourceVersion
			if _, err := client.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
				logger.Error(err, "update configmap failed", "target", name)
				return err
			}
			return nil
		})
	}
}

type UpdateEvent struct {
	FilePath  string
	Data      string
	Timestamp int64
}

type Filter interface {
	Truncate(filename string, data string) (string, error)
}

type ControllerOptions struct {
	ResourceKind    string
	Namespace       string
	Name            string
	OwnerReferences []metav1.OwnerReference
	SyncInterval    time.Duration
	Filters         []Filter
}

// Controller
type Controller struct {
	client *kubernetes.Clientset

	dataUpdateChan chan *UpdateEvent
	filters        []Filter
	options        ControllerOptions
	logger         logr.Logger
}

func NewController(client *kubernetes.Clientset, options ControllerOptions, logger logr.Logger) (*Controller, error) {
	logger = logger.WithName("Controller")

	if options.SyncInterval <= time.Second*3 {
		options.SyncInterval = time.Second * 10
	}
	if options.Namespace == "" {
		return nil, fmt.Errorf("namespace required")
	}
	if options.Name == "" {
		return nil, fmt.Errorf("configmap name required")
	}
	if len(options.OwnerReferences) == 0 {
		logger.Info("WARNING: resource owner reference not specified")
	}

	ctrl := Controller{
		client:         client,
		dataUpdateChan: make(chan *UpdateEvent, 100),
		options:        options,
		filters:        options.Filters,
		logger:         logger,
	}
	return &ctrl, nil
}

func (c *Controller) Handler(ctx context.Context, fs *FileStat) error {
	logger := c.logger.WithName("Handler")

	data, err := os.ReadFile(fs.FilePath())
	if err != nil {
		logger.Error(err, "load file content failed", "filepath", fs.FilePath())
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.dataUpdateChan <- &UpdateEvent{FilePath: fs.FilePath(), Data: string(data)}:
	}
	return nil
}

func (c *Controller) Run(ctx context.Context) error {
	logger := c.logger

	obj, err := LoadPersistentObject(ctx, c.client, c.options.ResourceKind, c.options.Namespace, c.options.Name)
	if err != nil {
		logger.Error(err, "load object failed", "kind", c.options.ResourceKind, "namespace", c.options.Namespace, "name", c.options.Name)
		return err
	}

	ticker := time.NewTicker(c.options.SyncInterval)
	defer ticker.Stop()

	isChanged := int64(0)
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-c.dataUpdateChan:
			if !ok {
				return nil
			}

			oldData := obj.Get(event.FilePath)
			oldSize := len(oldData)

			// etcd limiteds configmap/secret size limit to 1Mi
			if oldSize-len(oldData)+len(event.Data) >= 1024*1024*1024-4096 {
				logger.Error(fmt.Errorf("data size has exceed 1Mi"), "filepath", event.FilePath)
				continue
			}

			if event.Data == string(oldData) {
				continue
			}

			filename := path.Base(event.FilePath)
			if func() bool {
				for _, filter := range c.filters {
					if data, err := filter.Truncate(filename, event.Data); err != nil {
						return false
					} else {
						event.Data = data
					}
				}
				return true
			}() {
				obj.Set(filename, []byte(event.Data))
				isChanged += 1
			}
		case <-ticker.C:
			if isChanged == 0 {
				continue
			}

			func() {
				ctx, cancel := context.WithTimeout(ctx, c.options.SyncInterval-time.Second)
				defer cancel()

				if err := obj.Save(ctx, c.client, c.options.ResourceKind,
					c.options.Namespace, c.options.Name, c.options.OwnerReferences, logger); err != nil {
					logger.Error(err, "update configmap failed")
				}
			}()
			isChanged = 0
		}
	}
}
