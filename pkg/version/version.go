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
)

type ValkeyVersion string

const (
	ValkeyVersionUnknown ValkeyVersion = ""

	DefaultValKeyVersion = ValkeyVersion("8.0")
)

func (v ValkeyVersion) String() string {
	return string(v)
}

func (v ValkeyVersion) IsTLSSupported() bool {
	ver, err := semver.NewVersion(string(v))
	if err != nil {
		return false
	}
	return ver.Compare(MinTLSSupportedVersion) >= 0
}

func (v ValkeyVersion) IsACLSupported() bool {
	ver, err := semver.NewVersion(string(v))
	if err != nil {
		return false
	}
	return ver.Compare(MinACLSupportedVersion) >= 0
}

func (v ValkeyVersion) IsClusterShardSupported() bool {
	return true
}

func (v ValkeyVersion) CustomConfigs(arch core.Arch) map[string]string {
	if v == "" {
		return nil
	}

	ret := map[string]string{}
	ret["ignore-warnings"] = "ARM64-COW-BUG"
	if arch == core.ValkeyCluster {
		ret["cluster-allow-replica-migration"] = "no"
		ret["cluster-migration-barrier"] = "10"
	}
	return ret
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
