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

package config

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	if _, err := LoadValkeyVersionMap(); err != nil {
		panic(err)
	}
}

func LoadValkeyVersionMap() (map[string]string, error) {
	versionMap := map[string]string{}
	if mapVal := os.Getenv("VALKEY_VERSION_MAP"); mapVal != "" {
		if err := json.Unmarshal([]byte(mapVal), &versionMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal VALKEY_VERSION_MAP: %w", err)
		}
	}
	return versionMap, nil
}

func GetValkeyVersion(image string) string {
	if image == "" {
		return ""
	}
	if idx := strings.Index(image, ":"); idx != -1 {
		if dashIdx := strings.Index(image[idx+1:], "-"); dashIdx != -1 {
			return image[idx+1 : idx+1+dashIdx]
		}
		return image[idx+1:]
	}
	return ""
}

func Getenv(name string, defaults ...string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}

	for _, v := range defaults {
		if v != "" {
			return v
		}
	}
	return ""
}

func GetFullImageURL(path string, tag string) string {
	registry := Getenv("DEFAULT_REGISTRY")
	if registry == "" {
		return fmt.Sprintf("%s:%s", path, tag)
	}
	if strings.HasPrefix(path, registry) {
		return fmt.Sprintf("%s:%s", path, tag)
	}
	return fmt.Sprintf("%s/%s:%s", registry, path, tag)
}

func GetValkeyImageByVersion(version string) string {
	imageName := os.Getenv("VALKEY_IMAGE_NAME")
	if imageName == "" {
		imageName = "valkey/valkey"
	}

	versionMap, _ := LoadValkeyVersionMap()
	if versionMap != nil {
		if val, _ := versionMap[version]; val != "" {
			version = val
		}
	}
	return GetFullImageURL(imageName, version)
}

const (
	ImageVersionKeyPrefix = "buf.red/imageversions-"
)

func BuildImageVersionKey(typ string) string {
	return ImageVersionKeyPrefix + typ
}

func GetOperatorVersion() string {
	return Getenv("OPERATOR_VERSION", "latest")
}

func GetValkeyHelperImage(obj v1.Object) string {
	key := BuildImageVersionKey("valkey-helper")
	if obj != nil {
		if val := obj.GetAnnotations()[key]; val != "" {
			return val
		}
	}

	imgName := Getenv("OPERATOR_IMAGE_NAME", "chideat/valkey-operator")
	imgVersion := GetOperatorVersion()
	return GetFullImageURL(imgName, imgVersion)
}

func GetValkeyExporterImage(obj v1.Object) string {
	key := BuildImageVersionKey("exporter")
	if obj != nil {
		if val := obj.GetAnnotations()[key]; val != "" {
			return val
		}
	}
	imgName := Getenv("DEFAULT_EXPORTER_IMAGE_NAME", "oliver006/redis_exporter")
	imgVersion := Getenv("DEFAULT_EXPORTER_VERSION", "v1.67.0-alpine")
	return GetFullImageURL(imgName, imgVersion)
}

func LoadbalancerReadyTimeout() time.Duration {
	timeout := os.Getenv("LOADBALANCER_WAIT_TIMEOUT")
	if timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			return d
		}
	}
	return 2 * time.Minute
}

// DefaultIPFamily reports the IP family this cluster allocates by default,
// derived from the operator pod's own addresses.
//
// The downward API renders status.podIPs comma-separated, and its first entry
// is status.podIP by API contract — the cluster's primary family — so the head
// of the list settles it with no API call and no ordering logic of our own.
//
// This is an inference: the value describes the POD network, while its main
// consumer sets Service.spec.ipFamilies, which the SERVICE network governs.
// Kubernetes does not support a cluster whose pod and service networks are
// single-stack on different families, so the two agree in every configuration
// that can exist — but it is read from one and applied to the other, and a
// reader should know that.
//
// Returns "" when unresolvable (POD_IPS unset, as under `make run`, or an
// unparsable value). Callers must treat that as unspecified and leave the
// choice to the API server; treating it as IPv4 is the defect this exists to
// prevent.
func DefaultIPFamily() corev1.IPFamily {
	first, _, _ := strings.Cut(os.Getenv("POD_IPS"), ",")
	addr, err := netip.ParseAddr(strings.TrimSpace(first))
	if err != nil {
		return ""
	}
	// Is4 covers IPv4-mapped IPv6 (::ffff:1.2.3.4), which is an IPv4 address
	// however it is spelled.
	if addr.Is4() || addr.Is4In6() {
		return corev1.IPv4Protocol
	}
	return corev1.IPv6Protocol
}
