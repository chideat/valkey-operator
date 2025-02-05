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
*/package certbuilder

import (
	"fmt"
	"time"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GenerateCertName
func GenerateCertName(name string) string {
	return name + "-cert"
}

func GenerateSSLSecretName(name string) string {
	return fmt.Sprintf("%s-tls", name)
}

// GenerateServiceDNSName
func GenerateServiceDNSName(serviceName, namespace string) string {
	return fmt.Sprintf("%s.%s", serviceName, namespace)
}

// GenerateServiceDNSName
func GenerateHeadlessDNSName(podName, serviceName, namespace string) string {
	return fmt.Sprintf("%s.%s.%s", podName, serviceName, namespace)
}

func GenerateValkeyTLSOptions() string {
	return "--tls --cert /tls/tls.crt --key /tls/tls.key --cacert /tls/ca.crt"
}

// NewCertificate
func NewCertificate(inst types.Instance, dns []string, labels map[string]string) (*certv1.Certificate, error) {
	issuer := inst.Issuer()
	if issuer == nil {
		return nil, fmt.Errorf("issuer is nil")
	}
	return &certv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:            GenerateCertName(inst.GetName()),
			Namespace:       inst.GetNamespace(),
			Labels:          labels,
			OwnerReferences: util.BuildOwnerReferences(inst),
		},
		Spec: certv1.CertificateSpec{
			// Duration: 10 year
			Duration:   &metav1.Duration{Duration: 87600 * time.Hour},
			DNSNames:   dns,
			IssuerRef:  *issuer,
			SecretName: GenerateSSLSecretName(inst.GetName()),
		},
	}, nil
}
