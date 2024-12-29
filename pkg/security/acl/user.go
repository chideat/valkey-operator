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
	"encoding/json"
	"fmt"

	"sort"

	"github.com/chideat/valkey-operator/pkg/kubernetes"
	security "github.com/chideat/valkey-operator/pkg/security/password"
	"github.com/chideat/valkey-operator/pkg/types/user"
	core "github.com/chideat/valkey-operator/pkg/types/user"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LoadACLUsers load acls from configmap
func LoadACLUsers(ctx context.Context, clientset kubernetes.ClientSet, cm *corev1.ConfigMap) (Users, error) {
	users := Users{}
	if cm == nil {
		return users, nil
	}
	for name, userData := range cm.Data {
		if name == "" {
			name = core.DefaultUserName
		}

		var user core.User
		if err := json.Unmarshal([]byte(userData), &user); err != nil {
			return nil, fmt.Errorf("parse user %s failed, error=%s", name, err)
		}
		if user.Password != nil && user.Password.SecretName != "" {
			if secret, err := clientset.GetSecret(ctx, cm.Namespace, user.Password.SecretName); err != nil {
				return nil, err
			} else {
				user.Password, _ = core.NewPassword(secret)
			}
		}
		user.Name = name

		if err := user.Validate(); err != nil {
			return nil, fmt.Errorf(`user "%s" is invalid, %s`, user.Name, err)
		}
		users = append(users, &user)
	}
	return users, nil
}

func NewOperatorUser(ctx context.Context, clientset kubernetes.ClientSet, secretName, namespace string, ownerRefs []metav1.OwnerReference, ACL2Support bool) (*core.User, error) {
	// get secret
	oldSecret, _ := clientset.GetSecret(ctx, namespace, secretName)
	if oldSecret != nil {
		if data, ok := oldSecret.Data["password"]; ok && len(data) != 0 {
			return core.NewOperatorUser(oldSecret, ACL2Support)
		}
	}

	plainPasswd, err := security.GeneratePassword(security.MaxPasswordLen)
	if err != nil {
		return nil, fmt.Errorf("generate password for operator user failed, error=%s", err)
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            secretName,
			Namespace:       namespace,
			OwnerReferences: ownerRefs,
		},
		Data: map[string][]byte{
			"password": []byte(plainPasswd),
			"username": []byte(user.DefaultOperatorUserName),
		},
	}
	if err := clientset.CreateIfNotExistsSecret(ctx, namespace, &secret); err != nil {
		return nil, fmt.Errorf("generate password for operator failed, error=%s", err)
	}
	return core.NewOperatorUser(&secret, ACL2Support)
}

// Users
type Users []*core.User

// GetByRole
func (us Users) GetOpUser() *core.User {
	if us == nil {
		return nil
	}

	var commonUser *core.User
	for _, u := range us {
		if u.Role == core.RoleOperator {
			return u
		} else if u.Role == core.RoleDeveloper && commonUser == nil {
			commonUser = u
		}
	}
	return commonUser
}

// GetDefaultUser
func (us Users) GetDefaultUser() *core.User {
	for _, u := range us {
		if u.Name == "" || u.Name == core.DefaultUserName {
			return u
		}
	}
	return nil
}

// Encode
func (us Users) Encode(patch bool) map[string]string {
	ret := map[string]string{}
	for _, u := range us {
		if len(u.Rules) == 0 {
			u.Rules = append(u.Rules, &user.Rule{})
		}
		if patch {
			u.Rules[0] = user.PatchRedisClusterClientRequiredRules(u.Rules[0])
		}
		data, _ := json.Marshal(u)
		ret[u.Name] = string(data)
	}
	return ret
}

// IsChanged
func (us Users) IsChanged(n Users) bool {
	if len(us) != len(n) {
		return true
	}
	sort.SliceStable(us, func(i, j int) bool {
		return us[i].Name < us[j].Name
	})
	sort.SliceStable(n, func(i, j int) bool {
		return n[i].Name < n[j].Name
	})

	for i, oldUser := range us {
		newUser := n[i]
		if newUser.String() != oldUser.String() || newUser.Password.String() != oldUser.Password.String() {
			return true
		}
	}
	return false
}
