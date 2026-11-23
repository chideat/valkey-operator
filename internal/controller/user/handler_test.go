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
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/pkg/kubernetes/clientset/mocks"
	"github.com/chideat/valkey-operator/pkg/types"
	tuser "github.com/chideat/valkey-operator/pkg/types/user"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A secret that gives no password must fail the User, never fall back to "nopass".
// The check runs before any Valkey node is touched, so only GetSecret is mocked:
// reaching any other client call would fail the test on an unexpected mock call.
func TestUserHandler_Do_secretWithoutPassword(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string][]byte
		wantErr string
	}{
		{name: "empty password", data: map[string][]byte{"password": {}}, wantErr: "secret app-pw has no password"},
		{name: "no password keys", data: map[string][]byte{"username": []byte("app")}, wantErr: "missing password or password-sha256"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mocks.NewClientSet(t)
			client.On("GetSecret", mock.Anything, "ns", "app-pw").Return(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "app-pw", Namespace: "ns"},
				Data:       tt.data,
			}, nil).Once()

			h := NewUserHandler(client, nil, logr.Discard())
			err := h.Do(context.Background(), v1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "ns"},
				Spec: v1alpha1.UserSpec{
					AccountType:     v1alpha1.CustomAccount,
					Arch:            core.ValkeyCluster,
					Username:        "app",
					InstanceName:    "my-cluster",
					PasswordSecrets: []string{"app-pw"},
					AclRules:        "+@all ~*",
				},
			}, logr.Discard())
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Do() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

// Every secret of spec.passwordSecrets must be kept for the ACL configmap, not only the
// last one, or a restart drops all but one password and breaks a password rotation.
func TestUserHandler_loadPasswords_keepsEverySecret(t *testing.T) {
	const (
		hash1 = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
		hash2 = "60303ae22b998861bce3b28f33eec1be758a213c86c93c076dbe9f558c11c752"
	)
	client := mocks.NewClientSet(t)
	for name, hash := range map[string]string{"app-pw-1": hash1, "app-pw-2": hash2} {
		client.On("GetSecret", mock.Anything, "ns", name).Return(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"},
			Data:       map[string][]byte{"password-sha256": []byte(hash)},
		}, nil).Once()
	}

	h := NewUserHandler(client, nil, logr.Discard())
	rules, first, others, err := h.loadPasswords(context.Background(), v1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "ns"},
		Spec: v1alpha1.UserSpec{
			Username:        "app",
			PasswordSecrets: []string{"app-pw-1", "app-pw-2"},
		},
	})
	if err != nil {
		t.Fatalf("loadPasswords() error = %v", err)
	}
	if len(rules) != 2 || rules[0] != "#"+hash1 || rules[1] != "#"+hash2 {
		t.Errorf("rules = %v, want both hashes in order", rules)
	}

	// the configmap entry built from them references both secrets after a round trip
	u, err := types.NewUserFromValkeyUser("app", "+@all ~*", first)
	if err != nil {
		t.Fatal(err)
	}
	u.AdditionalPasswords = others
	data, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	var decoded tuser.User
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range decoded.AllPasswords() {
		names = append(names, p.SecretName)
	}
	if strings.Join(names, ",") != "app-pw-1,app-pw-2" {
		t.Errorf("configmap entry %s references secrets %v, want app-pw-1,app-pw-2", data, names)
	}
}

// An ordinary User must not take the operator username: it would replace the
// operator's own credentials. Do fails before any client call.
func TestUserHandler_Do_reservedUsername(t *testing.T) {
	h := NewUserHandler(mocks.NewClientSet(t), nil, logr.Discard())
	err := h.Do(context.Background(), v1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "fake-operator", Namespace: "ns"},
		Spec: v1alpha1.UserSpec{
			AccountType:     v1alpha1.CustomAccount,
			Arch:            core.ValkeyCluster,
			Username:        "operator",
			InstanceName:    "my-cluster",
			PasswordSecrets: []string{"app-pw"},
			AclRules:        "+@all ~*",
		},
	}, logr.Discard())
	if err == nil || !strings.Contains(err.Error(), "reserved for the system account") {
		t.Fatalf("Do() error = %v, want the reserved username error", err)
	}

	if err := ValidateReservedUsername(&v1alpha1.User{Spec: v1alpha1.UserSpec{
		AccountType: v1alpha1.SystemAccount,
		Username:    "operator",
	}}); err != nil {
		t.Errorf("system account named operator: unexpected error %v", err)
	}
}
