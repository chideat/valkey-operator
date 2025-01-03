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
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/types"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewFailoverSentinel(inst types.FailoverInstance) *v1alpha1.Sentinel {
	def := inst.Definition()

	sen := &v1alpha1.Sentinel{
		ObjectMeta: metav1.ObjectMeta{
			Name:            def.Name,
			Namespace:       def.Namespace,
			Labels:          def.Labels,
			Annotations:     def.Annotations,
			OwnerReferences: util.BuildOwnerReferences(def),
		},
		Spec: def.Spec.Sentinel.SentinelSpec,
	}

	sen.Spec.Image = lo.FirstOrEmpty([]string{def.Spec.Sentinel.Image, def.Spec.Image})
	sen.Spec.ImagePullPolicy = builder.GetPullPolicy(def.Spec.Sentinel.ImagePullPolicy, def.Spec.ImagePullPolicy)
	sen.Spec.ImagePullSecrets = lo.FirstOrEmpty([][]corev1.LocalObjectReference{
		def.Spec.Sentinel.ImagePullSecrets,
		def.Spec.ImagePullSecrets,
	})
	sen.Spec.Replicas = def.Spec.Sentinel.Replicas
	sen.Spec.Resources = def.Spec.Sentinel.Resources
	sen.Spec.CustomConfigs = def.Spec.Sentinel.CustomConfigs
	sen.Spec.Exporter = def.Spec.Sentinel.Exporter
	if sen.Spec.Access.EnableTLS {
		sen.Spec.Access.ExternalTLSSecret = builder.GetRedisSSLSecretName(inst.GetName())
	}
	return sen
}
