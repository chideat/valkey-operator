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
package failoverbuilder

import (
	"time"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	v12 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/sentinelbuilder"
	"github.com/chideat/valkey-operator/internal/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewCertificate
func NewCertificate(rf *v1alpha1.Failover, selectors map[string]string) *certv1.Certificate {
	return &certv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:            builder.GenerateCertName(rf.Name),
			Namespace:       rf.Namespace,
			Labels:          GetCommonLabels(rf.Name, selectors),
			OwnerReferences: util.BuildOwnerReferences(rf),
		},
		Spec: certv1.CertificateSpec{
			// 10 year
			Duration: &metav1.Duration{Duration: 87600 * time.Hour},
			DNSNames: []string{
				builder.GetServiceDNSName(GetValkeyROServiceName(rf.Name), rf.Namespace),
				builder.GetServiceDNSName(GetValkeyRWServiceName(rf.Name), rf.Namespace),
				builder.GetServiceDNSName(sentinelbuilder.GetSentinelStatefulSetName(rf.Name), rf.Namespace),
			},
			IssuerRef:  v12.ObjectReference{Kind: certv1.ClusterIssuerKind, Name: "cpaas-ca"},
			SecretName: builder.GetValkeySSLSecretName(rf.Name),
		},
	}
}
