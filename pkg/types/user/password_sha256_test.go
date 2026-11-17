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
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func newSecret(data map[string]string) *v1.Secret {
	s := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "pw"}, Data: map[string][]byte{}}
	for k, v := range data {
		s.Data[k] = []byte(v)
	}
	return s
}

func TestPassword_SHA256(t *testing.T) {
	hash := sha256Hex("Str0ng!Passw0rd")

	t.Run("hash only", func(t *testing.T) {
		p, err := NewPassword(newSecret(map[string]string{PasswordSHA256SecretKey: hash}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !p.IsHashed() {
			t.Error("expected hashed password")
		}
		if p.String() != "" {
			t.Errorf("String() must not expose anything for a hashed password, got %q", p.String())
		}
		if got := p.ACLRule(); got != "#"+hash {
			t.Errorf("ACLRule() = %q, want %q", got, "#"+hash)
		}
		if p.GetSecretName() != "pw" {
			t.Errorf("GetSecretName() = %q", p.GetSecretName())
		}
	})

	t.Run("hash is normalized", func(t *testing.T) {
		p, err := NewPassword(newSecret(map[string]string{PasswordSHA256SecretKey: " " + strings.ToUpper(hash) + "\n"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := p.ACLRule(); got != "#"+hash {
			t.Errorf("ACLRule() = %q, want %q", got, "#"+hash)
		}
	})

	t.Run("plaintext keeps working", func(t *testing.T) {
		p, err := NewPassword(newSecret(map[string]string{PasswordSecretKey: "Str0ng!Passw0rd"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.IsHashed() || p.ACLRule() != ">Str0ng!Passw0rd" || p.String() != "Str0ng!Passw0rd" {
			t.Errorf("unexpected plaintext password %+v", p)
		}
	})

	for name, data := range map[string]map[string]string{
		"both keys":     {PasswordSecretKey: "Str0ng!Passw0rd", PasswordSHA256SecretKey: hash},
		"short hash":    {PasswordSHA256SecretKey: hash[:63]},
		"not hex":       {PasswordSHA256SecretKey: strings.Repeat("z", 64)},
		"empty hash":    {PasswordSHA256SecretKey: ""},
		"no known keys": {"username": "foo"},
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			if _, err := NewPassword(newSecret(data)); err == nil {
				t.Error("expected error")
			}
		})
	}

	t.Run("nil password", func(t *testing.T) {
		var p *Password
		if p.IsHashed() || p.ACLRule() != "" {
			t.Error("nil password must have no rule")
		}
	})
}

func TestValidatePasswordSecret(t *testing.T) {
	hash := sha256Hex("anything")
	cases := []struct {
		name    string
		data    map[string]string
		wantErr bool
	}{
		{"valid hash", map[string]string{PasswordSHA256SecretKey: hash}, false},
		{"hash with empty password key", map[string]string{PasswordSHA256SecretKey: hash, PasswordSecretKey: ""}, false},
		{"invalid hash", map[string]string{PasswordSHA256SecretKey: "abc"}, true},
		{"both keys", map[string]string{PasswordSHA256SecretKey: hash, PasswordSecretKey: "Str0ng!Passw0rd"}, true},
		{"strong plaintext", map[string]string{PasswordSecretKey: "Str0ng!Passw0rd"}, false},
		{"weak plaintext", map[string]string{PasswordSecretKey: "weak"}, true},
		{"missing", map[string]string{}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidatePasswordSecret(newSecret(c.data))
			if (err != nil) != c.wantErr {
				t.Errorf("ValidatePasswordSecret() error = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}
