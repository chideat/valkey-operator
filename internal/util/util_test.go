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
*/package util

import (
	"fmt"
	"testing"
)

func TestCheckRule(t *testing.T) {
	tests := []struct {
		input    string
		expected error
	}{
		{"allkeys +@example +@keyspace +@read", fmt.Errorf("acl rule group example is not allowed")},
		{"allkeys -@write +@write +@geo +@pubsub", fmt.Errorf("acl rule group write is duplicated")},
		{"allkeys +@list +@hash -@invalid", fmt.Errorf("acl rule group invalid is not allowed")},
		{"allkeys +@keyspace +@read +@write +cluster|info", nil},
		{"allkeys +@keyspace +@read +@write +cluster|info on", fmt.Errorf("acl rule on is not allowed")},
		{"~* +@all -keys", nil},
		{"~* dsada", fmt.Errorf("acl rule dsada is not allowed")},
		{"~* >dsada", fmt.Errorf("acl password rule >dsada is not allowed")},
		{"~* <dsada", fmt.Errorf("acl password rule <dsada is not allowed")},
		{"allkeys ~test +@all -acl -flushall -flushdb -keys", nil},
		{"allkeys ~test +@all $sd -flushall -flushdb -keys", fmt.Errorf("acl rule $sd is not allowed")},
	}

	for _, test := range tests {
		err := CheckRule(test.input)
		if (err == nil && test.expected != nil) || (err != nil && test.expected == nil) || (err != nil && err.Error() != test.expected.Error()) {
			t.Errorf("For input '%s', expected error: %v, got: %v", test.input, test.expected, err)
		}
	}
}

func TestCheckUserRuleUpdate(t *testing.T) {
	tests := []struct {
		name    string
		rule    string
		wantErr bool
	}{
		{
			name:    "Test with +acl rule",
			rule:    "+acl",
			wantErr: true,
		},
		{
			name:    "Test with +@slow rule and no -acl",
			rule:    "+@slow",
			wantErr: true,
		},
		{
			name:    "Test with valid rule",
			rule:    "-acl +@read",
			wantErr: false,
		},
		{
			name:    "Test with dup acl rule",
			rule:    "+acl -acl +acl",
			wantErr: true,
		},
		{name: "exampele 1",
			rule:    "allkeys +@all -@dangerous",
			wantErr: false,
		},
		{name: "example 2",
			rule:    "allkeys -@all +@write +@read -@dangerous",
			wantErr: false,
		},
		{
			name:    "example 3",
			rule:    "allkeys -@all +@read -keys",
			wantErr: false,
		},
		{
			name:    "example 4",
			rule:    "allkeys +@all -acl",
			wantErr: false,
		},
		{
			name:    "default",
			rule:    "allkeys +@all -acl -flushall -flushdb -keys",
			wantErr: false,
		},
		{name: "acl",
			rule:    "+acl",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckUserRuleUpdate(tt.rule)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckDefaultUserRuleUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
