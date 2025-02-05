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
*/package slot

import (
	"fmt"
	"sort"
)

// Allocate
func Allocate(desired int, shards []*Slots) (ret []*Slots) {
	if desired <= 0 {
		return nil
	}

	for i := 0; i < desired; i++ {
		s := NewSlots()
		if i < len(shards) {
			s = s.Union(shards[i])
		}
		ret = append(ret, s)
	}
	avg, rest := ValkeyMaxSlots/desired, ValkeyMaxSlots%desired

	if len(shards) == 0 {
		offset := 0
		for i := 0; i < desired; i++ {
			count := avg
			if i < rest {
				count = avg + 1
			}
			_ = ret[i].Set(fmt.Sprintf("%d-%d", offset, offset+count-1), SlotAssigned)
			offset += count
		}
	} else if len(shards) < desired {
		var (
			// move out
			movingRet = make([]int, desired)
			movePlan  = make([]map[int]int, len(shards))
			movables  []int
		)
		for i := 0; i < desired; i++ {
			if i < rest {
				movingRet[i] = avg + 1
			} else {
				movingRet[i] = avg
			}
			if i < len(shards) {
				movables = append(movables, shards[i].Count(SlotAssigned)-movingRet[i])
			}
		}

		for i := len(shards); i < desired; i++ {
			want := movingRet[i]
			for j := 0; j < len(movables); j++ {
				if movables[j] <= 0 {
					continue
				}
				gap := movables[j] - want
				if gap >= 0 {
					if movePlan[j] == nil {
						movePlan[j] = map[int]int{}
					}
					movePlan[j][i] = want
					movables[j] -= want
					break
				} else if gap < 0 {
					if movePlan[j] == nil {
						movePlan[j] = map[int]int{}
					}
					movePlan[j][i] = movables[j]
					want = want - movables[j]
					movables[j] = 0
				}
			}
		}

		indexes := []int{}
		for i, plans := range movePlan {
			slots := ret[i].Slots()
			// keep order
			indexes = indexes[0:0]
			for t := range plans {
				indexes = append(indexes, t)
			}
			sort.Ints(indexes)

			for _, target := range indexes {
				count := plans[target]
				toMoveSlots := slots[len(slots)-count:]
				slots = slots[0 : len(slots)-count]
				for _, slot := range toMoveSlots {
					_ = ret[target].Set(slot, SlotAssigned)
					_ = ret[i].Set(slot, SlotUnassigned)
				}
			}
		}
	} else if len(shards) > desired {
		// 这里没有过分优化槽的缩容
		// 槽的缩容不像扩容，不规律的缩容肯定会导致槽信息的碎片化
		// TODO: 与其引入复杂的槽缩容算法不如加一个槽 Rebalance 功能，用于在集群空闲时重新规划并迁移槽
		var (
			// move in
			movingRet = make([]int, desired)
			movePlan  = map[int]map[int]int{}
			movables  []int
		)
		for i := 0; i < len(shards); i++ {
			if i < desired {
				if i < rest {
					movingRet[i] = avg + 1
				} else {
					movingRet[i] = avg
				}
				movables = append(movables, 0)
			} else {
				movables = append(movables, shards[i].Count(SlotAssigned))
			}
		}

		for i := 0; i < desired; i++ {
			want := movingRet[i] - shards[i].Count(SlotAssigned)
			for j := desired; j < len(shards); j++ {
				if movables[j] == 0 {
					continue
				}
				gap := movables[j] - want
				if gap >= 0 {
					if movePlan[j] == nil {
						movePlan[j] = map[int]int{}
					}
					movePlan[j][i] = want
					movables[j] -= want
					break
				} else if gap < 0 {
					if movePlan[j] == nil {
						movePlan[j] = map[int]int{}
					}
					movePlan[j][i] = movables[j]
					want = want - movables[j]
					movables[j] = 0
				}
			}
		}

		indexes := []int{}
		for i, plans := range movePlan {
			slots := shards[i].Slots()
			// keep order
			indexes = indexes[0:0]
			for t := range plans {
				indexes = append(indexes, t)
			}
			sort.Ints(indexes)

			for _, target := range indexes {
				count := plans[target]
				toMoveSlots := slots[0:count]
				slots = slots[count:]
				for _, slot := range toMoveSlots {
					_ = ret[target].Set(slot, SlotAssigned)
					_ = shards[i].Set(slot, SlotUnassigned)
				}
			}
		}
	}
	return
}
