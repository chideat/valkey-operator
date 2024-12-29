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

package builder

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func BuildMetricsRegex(regex []string) string {
	uniqueArr := func(m []string) []string {
		d := make([]string, 0)
		result := make(map[string]bool, len(m))
		for _, v := range m {
			if !result[v] {
				result[v] = true
				d = append(d, v)
			}
		}
		return d
	}
	return fmt.Sprintf("(%s)", strings.Join(uniqueArr(regex), "|"))
}

func GetPullPolicy(policies ...corev1.PullPolicy) corev1.PullPolicy {
	for _, policy := range policies {
		if policy != "" {
			return policy
		}
	}
	return corev1.PullIfNotPresent
}

func GenerateRedisTLSOptions() string {
	return "--tls --cert /tls/tls.crt --key /tls/tls.key --cacert /tls/ca.crt"
}

func GetPodSecurityContext(secctx *corev1.PodSecurityContext) (podSec *corev1.PodSecurityContext) {
	// 999 is the default userid for redis offical docker image
	// 1000 is the default groupid for redis offical docker image
	_, groupId := int64(999), int64(1000)
	if secctx == nil {
		podSec = &corev1.PodSecurityContext{FSGroup: &groupId}
	} else {
		podSec = &corev1.PodSecurityContext{}
		if secctx.FSGroup != nil {
			podSec.FSGroup = secctx.FSGroup
		}
	}
	return
}

func GetSecurityContext(secctx *corev1.PodSecurityContext) (sec *corev1.SecurityContext) {
	// 999 is the default userid for redis offical docker image
	// 1000 is the default groupid for redis offical docker image
	userId, groupId := int64(999), int64(1000)
	if secctx == nil {
		sec = &corev1.SecurityContext{
			RunAsUser:              &userId,
			RunAsGroup:             &groupId,
			RunAsNonRoot:           ptr.To(true),
			ReadOnlyRootFilesystem: ptr.To(true),
		}
	} else {
		sec = &corev1.SecurityContext{
			RunAsUser:              &userId,
			RunAsGroup:             &groupId,
			RunAsNonRoot:           ptr.To(true),
			ReadOnlyRootFilesystem: ptr.To(true),
		}
		if secctx.RunAsUser != nil {
			sec.RunAsUser = secctx.RunAsUser
		}
		if secctx.RunAsGroup != nil {
			sec.RunAsGroup = secctx.RunAsGroup
		}
		if secctx.RunAsNonRoot != nil {
			sec.RunAsNonRoot = secctx.RunAsNonRoot
		}
	}
	return
}
