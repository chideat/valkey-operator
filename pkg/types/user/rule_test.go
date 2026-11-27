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

package user

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/chideat/valkey-operator/pkg/version"
)

func TestNewRule(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{
			name:    "empty",
			raw:     "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "allcommands",
			raw:     "allcommands",
			want:    "+@all",
			wantErr: false,
		},
		{
			name:    "nocommands",
			raw:     "nocommands",
			want:    "-@all",
			wantErr: false,
		},
		{
			name:    "allchannels",
			raw:     "allchannels",
			want:    "&*",
			wantErr: false,
		},
		{
			name:    "read key",
			raw:     "%R~test",
			want:    "%R~test",
			wantErr: false,
		},
		{
			name:    "write key",
			raw:     "%W~test",
			want:    "%W~test",
			wantErr: false,
		},
		{
			name:    "test",
			raw:     "allkeys +@all -flushall -flushdb",
			want:    "+@all -flushall -flushdb ~*",
			wantErr: false,
		},
		{
			name:    "notdangerous",
			raw:     "allkeys +@all -@dangerous",
			want:    "+@all -@dangerous ~*",
			wantErr: false,
		},
		{
			name:    "readwrite",
			raw:     "allkeys -@all +@write +@read -@dangerous",
			want:    "-@all +@write +@read -@dangerous ~*",
			wantErr: false,
		},
		{
			name:    "readonly",
			raw:     "allkeys -@all +@read -keys",
			want:    "-@all +@read -keys ~*",
			wantErr: false,
		},
		{
			name:    "administrator",
			raw:     "allkeys +@all -acl",
			want:    "+@all -acl ~*",
			wantErr: false,
		},
		{
			name:    "support subcommand",
			raw:     "allkeys -@admin +config|get",
			want:    "-@admin +config|get ~*",
			wantErr: false,
		},
		{
			name:    "disable cmd enable subcommand",
			raw:     "allkeys -config +config|get",
			want:    "-config +config|get ~*",
			wantErr: false,
		},
		{
			name:    "fixed acl",
			raw:     "+@all -acl +acl|setuser -flushall -flushdb -keys ~* &*",
			want:    "+@all -acl -flushall -flushdb -keys +acl|setuser ~* &*",
			wantErr: false,
		},
		{
			name:    "withpassword",
			raw:     "allkeys +@all >admin@123",
			wantErr: true,
		},
		{
			name:    "sanitize-payload",
			raw:     "allkeys +@all sanitize-payload",
			wantErr: true,
		},
		{
			name:    "skip-sanitize-payload",
			raw:     "allkeys +@all skip-sanitize-payload",
			wantErr: true,
		},
		{
			name:    "not allowed category",
			raw:     "+@test",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := NewRule(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if rule != nil {
				if rule.Encode() != tt.want {
					t.Errorf("NewRule() = %v, want %v", rule, tt.want)
				}
			}
		})
	}
}

func TestRule_Format(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{
			name:    "test",
			raw:     "allkeys +@all -flushall -flushdb",
			want:    "+@all -flushall -flushdb ~*",
			wantErr: false,
		},
		{
			name:    "notdangerous",
			raw:     "allkeys +@all -@dangerous",
			want:    "+@all -@dangerous ~*",
			wantErr: false,
		},
		{
			name:    "readwrite",
			raw:     "allkeys -@all +@write +@read -@dangerous",
			want:    "-@all +@write +@read -@dangerous ~*",
			wantErr: false,
		},
		{
			name:    "readonly",
			raw:     "allkeys -@all +@read -keys",
			want:    "-@all +@read -keys ~*",
			wantErr: false,
		},
		{
			name:    "administrator",
			raw:     "allkeys +@all -acl",
			want:    "+@all -acl ~*",
			wantErr: false,
		},
		{
			name:    "support subcommand",
			raw:     "allkeys -@admin +config|get",
			want:    "-@admin +config|get ~*",
			wantErr: false,
		},
		{
			name:    "disable cmd enable subcommand",
			raw:     "allkeys -config +config|get",
			want:    "-config +config|get ~*",
			wantErr: false,
		},
		{
			name:    "fixed acl",
			raw:     "+@all -acl +acl|setuser -flushall -flushdb -keys ~* &*",
			want:    "+@all -acl -flushall -flushdb -keys +acl|setuser ~* &*",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := NewRule(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRule() error = %v, wantErr: %v", err, tt.wantErr)
				return
			}
			if rule != nil {
				if got := rule.Encode(); got != tt.want {
					t.Errorf("Rule.Encode() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestRule_Validate(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		disableAcl bool
		wantErr    bool
	}{
		{
			name:       "test",
			raw:        "allkeys +@all -flushall -flushdb",
			disableAcl: true,
			wantErr:    true,
		},
		{
			name:       "notdangerous",
			raw:        "allkeys +@all -@dangerous",
			disableAcl: true,
			wantErr:    false,
		},
		{
			name:       "readwrite",
			raw:        "allkeys -@all +@write +@read -@dangerous",
			disableAcl: true,
			wantErr:    false,
		},
		{
			name:       "readonly",
			raw:        "allkeys -@all +@read -keys",
			disableAcl: true,
			wantErr:    false,
		},
		{
			name:       "administrator",
			raw:        "allkeys +@all -acl",
			disableAcl: true,
			wantErr:    false,
		},
		{
			name:       "support subcommand",
			raw:        "allkeys -@admin +config|get",
			disableAcl: true,
			wantErr:    false,
		},
		{
			name:       "disable cmd enable subcommand",
			raw:        "allkeys -config +config|get",
			disableAcl: true,
			wantErr:    false,
		},
		{
			name:       "databases",
			raw:        "+@all -@dangerous +flushdb ~app:* &* db=2,3",
			disableAcl: true,
			wantErr:    false,
		},
		{
			name:       "default user for 7.0",
			raw:        "+@all -acl -flushall -flushdb -keys ~* &*",
			disableAcl: true,
			wantErr:    false,
		},
		{
			name:       "fixed acl",
			raw:        "+@all -acl +acl|setuser -flushall -flushdb -keys ~* &*",
			disableAcl: true,
			wantErr:    true,
		},
		{
			name:       "fixed acl",
			raw:        "+@all -acl +acl|setuser -flushall -flushdb -keys ~* &*",
			disableAcl: true,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewRule(tt.raw)
			if err != nil {
				t.Errorf("NewRule() error = %v, ", err)
				return
			}
			if err := r.Validate(tt.disableAcl); (err != nil) != tt.wantErr {
				t.Errorf("Rule.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewRule_Databases(t *testing.T) {
	tests := []struct {
		name            string
		raw             string
		want            string
		wantDatabases   []int
		wantNoDatabases bool
	}{
		{
			name:          "single database",
			raw:           "db=0",
			want:          "db=0",
			wantDatabases: []int{0},
		},
		{
			name:          "several databases",
			raw:           "db=2,3",
			want:          "db=2,3",
			wantDatabases: []int{2, 3},
		},
		{
			name:          "sorted and deduplicated",
			raw:           "db=3,1,3,2",
			want:          "db=1,2,3",
			wantDatabases: []int{1, 2, 3},
		},
		{
			name:          "largest id",
			raw:           "db=2147483647",
			want:          "db=2147483647",
			wantDatabases: []int{2147483647},
		},
		{
			name:          "leading zeros",
			raw:           "db=007",
			want:          "db=7",
			wantDatabases: []int{7},
		},
		{
			name:          "case insensitive",
			raw:           "DB=4",
			want:          "db=4",
			wantDatabases: []int{4},
		},
		{
			name:          "later db= replaces the earlier one",
			raw:           "db=1,2 db=3",
			want:          "db=3",
			wantDatabases: []int{3},
		},
		{
			name: "alldbs is the default",
			raw:  "alldbs",
			want: "",
		},
		{
			name: "alldbs after db=",
			raw:  "db=1 ALLDBS",
			want: "",
		},
		{
			name:            "resetdbs",
			raw:             "resetdbs",
			want:            "resetdbs",
			wantNoDatabases: true,
		},
		{
			name:            "resetdbs after db=",
			raw:             "db=1 ResetDBs",
			want:            "resetdbs",
			wantNoDatabases: true,
		},
		{
			name:          "db= after resetdbs",
			raw:           "resetdbs db=5",
			want:          "db=5",
			wantDatabases: []int{5},
		},
		{
			name:          "with other rules",
			raw:           "+@all -@dangerous +flushdb ~dashboard:* &* db=2,3",
			want:          "+@all -@dangerous +flushdb ~dashboard:* &* db=2,3",
			wantDatabases: []int{2, 3},
		},
		{
			name:          "db= first",
			raw:           "db=2,3 +@all -@dangerous +flushdb ~dashboard:* &*",
			want:          "+@all -@dangerous +flushdb ~dashboard:* &* db=2,3",
			wantDatabases: []int{2, 3},
		},
		{
			name:          "db= between rules",
			raw:           "allkeys db=1 -@all +@read -keys %R~cache:* &*",
			want:          "-@all +@read -keys ~* %R~cache:* &* db=1",
			wantDatabases: []int{1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := NewRule(tt.raw)
			if err != nil {
				t.Fatalf("NewRule() error = %v", err)
			}
			if !reflect.DeepEqual(rule.Databases, tt.wantDatabases) {
				t.Errorf("NewRule().Databases = %v, want %v", rule.Databases, tt.wantDatabases)
			}
			if rule.NoDatabases != tt.wantNoDatabases {
				t.Errorf("NewRule().NoDatabases = %v, want %v", rule.NoDatabases, tt.wantNoDatabases)
			}
			if rule.HasDatabaseRules() != (tt.wantNoDatabases || len(tt.wantDatabases) > 0) {
				t.Errorf("Rule.HasDatabaseRules() = %v", rule.HasDatabaseRules())
			}
			got := rule.Encode()
			if got != tt.want {
				t.Errorf("Rule.Encode() = %q, want %q", got, tt.want)
			}

			// round trip: what is encoded parses back to the same rule, and encodes the same
			again, err := NewRule(got)
			if err != nil {
				t.Fatalf("NewRule(%q) error = %v", got, err)
			}
			if !reflect.DeepEqual(again, rule) {
				t.Errorf("NewRule(Encode()) = %+v, want %+v", again, rule)
			}
			if again.Encode() != got {
				t.Errorf("Encode() is not stable: %q, then %q", got, again.Encode())
			}

			// the rule is stored as JSON in the ACL ConfigMap
			data, err := json.Marshal(rule)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			var decoded Rule
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if decoded.Encode() != got {
				t.Errorf("Encode() after JSON round trip = %q, want %q", decoded.Encode(), got)
			}
		})
	}
}

func TestNewRule_InvalidDatabases(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "no id", raw: "db=", wantErr: "invalid rule db=: no database id"},
		{name: "not a number", raw: "db=a", wantErr: `database id "a" is not a non-negative integer`},
		{name: "empty item", raw: "db=1,,2", wantErr: "empty database id"},
		{name: "leading comma", raw: "db=,1", wantErr: "empty database id"},
		{name: "trailing comma", raw: "db=1,", wantErr: "empty database id"},
		{name: "negative", raw: "db=-1", wantErr: `database id "-1" is not a non-negative integer`},
		{name: "plus sign", raw: "db=+1", wantErr: `database id "+1" is not a non-negative integer`},
		{name: "fraction", raw: "db=1.5", wantErr: `database id "1.5" is not a non-negative integer`},
		{name: "out of range", raw: "db=2147483648", wantErr: `database id "2147483648" is out of range`},
		{name: "overflow", raw: "db=99999999999999999999", wantErr: `database id "99999999999999999999" is out of range`},
		{name: "among other rules", raw: "+@all ~* db=1,x", wantErr: `invalid rule db=1,x: database id "x" is not a non-negative integer`},
		{name: "db without =", raw: "db", wantErr: "unsupported rule db"},
		{name: "colon instead of =", raw: "db:1", wantErr: "unsupported rule db:1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := NewRule(tt.raw)
			if err == nil {
				t.Fatalf("NewRule(%q) = %+v, want error", tt.raw, rule)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("NewRule(%q) error = %q, want it to contain %q", tt.raw, err, tt.wantErr)
			}
		})
	}
}

func TestRule_CheckVersion(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		version version.ValkeyVersion
		wantErr bool
	}{
		{name: "no database rule, unknown version", raw: "+@all ~*", version: version.ValkeyVersionUnknown},
		{name: "no database rule, 8.0", raw: "+@all ~*", version: "8.0"},
		{name: "alldbs, 8.0", raw: "+@all ~* alldbs", version: "8.0"},
		{name: "db=, unknown version", raw: "+@all ~* db=1", version: version.ValkeyVersionUnknown, wantErr: true},
		{name: "db=, 8.1", raw: "+@all ~* db=1", version: "8.1", wantErr: true},
		{name: "db=, 9.0", raw: "+@all ~* db=1", version: "9.0", wantErr: true},
		{name: "db=, 9.1", raw: "+@all ~* db=1", version: "9.1"},
		{name: "db=, 9.2", raw: "+@all ~* db=1", version: "9.2"},
		{name: "db=, 10.0", raw: "+@all ~* db=1", version: "10.0"},
		{name: "resetdbs, 9.0", raw: "+@all ~* resetdbs", version: "9.0", wantErr: true},
		{name: "resetdbs, 9.1", raw: "+@all ~* resetdbs", version: "9.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := NewRule(tt.raw)
			if err != nil {
				t.Fatalf("NewRule() error = %v", err)
			}
			if err := rule.CheckVersion(tt.version); (err != nil) != tt.wantErr {
				t.Errorf("Rule.CheckVersion(%q) error = %v, wantErr %v", tt.version, err, tt.wantErr)
			}
		})
	}
}
