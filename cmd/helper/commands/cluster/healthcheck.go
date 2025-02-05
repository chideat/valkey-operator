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
	"errors"
	"net"
	"time"

	"github.com/chideat/valkey-operator/pkg/valkey"
)

func Readiness(ctx context.Context, addr string, authInfo valkey.AuthInfo) error {
	client := valkey.NewValkeyClient(addr, authInfo)
	defer client.Close()

	nodes, err := client.Nodes(ctx)
	if err != nil {
		return err
	}
	self := nodes.Self()
	if self == nil {
		return errors.New("self node not found")
	}
	if !self.IsJoined() {
		return errors.New("node is not joined")
	}
	if self.IsFailed() {
		return errors.New("node is failed")
	}
	return nil
}

// Ping
func Ping(ctx context.Context, addr string, authInfo valkey.AuthInfo) error {
	client := valkey.NewValkeyClient(addr, authInfo)
	defer client.Close()

	if _, err := client.Do(ctx, "PING"); err != nil {
		return err
	}
	return nil
}

// TcpSocket
func TcpSocket(ctx context.Context, addr string, t time.Duration) error {
	conn, err := net.DialTimeout("tcp", addr, t)
	if err != nil {
		return err
	}
	conn.Close()

	return nil
}
