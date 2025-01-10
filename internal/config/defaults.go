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
	"errors"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	ErrImageNotFound = errors.New("image not found")
	ErrInvalidImage  = errors.New("invalid source image")
)

const DefaultValkeyVersion = "8.0"

var valkeyVersionEnvs = []string{
	"VALKEY_VERSION_7_2_IMAGE",
	"VALKEY_VERSION_7_4_IMAGE",
	"VALKEY_VERSION_8_0_IMAGE",
}

func GetOperatorVersion() string {
	return os.Getenv("VALKEY_OPERATOR_VERSION")
}

func GetValkeyVersion(image string) string {
	if image == "" {
		image = GetDefaultValkeyImage()
	}
	if idx := strings.Index(image, ":"); idx != -1 {
		if dashIdx := strings.Index(image[idx+1:], "-"); dashIdx != -1 {
			return image[idx+1 : idx+1+dashIdx]
		}
		return image[idx+1:]
	}
	return ""
}

func DebugEnabled() bool {
	return os.Getenv("DEBUG") != ""
}

func GetWatchNamespace() string {
	ns := os.Getenv("WATCH_NAMESPACE")
	if ns == "" {
		ns = "operators"
	}
	return ns
}

func GetPodName() string {
	return os.Getenv("POD_NAME")
}

func GetPodUid() string {
	return os.Getenv("POD_UID")
}

func Getenv(name string, defaults ...string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}

	for _, v := range defaults {
		if v != "" {
			return defaults[0]
		}
	}
	return ""
}

func GetDefaultValkeyImage() string {
	return Getenv("DEFAULT_VALKEY_IMAGE")
}

func GetValkeyImageByVersion(version string) (string, error) {
	wantedVersion, err := semver.NewVersion(version)
	if err != nil {
		return "", ErrInvalidImage
	}

	var lastErr error
	for _, name := range valkeyVersionEnvs {
		val := os.Getenv(name)
		if val == "" {
			continue
		}
		fields := strings.SplitN(val, ":", 2)
		if len(fields) != 2 {
			continue
		}

		ver, err := semver.NewVersion(fields[1])
		if err != nil {
			lastErr = err
			continue
		}
		if ver.Major() == wantedVersion.Major() && ver.Minor() == wantedVersion.Minor() {
			return val, nil
		}
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", ErrImageNotFound
}

func BuildImageVersionKey(typ string) string {
	return ImageVersionKeyPrefix + typ
}

func GetValkeyToolsImage(obj v1.Object) string {
	key := ImageVersionKeyPrefix + "valkey-operator"
	if obj != nil {
		if val := obj.GetAnnotations()[key]; val != "" {
			return val
		}
	}
	return Getenv("VALKEY_TOOLS_IMAGE")
}

func GetValkeyExporterImage(obj v1.Object) string {
	key := ImageVersionKeyPrefix + "redis-exporter"
	if obj != nil {
		if val := obj.GetAnnotations()[key]; val != "" {
			return val
		}
	}
	return Getenv("VALKEY_EXPORTER_IMAGE", Getenv("DEFAULT_EXPORTER_IMAGE"))
}
