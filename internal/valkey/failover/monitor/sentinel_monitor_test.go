package monitor

import (
	"context"
	"errors"
	"net"
	"testing"

	vkcli "github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

// fakeValkeyClient implements only Do; MonitoringMaster needs nothing else.
// The embedded nil interface panics if any other method is called, which keeps
// the fake honest about what the code under test actually touches.
type fakeValkeyClient struct {
	vkcli.ValkeyClient
	val any
	err error
}

func (f *fakeValkeyClient) Do(_ context.Context, _ string, _ ...any) (any, error) {
	return f.val, f.err
}

func newSentinelNode(val any, err error) *SentinelNode {
	return &SentinelNode{addr: "fake:26379", client: &fakeValkeyClient{val: val, err: err}}
}

// TestSentinelMonitorMasterErrorClassification drives the real Master() with
// injected client errors to verify each is classified into the right outcome:
// unreachable sentinels are skipped, a full partition requeues, context errors
// propagate, and reachable-but-no-master heals.
func TestSentinelMonitorMasterErrorClassification(t *testing.T) {
	dnsErr := &net.DNSError{Err: "no such host", Name: "sentinel", IsNotFound: true}
	netErr := &net.OpError{Op: "dial", Err: errors.New("connection refused")}
	noMaster := errors.New("No such master with that name")

	tests := []struct {
		name    string
		nodeErr error
		wantErr error
	}{
		{"all sentinels DNS-unresolvable is a full partition", dnsErr, ErrNoSentinelReachable},
		{"all sentinels connection-refused is a full partition", netErr, ErrNoSentinelReachable},
		{"context cancellation propagates as requeue", context.Canceled, context.Canceled},
		{"context deadline propagates as requeue", context.DeadlineExceeded, context.DeadlineExceeded},
		{"reachable sentinels agreeing on no master heals", noMaster, ErrNoMaster},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SentinelMonitor{
				groupName: "mymaster",
				logger:    logr.Discard(),
				nodes: []*SentinelNode{
					newSentinelNode(nil, tt.nodeErr),
					newSentinelNode(nil, tt.nodeErr),
					newSentinelNode(nil, tt.nodeErr),
				},
			}
			_, err := s.Master(context.Background())
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}
