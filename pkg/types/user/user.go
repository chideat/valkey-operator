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

package user

import (
	"fmt"
	"regexp"
	"strings"

	security "github.com/chideat/valkey-operator/pkg/security/password"
	v1 "k8s.io/api/core/v1"
)

const (
	// DefaultUserName from reids 6.0, there is a default user named "default"
	// for compatibility, the default user set as RoleDeveloper
	DefaultUserName         = "default"
	DefaultOperatorUserName = "operator"

	// password secret key name
	PasswordSecretKey = "password"
	// PasswordSHA256SecretKey is the secret key holding the hex SHA-256 of the
	// password instead of the password itself. The operator applies it with the
	// ACL "#<hash>" form, so the plaintext is never stored in the cluster.
	PasswordSHA256SecretKey = "password-sha256"
)

var passwordHashReg = regexp.MustCompile(`^[0-9a-f]{64}$`)

// NormalizePasswordHash trims and lowercases a hex SHA-256 digest and checks
// that it is exactly 64 hex characters, as Valkey requires for "#<hash>".
func NormalizePasswordHash(val string) (string, error) {
	hash := strings.ToLower(strings.TrimSpace(val))
	if !passwordHashReg.MatchString(hash) {
		return "", fmt.Errorf("%s must be a hex encoded SHA-256 digest (64 hex characters)", PasswordSHA256SecretKey)
	}
	return hash, nil
}

// ValidatePasswordSecret checks a user password secret. A secret holding
// password-sha256 is checked for a well-formed digest only: the password
// strength cannot be verified without the plaintext, so it is up to whoever
// generates the password.
func ValidatePasswordSecret(secret *v1.Secret) error {
	if secret == nil {
		return fmt.Errorf("nil password secret")
	}
	if hash, ok := secret.Data[PasswordSHA256SecretKey]; ok {
		if len(secret.Data[PasswordSecretKey]) > 0 {
			return fmt.Errorf("secret %s must set only one of %s and %s", secret.Name, PasswordSecretKey, PasswordSHA256SecretKey)
		}
		_, err := NormalizePasswordHash(string(hash))
		return err
	}
	return security.PasswordValidate(string(secret.Data[PasswordSecretKey]), 8, 32)
}

// ValidateOperatorPassword checks the password of the operator user. The operator
// authenticates against Valkey with it, so it must be a non-empty plaintext: a
// password-sha256 secret or an empty password would lock the operator out, or worse
// leave the operator user without a password ("nopass").
func ValidateOperatorPassword(p *Password) error {
	switch {
	case p == nil:
		return fmt.Errorf("operator user has no password")
	case p.IsHashed():
		return fmt.Errorf("operator user secret %s can not use %s", p.SecretName, PasswordSHA256SecretKey)
	case p.String() == "":
		return fmt.Errorf("operator user secret %s has an empty %s", p.SecretName, PasswordSecretKey)
	}
	return nil
}

// UserRole
type UserRole string

const (
	RoleOperator  UserRole = "Operator"
	RoleDeveloper UserRole = "Developer"
)

// NewUser
func NewUser(name string, role UserRole, secret *v1.Secret) (*User, error) {
	var (
		err    error
		passwd *Password
	)
	if secret != nil {
		if passwd, err = NewPassword(secret); err != nil {
			return nil, err
		}
	}
	Rules := []*Rule{{Categories: []string{"all"}, KeyPatterns: []string{"*"}, Channels: []string{"*"}}}
	if name == "" || name == DefaultUserName {
		name = DefaultUserName
		Rules = []*Rule{
			{
				Categories:         []string{"all"},
				KeyPatterns:        []string{"*"},
				DisallowedCommands: []string{"acl", "flushall", "flushdb", "keys"},
				Channels:           []string{"*"},
			},
		}
	}

	user := &User{
		Name:     name,
		Role:     role,
		Password: passwd,
		Rules:    Rules,
	}
	if err := user.Validate(); err != nil {
		return nil, err
	}
	return user, nil
}

// User
type User struct {
	Name     string    `json:"name"`
	Role     UserRole  `json:"role"`
	Password *Password `json:"password,omitempty"`
	Rules    []*Rule   `json:"rules,omitempty"`
}

var (
	usernameReg = regexp.MustCompile(`^[0-9a-zA-Z-]{0,31}$`)
)

func (u *User) GetPassword() *Password {
	if u == nil || u.Password == nil {
		return nil
	}
	return u.Password
}

// AppendRule
func (u *User) AppendRule(rules ...*Rule) error {
	if u == nil {
		return nil
	}
	u.Rules = append(u.Rules, rules...)
	return nil
}

func (u *User) Validate() error {
	if u == nil {
		return fmt.Errorf("nil user")
	}
	if !usernameReg.MatchString(u.Name) {
		return fmt.Errorf("invalid username which should match ^[0-9a-zA-Z-]{0,31}$")
	}

	if u.Role != RoleOperator && u.Role != RoleDeveloper {
		return fmt.Errorf(`unsupported user role "%s"`, u.Role)
	}
	return nil
}

// String
func (u *User) String() string {
	if u == nil {
		return ""
	}

	vals := []string{u.Name, string(u.Role)}
	for _, rule := range u.Rules {
		vals = append(vals, rule.Encode())
	}
	return strings.Join(vals, " ")
}

// Password
type Password struct {
	SecretName string `json:"secretName,omitempty"`
	secret     *v1.Secret
	data       string
	hash       string
}

// NewPassword
func NewPassword(secret *v1.Secret) (*Password, error) {
	if secret == nil {
		return nil, nil
	}

	p := Password{}
	if err := p.SetSecret(secret); err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *Password) GetSecretName() string {
	if p == nil {
		return ""
	}
	return p.SecretName
}

func (p *Password) SetSecret(secret *v1.Secret) error {
	if p == nil || secret == nil {
		return nil
	}

	plain, hasPlain := secret.Data[PasswordSecretKey]
	if hashVal, ok := secret.Data[PasswordSHA256SecretKey]; ok {
		if len(plain) > 0 {
			return fmt.Errorf("secret %s must set only one of %s and %s", secret.Name, PasswordSecretKey, PasswordSHA256SecretKey)
		}
		hash, err := NormalizePasswordHash(string(hashVal))
		if err != nil {
			return fmt.Errorf("secret %s: %w", secret.Name, err)
		}
		p.SecretName = secret.GetName()
		p.secret = secret
		p.data = ""
		p.hash = hash
		return nil
	}
	if !hasPlain {
		return fmt.Errorf("missing %s or %s field for secret %s", PasswordSecretKey, PasswordSHA256SecretKey, secret.Name)
	}
	p.SecretName = secret.GetName()
	p.secret = secret
	p.data = string(plain)
	p.hash = ""
	return nil
}

// IsHashed reports whether the password is only known by its SHA-256 digest.
func (p *Password) IsHashed() bool {
	return p != nil && p.hash != ""
}

// ACLRule returns the ACL SETUSER/ACL file argument that sets this password:
// "#<sha256>" for a hashed password, ">plaintext" otherwise, and "" when there
// is no password.
func (p *Password) ACLRule() string {
	switch {
	case p == nil:
		return ""
	case p.hash != "":
		return "#" + p.hash
	case p.data != "":
		return ">" + p.data
	}
	return ""
}

func (p *Password) Secret() *v1.Secret {
	if p == nil {
		return nil
	}
	return p.secret
}

// String return password in plaintext. It is empty for a hashed password, which
// therefore cannot be used to authenticate against Valkey.
func (p *Password) String() string {
	if p == nil {
		return ""
	}
	return p.data
}
