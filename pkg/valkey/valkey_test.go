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
package valkey

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gomodule/redigo/redis"
)

func TestParseAddress(t *testing.T) {
	testCases := []struct {
		input          string
		expectedIP     string
		expectedPort   int
		exportedString string
		expectError    bool
	}{
		{"192.168.1.1:8080", "192.168.1.1", 8080, "192.168.1.1:8080", false},
		{"1335::172:168:200:5d1:32428", "1335::172:168:200:5d1", 32428, "[1335::172:168:200:5d1]:32428", false},
		{"::1:6379", "::1", 6379, "[::1]:6379", false},
		{":6379", "", 6379, ":6379", false},
		{"localhost:6379", "localhost", 6379, "localhost:6379", false},
		{"invalid-ip:port", "", 0, "", true},
		{"invalid-ip", "", 0, "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			addr := Address(tc.input)
			if _, _, err := addr.parse(); err != nil {
				if !tc.expectError {
					t.Errorf("Expected an error, but got nil")
				}
			} else {
				if addr.Host() != tc.expectedIP {
					t.Errorf("Expected IP: %s, got: %s", tc.expectedIP, addr.Host())
				}
				if addr.Port() != tc.expectedPort {
					t.Errorf("Expected port: %d, got: %d", tc.expectedPort, addr.Port())
				}
				if addr.String() != tc.exportedString {
					t.Errorf("Expected string: %s, got: %s", tc.exportedString, addr.String())
				}
			}
		})
	}
}

func TestSentinelMonitorNode_Address(t *testing.T) {
	type args struct {
		Node *SentinelMonitorNode
	}

	testCases := []struct {
		name      string
		args      args
		wantError bool
		want      string
	}{
		{
			name:      "empty",
			args:      args{},
			wantError: false,
			want:      "",
		},
		{
			name: "ipv4",
			args: args{
				Node: &SentinelMonitorNode{
					IP:   "192.168.1.1",
					Port: "6379",
				},
			},
			wantError: false,
			want:      "192.168.1.1:6379",
		},
		{
			name: "ipv6",
			args: args{
				Node: &SentinelMonitorNode{
					IP:   "::1",
					Port: "6379",
				},
			},
			wantError: false,
			want:      "[::1]:6379",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			addr := tc.args.Node.Address()
			if addr != tc.want {
				t.Errorf("Expected: %s, got: %s", tc.want, addr)
			}
		})
	}
}

func TestSentinelMonitorNode_IsMaster(t *testing.T) {
	type args struct {
		Node *SentinelMonitorNode
	}

	testCases := []struct {
		name      string
		args      args
		wantError bool
		want      bool
	}{
		{
			name:      "empty",
			args:      args{},
			wantError: false,
		},
		{
			name: "is-master",
			args: args{
				Node: &SentinelMonitorNode{
					Flags: "master",
				},
			},
			wantError: false,
			want:      true,
		},
		{
			name: "master down",
			args: args{
				Node: &SentinelMonitorNode{
					Flags: "s_down,master",
				},
			},
			wantError: false,
			want:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.args.Node.IsMaster() != tc.want {
				t.Errorf("Expected: %t, got: %t", tc.want, tc.args.Node.IsMaster())
			}
		})
	}
}

func TestSentinelMonitorNode_IsFailovering(t *testing.T) {
	type args struct {
		Node *SentinelMonitorNode
	}

	testCases := []struct {
		name      string
		args      args
		wantError bool
		want      bool
	}{
		{
			name:      "empty",
			args:      args{},
			wantError: false,
		},
		{
			name: "is-failovering",
			args: args{
				Node: &SentinelMonitorNode{
					Flags: "master,failover_in_progress",
				},
			},
			wantError: false,
			want:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.args.Node.IsFailovering() != tc.want {
				t.Errorf("Expected: %t, got: %t", tc.want, tc.args.Node.IsFailovering())
			}
		})
	}
}

func TestNewValkeyClient(t *testing.T) {
	testCases := []struct {
		name     string
		addr     string
		authInfo AuthConfig
	}{
		{
			name: "No Auth",
			addr: "localhost:6379",
			authInfo: AuthConfig{
				Username: "",
				Password: "",
			},
		},
		{
			name: "With Password",
			addr: "localhost:6379",
			authInfo: AuthConfig{
				Username: "",
				Password: "password",
			},
		},
		{
			name: "With Username and Password",
			addr: "localhost:6379",
			authInfo: AuthConfig{
				Username: "user",
				Password: "password",
			},
		},
		{
			name: "With TLS",
			addr: "localhost:6379",
			authInfo: AuthConfig{
				Username:  "",
				Password:  "",
				TLSConfig: &tls.Config{},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewValkeyClient(tc.addr, tc.authInfo)
			if client == nil {
				t.Fatalf("Expected non-nil client")
			}

			valkeyClient, ok := client.(*valkeyClient)
			if !ok {
				t.Fatalf("Expected *valkeyClient type")
			}

			if valkeyClient.addr != tc.addr {
				t.Errorf("Expected addr: %s, got: %s", tc.addr, valkeyClient.addr)
			}

			if valkeyClient.authInfo.Username != tc.authInfo.Username {
				t.Errorf("Expected Username: %s, got: %s", tc.authInfo.Username, valkeyClient.authInfo.Username)
			}

			if valkeyClient.authInfo.Password != tc.authInfo.Password {
				t.Errorf("Expected Password: %s, got: %s", tc.authInfo.Password, valkeyClient.authInfo.Password)
			}

			if (valkeyClient.authInfo.TLSConfig == nil) != (tc.authInfo.TLSConfig == nil) {
				t.Errorf("Expected TLSConfig: %v, got: %v", tc.authInfo.TLSConfig, valkeyClient.authInfo.TLSConfig)
			}
		})
	}
}

func TestValkeyClient_Do(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("An error occurred while starting miniredis: %v", err)
	}
	defer s.Close()

	client := NewValkeyClient(s.Addr(), AuthConfig{})

	_, err = client.Do(context.Background(), "SET", "key", "value")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	val, err := redis.String(client.Do(context.Background(), "GET", "key"))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if val != "value" {
		t.Errorf("Expected value: %s, got: %s", "value", val)
	}
}

func TestValkeyClient_Ping(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("An error occurred while starting miniredis: %v", err)
	}
	defer s.Close()
	s.RequireUserAuth("user", "password")

	client := NewValkeyClient(s.Addr(), AuthConfig{
		Username: "user",
		Password: "password",
	})

	err = client.Ping(context.Background())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestValkeyClient_Info(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("An error occurred while starting miniredis: %v", err)
	}
	defer s.Close()

	s.RequireAuth("password")

	client := NewValkeyClient(s.Addr(), AuthConfig{
		Password: "password",
	})

	info, err := client.Info(context.Background())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if info == nil {
		t.Fatalf("Expected non-nil info")
	}
}

func generateSelfSignedCert() (certFile, keyFile string, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return "", "", err
	}

	certOut, err := os.CreateTemp("", "cert.pem")
	if err != nil {
		return "", "", err
	}
	defer certOut.Close()

	keyOut, err := os.CreateTemp("", "key.pem")
	if err != nil {
		return "", "", err
	}
	defer keyOut.Close()

	_ = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return "", "", err
	}
	_ = pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	return certOut.Name(), keyOut.Name(), nil
}

func TestValkeyClient_TLS(t *testing.T) {
	certFile, keyFile, err := generateSelfSignedCert()
	if err != nil {
		t.Fatalf("Failed to generate self-signed certificate: %v", err)
	}
	defer os.Remove(certFile)
	defer os.Remove(keyFile)

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("Failed to load X509 key pair: %v", err)
	}
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}

	s, err := miniredis.RunTLS(tlsConfig)
	if err != nil {
		t.Fatalf("An error occurred while starting miniredis: %v", err)
	}
	defer s.Close()

	s.RequireAuth("password")

	client := NewValkeyClient(s.Addr(), AuthConfig{
		Password:  "password",
		TLSConfig: tlsConfig,
	})

	info, err := client.Info(context.Background())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if info == nil {
		t.Fatalf("Expected non-nil Info")
	}
}
