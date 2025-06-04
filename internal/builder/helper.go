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

package builder

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/chideat/valkey-operator/api/core"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func ChecksumKey(typ string) string {
	return fmt.Sprintf("%s-%s", ChecksumLabelKey, strings.ToLower(typ))
}

func ResourcePrefix(arch core.Arch) string {
	// NOTE: compatibility with redis-operator
	switch arch {
	case core.ValkeyCluster:
		return "drc"
	case core.ValkeySentinel:
		return "rfs"
	default:
		return "rfr"
	}
}

func LocalhostAlias(family corev1.IPFamily) corev1.HostAlias {
	localhost := "127.0.0.1"
	if family == corev1.IPv6Protocol {
		localhost = "::1"
	}
	return corev1.HostAlias{
		IP:        localhost,
		Hostnames: []string{"local.inject"},
	}
}

func GetPullPolicy(policies ...corev1.PullPolicy) corev1.PullPolicy {
	for _, policy := range policies {
		if policy != "" {
			return policy
		}
	}
	return corev1.PullIfNotPresent
}

func GetPodSecurityContext(secctx *corev1.PodSecurityContext) (podSec *corev1.PodSecurityContext) {
	// 999 is the default userid for offical docker image
	// 1000 is the default groupid for offical docker image
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
	// 999 is the default userid for offical docker image
	// 1000 is the default groupid for offical docker image
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

func GetContainerSecurityContext(secctx *corev1.SecurityContext) (sec *corev1.SecurityContext) {
	// 999 is the default userid for offical docker image
	// 1000 is the default groupid for offical docker image
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

func ParsePodIndex(name string) (index int, err error) {
	fields := strings.Split(name, "-")
	if len(fields) < 2 {
		return -1, fmt.Errorf("invalid pod name %s", name)
	}
	if index, err = strconv.Atoi(fields[len(fields)-1]); err != nil {
		return -1, fmt.Errorf("invalid pod name %s", name)
	}
	return index, nil
}

func ParsePodShardAndIndex(name string) (shard int, index int, err error) {
	fields := strings.Split(name, "-")
	if len(fields) < 3 {
		return -1, -1, fmt.Errorf("invalid pod name %s", name)
	}
	if index, err = strconv.Atoi(fields[len(fields)-1]); err != nil {
		return -1, -1, fmt.Errorf("invalid pod name %s", name)
	}
	if shard, err = strconv.Atoi(fields[len(fields)-2]); err != nil {
		return -1, -1, fmt.Errorf("invalid pod name %s", name)
	}
	return shard, index, nil
}

func MergeAnnotations(t, s map[string]string) map[string]string {
	if t == nil {
		return s
	}
	if s == nil {
		return t
	}

	for k, v := range s {
		if k == RestartAnnotationKey {
			tRestartAnn := t[k]
			if tRestartAnn == "" && v != "" {
				t[k] = v
			}

			tTime, err1 := time.Parse(time.RFC3339Nano, tRestartAnn)
			sTime, err2 := time.Parse(time.RFC3339Nano, v)
			if err1 != nil || err2 != nil || sTime.After(tTime) {
				t[k] = v
			} else {
				t[k] = tRestartAnn
			}
		} else {
			t[k] = v
		}
	}
	return t
}
