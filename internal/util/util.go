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

package util

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

var (
	memUnitReg = regexp.MustCompile(`^[0-9]+(b|k|kb|m|mb|g|gb)$`)
)

// UnifyValueUnit value convert, not care the config itself
func UnifyValueUnit(v string) string {
	fields := strings.Fields(strings.ToLower(v))
	changed := false
	for i, field := range fields {
		if memUnitReg.MatchString(field) {
			if field, err := ConvertMemoryUnit(field); err == nil {
				fields[i] = field
				changed = true
			}
		}
	}

	if changed {
		return strings.Join(fields, " ")
	}
	return v
}

// ConvertMemoryUnit
func ConvertMemoryUnit(p string) (string, error) {
	u := strings.ToLower(p)
	digits := u

	var mul int64 = 1
	if strings.HasSuffix(u, "k") {
		digits = u[:len(u)-len("k")]
		mul = 1000
	} else if strings.HasSuffix(u, "kb") {
		digits = u[:len(u)-len("kb")]
		mul = 1024
	} else if strings.HasSuffix(u, "m") {
		digits = u[:len(u)-len("m")]
		mul = 1000 * 1000
	} else if strings.HasSuffix(u, "mb") {
		digits = u[:len(u)-len("mb")]
		mul = 1024 * 1024
	} else if strings.HasSuffix(u, "g") {
		digits = u[:len(u)-len("g")]
		mul = 1000 * 1000 * 1000
	} else if strings.HasSuffix(u, "gb") {
		digits = u[:len(u)-len("gb")]
		mul = 1024 * 1024 * 1024
	} else if strings.HasSuffix(u, "b") {
		digits = u[:len(u)-len("b")]
		mul = 1
	}

	val, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(val*mul, 10), nil
}

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

func GenerateObjectSig(data any, salt string) (string, error) {
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
