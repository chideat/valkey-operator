package e2e

import (
	"context"
	"time"

	rdsv1alpha1 "github.com/chideat/valkey-operator/api/rds/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type Spec struct {
	Name    string
	Labels  []string
	Timeout time.Duration
	Skip    bool
	Func    func(context.Context, *rdsv1alpha1.Valkey)
}

type TestData struct {
	When       string
	BeforeEach func(version string, accessType corev1.ServiceType) *rdsv1alpha1.Valkey
	Specs      []Spec
	AfterEach  func(*rdsv1alpha1.Valkey)
}
