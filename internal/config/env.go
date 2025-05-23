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
	"os"
	"strings"

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
			return defaults[0]
		}
	}
	return ""
}

func GetFullImageURL(path string, tag string) string {
	registry := Getenv("DEFAULT_REGISTRY")
	if registry == "" {
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
