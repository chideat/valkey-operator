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

package e2e

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/chideat/valkey-operator/api/core"
	rdsv1alpha1 "github.com/chideat/valkey-operator/api/rds/v1alpha1"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder/certbuilder"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TLS coverage exists because every part of it has been broken in a shipped
// release without any test noticing:
//
//   - the operator never registered the cert-manager types in its scheme, so an
//     instance with access.enableTLS could not get past Initializing — the
//     Certificate it needed to create could not even be serialised;
//   - the config used to reach the nodes set InsecureSkipVerify, so the CA in
//     the secret was never consulted and any certificate at all was accepted.
//
// Neither is visible from a non-TLS instance, which is why the rest of the
// suite stayed green throughout. These cases deploy a real TLS instance against
// a real cert-manager CA and then verify a chain through the operator's own
// helper, so both failures surface here.

const (
	// tlsSelfSignedIssuerName bootstraps the CA below; nothing else uses it.
	tlsSelfSignedIssuerName = "valkey-e2e-selfsigned"
	// tlsCAIssuerName is the issuer the TLS instances reference. It is a real
	// CA rather than a self-signed issuer so the issued certificates carry a
	// chain to a separate root — the arrangement a user actually has, and the
	// one that makes chain verification meaningful.
	tlsCAIssuerName = "valkey-e2e-ca"
)

// tlsIssuerKind reports which cert-manager issuer kind the TLS cases use.
//
// A namespaced Issuer is the default because it keeps every artifact these
// cases create inside the test namespace. A CA ClusterIssuer would instead have
// to keep its signing secret in cert-manager's cluster resource namespace,
// which on a cluster where cert-manager is part of the platform is a namespace
// the platform owns and these tests have no business writing into. Set
// TLS_TEST_ISSUER_KIND=ClusterIssuer to exercise that path where the cluster is
// disposable.
func tlsIssuerKind() string {
	if k := utils.GetEnv("TLS_TEST_ISSUER_KIND"); k != "" {
		return k
	}
	return "Issuer"
}

// tlsIssuerNamespace is where the CA certificate and its signing secret live.
// A CA Issuer reads its secret from its own namespace; a CA ClusterIssuer reads
// it from cert-manager's cluster resource namespace, which CERT_MANAGER_NAMESPACE
// must match.
func tlsIssuerNamespace() string {
	if tlsIssuerKind() == "ClusterIssuer" {
		return utils.GetEnv("CERT_MANAGER_NAMESPACE", "cert-manager")
	}
	return testNamespace
}

// tlsTestVersion picks the valkey version the TLS cases run against.
//
// TLS wiring is independent of the valkey version but costs a full instance
// rollout per combination, so these cases are pinned to one version instead of
// the whole matrix. They still run against every access mode: ClusterIP and
// NodePort reach the nodes through different addresses, and that is the axis
// TLS is actually sensitive to.
func tlsTestVersion() string {
	if v := utils.GetEnv("TLS_TEST_VERSION"); v != "" {
		return v
	}
	if len(supportedVersions) == 0 {
		return ""
	}
	return supportedVersions[len(supportedVersions)-1]
}

// createTLSIssuer creates an issuer of the configured kind, ignoring an issuer
// a previous run left behind.
func createTLSIssuer(ctx context.Context, name string, config certv1.IssuerConfig) {
	var obj client.Object
	if tlsIssuerKind() == "ClusterIssuer" {
		obj = &certv1.ClusterIssuer{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       certv1.IssuerSpec{IssuerConfig: config},
		}
	} else {
		obj = &certv1.Issuer{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
			Spec:       certv1.IssuerSpec{IssuerConfig: config},
		}
	}
	if err := k8sClient.Create(ctx, obj); err != nil && !errors.IsAlreadyExists(err) {
		Expect(err).To(Succeed())
	}
}

// waitTLSIssuerReady blocks until the issuer can actually sign.
//
// An issuer that never goes ready stalls every TLS instance in Initializing
// waiting for a certificate that is never issued -- indistinguishable from the
// operator being unable to create the Certificate at all. Failing here instead
// keeps the cause obvious.
func waitTLSIssuerReady(ctx context.Context, name string) {
	By(fmt.Sprintf("waiting for %s %s to be ready", tlsIssuerKind(), name))

	key := types.NamespacedName{Name: name}
	if tlsIssuerKind() != "ClusterIssuer" {
		key.Namespace = testNamespace
	}
	Eventually(func() (string, error) {
		var conditions []certv1.IssuerCondition
		if tlsIssuerKind() == "ClusterIssuer" {
			var issuer certv1.ClusterIssuer
			if err := k8sClient.Get(ctx, key, &issuer); err != nil {
				return "", err
			}
			conditions = issuer.Status.Conditions
		} else {
			var issuer certv1.Issuer
			if err := k8sClient.Get(ctx, key, &issuer); err != nil {
				return "", err
			}
			conditions = issuer.Status.Conditions
		}
		for _, cond := range conditions {
			if cond.Type == certv1.IssuerConditionReady {
				return fmt.Sprintf("%s: %s", cond.Status, cond.Message), nil
			}
		}
		return "no ready condition yet", nil
	}).WithTimeout(time.Minute*3).WithPolling(time.Second*5).
		Should(HavePrefix(string(certmetav1.ConditionTrue)),
			"issuer %s is not usable; its signing secret is read from namespace %q",
			name, tlsIssuerNamespace())
}

// ensureTLSIssuer creates the CA issuer the TLS instances reference.
//
// cert-manager ships no issuer of its own, so without this an instance with
// access.enableTLS would fail for a missing issuer rather than for anything the
// operator did. The chain is bootstrapped rather than self-signed per instance
// so the issued certificates verify against a separate root, which is what
// makes chain verification mean anything.
func ensureTLSIssuer(ctx context.Context) {
	By("creating the self-signed bootstrap issuer")
	createTLSIssuer(ctx, tlsSelfSignedIssuerName, certv1.IssuerConfig{
		SelfSigned: &certv1.SelfSignedIssuer{},
	})
	waitTLSIssuerReady(ctx, tlsSelfSignedIssuerName)

	By("issuing the CA certificate")
	caCert := &certv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tlsCAIssuerName,
			Namespace: tlsIssuerNamespace(),
		},
		Spec: certv1.CertificateSpec{
			IsCA:       true,
			CommonName: tlsCAIssuerName,
			SecretName: tlsCAIssuerName,
			PrivateKey: &certv1.CertificatePrivateKey{
				Algorithm: certv1.ECDSAKeyAlgorithm,
				Size:      256,
			},
			IssuerRef: certmetav1.IssuerReference{
				Name:  tlsSelfSignedIssuerName,
				Kind:  tlsIssuerKind(),
				Group: certv1.SchemeGroupVersion.Group,
			},
		},
	}
	if err := k8sClient.Create(ctx, caCert); err != nil && !errors.IsAlreadyExists(err) {
		Expect(err).To(Succeed())
	}

	By("waiting for the CA secret to be issued")
	Eventually(func() error {
		var secret corev1.Secret
		return k8sClient.Get(ctx, types.NamespacedName{
			Name:      tlsCAIssuerName,
			Namespace: tlsIssuerNamespace(),
		}, &secret)
	}).WithTimeout(time.Minute * 3).WithPolling(time.Second * 5).Should(Succeed())

	By("creating the CA issuer")
	createTLSIssuer(ctx, tlsCAIssuerName, certv1.IssuerConfig{
		CA: &certv1.CAIssuer{SecretName: tlsCAIssuerName},
	})
	waitTLSIssuerReady(ctx, tlsCAIssuerName)
}

// getInstanceTLSSecret returns the secret holding the instance certificate.
// Every node of an instance — valkey and sentinel alike — serves this one
// certificate, which is what lets a single client config reach all of them.
func getInstanceTLSSecret(ctx context.Context, inst *rdsv1alpha1.Valkey) (*corev1.Secret, error) {
	var secret corev1.Secret
	key := types.NamespacedName{
		Name:      certbuilder.GenerateSSLSecretName(inst.GetName()),
		Namespace: inst.GetNamespace(),
	}
	if err := k8sClient.Get(ctx, key, &secret); err != nil {
		return nil, fmt.Errorf("get tls secret %s failed: %w", key, err)
	}
	return &secret, nil
}

// newTLSInstance builds a TLS-enabled instance of the given architecture.
func newTLSInstance(arch core.Arch, accessType corev1.ServiceType) *rdsv1alpha1.Valkey {
	version := tlsTestVersion()
	if version == "" {
		return nil
	}

	inst := &rdsv1alpha1.Valkey{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-tls-%s", arch, accessSuffix(accessType)),
			Namespace: testNamespace,
		},
		Spec: rdsv1alpha1.ValkeySpec{
			Arch:    arch,
			Version: version,
			Replicas: &rdsv1alpha1.ValkeyReplicas{
				ReplicasOfShard: 2,
			},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("200Mi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("200Mi"),
				},
			},
			Access: core.InstanceAccess{
				ServiceType:    accessType,
				EnableTLS:      true,
				CertIssuer:     tlsCAIssuerName,
				CertIssuerType: tlsIssuerKind(),
			},
			Exporter: &rdsv1alpha1.ValkeyExporter{},
		},
	}
	if arch == core.ValkeyCluster {
		inst.Spec.Replicas.Shards = 3
	}
	if arch == core.ValkeyFailover {
		inst.Spec.Sentinel = &v1alpha1.SentinelSettings{
			SentinelSpec: v1alpha1.SentinelSpec{Replicas: 3},
		}
	}
	if defaltStorageClass != "" {
		inst.Spec.Storage = &core.Storage{
			StorageClassName: ptr.To(defaltStorageClass),
			Capacity:         ptr.To(resource.MustParse("1Gi")),
		}
	}

	var tmpInst rdsv1alpha1.Valkey
	if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(inst), &tmpInst); err == nil {
		return &tmpInst
	} else if !errors.IsNotFound(err) {
		AbortSuite(fmt.Sprintf("failed to get valkey instance: %v", err))
	}
	return inst
}

// checkInstanceCertificate asserts the operator issued a certificate for the
// instance and that it covers the names the nodes are reached by.
//
// The Certificate object existing at all is the assertion that would have
// caught the missing cert-manager scheme registration: without it the operator
// cannot construct this object and the instance never leaves Initializing.
func checkInstanceCertificate(ctx context.Context, inst *rdsv1alpha1.Valkey) {
	By("checking the operator issued a certificate")
	var cert certv1.Certificate
	Eventually(func() error {
		return k8sClient.Get(ctx, types.NamespacedName{
			Name:      certbuilder.GenerateCertName(inst.GetName()),
			Namespace: inst.GetNamespace(),
		}, &cert)
	}).WithTimeout(time.Minute * 5).WithPolling(time.Second * 5).Should(Succeed())

	Expect(cert.Spec.DNSNames).NotTo(BeEmpty(),
		"the certificate carries no DNS names, so no server name can satisfy verification")
	Expect(cert.Spec.SecretName).To(Equal(certbuilder.GenerateSSLSecretName(inst.GetName())))

	By("checking the certificate was actually issued")
	Eventually(func() (bool, error) {
		var c certv1.Certificate
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&cert), &c); err != nil {
			return false, err
		}
		for _, cond := range c.Status.Conditions {
			if cond.Type == certv1.CertificateConditionReady {
				return cond.Status == certmetav1.ConditionTrue, nil
			}
		}
		return false, nil
	}).WithTimeout(time.Minute * 5).WithPolling(time.Second * 5).Should(BeTrue())

	By("checking the issued secret carries a usable chain")
	secret, err := getInstanceTLSSecret(ctx, inst)
	Expect(err).To(Succeed())
	for _, key := range []string{corev1.TLSCertKey, corev1.TLSPrivateKeyKey, "ca.crt"} {
		Expect(secret.Data).To(HaveKey(key),
			"without %s the operator cannot verify the chain and falls back to trusting anything", key)
	}
}

// checkInstanceTLSVerification dials a node and completes a handshake using the
// operator's own client config.
//
// This is the regression guard for the MITM fix: the config must verify the
// chain (not skip it), and verification must still succeed against a real
// cert-manager certificate even though the node is dialled by IP while the
// certificate carries DNS names only.
func checkInstanceTLSVerification(ctx context.Context, inst *rdsv1alpha1.Valkey) {
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
	Expect(inst.Status.Nodes).NotTo(BeEmpty())

	secret, err := getInstanceTLSSecret(ctx, inst)
	Expect(err).To(Succeed())

	By("building the client config with the operator's helper")
	conf, err := util.LoadCertConfigFromSecret(secret)
	Expect(err).To(Succeed())
	Expect(conf.InsecureSkipVerify).To(BeFalse(),
		"certificate verification is disabled, so the CA in the secret is never consulted "+
			"and any certificate a MITM presents would be accepted")
	Expect(conf.RootCAs).NotTo(BeNil())
	Expect(conf.ServerName).NotTo(BeEmpty(),
		"no server name is set, so verification can never match a certificate that carries DNS names only")

	By("completing a verified handshake against every node")
	for _, node := range inst.Status.Nodes {
		addr := net.JoinHostPort(node.IP, node.Port)
		Eventually(func() error {
			dialer := &net.Dialer{Timeout: time.Second * 10}
			conn, err := tls.DialWithDialer(dialer, "tcp", addr, conf.Clone())
			if err != nil {
				return fmt.Errorf("verified handshake with %s failed: %w", addr, err)
			}
			defer conn.Close()

			state := conn.ConnectionState()
			if len(state.VerifiedChains) == 0 {
				return fmt.Errorf("handshake with %s completed without a verified chain", addr)
			}
			return nil
		}).WithTimeout(time.Minute * 3).WithPolling(time.Second * 5).Should(Succeed())
	}
}

// checkPlaintextIsRejected asserts a TLS node does not also serve plaintext.
func checkPlaintextIsRejected(ctx context.Context, inst *rdsv1alpha1.Valkey) {
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
	Expect(inst.Status.Nodes).NotTo(BeEmpty())

	node := inst.Status.Nodes[0]
	addr := net.JoinHostPort(node.IP, node.Port)

	By("checking a plaintext client cannot talk to a TLS node")
	conn, err := net.DialTimeout("tcp", addr, time.Second*10)
	if err != nil {
		// Refusing the connection outright is an equally good answer.
		return
	}
	defer conn.Close()

	Expect(conn.SetDeadline(time.Now().Add(time.Second * 10))).To(Succeed())
	// A plaintext PING is not a TLS ClientHello, so a TLS listener must not
	// answer it with a valkey reply.
	_, err = conn.Write([]byte("PING\r\n"))
	if err != nil {
		return
	}
	buf := make([]byte, 16)
	n, err := conn.Read(buf)
	if err == nil && n > 0 {
		Expect(string(buf[:n])).NotTo(HavePrefix("+PONG"),
			"the node answered an unencrypted PING, so enableTLS is not actually enforced")
	}
}

// tlsSpecs returns the TLS specs for one architecture.
func tlsSpecs(archLabel string) []Spec {
	labels := []string{archLabel, "tls"}
	return []Spec{
		{
			Name:   "deploy a TLS enabled instance",
			Labels: append(labels, "deploy"),
			Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
				if inst == nil {
					Skip("no valkey version available for the TLS tests")
				}
				ensureTLSIssuer(ctx)

				By("creating the TLS enabled instance")
				if err := k8sClient.Create(ctx, inst); err != nil && !errors.IsAlreadyExists(err) {
					Expect(err).To(Succeed())
				}

				// An instance that cannot get its certificate stalls in
				// Initializing rather than failing, so this is where a missing
				// cert-manager registration or a broken issuer shows up.
				By("checking the instance becomes ready")
				waitInstanceStatusReady(ctx, inst, time.Minute*15)
			},
		},
		{
			Name:   "certificate is issued and covers the instance",
			Labels: append(labels, "cert"),
			Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
				if inst == nil {
					Skip("no valkey version available for the TLS tests")
				}
				checkInstanceCertificate(ctx, inst)
			},
		},
		{
			Name:   "nodes present a certificate that verifies against the instance CA",
			Labels: append(labels, "verify"),
			Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
				if inst == nil {
					Skip("no valkey version available for the TLS tests")
				}
				checkInstanceTLSVerification(ctx, inst)
			},
		},
		{
			Name:   "plaintext access is refused",
			Labels: append(labels, "verify"),
			Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
				if inst == nil {
					Skip("no valkey version available for the TLS tests")
				}
				checkPlaintextIsRejected(ctx, inst)
			},
		},
		{
			Name:   "read/write data over TLS",
			Labels: append(labels, "readwrite"),
			Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
				if inst == nil {
					Skip("no valkey version available for the TLS tests")
				}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
				checkInstanceWrite(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
				checkInstanceRead(ctx, inst, valkeyDefaultUsername, valkeyDefaultPassword)
			},
		},
		{
			Name:   "instance stays ready and verifiable after a pod restart",
			Labels: append(labels, "restart"),
			Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
				if inst == nil {
					Skip("no valkey version available for the TLS tests")
				}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(inst), inst)).To(Succeed())
				Expect(inst.Status.Nodes).NotTo(BeEmpty())

				By("restarting a node")
				killPod(ctx, inst.GetNamespace(), inst.Status.Nodes[0].PodName)

				By("checking the instance recovers")
				waitInstanceStatusReady(ctx, inst, time.Minute*15)

				// A node that comes back must still serve a certificate the
				// operator can verify, otherwise reconciliation stalls the
				// moment a pod is rescheduled.
				checkInstanceTLSVerification(ctx, inst)
			},
		},
		{
			Name:   "delete the TLS enabled instance",
			Labels: append(labels, "delete"),
			Func: func(ctx context.Context, inst *rdsv1alpha1.Valkey) {
				if inst == nil {
					Skip("no valkey version available for the TLS tests")
				}
				deleteInstance(ctx, inst)

				By("checking the certificate is cleaned up with the instance")
				Eventually(func() bool {
					var cert certv1.Certificate
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      certbuilder.GenerateCertName(inst.GetName()),
						Namespace: inst.GetNamespace(),
					}, &cert)
					return errors.IsNotFound(err)
				}).WithTimeout(time.Minute * 5).WithPolling(time.Second * 5).Should(BeTrue())
			},
		},
	}
}

// tlsTestData builds the TLS case for one architecture. The version is pinned
// inside newTLSInstance, so the case reports itself as skipped rather than
// deploying an instance for every version in the matrix.
func tlsTestData(arch core.Arch, archLabel string) TestData {
	return TestData{
		When: "with TLS enabled",
		BeforeEach: func(version string, accessType corev1.ServiceType) *rdsv1alpha1.Valkey {
			if version != tlsTestVersion() {
				return nil
			}
			return newTLSInstance(arch, accessType)
		},
		Specs: tlsSpecs(archLabel),
	}
}

func init() {
	clusterTestCases = append(clusterTestCases, tlsTestData(core.ValkeyCluster, "cluster"))
	failoverTestCases = append(failoverTestCases, tlsTestData(core.ValkeyFailover, "failover"))
	replicationTestCases = append(replicationTestCases, tlsTestData(core.ValkeyReplica, "replication"))
}
