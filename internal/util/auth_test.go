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

package util

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestLoadCertConfigFromSecret(t *testing.T) {
	tests := []struct {
		name    string
		secret  *corev1.Secret
		wantErr bool
	}{
		{
			name:    "nil secret",
			secret:  nil,
			wantErr: true,
		},
		{
			name: "missing TLS cert",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					corev1.TLSPrivateKeyKey: []byte("private-key"),
					"ca.crt":                []byte("ca-cert"),
				},
			},
			wantErr: true,
		},
		{
			name: "missing TLS private key",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					corev1.TLSCertKey: []byte("cert"),
					"ca.crt":          []byte("ca-cert"),
				},
			},
			wantErr: true,
		},
		{
			name: "missing CA cert",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					corev1.TLSCertKey:       []byte("cert"),
					corev1.TLSPrivateKeyKey: []byte("private-key"),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid cert pair",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					corev1.TLSCertKey:       []byte("invalid-cert"),
					corev1.TLSPrivateKeyKey: []byte("invalid-key"),
					"ca.crt":                []byte("ca-cert"),
				},
			},
			wantErr: true,
		},
		{
			name: "valid cert pair",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					corev1.TLSCertKey: []byte(`-----BEGIN CERTIFICATE-----
MIICHjCCAcSgAwIBAgIIFPUFwnwIWrcwCgYIKoZIzj0EAwIwQTEWMBQGA1UEChMN
UmVkIEhhdCwgSW5jLjEnMCUGA1UEAxMeb2xtLXNlbGZzaWduZWQtM2U4Zjc5OThl
MDgzYzJmMB4XDTI1MDQxNjAyNTczNVoXDTI3MDQxNjAyNTczNVowRjEWMBQGA1UE
ChMNUmVkIEhhdCwgSW5jLjEsMCoGA1UEAxMjcmVkaXMtb3BlcmF0b3Itc2Vydmlj
ZS5yZWRpcy1zeXN0ZW0wWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAATkqH/R45pn
SafVofuyGBKFZND9Ml11P/Q+X/9BWatKXkvWTcusE5//tfyyGSKa0LwBRMO+PEsy
Qs3ERWbyPMepo4GgMIGdMBMGA1UdJQQMMAoGCCsGAQUFBwMBMAwGA1UdEwEB/wQC
MAAwHwYDVR0jBBgwFoAUxy729h/TnbNpEcDfZ5aEe/wD6J4wVwYDVR0RBFAwToIj
cmVkaXMtb3BlcmF0b3Itc2VydmljZS5yZWRpcy1zeXN0ZW2CJ3JlZGlzLW9wZXJh
dG9yLXNlcnZpY2UucmVkaXMtc3lzdGVtLnN2YzAKBggqhkjOPQQDAgNIADBFAiEA
7+UwIbLqbrZ0QyljHUp3L/DsGE7BlFAIpVD2pYTUTOsCIHBWJ2Tvp9XmNChAs2gt
88B76kLduA/X3Xo08KZEE3bz
-----END CERTIFICATE-----`),
					corev1.TLSPrivateKeyKey: []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIA6exAB1RReEeP3EXskWTeFsY9bS6CwpVw5grZgSA+UXoAoGCCqGSM49
AwEHoUQDQgAE5Kh/0eOaZ0mn1aH7shgShWTQ/TJddT/0Pl//QVmrSl5L1k3LrBOf
/7X8shkimtC8AUTDvjxLMkLNxEVm8jzHqQ==
-----END EC PRIVATE KEY-----`),
					"ca.crt": []byte(`-----BEGIN CERTIFICATE-----
MIIBujCCAWCgAwIBAgIIA+j3mY4IPC8wCgYIKoZIzj0EAwIwQTEWMBQGA1UEChMN
UmVkIEhhdCwgSW5jLjEnMCUGA1UEAxMeb2xtLXNlbGZzaWduZWQtM2U4Zjc5OThl
MDgzYzJmMB4XDTI1MDQxNjAyNTczNVoXDTI3MDQxNjAyNTczNVowQTEWMBQGA1UE
ChMNUmVkIEhhdCwgSW5jLjEnMCUGA1UEAxMeb2xtLXNlbGZzaWduZWQtM2U4Zjc5
OThlMDgzYzJmMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE+X6Mww3UD/fSKTkO
UA1TWcIpyhRNej2ksuaBQl+KCX+8D2SU7+BhMiLOC2t34kds/Qx36i9/fW6wOM4l
qDQpzaNCMEAwDgYDVR0PAQH/BAQDAgIEMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0O
BBYEFMcu9vYf052zaRHA32eWhHv8A+ieMAoGCCqGSM49BAMCA0gAMEUCIQDnRs4W
mDmvHyp48EldBQgnnrFHVX/7Es9bl8wI16SzYQIgG2sjF2Y+9wlrg1DF8MQkETcX
uNf5eaXeMKcK1q1C7Ig=
-----END CERTIFICATE-----`),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := LoadCertConfigFromSecret(tt.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadCertConfigFromSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && config == nil {
				t.Errorf("LoadCertConfigFromSecret() returned nil config but expected a valid config")
			}
		})
	}
}

func TestAuthCheckRule(t *testing.T) {
	tests := []struct {
		name     string
		aclRules string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid rule with allowed groups",
			aclRules: "allkeys +@keyspace +@read +@write",
			wantErr:  false,
		},
		{
			name:     "invalid group",
			aclRules: "allkeys +@example",
			wantErr:  true,
			errMsg:   "acl rule group example is not allowed",
		},
		{
			name:     "duplicated group",
			aclRules: "allkeys +@read -@read",
			wantErr:  true,
			errMsg:   "acl rule group read is duplicated",
		},
		{
			name:     "not allowed 'on' rule",
			aclRules: "allkeys +@read on",
			wantErr:  true,
			errMsg:   "acl rule on is not allowed",
		},
		{
			name:     "not allowed 'nopass' rule",
			aclRules: "allkeys +@read nopass",
			wantErr:  true,
			errMsg:   "acl rule nopass is not allowed",
		},
		{
			name:     "invalid command rule",
			aclRules: "allkeys invalidcommand",
			wantErr:  true,
			errMsg:   "acl rule invalidcommand is not allowed",
		},
		{
			name:     "not allowed password rule with >",
			aclRules: "allkeys >password",
			wantErr:  true,
			errMsg:   "acl password rule >password is not allowed",
		},
		{
			name:     "not allowed password rule with <",
			aclRules: "allkeys <password",
			wantErr:  true,
			errMsg:   "acl password rule <password is not allowed",
		},
		{
			name:     "valid rules with channel pattern",
			aclRules: "allkeys +@keyspace +@read +@write %*",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckRule(tt.aclRules)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && err.Error() != fmt.Errorf("%s", tt.errMsg).Error() {
				t.Errorf("CheckRule() error = %v, wantErr %v", err, tt.errMsg)
			}
		})
	}
}

func TestAuthCheckUserRuleUpdate(t *testing.T) {
	tests := []struct {
		name    string
		rule    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "forbidden +acl rule",
			rule:    "+acl",
			wantErr: true,
			errMsg:  "acl rule +acl is not invalid",
		},
		{
			name:    "dangerous permission without -acl",
			rule:    "+@admin",
			wantErr: true,
			errMsg:  "acl rule +@admin include acl command,need add '-acl' rules",
		},
		{
			name:    "slow permission without -acl",
			rule:    "+@slow",
			wantErr: true,
			errMsg:  "acl rule +@slow include acl command,need add '-acl' rules",
		},
		{
			name:    "all permission without -acl",
			rule:    "+@all",
			wantErr: true,
			errMsg:  "acl rule +@all include acl command,need add '-acl' rules",
		},
		{
			name:    "acl prefix command",
			rule:    "+acl|set",
			wantErr: true,
			errMsg:  "acl rule +acl|set include acl command,need add '-acl' rules",
		},
		{
			name:    "allowed rule with -acl",
			rule:    "+@all -acl",
			wantErr: false,
		},
		{
			name:    "read only permissions",
			rule:    "+@read -@write",
			wantErr: false,
		},
		{
			name:    "default safe rule",
			rule:    "allkeys +@all -acl -flushall -flushdb -keys",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckUserRuleUpdate(tt.rule)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckUserRuleUpdate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && err.Error() != fmt.Errorf("%s", tt.errMsg).Error() {
				t.Errorf("CheckUserRuleUpdate() error = %v, wantErr %v", err, tt.errMsg)
			}
		})
	}
}
