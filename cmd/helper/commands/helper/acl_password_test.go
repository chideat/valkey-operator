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

package helper

import (
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_loadUserPassword(t *testing.T) {
	const hash = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08" // sha256("test")

	secret := func(data map[string]string) *corev1.Secret {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "pw"}, Data: map[string][]byte{}}
		for k, v := range data {
			s.Data[k] = []byte(v)
		}
		return s
	}

	tests := []struct {
		name     string
		username string
		data     map[string]string
		wantRule string
		wantSkip bool
		wantErr  bool
	}{
		// operator: anything but a non-empty plaintext is an error, never "nopass"
		{name: "operator plaintext", username: "operator", data: map[string]string{"password": "anything"}, wantRule: ">anything"},
		{name: "operator nothing specified", username: "operator", data: map[string]string{"username": "operator"}, wantErr: true},
		{name: "operator empty password", username: "operator", data: map[string]string{"password": ""}, wantErr: true},
		{name: "operator both keys", username: "operator", data: map[string]string{"password": "anything", "password-sha256": hash}, wantErr: true},
		{name: "operator hashed", username: "operator", data: map[string]string{"password-sha256": hash}, wantErr: true},

		// default user: Valkey recreates it without a password when the ACL file leaves
		// it out, so an unusable secret is an error, never a skip
		{name: "default hashed", username: "default", data: map[string]string{"password-sha256": hash}, wantRule: "#" + hash},
		{name: "default plaintext", username: "default", data: map[string]string{"password": "Str0ng!Passw0rd"}, wantRule: ">Str0ng!Passw0rd"},
		{name: "default weak plaintext is kept", username: "default", data: map[string]string{"password": "weak"}, wantRule: ">weak"},
		{name: "default nothing specified", username: "default", data: map[string]string{}, wantErr: true},
		{name: "default empty password", username: "default", data: map[string]string{"password": ""}, wantErr: true},
		{name: "default both keys", username: "default", data: map[string]string{"password": "Str0ng!Passw0rd", "password-sha256": hash}, wantErr: true},
		{name: "default malformed hash", username: "default", data: map[string]string{"password-sha256": "abc"}, wantErr: true},

		// other users: an unusable secret leaves the user out, so it can not log in
		{name: "user hashed", username: "app", data: map[string]string{"password-sha256": hash}, wantRule: "#" + hash},
		{name: "user strong plaintext", username: "app", data: map[string]string{"password": "Str0ng!Passw0rd"}, wantRule: ">Str0ng!Passw0rd"},
		{name: "user weak plaintext", username: "app", data: map[string]string{"password": "weak"}, wantSkip: true},
		{name: "user nothing specified", username: "app", data: map[string]string{}, wantSkip: true},
		{name: "user both keys", username: "app", data: map[string]string{"password": "Str0ng!Passw0rd", "password-sha256": hash}, wantSkip: true},
		{name: "user malformed hash", username: "app", data: map[string]string{"password-sha256": "abc"}, wantSkip: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			passwd, skip, err := loadUserPassword(tt.username, secret(tt.data))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if skip != tt.wantSkip {
				t.Fatalf("skip = %v, want %v", skip, tt.wantSkip)
			}
			if tt.wantErr || tt.wantSkip {
				if passwd != nil {
					t.Fatalf("expected no password, got rule %q", passwd.ACLRule())
				}
				return
			}
			if got := passwd.ACLRule(); got != tt.wantRule {
				t.Errorf("ACLRule() = %q, want %q", got, tt.wantRule)
			}
		})
	}
}

func Test_loadACLUsers(t *testing.T) {
	const (
		hash1 = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08" // sha256("test")
		hash2 = "60303ae22b998861bce3b28f33eec1be758a213c86c93c076dbe9f558c11c752" // sha256("test2")
	)
	secrets := map[string]map[string]string{
		"op-secret":   {"password": "operator-password"},
		"app-hash-1":  {"password-sha256": hash1},
		"app-hash-2":  {"password-sha256": hash2},
		"bad-secret":  {"password-sha256": "abc"},
		"none-secret": {"username": "x"},
	}
	getSecret := func(name string) (*corev1.Secret, error) {
		data, ok := secrets[name]
		if !ok {
			return nil, fmt.Errorf("secret %s not found", name)
		}
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name}, Data: map[string][]byte{}}
		for k, v := range data {
			s.Data[k] = []byte(v)
		}
		return s, nil
	}
	const opEntry = `{"name":"operator","role":"Operator","password":{"secretName":"op-secret"},"rules":[{"categories":["all"],"keyPatterns":["*"]}]}`
	configMap := func(data map[string]string) *corev1.ConfigMap {
		return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "acl", Namespace: "ns"}, Data: data}
	}
	generate := func(cm *corev1.ConfigMap) (map[string]string, error) {
		users, err := loadACLUsers(cm, getSecret)
		if err != nil {
			return nil, err
		}
		ret := map[string]string{}
		for _, u := range users {
			ret[u.Name] = strings.Join(formatACLSetCommand(u), " ")
		}
		return ret, nil
	}

	t.Run("operator entry without secret reference fails", func(t *testing.T) {
		// an older configmap that already lost the operator's secret reference
		for name, entry := range map[string]string{
			"no password field": `{"name":"operator","role":"Operator","rules":[{"categories":["all"],"keyPatterns":["*"]}]}`,
			"empty secret name": `{"name":"operator","role":"Operator","password":{},"rules":[{"categories":["all"],"keyPatterns":["*"]}]}`,
		} {
			acl, err := generate(configMap(map[string]string{"operator": entry}))
			if err == nil {
				t.Errorf("%s: expected an error, got ACL %v", name, acl)
			}
		}
	})

	t.Run("default user with unusable secret fails", func(t *testing.T) {
		for _, secretName := range []string{"bad-secret", "none-secret"} {
			cm := configMap(map[string]string{
				"operator": opEntry,
				"default":  `{"name":"default","role":"Developer","password":{"secretName":"` + secretName + `"},"rules":[{"categories":["all"],"keyPatterns":["*"]}]}`,
			})
			if acl, err := generate(cm); err == nil {
				t.Errorf("%s: expected an error, got ACL %v", secretName, acl)
			}
		}
	})

	t.Run("application user with unusable secret is left out", func(t *testing.T) {
		cm := configMap(map[string]string{
			"operator": opEntry,
			"app":      `{"name":"app","role":"Developer","password":{"secretName":"bad-secret"},"rules":[{"categories":["all"],"keyPatterns":["*"]}]}`,
		})
		acl, err := generate(cm)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := acl["app"]; ok {
			t.Errorf("app user must be left out, got %q", acl["app"])
		}
		if !strings.Contains(acl["operator"], ">operator-password") {
			t.Errorf("operator rule = %q", acl["operator"])
		}
	})

	t.Run("every password secret is applied", func(t *testing.T) {
		cm := configMap(map[string]string{
			"operator": opEntry,
			"app":      `{"name":"app","role":"Developer","password":{"secretName":"app-hash-1"},"additionalPasswords":[{"secretName":"app-hash-2"}],"rules":[{"categories":["all"],"keyPatterns":["*"]}]}`,
		})
		acl, err := generate(cm)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(acl["app"], "#"+hash1) || !strings.Contains(acl["app"], "#"+hash2) || strings.Contains(acl["app"], "nopass") {
			t.Errorf("app rule = %q, want both hashes", acl["app"])
		}
	})

	t.Run("one unusable secret leaves the whole user out", func(t *testing.T) {
		cm := configMap(map[string]string{
			"operator": opEntry,
			"app":      `{"name":"app","role":"Developer","password":{"secretName":"app-hash-1"},"additionalPasswords":[{"secretName":"bad-secret"}],"rules":[{"categories":["all"],"keyPatterns":["*"]}]}`,
		})
		acl, err := generate(cm)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := acl["app"]; ok {
			t.Errorf("app user must be left out, got %q", acl["app"])
		}
	})
}
