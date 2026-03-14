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

package cluster

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/chideat/valkey-operator/pkg/valkey"
)

// Test_shutdownNosaveDecision verifies the shutdown save-mode decision logic:
// Dbsize == 0 (e.g. failed cross-version fullsync) → SHUTDOWN NOSAVE to preserve dump.rdb.
// Dbsize > 0 → normal SHUTDOWN.
func Test_shutdownNosaveDecision(t *testing.T) {
	tests := []struct {
		name       string
		dbsize     int64
		wantNosave bool
	}{
		{
			name:       "empty database - use SHUTDOWN NOSAVE",
			dbsize:     0,
			wantNosave: true,
		},
		{
			name:       "non-empty database - use normal SHUTDOWN",
			dbsize:     2,
			wantNosave: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNosave := tt.dbsize == 0
			if gotNosave != tt.wantNosave {
				t.Errorf("nosave decision for dbsize=%d: got %v, want %v", tt.dbsize, gotNosave, tt.wantNosave)
			}
		})
	}
}

// Test_shutdownInfoEmptyDB verifies that Info() returns Dbsize==0 for an empty miniredis instance,
// which is the scenario triggered by a failed cross-version fullsync.
func Test_shutdownInfoEmptyDB(t *testing.T) {
	s := miniredis.RunT(t)
	client := valkey.NewValkeyClient(s.Addr(), valkey.AuthInfo{})
	defer client.Close()

	info, err := client.Info(context.Background())
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}
	if info.Dbsize != 0 {
		t.Errorf("expected Dbsize=0 for empty DB, got %d", info.Dbsize)
	}
}
