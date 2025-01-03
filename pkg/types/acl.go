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

package types

import (
	"encoding/json"
	"slices"
	"sort"

	"github.com/chideat/valkey-operator/pkg/types/user"
	core "github.com/chideat/valkey-operator/pkg/types/user"
	v1 "k8s.io/api/core/v1"
)

func NewUserFromValkeyUser(username, ruleStr string, pwd *user.Password) (*user.User, error) {
	rules := []*user.Rule{}
	if ruleStr != "" {
		rule, err := user.NewRule(ruleStr)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	role := user.RoleDeveloper
	if username == user.DefaultOperatorUserName {
		role = user.RoleOperator
	}

	user := user.User{
		Name:     username,
		Role:     role,
		Rules:    rules,
		Password: pwd,
	}
	return &user, nil
}

// NewSentinelUser
func NewSentinelUser(name string, role user.UserRole, secret *v1.Secret) (*user.User, error) {
	var (
		err    error
		passwd *user.Password
	)
	if secret != nil {
		if passwd, err = user.NewPassword(secret); err != nil {
			return nil, err
		}
	}

	user := &user.User{
		Name:     name,
		Role:     role,
		Password: passwd,
	}
	if err := user.Validate(); err != nil {
		return nil, err
	}
	return user, nil
}

/*
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
*/

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
			u.Rules[0] = PatchClusterClientRequiredRules(u.Rules[0])
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

// NewOperatorUser
func NewOperatorUser(secret *v1.Secret, acl2Support bool) (*user.User, error) {
	rule := user.Rule{Categories: []string{"all"}, DisallowedCommands: []string{"keys"}, KeyPatterns: []string{"*"}}
	if acl2Support {
		rule.Channels = []string{"*"}
	}
	u := user.User{
		Name: user.DefaultOperatorUserName,
		Role: user.RoleOperator,
		Rules: []*user.Rule{
			&rule,
		},
	}
	if secret != nil {
		if passwd, err := user.NewPassword(secret); err != nil {
			return nil, err
		} else {
			u.Password = passwd
		}
	}
	return &u, nil
}

// Patch Valkey cluster client required rules
func PatchClusterClientRequiredRules(rule *user.Rule) *user.Rule {
	clusterRules := []string{
		"cluster|slots",
		"cluster|nodes",
		"cluster|info",
		"cluster|keyslot",
		"cluster|getkeysinslot",
		"cluster|countkeysinslot",
	}
	// remove required rules
	cmds := rule.DisallowedCommands
	rule.DisallowedCommands = rule.DisallowedCommands[:0]
	for _, cmd := range cmds {
		if slices.Contains(clusterRules, cmd) {
			continue
		} else {
			rule.DisallowedCommands = append(rule.DisallowedCommands, cmd)
		}
	}
	requiredRules := map[string]bool{}
	for _, cmd := range clusterRules {
		requiredRules[cmd] = false
		if rule.IsCommandEnabled(cmd, nil) {
			requiredRules[cmd] = true
		}
	}
	if rule.IsCommandEnabled("cluster", []string{"all", "admin", "slow", "dangerous"}) {
		for key := range requiredRules {
			requiredRules[key] = true
		}
	}
	for _, cmd := range clusterRules {
		if !requiredRules[cmd] {
			rule.AllowedCommands = append(rule.AllowedCommands, cmd)
		}
	}
	return rule
}

func PatchPubsubRules(rule *user.Rule) *user.Rule {
	if len(rule.Channels) > 0 {
		return rule
	}

	cmds := map[string][]string{
		"psubscribe":           {"all", "pubsub", "slow"},
		"publish":              {"all", "pubsub", "fast"},
		"pubsub":               {"all", "slow"},
		"pubsub|numpat":        {"all", "pubsub", "slow"},
		"pubsub|channels":      {"all", "pubsub", "slow"},
		"pubsub|numsub":        {"all", "pubsub", "slow"},
		"pubsub|shardnumsub":   {"all", "pubsub", "slow"},
		"pubsub|shardchannels": {"all", "pubsub", "slow"},
		"punsubscribe":         {"all", "pubsub", "slow"},
		"spublish":             {"all", "pubsub", "fast"},
		"ssubscribe":           {"all", "pubsub", "slow"},
		"subscribe":            {"all", "pubsub", "slow"},
		"sunsubscribe":         {"all", "pubsub", "slow"},
		"unsubscribe":          {"all", "pubsub", "slow"},
	}
	isAnyEnabled := false
	for cmd, cates := range cmds {
		if rule.IsCommandEnabled(cmd, cates) {
			isAnyEnabled = true
			break
		}
	}
	if isAnyEnabled {
		rule.Channels = append(rule.Channels, "*")
	}
	return rule
}
