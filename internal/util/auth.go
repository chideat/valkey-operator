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
	"os"

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
	if !caCertPool.AppendCertsFromPEM(secret.Data["ca.crt"]) {
		return nil, fmt.Errorf("tls secret carries no usable CA certificate")
	}

	// The operator dials nodes by pod IP while the issued certificates carry DNS
	// SANs only, so the default hostname check can never match and this used to
	// be InsecureSkipVerify. That made RootCAs dead weight: any certificate at
	// all was accepted, including one a MITM on the pod network could present.
	//
	// The nodes serve exactly the certificate held in this secret, so verifying
	// against a name it already carries keeps the check satisfiable while the
	// chain is still validated against the instance CA. Name verification adds
	// nothing here -- every node of an instance shares one certificate -- the
	// security comes from the chain check this restores.
	serverName, err := certificateServerName(cert)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		ServerName:   serverName,
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{cert},
	}, nil
}

// certificateServerName returns a name the certificate actually attests to, for
// use as tls.Config.ServerName.
func certificateServerName(cert tls.Certificate) (string, error) {
	leaf := cert.Leaf
	if leaf == nil {
		if len(cert.Certificate) == 0 {
			return "", errors.New("tls certificate carries no leaf")
		}
		var err error
		if leaf, err = x509.ParseCertificate(cert.Certificate[0]); err != nil {
			return "", err
		}
	}
	if len(leaf.DNSNames) > 0 {
		return leaf.DNSNames[0], nil
	}
	if leaf.Subject.CommonName != "" {
		return leaf.Subject.CommonName, nil
	}
	return "", errors.New("tls certificate carries no name to verify against")
}

// LoadCertConfigFromFiles builds a TLS client config from certificate files on
// disk, as mounted into the instance pods.
func LoadCertConfigFromFiles(certFile, keyFile, caCertFile string) (*tls.Config, error) {
	if certFile == "" || keyFile == "" {
		return nil, errors.New("tls certificate and key paths are required")
	}
	if caCertFile == "" {
		return nil, errors.New("tls ca certificate path is required")
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	caCert, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("%s carries no usable CA certificate", caCertFile)
	}
	serverName, err := certificateServerName(cert)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		ServerName:   serverName,
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{cert},
	}, nil
}
