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
	"reflect"
	"testing"
)

var (
	slotsA = &Slots{}
	slotsB = &Slots{}
	slotsC = &Slots{}
	slotsD = &Slots{}

	slotsInterDB = &Slots{}
	slotsInterDC = &Slots{}
	allSlots     *Slots
)

func init() {
	if err := slotsA.Set("0-5461", SlotAssigned); err != nil {
		panic(err)
	}
	if err := slotsB.Set("5462-10922", SlotAssigned); err != nil {
		panic(err)
	}
	if err := slotsC.Set("10923-16383", SlotAssigned); err != nil {
		panic(err)
	}
	if err := slotsD.Set("0-5461,5464,10922,16000-16111", SlotAssigned); err != nil {
		panic(err)
	}
	if err := slotsInterDB.Set("5464,10922", SlotAssigned); err != nil {
		panic(err)
	}
	if err := slotsInterDC.Set("16000-16111", SlotAssigned); err != nil {
		panic(err)
	}

	allSlots = slotsA.Union(slotsB, slotsC)
}

func TestNewSlotAssignStatusFromString(t *testing.T) {
	tests := []struct {
		name string
		v    string
		want SlotAssignStatus
	}{
		{"SlotImporting", "SlotImporting", SlotImporting},
		{"SlotAssigned", "SlotAssigned", SlotAssigned},
		{"SlotMigrating", "SlotMigrating", SlotMigrating},
		{"SlotUnassigned", "SlotUnassigned", SlotUnassigned},
		{"SlotUnassigned", "Unknown", SlotUnassigned},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewSlotAssignStatusFromString(tt.v)
			if got != tt.want {
				t.Errorf("NewSlotAssignStatusFromString() = %v, want %v", got, tt.want)
			}
			if got.String() != tt.name {
				t.Errorf("SlotAssignStatus.String() = %v, want %v", got.String(), tt.name)
			}
		})
	}
}

func TestSlots_IsFullfilled(t *testing.T) {
	tests := []struct {
		name         string
		s            *Slots
		wantFullfill bool
	}{
		{
			name:         "nil",
			s:            nil,
			wantFullfill: false,
		},
		{
			name:         "slotsA",
			s:            slotsA,
			wantFullfill: false,
		},
		{
			name:         "slotsB",
			s:            slotsB,
			wantFullfill: false,
		},
		{
			name:         "slotsC",
			s:            slotsC,
			wantFullfill: false,
		},
		{
			name:         "allSlots",
			s:            allSlots,
			wantFullfill: true,
		},
		{
			name:         "slotsD",
			s:            slotsD,
			wantFullfill: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.IsFullfilled(); got != tt.wantFullfill {
				t.Errorf("Slots.IsFullfilled() = %v, want %v, %v", got, tt.wantFullfill, tt.s.Slots())
			}
		})
	}
}

func TestSlots_Inter(t *testing.T) {
	type args struct {
		n *Slots
	}
	tests := []struct {
		name string
		s    *Slots
		args args
		want *Slots
	}{
		{
			name: "with_inter_d_b",
			s:    slotsD,
			args: args{
				n: slotsB,
			},
			want: slotsInterDB,
		},
		{
			name: "with_inter_d_c",
			s:    slotsD,
			args: args{
				n: slotsC,
			},
			want: slotsInterDC,
		},
		{
			name: "nil left",
			s:    nil,
			args: args{
				n: slotsB,
			},
			want: NewSlots(),
		},
		{
			name: "nil right",
			s:    slotsA,
			args: args{
				n: nil,
			},
			want: NewSlots(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.Inter(tt.args.n); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Slots.Inter() = %v, want %v", got.Slots(), tt.want.Slots())
			}
		})
	}
}

func TestSlots_Equals(t *testing.T) {
	slotsA := Slots{}
	slotsB := Slots{}
	slotsC := Slots{}
	slotsD := Slots{}
	for i := 0; i < 1000; i++ {
		if i < 500 {
			status := SlotAssigned
			if i%2 == 0 {
				status = SlotMigrating
			}
			_ = slotsA.Set(i, SlotAssignStatus(status))
		} else {
			_ = slotsA.Set(i, SlotImporting)
		}
	}
	_ = slotsB.Set("0-499", SlotAssigned)
	_ = slotsC.Set("0-498", SlotAssigned)
	_ = slotsC.Set("499", SlotImporting)
	_ = slotsD.Set("0-498", SlotAssigned)
	_ = slotsD.Set("499", SlotMigrating)
	type args struct {
		old *Slots
	}
	tests := []struct {
		name string
		s    *Slots
		args args
		want bool
	}{
		{
			name: "equals",
			s:    &slotsA,
			args: args{old: &slotsB},
			want: true,
		},
		{
			name: "nil left",
			s:    nil,
			args: args{old: &slotsB},
			want: false,
		},
		{
			name: "nil right",
			s:    &slotsA,
			args: args{old: nil},
			want: false,
		},
		{
			name: "nil left right",
			s:    nil,
			args: args{old: nil},
			want: true,
		},
		{
			name: "importing",
			s:    &slotsA,
			args: args{old: &slotsC},
			want: false,
		},
		{
			name: "migrating",
			s:    &slotsA,
			args: args{old: &slotsD},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.Equals(tt.args.old); got != tt.want {
				t.Errorf("Slots.Equals() = %v, want %v %v %v", got, tt.want, tt.s.Slots(), tt.args.old.Slots())
			}
		})
	}
}

func TestSlots_Union(t *testing.T) {
	type args struct {
		slots []*Slots
	}
	tests := []struct {
		name string
		s    *Slots
		args args
		want *Slots
	}{
		{
			name: "all",
			s:    slotsA,
			args: args{slots: []*Slots{slotsB, slotsC}},
			want: allSlots,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.Union(tt.args.slots...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Slots.Union() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSlots_String(t *testing.T) {
	slots := Slots{}
	_ = slots.Set(0, SlotAssigned)
	_ = slots.Set("1-100", SlotImporting)
	_ = slots.Set("1000-2000", SlotMigrating)
	_ = slots.Set("5000-10000", SlotAssigned)
	_ = slots.Set("16111,16121,16131", SlotImporting)
	_ = slots.Set("16112,16122,16132", SlotMigrating)
	_ = slots.Set("16113,16123,16153", SlotAssigned)
	_ = slots.Set("5201,5233,5400", SlotUnassigned)

	tests := []struct {
		name    string
		s       *Slots
		wantRet string
	}{
		{
			name:    "nil",
			s:       nil,
			wantRet: "",
		},
		{
			name:    "slots",
			s:       &slots,
			wantRet: "0,1000-2000,5000-5200,5202-5232,5234-5399,5401-10000,16112-16113,16122-16123,16132,16153",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotRet := tt.s.String(); !reflect.DeepEqual(gotRet, tt.wantRet) {
				t.Errorf("Slots.Slots() = %v, want %v", gotRet, tt.wantRet)
			}
		})
	}
}

func TestSlots_Status(t *testing.T) {
	slots := Slots{}
	_ = slots.Set(0, SlotAssigned)
	_ = slots.Set(100, SlotImporting)
	_ = slots.Set(1, SlotMigrating)
	_ = slots.Set(10000, SlotMigrating)
	_ = slots.Set(10000, SlotUnassigned)
	_ = slots.Set(11000, SlotImporting)
	_ = slots.Set(11000, SlotAssigned)
	type args struct {
		i int
	}

	tests := []struct {
		name string
		s    *Slots
		args args
		want SlotAssignStatus
	}{
		{
			name: "nil",
			s:    nil,
			args: args{i: 0},
			want: SlotUnassigned,
		},
		{
			name: "assigned",
			s:    &slots,
			args: args{i: 0},
			want: SlotAssigned,
		},
		{
			name: "importing",
			s:    &slots,
			args: args{i: 100},
			want: SlotImporting,
		},
		{
			name: "migrating",
			s:    &slots,
			args: args{i: 1},
			want: SlotMigrating,
		},
		{
			name: "unassigned",
			s:    &slots,
			args: args{i: 10000},
			want: SlotUnassigned,
		},
		{
			name: "importing=>assigined",
			s:    &slots,
			args: args{i: 11000},
			want: SlotAssigned,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.Status(tt.args.i); got != tt.want {
				t.Errorf("Slots.Status() = %v, want %v, %v", got, tt.want, tt.s)
			}
		})
	}
}

func TestSlots_Sub(t *testing.T) {
	var (
		slotsA = &Slots{}
		slotsB = &Slots{}
		slotsC = &Slots{}

		slotsDiff = &Slots{}
	)

	_ = slotsA.Set("0-1000", SlotImporting)
	_ = slotsA.Set("999-2000", SlotAssigned)
	_ = slotsA.Set("2001-3000", SlotMigrating)
	_ = slotsA.Set("5001-6000", SlotUnassigned)

	_ = slotsB.Set("0-1000", SlotAssigned)
	_ = slotsB.Set("999-2000", SlotMigrating)
	_ = slotsB.Set("2001-3000", SlotAssigned)
	_ = slotsB.Set("5001-6000", SlotAssigned)

	_ = slotsC.Set("0-998", SlotMigrating)
	_ = slotsC.Set("2000", SlotAssigned)
	_ = slotsC.Set("2100", SlotAssigned)
	_ = slotsC.Set("2101", SlotMigrating)
	_ = slotsC.Set("2102", SlotImporting)
	_ = slotsC.Set("2103", SlotUnassigned)
	_ = slotsC.Set("5001", SlotAssigned)
	_ = slotsC.Set("5002", SlotImporting)
	_ = slotsC.Set("5003", SlotMigrating)

	_ = slotsDiff.Set("999-1999,2001-2099,2102-3000", SlotAssigned)

	type args struct {
		n *Slots
	}
	tests := []struct {
		name string
		s    *Slots
		args args
		want *Slots
	}{
		{
			name: "no sub",
			s:    slotsA,
			args: args{n: slotsB},
			want: &Slots{},
		},
		{
			name: "sub",
			s:    slotsA,
			args: args{n: slotsC},
			want: slotsDiff,
		},
		{
			name: "nil right",
			s:    slotsA,
			args: args{n: nil},
			want: slotsA,
		},
		{
			name: "nil left",
			s:    nil,
			args: args{n: slotsA},
			want: NewSlots(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.Sub(tt.args.n); !reflect.DeepEqual(got.data, tt.want.data) {
				t.Errorf("Slots.Sub() = %v, want %v", got.Slots(), tt.want.Slots())
			}
		})
	}
}

func TestSlots_Load(t *testing.T) {
	slots := NewSlots()
	type args struct {
		v interface{}
	}
	tests := []struct {
		name    string
		slots   *Slots
		args    args
		wantErr bool
	}{
		{
			name:  "nil",
			slots: nil,
			args: args{
				v: []string{"5000-5100", "[1002-<-67ed2db8d677e59ec4a4cefb06858cf2a1a89fa1]", "[77->-e7d1eecce10fd6bb5eb35b9f99a514335d9ba9ca]"},
			},
			wantErr: false,
		},
		{
			name:  "load",
			slots: slots,
			args: args{
				v: []string{"5000-5100", "[1002-<-67ed2db8d677e59ec4a4cefb06858cf2a1a89fa1]", "[77->-e7d1eecce10fd6bb5eb35b9f99a514335d9ba9ca]"},
			},
			wantErr: false,
		},
		{
			name:  "load more",
			slots: slots,
			args: args{
				v: []string{"5000-5100,10000-16666"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.slots.Load(tt.args.v); (err != nil) != tt.wantErr {
				t.Errorf("Slots.Load() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSlots_Count(t *testing.T) {
	slots := NewSlots()
	if err := slots.Load([]string{
		"1-100",
		"[1002-<-67ed2db8d677e59ec4a4cefb06858cf2a1a89fa1]",
		"77->-e7d1eecce10fd6bb5eb35b9f99a514335d9ba9ca]",
	}); err != nil {
		t.Fatal(err)
	}

	type args struct {
		status SlotAssignStatus
	}
	tests := []struct {
		name  string
		slots *Slots
		args  args
		wantC int
	}{
		{
			name:  "nil",
			slots: nil,
			args: args{
				status: SlotAssigned,
			},
			wantC: 0,
		},
		{
			name:  "assigned",
			slots: slots,
			args: args{
				status: SlotAssigned,
			},
			wantC: 100,
		},
		{
			name:  "importing",
			slots: slots,
			args: args{
				status: SlotImporting,
			},
			wantC: 1,
		},
		{
			name:  "migrating",
			slots: slots,
			args: args{
				status: SlotMigrating,
			},
			wantC: 1,
		},
		{
			name:  "unassigned",
			slots: slots,
			args: args{
				status: SlotUnassigned,
			},
			wantC: 16284,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotC := tt.slots.Count(tt.args.status); gotC != tt.wantC {
				t.Errorf("Slots.Count() = %v, want %v. %v", gotC, tt.wantC, tt.slots.Slots())
			}
		})
	}
}

func TestLoadSlots(t *testing.T) {
	tests := []struct {
		name    string
		v       any
		wantErr bool
	}{
		{
			name:    "number",
			v:       100,
			wantErr: false,
		},
		{
			name:    "number2",
			v:       []int{100, 200},
			wantErr: false,
		},
		{
			name:    "invalid number",
			v:       "xxxx",
			wantErr: true,
		},
		{
			name:    "number str",
			v:       "0-100",
			wantErr: false,
		},
		{
			name:    "invalid number str",
			v:       "xxx-yyy",
			wantErr: true,
		},
		{
			name:    "invalid number str",
			v:       "-",
			wantErr: true,
		},
		{
			name:    "not complete range",
			v:       "0-100,200-",
			wantErr: true,
		},
		{
			name:    "invalid range",
			v:       "200-100",
			wantErr: true,
		},
		{
			name:    "not complete migrate",
			v:       "77->-",
			wantErr: true,
		},
		{
			name:    "not complete migrate",
			v:       "->-e7d1eecce10fd6bb5eb35b9f99a514335d9ba9ca",
			wantErr: true,
		},
		{
			name:    "invalid slot migrate",
			v:       "xxx->-e7d1eecce10fd6bb5eb35b9f99a514335d9ba9ca",
			wantErr: true,
		},
		{
			name:    "not complete import",
			v:       "77-<-",
			wantErr: true,
		},
		{
			name:    "not complete import",
			v:       "-<-e7d1eecce10fd6bb5eb35b9f99a514335d9ba9ca",
			wantErr: true,
		},
		{
			name:    "invalid slot import",
			v:       "xxx-<-e7d1eecce10fd6bb5eb35b9f99a514335d9ba9ca",
			wantErr: true,
		},
		{
			name:    "error",
			v:       struct{}{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadSlots(tt.v)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadSlots() error = %v, wantErr %v", err, false)
			}
		})
	}
}

func TestSetSlots(t *testing.T) {
	tests := []struct {
		name    string
		v       any
		wantErr bool
	}{
		{
			name:    "number",
			v:       100,
			wantErr: false,
		},
		{
			name:    "number2",
			v:       []int{100, 200},
			wantErr: false,
		},
		{
			name:    "invalid number",
			v:       "xxxx",
			wantErr: true,
		},
		{
			name:    "number str",
			v:       "0-100",
			wantErr: false,
		},
		{
			name:    "invalid number str",
			v:       "xxx-yyy",
			wantErr: true,
		},
		{
			name:    "invalid number str",
			v:       "-",
			wantErr: true,
		},
		{
			name:    "not complete range",
			v:       "0-100,200-",
			wantErr: true,
		},
		{
			name:    "invalid range",
			v:       "200-100",
			wantErr: true,
		},
		{
			name:    "string arr",
			v:       []string{"0-100", "200-300"},
			wantErr: false,
		},
		{
			name:    "string arr",
			v:       []string{"0-100", "200-"},
			wantErr: true,
		},
		{
			name:    "not complete migrate",
			v:       "77->-",
			wantErr: true,
		},
		{
			name:    "not complete migrate",
			v:       "->-e7d1eecce10fd6bb5eb35b9f99a514335d9ba9ca",
			wantErr: true,
		},
		{
			name:    "invalid slot migrate",
			v:       "xxx->-e7d1eecce10fd6bb5eb35b9f99a514335d9ba9ca",
			wantErr: true,
		},
		{
			name:    "not complete import",
			v:       "77-<-",
			wantErr: true,
		},
		{
			name:    "not complete import",
			v:       "-<-e7d1eecce10fd6bb5eb35b9f99a514335d9ba9ca",
			wantErr: true,
		},
		{
			name:    "invalid slot import",
			v:       "xxx-<-e7d1eecce10fd6bb5eb35b9f99a514335d9ba9ca",
			wantErr: true,
		},
		{
			name:    "error",
			v:       struct{}{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slot := NewSlots()
			err := slot.Set(tt.v, SlotAssigned)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetSlots() error = %v, wantErr %v", err, false)
			}
		})
	}
}

func TestNewFullSlots(t *testing.T) {
	slots := NewFullSlots()
	if !slots.IsFullfilled() {
		t.Errorf("NewFullSlots() = not fullfilled, want fullfilled")
	}
	if !IsFullfilled(slots) {
		t.Errorf("IsFullfilled() = fullfilled, want not fullfilled")
	}
}

func TestSlots_SlotsByStatus(t *testing.T) {
	slots := NewSlots()
	_ = slots.Set("0-100", SlotAssigned)
	if len(slots.SlotsByStatus(SlotAssigned)) != 101 {
		t.Errorf("Slots.SlotsByStatus() = %v, want %v", len(slots.SlotsByStatus(SlotAssigned)), 101)
	}
	if val := (*Slots)(nil).SlotsByStatus(SlotAssigned); val != nil {
		t.Errorf("Slots.SlotsByStatus() = %v, want %v", val, nil)
	}
}

func TestSlots_IsSet(t *testing.T) {
	slots := NewSlots()
	_ = slots.Set("0-100", SlotAssigned)
	if !slots.IsSet(50) {
		t.Errorf("Slots.IsSet() = false, want true")
	}
	if (*Slots)(nil).IsSet(0) {
		t.Errorf("Slots.IsSet() = true, want false")
	}
}

func TestSlots_MoveingStatus(t *testing.T) {
	slots := NewSlots()
	_ = slots.Set("0-100", SlotImporting)
	_ = slots.Set("200-300", SlotMigrating)
	status, _ := slots.MoveingStatus(50)
	if status != SlotImporting {
		t.Errorf("Slots.MoveingStatus() = %v, want %v", status, SlotImporting)
	}
	status, _ = slots.MoveingStatus(201)
	if status != SlotMigrating {
		t.Errorf("Slots.MoveingStatus() = %v, want %v", status, SlotMigrating)
	}
	status, _ = slots.MoveingStatus(401)
	if status != SlotUnassigned {
		t.Errorf("Slots.MoveingStatus() = %v, want %v", status, SlotUnassigned)
	}
	if status, _ := (*Slots)(nil).MoveingStatus(0); status != SlotUnassigned {
		t.Errorf("Slots.MoveingStatus() = %v, want %v", status, SlotUnassigned)
	}
}
