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

package version

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/chideat/valkey-operator/api/core"
)

var (
	MinTLSSupportedVersion, _ = semver.NewVersion("7.4.0")
	MinACLSupportedVersion, _ = semver.NewVersion("7.4.0")

	// ValkeyVersion-typed gates (used with IsAtLeast)
	MinVectorSetsVersion        = ValkeyVersion("8.0")
	MinLatencyTrackingVersion   = ValkeyVersion("9.0")
	MinLazyfreeUserFlushVersion = ValkeyVersion("9.0")
)

type ValkeyVersion string

const (
	ValkeyVersionUnknown ValkeyVersion = ""

	DefaultValKeyVersion = ValkeyVersion("8.0")
)

func (v ValkeyVersion) String() string {
	return string(v)
}

func (v ValkeyVersion) CustomConfigs(arch core.Arch) map[string]string {
	if v == "" {
		return nil
	}

	ret := map[string]string{
		"ignore-warnings": "ARM64-COW-BUG",
	}

	if arch == core.ValkeyCluster {
		ret["cluster-allow-replica-migration"] = "no"
		ret["cluster-migration-barrier"] = "10"
	}

	// Valkey 9.0+: latency tracking on by default upstream; make explicit
	if v.IsAtLeast(MinLatencyTrackingVersion) {
		ret["latency-tracking"] = "yes"
	}

	// Valkey 9.0+: lazyfree-lazy-user-flush default changed
	if v.IsAtLeast(MinLazyfreeUserFlushVersion) {
		ret["lazyfree-lazy-user-flush"] = "yes"
	}

	return ret
}

// Compare compares two versions.
//
// if v > other, return 1
// if v < other, return -1
// if v == other, return 0
// if error occurred, return -2
func (v ValkeyVersion) Compare(other ValkeyVersion) int {
	if v == "" && other == "" {
		return 0
	}
	if v == "" {
		return -1
	}
	if other == "" {
		return 1
	}
	v1, err := semver.NewVersion(string(v))
	if err != nil {
		return -2
	}
	v2, err := semver.NewVersion(string(other))
	if err != nil {
		return -2
	}
	return v1.Compare(v2)
}

// IsAtLeast returns true if v >= minVersion according to Compare.
func (v ValkeyVersion) IsAtLeast(minVersion ValkeyVersion) bool {
	return v.Compare(minVersion) >= 0
}

// ParseVersion
func ParseValkeyVersion(v string) (ValkeyVersion, error) {
	ver, err := semver.NewVersion(v)
	if err != nil {
		return "", err
	}
	return ValkeyVersion(fmt.Sprintf("%d.%d", ver.Major(), ver.Minor())), nil
}

// ParseValkeyVersionFromImage
func ParseValkeyVersionFromImage(u string) (ValkeyVersion, error) {
	if index := strings.LastIndex(u, ":"); index > 0 {
		version := u[index+1:]
		if version == "latest" {
			return DefaultValKeyVersion, nil
		}
		return ParseValkeyVersion(version)
	} else {
		return "", errors.New("invalid image")
	}
}
