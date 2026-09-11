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

// IPFamilySpec renders the Service IP-family fields for an access preference.
//
// An unset preference yields (nil, nil): the API server then assigns the
// cluster's own families, the only choice that is correct on IPv4-only,
// IPv6-only and dual-stack alike. Pinning IPv4 for "unspecified" makes every
// Service outright invalid on a single-stack IPv6 cluster —
// `spec.ipFamilies[0]: Invalid value: "IPv4": not configured on this cluster` —
// so nothing deploys at all.
//
// In normal operation the preference is resolved once by the Valkey defaulting
// webhook (config.DefaultIPFamily) and every consumer sees the same family,
// which matters because a Valkey cluster or sentinel registers exactly one
// address per node. This unset path is the floor for when that did not happen:
// ENABLE_WEBHOOKS=false, a resource created before the operator upgrade, or an
// unreachable webhook.
func IPFamilySpec(prefer corev1.IPFamily) ([]corev1.IPFamily, *corev1.IPFamilyPolicy) {
	if prefer == "" {
		return nil, nil
	}
	return []corev1.IPFamily{prefer}, ptr.To(corev1.IPFamilyPolicySingleStack)
}

// LocalhostAlias is the /etc/hosts entry every client inside the pod (probes,
// lifecycle hooks, the agent, the exporter) resolves local.inject through.
// valkey-server cannot bind that alias by name: it looks a hostname up as IPv4
// and aborts when the entry is ::1. So cmd/run_{cluster,failover,sentinel}.sh
// bind the literal instead, derived from the IP_FAMILY_PREFER env that carries
// the same field. The two mappings must agree: IPv6 -> ::1, anything else, the
// unset value included -> 127.0.0.1.
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

// 999 is the default userid for the official docker image
// 1000 is the default groupid for the official docker image
const (
	defaultValkeyUserID  int64 = 999
	defaultValkeyGroupID int64 = 1000
)

// GetPodSecurityContext fills in the pod-level defaults, preserving anything the caller
// already set. Only FSGroup and SeccompProfile belong here; per-container hardening is
// GetContainerSecurityContext's job.
func GetPodSecurityContext(secctx *corev1.PodSecurityContext) *corev1.PodSecurityContext {
	groupID := defaultValkeyGroupID
	if secctx == nil {
		secctx = &corev1.PodSecurityContext{}
	}

	if secctx.FSGroup == nil {
		secctx.FSGroup = &groupID
	}
	if secctx.SeccompProfile == nil {
		secctx.SeccompProfile = &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		}
	}
	return secctx
}

// GetContainerSecurityContext fills in defaults that satisfy the Pod Security Admission
// "restricted" profile, leaving any field the caller set alone.
//
// Every field below is required by that profile, so omitting one means the pod is rejected
// outright on a namespace that enforces it.
func GetContainerSecurityContext(secctx *corev1.SecurityContext) *corev1.SecurityContext {
	userID, groupID := defaultValkeyUserID, defaultValkeyGroupID
	if secctx == nil {
		secctx = &corev1.SecurityContext{}
	}

	if secctx.RunAsUser == nil {
		secctx.RunAsUser = &userID
	}
	if secctx.RunAsGroup == nil {
		secctx.RunAsGroup = &groupID
	}
	if *secctx.RunAsUser != 0 {
		if secctx.RunAsNonRoot == nil {
			secctx.RunAsNonRoot = ptr.To(true)
		}
	} else {
		// a caller asking for uid 0 means it: init containers fixing file ownership cannot
		// also claim to be non-root, and leaving both set makes the kubelet refuse the pod
		secctx.RunAsNonRoot = nil
	}
	if secctx.ReadOnlyRootFilesystem == nil {
		secctx.ReadOnlyRootFilesystem = ptr.To(true)
	}
	if secctx.AllowPrivilegeEscalation == nil {
		secctx.AllowPrivilegeEscalation = ptr.To(false)
	}
	if secctx.Capabilities == nil {
		secctx.Capabilities = &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		}
	}
	if secctx.Privileged == nil {
		secctx.Privileged = ptr.To(false)
	}
	if secctx.SeccompProfile == nil {
		secctx.SeccompProfile = &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		}
	}
	return secctx
}

// GetSecurityContext derives a container security context from the pod-level one a CR
// supplies, then hardens it. Several builders expose only a PodSecurityContext field.
func GetSecurityContext(podsecctx *corev1.PodSecurityContext) *corev1.SecurityContext {
	if podsecctx == nil {
		podsecctx = &corev1.PodSecurityContext{}
	}
	return GetContainerSecurityContext(&corev1.SecurityContext{
		SELinuxOptions: podsecctx.SELinuxOptions,
		WindowsOptions: podsecctx.WindowsOptions,
		RunAsUser:      podsecctx.RunAsUser,
		RunAsGroup:     podsecctx.RunAsGroup,
		RunAsNonRoot:   podsecctx.RunAsNonRoot,
		SeccompProfile: podsecctx.SeccompProfile,
	})
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

func MergeRestartAnnotation(n, o map[string]string) map[string]string {
	if n == nil {
		n = make(map[string]string)
	}

	oldTimeStr, exists := o[RestartAnnotationKey]
	if !exists || oldTimeStr == "" {
		return n
	}
	oldTime, err := time.Parse(time.RFC3339Nano, oldTimeStr)
	if err != nil {
		return n
	}

	newTimeStr, exists := n[RestartAnnotationKey]
	if !exists || newTimeStr == "" {
		n[RestartAnnotationKey] = oldTimeStr
		return n
	}
	newTime, err := time.Parse(time.RFC3339Nano, newTimeStr)
	if err != nil {
		n[RestartAnnotationKey] = oldTimeStr
		return n
	}

	if oldTime.After(newTime) {
		n[RestartAnnotationKey] = oldTimeStr
		return n
	}
	return n
}

func IsPodAnnotationDiff(d map[string]string, s map[string]string) bool {
	if len(d) != len(s) {
		return true
	}

	for k, v := range d {
		if k == RestartAnnotationKey {
			if v == "" {
				continue
			}
			targetV := s[RestartAnnotationKey]
			if targetV == "" {
				return true
			}
			newTime, err1 := time.Parse(time.RFC3339Nano, v)
			targetTime, err2 := time.Parse(time.RFC3339Nano, targetV)
			if err1 != nil || err2 != nil {
				return true
			}
			if newTime.After(targetTime) {
				return true
			}
		} else if s[k] != v {
			return true
		}
	}
	return false
}
