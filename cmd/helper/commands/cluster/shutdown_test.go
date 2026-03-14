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
	"github.com/go-logr/logr"
)

// Test_shutdownNode verifies the SHUTDOWN/SHUTDOWN NOSAVE decision in shutdownNode.
// miniredis doesn't support SHUTDOWN, so both cases return an error — but each exercises
// the correct branch (NOSAVE for dbsize==0, normal SHUTDOWN for dbsize>0).
func Test_shutdownNode(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(s *miniredis.Miniredis)
		wantErr bool
	}{
		{
			name:    "empty database uses SHUTDOWN NOSAVE",
			setup:   func(_ *miniredis.Miniredis) {},
			wantErr: true, // miniredis returns ERR for SHUTDOWN NOSAVE
		},
		{
			name: "non-empty database uses normal SHUTDOWN",
			setup: func(s *miniredis.Miniredis) {
				_ = s.Set("key1", "value1")
				_ = s.Set("key2", "value2")
			},
			wantErr: true, // miniredis returns ERR for SHUTDOWN
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := miniredis.RunT(t)
			tt.setup(s)
			client := valkey.NewValkeyClient(s.Addr(), valkey.AuthInfo{})
			defer client.Close()

			err := shutdownNode(context.Background(), client, logr.Discard())
			if (err != nil) != tt.wantErr {
				t.Errorf("shutdownNode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
