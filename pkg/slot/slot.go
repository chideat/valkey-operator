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
package slot

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/samber/lo"
)

const (
	RedisMaxSlots = 16384
)

// SlotAssignStatus slot assign status
type SlotAssignStatus int

const (
	// SlotUnassigned - slot assigned
	SlotUnassigned SlotAssignStatus = 0
	// SlotImporting - slot is in importing status
	SlotImporting SlotAssignStatus = 1
	// SlotAssigned - slot is assigned
	SlotAssigned SlotAssignStatus = 2
	// SlotMigrating - slot is in migrating status
	SlotMigrating SlotAssignStatus = 3
)

func NewSlotAssignStatusFromString(v string) SlotAssignStatus {
	switch v {
	case "SlotImporting":
		return 1
	case "SlotAssigned":
		return 2
	case "SlotMigrating":
		return 3
	}
	return 0
}

func (s SlotAssignStatus) String() string {
	switch s {
	case SlotImporting:
		return "SlotImporting"
	case SlotMigrating:
		return "SlotMigrating"
	case SlotAssigned:
		return "SlotAssigned"
	}
	return "SlotUnassigned"
}

// Slots use two byte represent the slot status
// 00 - unassigned
// 01 - importing
// 10 - assigned
// 11 - migrating
// Slots

type Slots struct {
	data           [4096]uint8
	migratingSlots map[int]string
	importingSlots map[int]string
}

func NewSlots() *Slots {
	return &Slots{
		migratingSlots: map[int]string{},
		importingSlots: map[int]string{},
	}
}

func LoadSlots(v interface{}) (*Slots, error) {
	s := NewSlots()
	if err := s.Load(v); err != nil {
		return nil, err
	}
	return s, nil
}

func NewFullSlots() *Slots {
	s := NewSlots()
	for i := 0; i < RedisMaxSlots; i++ {
		_ = s.Set(i, SlotAssigned)
	}
	return s
}

func IsFullfilled(slots ...*Slots) bool {
	var ret *Slots
	for _, s := range slots {
		ret = ret.Union(s)
	}
	return ret.IsFullfilled()
}

// IsFullfilled check if this slots if fullfilled
func (s *Slots) IsFullfilled() bool {
	if s == nil {
		return false
	}

	for i := 0; i < RedisMaxSlots; i++ {
		index, offset := convertIndex(i)

		if (s.data[index]>>offset)&uint8(SlotAssigned) == 0 {
			return false
		}
	}
	return true
}

func (s *Slots) parseStrSlots(v string) (ret map[int]SlotAssignStatus, nodes map[int]string, err error) {
	ret = map[int]SlotAssignStatus{}
	nodes = map[int]string{}

	fields := strings.Fields(strings.ReplaceAll(v, ",", " "))
	for _, field := range fields {
		field = strings.TrimSuffix(strings.TrimPrefix(field, "["), "]")
		if strings.Contains(field, "-<-") {
			moveFields := strings.SplitN(field, "-<-", 2)
			if len(moveFields) != 2 || len(moveFields[0]) == 0 || len(moveFields[1]) == 0 {
				return nil, nil, fmt.Errorf("invalid slot %s", field)
			}
			start, err := strconv.ParseInt(moveFields[0], 10, 32)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid slot %s", field)
			}
			ret[int(start)] = SlotImporting
			nodes[int(start)] = moveFields[1]
		} else if strings.Contains(field, "->-") {
			moveFields := strings.SplitN(field, "->-", 2)
			if len(moveFields) != 2 || len(moveFields[0]) == 0 || len(moveFields[1]) == 0 {
				return nil, nil, fmt.Errorf("invalid slot %s", field)
			}
			start, err := strconv.ParseInt(moveFields[0], 10, 32)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid slot %s", field)
			}
			ret[int(start)] = SlotMigrating
			nodes[int(start)] = moveFields[1]
		} else if strings.Contains(field, "-") {
			rangeFields := strings.SplitN(field, "-", 2)
			start, err := strconv.ParseInt(rangeFields[0], 10, 32)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid range slot %s", field)
			}
			end, err := strconv.ParseInt(rangeFields[1], 10, 32)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid range slot %s", field)
			}
			if start >= end {
				return nil, nil, fmt.Errorf("invalid range slot %s", field)
			}
			for i := int(start); i <= int(end); i++ {
				ret[i] = SlotAssigned
			}
		} else {
			slotIndex, err := strconv.ParseInt(field, 10, 32)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid range slot %s", field)
			}
			ret[int(slotIndex)] = SlotAssigned
		}
	}
	return
}

func (s *Slots) Load(v interface{}) error {
	if s == nil {
		return nil
	}
	handler := func(i int, status SlotAssignStatus, nodeId string) error {
		if (i >= RedisMaxSlots) || (i < 0) {
			return fmt.Errorf("invalid slot %d", i)
		}
		index, offset := convertIndex(i)
		s.data[index] = s.data[index]&^(uint8(3)<<offset) | (uint8(status) << offset)

		if nodeId != "" {
			switch status {
			case SlotImporting:
				s.importingSlots[i] = nodeId
			case SlotMigrating:
				s.migratingSlots[i] = nodeId
			}
		}
		return nil
	}
	status := SlotAssignStatus(SlotAssigned)
	switch vv := v.(type) {
	case int:
		return handler(vv, status, "")
	case []int:
		for _, i := range vv {
			if err := handler(i, status, ""); err != nil {
				return err
			}
		}
	case string:
		ret, nodes, err := s.parseStrSlots(vv)
		if err != nil {
			return err
		}
		keys := lo.Keys(ret)
		sort.Ints(keys)
		for _, key := range keys {
			status := ret[key]
			if err = handler(key, status, nodes[key]); err != nil {
				return err
			}
		}
	case []string:
		ret, nodes, err := s.parseStrSlots(strings.Join(vv, " "))
		if err != nil {
			return err
		}

		keys := lo.Keys(ret)
		sort.Ints(keys)
		for _, key := range keys {
			status := ret[key]
			if err = handler(key, status, nodes[key]); err != nil {
				return err
			}
		}
	default:
		return errors.New("unsupported slot render type")
	}
	return nil
}

// Set
func (s *Slots) Set(v interface{}, status SlotAssignStatus) error {
	if s == nil {
		return nil
	}
	handler := func(i int, status SlotAssignStatus) {
		index, offset := convertIndex(i)
		s.data[index] = s.data[index]&^(uint8(3)<<offset) | (uint8(status) << offset)
	}

	switch vv := v.(type) {
	case int:
		handler(vv, status)
	case []int:
		for _, i := range vv {
			handler(i, status)
		}
	case string:
		ret, _, err := s.parseStrSlots(vv)
		if err != nil {
			return err
		}
		for i := range ret {
			handler(i, status)
		}
	case []string:
		ret, _, err := s.parseStrSlots(strings.Join(vv, " "))
		if err != nil {
			return err
		}
		for i := range ret {
			handler(i, status)
		}
	default:
		return errors.New("unsupported slot render type")
	}
	return nil
}

// Equals check if this two slots equals (no care about the status)
func (s *Slots) Equals(old *Slots) bool {
	if s == nil {
		return old == nil
	}
	if old == nil {
		return false
	}
	for i := 0; i < len(s.data); i++ {
		if (s.data[i] & 0xAA) != (old.data[i] & 0xAA) {
			return false
		}
	}
	return true
}

// Inter return the intersection with n (no care about the status)
func (s *Slots) Inter(n *Slots) *Slots {
	if n == nil || s == nil {
		return NewSlots()
	}

	var ret Slots
	for i := 0; i < len(s.data); i++ {
		ret.data[i] = (s.data)[i] & (n.data)[i] & 0xAA
	}
	return &ret
}

func (s *Slots) Sub(n *Slots) *Slots {
	if s == nil {
		return NewSlots()
	}
	if n == nil {
		ret := NewSlots()
		ret.data = s.data
		return ret
	}
	ret := NewSlots()
	for i := 0; i < RedisMaxSlots; i++ {
		index, offset := convertIndex(i)

		valA := ((s.data[index] >> offset) & uint8(SlotAssigned))
		valB := ((n.data[index] >> offset) & uint8(SlotAssigned))
		if valA == uint8(SlotAssigned) && valB == 0 {
			ret.data[index] |= (uint8(SlotAssigned) << offset)
		}
	}
	return ret
}

// Union ignore slot status
func (s *Slots) Union(slots ...*Slots) *Slots {
	ret := NewSlots()
	if s != nil {
		ret.data = s.data
	}
	for i := 0; i < len(ret.data); i++ {
		// clean slot status
		ret.data[i] &= 0xAA
		for j := 0; j < len(slots); j++ {
			if slots[j] != nil {
				ret.data[i] |= (slots[j].data[i] & 0xAA)
			}
		}
	}
	return ret
}

// Slots
func (s *Slots) Slots(ss ...SlotAssignStatus) (ret []int) {
	if s == nil {
		return nil
	}

	if len(ss) == 0 {
		ss = []SlotAssignStatus{SlotAssigned}
	}
	for i := 0; i < RedisMaxSlots; i++ {
		index, offset := convertIndex(i)
		for _, st := range ss {
			if (s.data[index]>>offset)&uint8(SlotMigrating) == uint8(st) {
				ret = append(ret, i)
				break
			}
		}
	}
	return
}

// SlotsByStatus
func (s *Slots) SlotsByStatus(status SlotAssignStatus) (ret []int) {
	if s == nil {
		return nil
	}
	for i := 0; i < RedisMaxSlots; i++ {
		index, offset := convertIndex(i)
		if (s.data[index]>>offset)&uint8(SlotMigrating) == uint8(status) {
			ret = append(ret, i)
		}
	}
	return
}

// String
func (s *Slots) String() string {
	if s == nil {
		return ""
	}

	var (
		lastStartIndex = -1
		lastIndex      = -1
		ret            []string
	)
	for i := 0; i < RedisMaxSlots; i++ {
		index, offset := convertIndex(i)
		if (s.data[index]>>offset)&uint8(SlotAssigned) > 0 {
			if lastIndex == -1 {
				lastIndex = i
				lastStartIndex = i
			} else if lastIndex != i-1 {
				if lastStartIndex == lastIndex {
					ret = append(ret, fmt.Sprintf("%d", lastStartIndex))
				} else {
					ret = append(ret, fmt.Sprintf("%d-%d", lastStartIndex, lastIndex))
				}
				lastIndex = i
				lastStartIndex = i
			} else {
				lastIndex = i
			}
		}
	}
	if lastIndex != -1 {
		if lastStartIndex == lastIndex {
			ret = append(ret, fmt.Sprintf("%d", lastStartIndex))
		} else {
			ret = append(ret, fmt.Sprintf("%d-%d", lastStartIndex, lastIndex))
		}
	}
	return strings.Join(ret, ",")
}

func (s *Slots) Count(status SlotAssignStatus) (c int) {
	if s == nil {
		return
	}

	mask := uint8(SlotMigrating)
	if status == SlotUnassigned || status == SlotAssigned {
		mask = uint8(SlotAssigned)
	}

	for i := 0; i < RedisMaxSlots; i++ {
		index, offset := convertIndex(i)
		if (s.data[index]>>offset)&mask == uint8(status) {
			c += 1
		}
	}
	return
}

func (s *Slots) Status(i int) SlotAssignStatus {
	if s == nil {
		return SlotUnassigned
	}
	index, offset := convertIndex(i)
	return SlotAssignStatus((s.data[index] >> offset) & 3)
}

func (s *Slots) IsSet(i int) bool {
	if s == nil {
		return false
	}
	index, offset := convertIndex(i)
	return (s.data[index]>>offset)&uint8(SlotAssigned) == uint8(SlotAssigned)
}

func (s *Slots) IsImporting() bool {
	if s == nil {
		return false
	}
	return len(s.importingSlots) > 0
}

func (s *Slots) IsMigration() bool {
	if s == nil {
		return false
	}
	return len(s.migratingSlots) > 0
}

func (s *Slots) MoveingStatus(i int) (SlotAssignStatus, string) {
	if s == nil {
		return SlotUnassigned, ""
	}
	index, offset := convertIndex(i)
	status := SlotAssignStatus((s.data[index] >> offset) & 3)
	switch status {
	case SlotImporting:
		return status, s.importingSlots[i]
	case SlotMigrating:
		return status, s.migratingSlots[i]
	}
	return status, ""
}

func convertIndex(i int) (int, int) {
	index := i * 2 / 8
	offset := i*2 - index*8
	return index, offset
}
