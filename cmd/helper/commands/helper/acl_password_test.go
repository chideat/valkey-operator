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
