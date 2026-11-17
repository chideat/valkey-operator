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
	"strings"
	"testing"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/pkg/kubernetes/clientset/mocks"
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
