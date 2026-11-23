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

package acl

import (
	"context"
	"strings"
	"testing"

	"github.com/chideat/valkey-operator/pkg/kubernetes/clientset/mocks"
	"github.com/chideat/valkey-operator/pkg/types/user"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLoadACLUsers(t *testing.T) {
	ctx := context.TODO()

	clientset := mocks.NewClientSet(t)
	t.Run("nil ConfigMap", func(t *testing.T) {
		users, err := LoadACLUsers(ctx, clientset, nil)
		if err != nil {
			t.Errorf("LoadACLUsers() error = %v, wantErr %v", err, false)
		}
		if len(users) != 0 {
			t.Errorf("LoadACLUsers() = %v, want %v", len(users), 0)
		}
	})

	t.Run("default name ConfigMap", func(t *testing.T) {
		namespace := "default"
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cm",
				Namespace: namespace,
			},
			Data: map[string]string{
				"": `{"name":"","role":"Developer","rules":[{"categories":["all"],"disallowedCommands":["acl","flushall","flushdb","keys"],"keyPatterns":["*"]}]}`,
			},
		}
		users, err := LoadACLUsers(ctx, clientset, cm)
		if err != nil {
			t.Errorf("LoadACLUsers() error = %v, wantErr %v", err, false)
		}
		if len(users) != 1 {
			t.Errorf("LoadACLUsers() = %v, want %v", len(users), 1)
		}
		if users[0].Name != user.DefaultUserName {
			t.Errorf("LoadACLUsers() = %v, want %v", users[0].Name, user.DefaultUserName)
		}
	})

	t.Run("valid ConfigMap", func(t *testing.T) {
		secretName := "valkey-sen-customport-sm8r5"
		namespace := "default"
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"password": []byte("password"),
			},
		}
		clientset.On("GetSecret", ctx, namespace, secret.Name).Return(secret, nil)
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cm",
				Namespace: namespace,
			},
			Data: map[string]string{
				"default": `{"name":"default","role":"Developer","password":{"secretName":"valkey-sen-customport-sm8r5"},"rules":[{"categories":["all"],"disallowedCommands":["acl","flushall","flushdb","keys"],"keyPatterns":["*"]}]}`,
			},
		}
		users, err := LoadACLUsers(ctx, clientset, cm)
		if err != nil {
			t.Errorf("LoadACLUsers() error = %v, wantErr %v", err, false)
		}
		if len(users) != 1 {
			t.Errorf("LoadACLUsers() = %v, want %v", len(users), 1)
		}
	})

	t.Run("invalid username", func(t *testing.T) {
		cm := &corev1.ConfigMap{
			Data: map[string]string{
				"abc+123": `{"name":"abc+123","role":"Developer","rules":[{"categories":["all"],"disallowedCommands":["acl","flushall","flushdb","keys"],"keyPatterns":["*"]}]}`,
			},
		}
		_, err := LoadACLUsers(ctx, clientset, cm)
		if err == nil {
			t.Errorf("LoadACLUsers() error = %v, wantErr %v", err, true)
		}
	})

	t.Run("invalid role", func(t *testing.T) {
		cm := &corev1.ConfigMap{
			Data: map[string]string{
				"test": `{"name":"test","role":"abc","rules":[{"categories":["all"],"disallowedCommands":["acl","flushall","flushdb","keys"],"keyPatterns":["*"]}]}`,
			},
		}
		_, err := LoadACLUsers(ctx, clientset, cm)
		if err == nil {
			t.Errorf("LoadACLUsers() error = %v, wantErr %v", err, true)
		}
	})

	t.Run("invalid ConfigMap", func(t *testing.T) {
		cm := &corev1.ConfigMap{
			Data: map[string]string{
				"default": `invalid json`,
			},
		}
		_, err := LoadACLUsers(ctx, clientset, cm)
		if err == nil {
			t.Errorf("LoadACLUsers() error = %v, wantErr %v", err, true)
		}
	})
}

func TestLoadACLUsersPasswordChecks(t *testing.T) {
	const (
		ns    = "ns"
		hash1 = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
		hash2 = "60303ae22b998861bce3b28f33eec1be758a213c86c93c076dbe9f558c11c752"
		rules = `"rules":[{"categories":["all"],"keyPatterns":["*"]}]`
	)
	ctx := context.TODO()
	newClient := func(t *testing.T, secrets map[string]map[string]string) *mocks.ClientSet {
		clientset := mocks.NewClientSet(t)
		for name, data := range secrets {
			s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}, Data: map[string][]byte{}}
			for k, v := range data {
				s.Data[k] = []byte(v)
			}
			clientset.On("GetSecret", ctx, ns, name).Return(s, nil).Maybe()
		}
		return clientset
	}
	cm := func(data map[string]string) *corev1.ConfigMap {
		return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "acl", Namespace: ns}, Data: data}
	}

	t.Run("operator entry without secret reference fails", func(t *testing.T) {
		// an older configmap that already lost the operator's secret reference
		for name, entry := range map[string]string{
			"no password field": `{"name":"operator","role":"Operator",` + rules + `}`,
			"empty secret name": `{"name":"operator","role":"Operator","password":{},` + rules + `}`,
		} {
			if _, err := LoadACLUsers(ctx, newClient(t, nil), cm(map[string]string{"operator": entry})); err == nil {
				t.Errorf("%s: expected an error", name)
			}
		}
	})

	t.Run("default user with unusable secret fails", func(t *testing.T) {
		for name, data := range map[string]map[string]string{
			"malformed hash":  {"password-sha256": "abc"},
			"no password key": {"username": "default"},
			"empty password":  {"password": ""},
		} {
			clientset := newClient(t, map[string]map[string]string{"def-pw": data})
			entry := `{"name":"default","role":"Developer","password":{"secretName":"def-pw"},` + rules + `}`
			if _, err := LoadACLUsers(ctx, clientset, cm(map[string]string{"default": entry})); err == nil {
				t.Errorf("%s: expected an error", name)
			}
		}
	})

	t.Run("application user with unusable secret keeps its reference", func(t *testing.T) {
		clientset := newClient(t, map[string]map[string]string{"app-pw": {"password-sha256": "abc"}})
		entry := `{"name":"app","role":"Developer","password":{"secretName":"app-pw"},` + rules + `}`
		users, err := LoadACLUsers(ctx, clientset, cm(map[string]string{"app": entry}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(users) != 1 || users[0].Password.GetSecretName() != "app-pw" {
			t.Errorf("app user must keep its secret reference, got %+v", users)
		}
	})

	t.Run("every password secret is loaded", func(t *testing.T) {
		clientset := newClient(t, map[string]map[string]string{
			"app-pw-1": {"password-sha256": hash1},
			"app-pw-2": {"password-sha256": hash2},
		})
		entry := `{"name":"app","role":"Developer","password":{"secretName":"app-pw-1"},"additionalPasswords":[{"secretName":"app-pw-2"}],` + rules + `}`
		users, err := LoadACLUsers(ctx, clientset, cm(map[string]string{"app": entry}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := users[0].PasswordRules()
		if len(got) != 2 || got[0] != "#"+hash1 || got[1] != "#"+hash2 {
			t.Errorf("PasswordRules() = %v, want both hashes", got)
		}
		// and writing it back keeps both references
		if encoded := users.Encode(false)["app"]; !strings.Contains(encoded, "app-pw-1") || !strings.Contains(encoded, "app-pw-2") {
			t.Errorf("encoded entry %s lost a secret reference", encoded)
		}
	})
}
