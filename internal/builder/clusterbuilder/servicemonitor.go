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
package clusterbuilder

import (
	v1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	smv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultScrapInterval = "60s"
	DefaultScrapeTimeout = "10s"
)

const (
	ValkeyClusterServiceMonitorName = "valkey-cluster"
)

func NewServiceMonitorForCR(cluster *v1alpha1.Cluster) *smv1.ServiceMonitor {
	labels := map[string]string{
		builder.ManagedByLabel: "valkey-operator",
	}

	interval := DefaultScrapInterval
	scrapeTimeout := DefaultScrapeTimeout

	sm := &smv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name: ValkeyClusterServiceMonitorName,
			Labels: map[string]string{
				"prometheus": "kube-prometheus",
			},
		},
		Spec: smv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			NamespaceSelector: smv1.NamespaceSelector{
				Any: true,
			},
			Endpoints: []smv1.Endpoint{
				{
					HonorLabels:   true,
					Port:          "metrics",
					Path:          "/metrics",
					Interval:      smv1.Duration(interval),
					ScrapeTimeout: smv1.Duration(scrapeTimeout),
				},
			},
			TargetLabels: []string{"arch"},
		},
	}
	return sm
}
