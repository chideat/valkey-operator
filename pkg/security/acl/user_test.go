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

package acl

import (
	"context"
	"testing"

	"github.com/chideat/valkey-operator/pkg/kubernetes/clientset/mocks"
	core "github.com/chideat/valkey-operator/pkg/types/user"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
		if users[0].Name != core.DefaultUserName {
			t.Errorf("LoadACLUsers() = %v, want %v", users[0].Name, core.DefaultUserName)
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

func TestNewOperatorUser(t *testing.T) {
	ctx := context.TODO()
	ownerRefs := []metav1.OwnerReference{}

	t.Run("existing secret", func(t *testing.T) {
		namespace := "default"
		secretName := "test-secret"
		clientset := mocks.NewClientSet(t)
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"password": []byte("password"),
			},
		}
		clientset.On("CreateSecret", ctx, namespace, secret).Return(nil)
		clientset.On("GetSecret", ctx, namespace, secret.Name).Return(secret, nil)
		if err := clientset.CreateSecret(ctx, namespace, secret); err != nil {
			t.Errorf("CreateSecret() error = %v, wantErr %v", err, false)
		}
		user, err := NewOperatorUser(ctx, clientset, secretName, namespace, ownerRefs, false)
		if err != nil {
			t.Errorf("NewOperatorUser() error = %v, wantErr %v", err, false)
		}
		if user == nil {
			t.Errorf("NewOperatorUser() = %v, want non-nil user", user)
		}
	})

	t.Run("non-existing secret", func(t *testing.T) {
		clientset := mocks.NewClientSet(t)
		namespace := "default2"
		secretName := "test-secret2"
		statusErr := &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
		clientset.On("GetSecret", ctx, namespace, secretName).Return(nil, statusErr)
		clientset.On("CreateIfNotExistsSecret", ctx, namespace, mock.Anything).Return(nil)
		user, err := NewOperatorUser(ctx, clientset, secretName, namespace, ownerRefs, false)
		if err != nil {
			t.Errorf("NewOperatorUser() error = %v, wantErr %v", err, false)
		}
		if user == nil {
			t.Errorf("NewOperatorUser() = %v, want non-nil user", user)
		}
	})
}

func TestUsers_GetOpUser(t *testing.T) {
	t.Run("empty Users slice", func(t *testing.T) {
		users := Users{}
		if got := users.GetOpUser(); got != nil {
			t.Errorf("Users.GetOpUser() = %v, want %v", got, nil)
		}
	})

	t.Run("Users with operator", func(t *testing.T) {
		users := Users{
			&core.User{Name: "op", Role: core.RoleOperator},
		}
		if got := users.GetOpUser(); got == nil || got.Role != core.RoleOperator {
			t.Errorf("Users.GetOpUser() = %v, want %v", got, core.RoleOperator)
		}
	})

	t.Run("Users with only developers", func(t *testing.T) {
		users := Users{
			&core.User{Name: "dev", Role: core.RoleDeveloper},
		}
		if got := users.GetOpUser(); got == nil || got.Role != core.RoleDeveloper {
			t.Errorf("Users.GetOpUser() = %v, want %v", got, core.RoleDeveloper)
		}
	})
}

func TestUsers_GetDefaultUser(t *testing.T) {
	t.Run("empty Users slice", func(t *testing.T) {
		users := Users{}
		if got := users.GetDefaultUser(); got != nil {
			t.Errorf("Users.GetDefaultUser() = %v, want %v", got, nil)
		}
	})

	t.Run("Users with default user", func(t *testing.T) {
		users := Users{
			&core.User{Name: core.DefaultUserName, Role: core.RoleDeveloper},
		}
		if got := users.GetDefaultUser(); got == nil || got.Name != core.DefaultUserName {
			t.Errorf("Users.GetDefaultUser() = %v, want %v", got, core.DefaultUserName)
		}
	})

	t.Run("Users with no default user", func(t *testing.T) {
		users := Users{
			&core.User{Name: "custom", Role: core.RoleDeveloper},
		}
		if got := users.GetDefaultUser(); got != nil {
			t.Errorf("Users.GetDefaultUser() = %v, want %v", got, nil)
		}
	})
}

func TestUsers_Encode(t *testing.T) {
	t.Run("empty Users slice", func(t *testing.T) {
		users := Users{}
		if got := users.Encode(false); len(got) != 0 {
			t.Errorf("Users.Encode() = %v, want %v", got, map[string]string{})
		}
	})

	t.Run("Users with valid users", func(t *testing.T) {
		users := Users{
			&core.User{Name: "user1", Role: core.RoleDeveloper},
		}
		got := users.Encode(false)
		if len(got) != 1 {
			t.Errorf("Users.Encode() = %v, want %v", len(got), 1)
		}
	})
}

func TestUsers_IsChanged(t *testing.T) {
	t.Run("different lengths", func(t *testing.T) {
		users1 := Users{
			&core.User{Name: "user1", Role: core.RoleDeveloper},
		}
		users2 := Users{}
		if got := users1.IsChanged(users2); got != true {
			t.Errorf("Users.IsChanged() = %v, want %v", got, true)
		}
	})

	t.Run("identical Users slices", func(t *testing.T) {
		users1 := Users{
			&core.User{Name: "user1", Role: core.RoleDeveloper},
		}
		users2 := Users{
			&core.User{Name: "user1", Role: core.RoleDeveloper},
		}
		if got := users1.IsChanged(users2); got != false {
			t.Errorf("Users.IsChanged() = %v, want %v", got, false)
		}
	})

	t.Run("different Users slices", func(t *testing.T) {
		users1 := Users{
			&core.User{Name: "user1", Role: core.RoleDeveloper},
		}
		users2 := Users{
			&core.User{Name: "user2", Role: core.RoleDeveloper},
		}
		if got := users1.IsChanged(users2); got != true {
			t.Errorf("Users.IsChanged() = %v, want %v", got, true)
		}
	})
}
