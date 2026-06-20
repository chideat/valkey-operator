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

package util

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func LoadCertConfigFromSecret(secret *corev1.Secret) (*tls.Config, error) {
	if secret == nil {
		return nil, errors.New("tls secret is nil")
	}

	if secret.Data[corev1.TLSCertKey] == nil || secret.Data[corev1.TLSPrivateKeyKey] == nil ||
		secret.Data["ca.crt"] == nil {
		return nil, fmt.Errorf("tls secret is invalid")
	}
	cert, err := tls.X509KeyPair(secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey])
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(secret.Data["ca.crt"])

	return &tls.Config{
		InsecureSkipVerify: true, // #nosec
		RootCAs:            caCertPool,
		Certificates:       []tls.Certificate{cert},
	}, nil
}
