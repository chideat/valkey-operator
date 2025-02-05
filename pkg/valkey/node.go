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
*/package valkey

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/chideat/valkey-operator/pkg/slot"
)

const (
	MasterRole = "master"
	SlaveRole  = "slave"
)

type ClusterNodeAuxFields struct {
	ShardID  string `json:"shard-id"`
	NodeName string `json:"nodename"`
	TCPPort  int64  `json:"tcp-port"`
	TLSPort  int64  `json:"tls-port"`
	raw      string
}

func (c *ClusterNodeAuxFields) Raw() string {
	return c.raw
}

type ClusterNode struct {
	Id        string
	Addr      string
	RawFlag   string
	BusPort   string
	AuxFields ClusterNodeAuxFields
	Role      string
	MasterId  string
	PingSend  int64
	PongRecv  int64
	Epoch     int64
	LinkState string

	slots   []string
	rawInfo string
}

func (n *ClusterNode) IsSelf() bool {
	if n == nil {
		return false
	}
	return strings.HasPrefix(n.RawFlag, "myself")
}

func (n *ClusterNode) IsFailed() bool {
	if n == nil {
		return true
	}
	return strings.Contains(n.RawFlag, "fail")
}

func (n *ClusterNode) IsConnected() bool {
	if n == nil {
		return false
	}
	return n.LinkState == "connected"
}

func (n *ClusterNode) IsJoined() bool {
	if n == nil {
		return false
	}
	return n.Addr != ""
}

func (n *ClusterNode) Slots() *slot.Slots {
	if n == nil {
		return nil
	}
	if n.Role == SlaveRole || len(n.slots) == 0 {
		return nil
	}

	slots := slot.NewSlots()
	_ = slots.Load(n.slots)

	return slots
}

func (n *ClusterNode) Raw() string {
	if n == nil {
		return ""
	}
	return n.rawInfo
}

type ClusterNodes []*ClusterNode

func (ns ClusterNodes) Get(id string) *ClusterNode {
	for _, n := range ns {
		if n.Id == id {
			return n
		}
	}
	return nil
}

func (ns ClusterNodes) Self() *ClusterNode {
	for _, n := range ns {
		if n.IsSelf() {
			return n
		}
	}
	return nil
}

func (ns ClusterNodes) Replicas(id string) (ret []*ClusterNode) {
	if len(ns) == 0 {
		return
	}
	for _, n := range ns {
		if n.MasterId == id {
			ret = append(ret, n)
		}
	}
	return
}

func (ns ClusterNodes) Masters() (ret []*ClusterNode) {
	for _, n := range ns {
		if n.Role == MasterRole && len(n.slots) > 0 {
			ret = append(ret, n)
		}
	}
	return
}

func (ns ClusterNodes) Marshal() ([]byte, error) {
	data := []map[string]string{}
	for _, n := range ns {
		d := map[string]string{}
		d["id"] = n.Id
		d["addr"] = n.Addr
		d["bus_port"] = n.BusPort
		if data, _ := json.Marshal(&n.AuxFields); data != nil {
			d["aux_fields"] = string(data)
		}
		d["flags"] = n.RawFlag
		d["role"] = n.Role
		d["master_id"] = n.MasterId
		d["ping_send"] = strconv.FormatInt(n.PingSend, 10)
		d["pong_recv"] = strconv.FormatInt(n.PongRecv, 10)
		d["config_epoch"] = strconv.FormatInt(n.Epoch, 10)
		d["link_state"] = n.LinkState
		d["slots"] = strings.Join(n.slots, ",")
		data = append(data, d)
	}
	return json.Marshal(data)
}

var (
	invalidAddrReg = regexp.MustCompile(`^:\d+$`)
)

// ParseNodeFromClusterNode
// format:
//
//	<id> <ip:port@cport> <flags> <master> <ping-sent> <pong-recv> <config-epoch> <link-state> <slot> <slot> ... <slot>
func ParseNodeFromClusterNode(line string) (*ClusterNode, error) {
	fields := strings.Fields(line)
	if len(fields) < 8 {
		return nil, fmt.Errorf("invalid node info %v", fields)
	}

	aux := ClusterNodeAuxFields{
		raw: fields[1],
	}
	auxFields := strings.Split(fields[1], ",")
	addrPair := strings.SplitN(auxFields[0], "@", 2)
	if len(addrPair) != 2 {
		return nil, fmt.Errorf("invalid node info %v", fields)
	}
	if invalidAddrReg.MatchString(addrPair[0]) {
		addrPair[0] = ""
	}
	for _, af := range auxFields[1:] {
		kv := strings.Split(af, "=")
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "shard-id":
			aux.ShardID = kv[1]
		case "nodename":
			aux.NodeName = kv[1]
		case "tcp-port":
			aux.TCPPort, _ = strconv.ParseInt(kv[1], 10, 64)
		case "tls-port":
			aux.TLSPort, _ = strconv.ParseInt(kv[1], 10, 64)
		}
	}

	role := fields[2]
	if strings.Contains(fields[2], SlaveRole) {
		role = SlaveRole
	} else if strings.Contains(fields[2], MasterRole) {
		role = MasterRole
	}
	pingSend, _ := strconv.ParseInt(fields[4], 10, 64)
	pongRecv, _ := strconv.ParseInt(fields[5], 10, 64)
	epoch, _ := strconv.ParseInt(fields[6], 10, 64)

	node := &ClusterNode{
		Id:        fields[0],
		Addr:      addrPair[0],
		BusPort:   addrPair[1],
		AuxFields: aux,
		RawFlag:   fields[2],
		Role:      role,
		MasterId:  strings.TrimPrefix(fields[3], "-"),
		PingSend:  pingSend,
		PongRecv:  pongRecv,
		Epoch:     epoch,
		LinkState: fields[7],
		slots:     fields[8:],
		rawInfo:   line,
	}
	return node, nil
}

// parseNodes
func ParseNodes(data string) (nodes ClusterNodes, err error) {
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "vars") {
			continue
		}
		if node, err := ParseNodeFromClusterNode(line); err != nil {
			return nil, err
		} else {
			nodes = append(nodes, node)
		}
	}
	return
}
