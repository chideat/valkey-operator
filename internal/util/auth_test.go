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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

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

// issueTestCert mints a CA and a leaf signed by it, PEM-encoded as they appear
// in an instance TLS secret.
func issueTestCert(t *testing.T, dnsName string) (caPEM, certPEM, keyPEM []byte) {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ca key: %v", err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create ca: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse ca: %v", err)
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: dnsName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:     []string{dnsName},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create leaf: %v", err)
	}
	leafKeyDER, err := x509.MarshalECPrivateKey(leafKey)
	if err != nil {
		t.Fatalf("marshal leaf key: %v", err)
	}

	caPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: leafKeyDER})
	return caPEM, certPEM, keyPEM
}

func tlsSecret(caPEM, certPEM, keyPEM []byte) *corev1.Secret {
	return &corev1.Secret{Data: map[string][]byte{
		corev1.TLSCertKey:       certPEM,
		corev1.TLSPrivateKeyKey: keyPEM,
		"ca.crt":                caPEM,
	}}
}

// handshake dials a TLS server presenting serverCert using clientConf.
func handshake(t *testing.T, clientConf *tls.Config, serverCert tls.Certificate) error {
	t.Helper()

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{serverCert},
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		if conn, err := ln.Accept(); err == nil {
			_ = conn.(*tls.Conn).Handshake()
			conn.Close()
		}
	}()

	// Dial the loopback address, never the certificate's DNS name: this is what
	// the operator does when it connects to a pod IP.
	conf := clientConf.Clone()
	conn, err := tls.Dial("tcp", ln.Addr().String(), conf)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Handshake()
}

// TestLoadCertConfigFromSecretVerifiesTheChain pins the security property this
// config is supposed to have. It previously set InsecureSkipVerify, so RootCAs
// was dead weight and a certificate from any issuer was accepted.
func TestLoadCertConfigFromSecretVerifiesTheChain(t *testing.T) {
	const dnsName = "drc-test-0.drc-test.default"

	caPEM, certPEM, keyPEM := issueTestCert(t, dnsName)
	conf, err := LoadCertConfigFromSecret(tlsSecret(caPEM, certPEM, keyPEM))
	if err != nil {
		t.Fatalf("LoadCertConfigFromSecret: %v", err)
	}

	if conf.InsecureSkipVerify {
		t.Error("InsecureSkipVerify must stay off; it accepts any certificate")
	}
	if conf.RootCAs == nil {
		t.Error("RootCAs must be populated from the instance CA")
	}
	if conf.ServerName != dnsName {
		t.Errorf("ServerName = %q, want %q taken from the certificate itself", conf.ServerName, dnsName)
	}

	t.Run("accepts the instance certificate over a connection dialled by IP", func(t *testing.T) {
		serverCert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			t.Fatalf("server keypair: %v", err)
		}
		if err := handshake(t, conf, serverCert); err != nil {
			t.Fatalf("handshake against the instance certificate failed: %v", err)
		}
	})

	t.Run("rejects a certificate from a different CA", func(t *testing.T) {
		// Same DNS name, different issuer: what a MITM on the pod network would
		// present. Under InsecureSkipVerify this succeeded.
		_, foreignCertPEM, foreignKeyPEM := issueTestCert(t, dnsName)
		foreignCert, err := tls.X509KeyPair(foreignCertPEM, foreignKeyPEM)
		if err != nil {
			t.Fatalf("foreign keypair: %v", err)
		}
		if err := handshake(t, conf, foreignCert); err == nil {
			t.Fatal("handshake succeeded against a certificate this CA never issued")
		}
	})
}
