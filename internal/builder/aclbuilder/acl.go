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

package aclbuilder

import (
	"fmt"
	"strings"

	"github.com/chideat/valkey-operator/api/core"
	v1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/util"
	security "github.com/chideat/valkey-operator/pkg/security/password"
	"github.com/chideat/valkey-operator/pkg/types"
	"github.com/chideat/valkey-operator/pkg/types/user"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateACLConfigMapName(arch core.Arch, name string) string {
	return fmt.Sprintf("%s-acl-%s", builder.ResourcePrefix(arch), name)
}

func GenerateACLOperatorSecretName(arch core.Arch, name string) string {
	return fmt.Sprintf("%s-acl-%s-%s-secret", arch, name, user.DefaultOperatorUserName)
}

func GenerateUserResourceName(arch core.Arch, instName, name string) string {
	return fmt.Sprintf("%s-acl-%s-%s", arch, instName, name)
}

func GenerateOperatorUserResourceName(arch core.Arch, name string) string {
	return GenerateUserResourceName(arch, name, user.DefaultOperatorUserName)
}

func GenerateDefaultUserResourceName(arch core.Arch, name string) string {
	return GenerateUserResourceName(arch, name, user.DefaultUserName)
}

func GenerateOperatorsUser(inst types.Instance, passwordsecret string) v1alpha1.User {
	passwordsecrets := []string{}
	if passwordsecret != "" {
		passwordsecrets = append(passwordsecrets, passwordsecret)
	}

	// NOTE: disable flushall and flushdb for security for inner system user.
	// If want to truncate all data, user should create a custom user with required permission.
	rule := "+@all -flushall -flushdb ~* &*"
	return v1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GenerateOperatorUserResourceName(inst.Arch(), inst.GetName()),
			Namespace: inst.GetNamespace(),
		},
		Spec: v1alpha1.UserSpec{
			AccountType:     v1alpha1.SystemAccount,
			Arch:            inst.Arch(),
			InstanceName:    inst.GetName(),
			Username:        "operator",
			PasswordSecrets: passwordsecrets,
			AclRules:        rule,
		},
	}
}

func GenerateUser(inst types.Instance, u *user.User) *v1alpha1.User {
	var (
		name            = GenerateUserResourceName(inst.Arch(), inst.GetName(), u.Name)
		accountType     v1alpha1.AccountType
		passwordSecrets = []string{}
	)
	switch u.Role {
	case user.RoleOperator:
		accountType = v1alpha1.SystemAccount
	default:
		accountType = v1alpha1.CustomAccount
	}
	if u.GetPassword().GetSecretName() != "" {
		passwordSecrets = append(passwordSecrets, u.GetPassword().GetSecretName())
	}
	var rules []string
	for _, rule := range u.Rules {
		rules = append(rules, rule.Encode())
	}

	return &v1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   inst.GetNamespace(),
			Annotations: map[string]string{},
			Labels:      map[string]string{},
		},
		Spec: v1alpha1.UserSpec{
			AccountType:     accountType,
			Arch:            inst.Arch(),
			InstanceName:    inst.GetName(),
			Username:        u.Name,
			PasswordSecrets: passwordSecrets,
			AclRules:        strings.Join(rules, " "),
		},
	}
}

func GenerateOperatorSecret(inst types.Instance) *corev1.Secret {
	randPassword, _ := security.GeneratePassword(security.MaxPasswordLen)

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            GenerateACLOperatorSecretName(inst.Arch(), inst.GetName()),
			Namespace:       inst.GetNamespace(),
			OwnerReferences: inst.GetOwnerReferences(),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"password": []byte(randPassword),
			"username": []byte("operator"),
		},
	}
}

func GenerateACLConfigMap(inst types.Instance, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            GenerateACLConfigMapName(inst.Arch(), inst.GetName()),
			Namespace:       inst.GetNamespace(),
			OwnerReferences: util.BuildOwnerReferences(inst),
		},
		Data: data,
	}
}
