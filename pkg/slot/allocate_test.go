/*
Copyright 2023 The RedisOperator Authors.

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
	"math"
	"reflect"
	"testing"
)

func testLoadSlots(v string, status SlotAssignStatus) *Slots {
	s := NewSlots()
	_ = s.Set(v, status)
	return s
}

func TestAllocate(t *testing.T) {
	// 3
	type args struct {
		desired int
		current []*Slots
	}
	tests := []struct {
		name    string
		args    args
		wantRet []*Slots
	}{
		{
			name: "new-3",
			args: args{desired: 3},
			wantRet: []*Slots{
				testLoadSlots("0-5461", SlotAssigned),
				testLoadSlots("5462-10922", SlotAssigned),
				testLoadSlots("10923-16383", SlotAssigned),
			},
		},
		{
			name: "new-4",
			args: args{desired: 4},
			wantRet: []*Slots{
				testLoadSlots("0-4095", SlotAssigned),
				testLoadSlots("4096-8191", SlotAssigned),
				testLoadSlots("8192-12287", SlotAssigned),
				testLoadSlots("12288-16383", SlotAssigned),
			},
		},
		{
			name: "new-5",
			args: args{desired: 5},
			wantRet: []*Slots{
				testLoadSlots("0-3276", SlotAssigned),
				testLoadSlots("3277-6553", SlotAssigned),
				testLoadSlots("6554-9830", SlotAssigned),
				testLoadSlots("9831-13107", SlotAssigned),
				testLoadSlots("13108-16383", SlotAssigned),
			},
		},
		{
			name: "new-6",
			args: args{desired: 6},
			wantRet: []*Slots{
				testLoadSlots("0-2730", SlotAssigned),
				testLoadSlots("2731-5461", SlotAssigned),
				testLoadSlots("5462-8192", SlotAssigned),
				testLoadSlots("8193-10923", SlotAssigned),
				testLoadSlots("10924-13653", SlotAssigned),
				testLoadSlots("13654-16383", SlotAssigned),
			},
		},
		{
			name: "new-128",
			args: args{desired: 128},
		},
		{
			name: "new-256",
			args: args{desired: 256},
		},

		// scale up
		{
			name: "scaleup-3=>4",
			args: args{desired: 4, current: Allocate(3, nil)},
			wantRet: []*Slots{
				testLoadSlots("0-4095", SlotAssigned),
				testLoadSlots("5462-9557", SlotAssigned),
				testLoadSlots("10923-15018", SlotAssigned),
				testLoadSlots("4096-5461,9558-10922,15019-16383", SlotAssigned),
			},
		},
		{
			name: "scaleup-3=>5",
			args: args{desired: 5, current: Allocate(3, nil)},
			wantRet: []*Slots{
				testLoadSlots("0-3276", SlotAssigned),
				testLoadSlots("5462-8738", SlotAssigned),
				testLoadSlots("10923-14199", SlotAssigned),
				testLoadSlots("3277-5461,9831-10922", SlotAssigned),
				testLoadSlots("8739-9830,14200-16383", SlotAssigned),
			},
		},
		{
			name: "scaleup-3=>6",
			args: args{desired: 6, current: Allocate(3, nil)},
			wantRet: []*Slots{
				testLoadSlots("0-2730", SlotAssigned),
				testLoadSlots("5462-8192", SlotAssigned),
				testLoadSlots("10923-13653", SlotAssigned),
				testLoadSlots("2731-5461", SlotAssigned),
				testLoadSlots("8193-10922", SlotAssigned),
				testLoadSlots("13654-16383", SlotAssigned),
			},
		},
		{
			name: "scaleup-3=>4=>7",
			args: args{desired: 7, current: Allocate(4, Allocate(3, nil))},
		},
		{
			name: "scaleup-3=>7",
			args: args{desired: 7, current: Allocate(3, nil)},
			wantRet: []*Slots{
				testLoadSlots("0-2340", SlotAssigned),
				testLoadSlots("5462-7802", SlotAssigned),
				testLoadSlots("10923-13263", SlotAssigned),
				testLoadSlots("3121-5461", SlotAssigned),
				testLoadSlots("2341-3120,9363-10922", SlotAssigned),
				testLoadSlots("7803-9362,15604-16383", SlotAssigned),
				testLoadSlots("13264-15603", SlotAssigned),
			},
		},
		{
			name: "scaleup-4=>32",
			args: args{desired: 32, current: Allocate(4, nil)},
		},
		{
			name: "scaleup-4=>32=>128",
			args: args{desired: 128, current: Allocate(32, Allocate(4, nil))},
		},

		// scale down
		{
			name: "scaledown-3=>7=>3",
			args: args{desired: 3, current: Allocate(7, Allocate(3, nil))},
			wantRet: []*Slots{
				testLoadSlots("0-5461", SlotAssigned),
				testLoadSlots("5462-10922", SlotAssigned),
				testLoadSlots("10923-16383", SlotAssigned),
			},
		},
		{
			name: "scaledown-3=>4=>7=>3",
			args: args{desired: 3, current: Allocate(7, Allocate(4, Allocate(3, nil)))},
		},
		{
			name: "scaledown-3=>6=>3",
			args: args{desired: 3, current: Allocate(6, Allocate(3, nil))},
			wantRet: []*Slots{
				testLoadSlots("0-5461", SlotAssigned),
				testLoadSlots("5462-10922", SlotAssigned),
				testLoadSlots("10923-16383", SlotAssigned),
			},
		},
		{
			name: "scaledown-128=>4",
			args: args{desired: 4, current: Allocate(128, nil)},
		},
		{
			name: "scaledown-128=>32=>4",
			args: args{desired: 4, current: Allocate(32, Allocate(128, nil))},
			wantRet: []*Slots{
				testLoadSlots("0-127,512-1407,4096-4479,5632-8319", SlotAssigned),
				testLoadSlots("128-255,1408-2303,4480-4863,8320-11007", SlotAssigned),
				testLoadSlots("256-383,2304-3199,4864-5247,11008-13695", SlotAssigned),
				testLoadSlots("384-511,3200-4095,5248-5631,13696-16383", SlotAssigned),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRet := Allocate(tt.args.desired, tt.args.current)
			if tt.wantRet != nil && !reflect.DeepEqual(gotRet, tt.wantRet) {
				t.Errorf("Allocate() = %v, want = %v", gotRet, tt.wantRet)
			}
			if len(gotRet) != tt.args.desired {
				t.Errorf("Allocate() = %v, want = %d shard", gotRet, tt.args.desired)
			}

			allSlots := NewSlots()
			maxCount := int(math.Ceil(float64(RedisMaxSlots) / float64(tt.args.desired)))
			for _, slot := range gotRet {
				allSlots = allSlots.Union(slot)
				if count := slot.Count(SlotAssigned); count != maxCount && maxCount != count+1 {
					t.Errorf("Allocate() = %v, allocate gap large than 1", gotRet)
				}
			}
			if !allSlots.IsFullfilled() {
				t.Errorf("Allocate() = %v, IsFullfilled=%v", gotRet, allSlots.IsFullfilled())
			}
		})
	}
}
