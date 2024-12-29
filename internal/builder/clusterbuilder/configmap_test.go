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

package clusterbuilder

import (
	"reflect"
	"testing"
)

func TestParseRenameConfigs(t *testing.T) {
	type args struct {
		val string
	}
	tests := []struct {
		name    string
		args    args
		wantRet map[string]string
		wantErr bool
	}{
		{
			name:    "forbid",
			args:    args{`flushall "" flushdb ""`},
			wantRet: map[string]string{"flushall": `""`, "flushdb": `""`},
			wantErr: false,
		},
		{
			name:    "disable forbid",
			args:    args{``},
			wantRet: map[string]string{},
			wantErr: false,
		},
		{
			name:    "disable forbid 2",
			args:    args{`flushall "" flushdb "" flushall "flushall" flushdb "flushdb"`},
			wantRet: map[string]string{},
			wantErr: false,
		},
		{
			name:    "rules with qoutes",
			args:    args{`set "abc"`},
			wantRet: map[string]string{"set": "abc"},
			wantErr: false,
		},
		{
			name:    "rules with qoutes",
			args:    args{`set 'abc'`},
			wantRet: map[string]string{"set": "abc"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRet, err := ParseRenameConfigs(tt.args.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRenameConfigs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRet, tt.wantRet) {
				t.Errorf("ParseRenameConfigs() = %v, want %v", gotRet, tt.wantRet)
			}
		})
	}
}
