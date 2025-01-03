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
		secretName := "redis-sen-customport-sm8r5"
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
				"default": `{"name":"default","role":"Developer","password":{"secretName":"redis-sen-customport-sm8r5"},"rules":[{"categories":["all"],"disallowedCommands":["acl","flushall","flushdb","keys"],"keyPatterns":["*"]}]}`,
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
