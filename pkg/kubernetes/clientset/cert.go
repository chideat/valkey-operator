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

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Certificate interface {
	GetCertificate(ctx context.Context, namespace string, name string) (*certv1.Certificate, error)
	CreateCertificate(ctx context.Context, namespace string, cert *certv1.Certificate) error
	CreateIfNotExistsCertificate(ctx context.Context, namespace string, cert *certv1.Certificate) error
}

type CertOption struct {
	client client.Client
	logger logr.Logger
}

func NewCert(kubeClient client.Client, logger logr.Logger) Certificate {
	logger = logger.WithValues("service", "k8s.configMap")
	return &CertOption{
		client: kubeClient,
		logger: logger,
	}
}

func (c *CertOption) GetCertificate(ctx context.Context, namespace string, name string) (*certv1.Certificate, error) {
	cert := &certv1.Certificate{}
	if err := c.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, cert); err != nil {
		return nil, err
	}
	return cert, nil
}

func (c *CertOption) CreateCertificate(ctx context.Context, namespace string, cert *certv1.Certificate) error {
	err := c.client.Create(ctx, cert)
	if err != nil {
		return err
	}
	c.logger.WithValues("namespace", namespace, "cert", cert.Name).V(3).Info("cert created")
	return err

}

func (c *CertOption) CreateIfNotExistsCertificate(ctx context.Context, namespace string, cert *certv1.Certificate) error {
	err := c.client.Get(ctx, types.NamespacedName{Name: cert.Name, Namespace: cert.Namespace}, cert)
	if errors.IsNotFound(err) {
		err = c.CreateCertificate(ctx, namespace, cert)
		return err
	} else if err != nil {
		return err
	}
	return nil
}
