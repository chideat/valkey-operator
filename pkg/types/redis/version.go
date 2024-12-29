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

package redis

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/chideat/valkey-operator/api/core"
)

var (
	MinTLSSupportedVersion, _  = semver.NewVersion("6.0-AAA")
	MinACLSupportedVersion     = MinTLSSupportedVersion
	MinACL2SupportedVersion, _ = semver.NewVersion("7.0-AAA")

	_Redis5Version, _ = semver.NewVersion("5.0-AAA")
	_Redis6Version, _ = semver.NewVersion("6.0-AAA")
	_Redis7Version, _ = semver.NewVersion("7.0-AAA")

	_ = _Redis6Version
)

// RedisVersion
type RedisVersion string

const (
	// RedisVersionUnknown
	RedisVersionUnknown RedisVersion = ""
	// RedisVersion4
	RedisVersion4 RedisVersion = "4.0"
	// RedisVersion5
	RedisVersion5 RedisVersion = "5.0"
	// RedisVersion6, Supports ACL, io-threads
	RedisVersion6 RedisVersion = "6.0"
	// RedisVersion6_2
	RedisVersion6_2 RedisVersion = "6.2"
	// RedisVersion7
	RedisVersion7 RedisVersion = "7.0"
	// Supports ACL2, Function
	RedisVersion7_2 RedisVersion = "7.2"
	// Supports modules
	ActiveRedisVersion6 RedisVersion = "6.0"
)

func (v RedisVersion) String() string {
	return string(v)
}

func (v RedisVersion) IsTLSSupported() bool {
	ver, err := semver.NewVersion(string(v))
	if err != nil {
		return false
	}
	return ver.Compare(MinTLSSupportedVersion) >= 0
}

func (v RedisVersion) IsACLSupported() bool {
	ver, err := semver.NewVersion(string(v))
	if err != nil {
		return false
	}
	return ver.Compare(MinACLSupportedVersion) >= 0
}

func (v RedisVersion) IsACL2Supported() bool {
	ver, err := semver.NewVersion(string(v))
	if err != nil {
		return false
	}
	return ver.Compare(MinACL2SupportedVersion) >= 0
}

func (v RedisVersion) IsClusterShardSupported() bool {
	ver, err := semver.NewVersion(string(v))
	if err != nil {
		return false
	}
	return ver.Compare(_Redis7Version) >= 0
}

func (v RedisVersion) CustomConfigs(arch RedisArch) map[string]string {
	if v == RedisVersionUnknown {
		return nil
	}

	ver, err := semver.NewVersion(string(v))
	if err != nil {
		return nil
	}

	ret := map[string]string{}
	if ver.Compare(_Redis5Version) >= 0 {
		ret["ignore-warnings"] = "ARM64-COW-BUG"
	}
	if arch == core.ValkeyCluster {
		switch {
		case ver.Compare(_Redis7Version) >= 0:
			ret["cluster-allow-replica-migration"] = "no"
			ret["cluster-migration-barrier"] = "10"
		default:
			ret["cluster-migration-barrier"] = "10"
		}
	}
	return ret
}

// ParseVersion
func ParseRedisVersion(v string) (RedisVersion, error) {
	ver, err := semver.NewVersion(v)
	if err != nil {
		return "", err
	}
	return RedisVersion(fmt.Sprintf("%d.%d", ver.Major(), ver.Minor())), nil
}

// ParseRedisVersionFromImage
func ParseRedisVersionFromImage(u string) (RedisVersion, error) {
	if index := strings.LastIndex(u, ":"); index > 0 {
		version := u[index+1:]
		if version == "latest" {
			return RedisVersion6, nil
		}
		return ParseRedisVersion(version)
	} else {
		return "", errors.New("invalid image")
	}
}
