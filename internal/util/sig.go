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
*/package util

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

type SignaturableObject interface {
	string | []byte
}

func mapSigGenerator[T SignaturableObject](obj map[string]T, salt string) (string, error) {
	var (
		keys []string
		data []string
	)
	for key := range obj {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		data = append(data, fmt.Sprintf("%v", obj[key]))
	}
	return fmt.Sprintf("%x", sha256.Sum256(append([]byte(salt), []byte(strings.Join(data, "\n"))...))), nil
}

func GenerateObjectSig(data interface{}, salt string) (string, error) {
	if data == nil {
		return "", nil
	}

	switch val := data.(type) {
	case string:
		return fmt.Sprintf("%x", sha256.Sum256(append([]byte(salt), []byte(val)...))), nil
	case []byte:
		return fmt.Sprintf("%x", sha256.Sum256(append([]byte(salt), val...))), nil
	case *corev1.ConfigMap:
		return mapSigGenerator(val.Data, salt)
	case *corev1.Secret:
		return mapSigGenerator(val.Data, salt)
	default:
		return "", fmt.Errorf("unsupported data type %T", data)
	}
}
