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

package commands

import (
	"reflect"
	"testing"
)

func TestParseMonitorURI(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		want    []string
		wantErr bool
	}{
		{
			// Exactly what failoverbuilder writes on an IPv6 cluster; url.Parse rejects it.
			name: "three IPv6 sentinels as the builder writes them",
			uri:  "sentinel://[fd00:10:244::11]:26379,[fd00:10:244::12]:26379,[fd00:10:244::13]:26379",
			want: []string{"[fd00:10:244::11]:26379", "[fd00:10:244::12]:26379", "[fd00:10:244::13]:26379"},
		},
		{
			name: "three IPv4 sentinels",
			uri:  "sentinel://10.0.0.1:26379,10.0.0.2:26379,10.0.0.3:26379",
			want: []string{"10.0.0.1:26379", "10.0.0.2:26379", "10.0.0.3:26379"},
		},
		{
			name: "single IPv6 sentinel",
			uri:  "sentinel://[fd00::1]:26379",
			want: []string{"[fd00::1]:26379"},
		},
		{
			name: "bare IPv6 elements are made dialable",
			uri:  "sentinel://fd00::1:26379,fd00::2:26379",
			want: []string{"[fd00::1]:26379", "[fd00::2]:26379"},
		},
		{
			name: "hostnames",
			uri:  "sentinel://rfs-x-0.rfs-x.ns.svc:26379,rfs-x-1.rfs-x.ns.svc:26379",
			want: []string{"rfs-x-0.rfs-x.ns.svc:26379", "rfs-x-1.rfs-x.ns.svc:26379"},
		},
		{
			name: "no scheme",
			uri:  "10.0.0.1:26379,10.0.0.2:26379",
			want: []string{"10.0.0.1:26379", "10.0.0.2:26379"},
		},
		{
			name: "whitespace and a trailing comma",
			uri:  " sentinel://10.0.0.1:26379, 10.0.0.2:26379, ",
			want: []string{"10.0.0.1:26379", "10.0.0.2:26379"},
		},
		{name: "empty", uri: "", wantErr: true},
		{name: "scheme only", uri: "sentinel://", wantErr: true},
		{name: "element without a port", uri: "sentinel://10.0.0.1:26379,10.0.0.2", wantErr: true},
		{name: "element without a host", uri: "sentinel://:26379", wantErr: true},
		{name: "IPv6 element without a port", uri: "sentinel://[fd00::1]", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMonitorURI(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseMonitorURI(%q) error = %v, wantErr %v", tt.uri, err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseMonitorURI(%q) = %q, want %q", tt.uri, got, tt.want)
			}
		})
	}
}
