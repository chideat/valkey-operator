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
	"encoding/json"
	"fmt"

	"github.com/chideat/valkey-operator/pkg/kubernetes"
	security "github.com/chideat/valkey-operator/pkg/security/password"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/types/user"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LoadACLUsers load acls from configmap
func LoadACLUsers(ctx context.Context, clientset kubernetes.ClientSet, cm *corev1.ConfigMap) (types.Users, error) {
	users := types.Users{}
	if cm == nil {
		return users, nil
	}
	for name, userData := range cm.Data {
		if name == "" {
			name = user.DefaultUserName
		}

		var u user.User
		if err := json.Unmarshal([]byte(userData), &u); err != nil {
			return nil, fmt.Errorf("parse user %s failed, error=%s", name, err)
		}
		if u.Password != nil && u.Password.SecretName != "" {
			if secret, err := clientset.GetSecret(ctx, cm.Namespace, u.Password.SecretName); err != nil {
				return nil, err
			} else {
				u.Password, _ = user.NewPassword(secret)
			}
		}
		u.Name = name

		if err := u.Validate(); err != nil {
			return nil, fmt.Errorf(`user "%s" is invalid, %s`, u.Name, err)
		}
		users = append(users, &u)
	}
	return users, nil
}

func NewOperatorUser(ctx context.Context, clientset kubernetes.ClientSet, secretName, namespace string, ownerRefs []metav1.OwnerReference) (*user.User, error) {
	// get secret
	oldSecret, _ := clientset.GetSecret(ctx, namespace, secretName)
	if oldSecret != nil {
		if data, ok := oldSecret.Data["password"]; ok && len(data) != 0 {
			return types.NewOperatorUser(oldSecret)
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
	return types.NewOperatorUser(&secret)
}
