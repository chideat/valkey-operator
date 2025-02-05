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
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/go-logr/logr"
)

func Test_portClusterNodesConf(t *testing.T) {
	type args struct {
		ctx    context.Context
		data   []byte
		logger logr.Logger
		opts   *HealOptions
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "new init node",
			args: args{
				ctx: context.Background(),
				data: []byte(`267600f4b192a940a20759aa0ebeee22f41d69e6 :0@0,shard-id=8a6d146963271fcd409b36704c86650827ef73a1 myself,master - 0 0 0 connected
vars currentEpoch 0 lastVoteEpoch 0`),
				logger: logr.Discard(),
				opts: &HealOptions{
					Namespace:  "default",
					PodName:    "drc-c77-0-0",
					Workspace:  "/data",
					TargetName: "nodes.conf",
					Prefix:     "sync-",
					ShardID:    "8a6d146963271fcd409b36704c86650827ef73a1",
					NodeFile:   "/data/nodes.conf",
				},
			},
			want: []byte(`267600f4b192a940a20759aa0ebeee22f41d69e6 :0@0,shard-id=8a6d146963271fcd409b36704c86650827ef73a1 myself,master - 0 0 0 connected
vars currentEpoch 0 lastVoteEpoch 0`),
			wantErr: false,
		},
		{
			name: "6 nodes",
			args: args{
				ctx: context.Background(),
				data: []byte(`f89575a0d78cdc25b5ea1886bb88f1d979026c7d 192.168.132.183:32295@31500 slave c8b765997335f66f892ca6840f7f0b6df8200638 0 1709546093510 1 connected
c8b765997335f66f892ca6840f7f0b6df8200638 192.168.132.208:30471@30566 master - 0 1709546095505 1 connected 10923-16383
4cc7fd15a841f081f8c956b0432f75baa170ea97 192.168.132.208:30969@30670 slave a8fa11eb3c3cc0c8115b8fe191d1e8ce92b857a5 0 1709546094000 2 connected
e8cd1219f9d712f7a1002962625f4f6ab46e4a69 192.168.132.209:31741@30771 myself,master - 0 1709546091000 3 connected 0-5461
c4db03ea65954e1c2ced6135b8622360b5bf6ca7 192.168.132.183:31176@32087 slave e8cd1219f9d712f7a1002962625f4f6ab46e4a69 0 1709546092000 3 connected
a8fa11eb3c3cc0c8115b8fe191d1e8ce92b857a5 192.168.132.183:30550@31597 master - 0 1709546094511 2 connected 5462-10922
vars currentEpoch 5 lastVoteEpoch 0`),
				logger: logr.Discard(),
				opts: &HealOptions{
					Namespace:  "default",
					PodName:    "drc-c6-0-0",
					Workspace:  "/data",
					TargetName: "nodes.conf",
					Prefix:     "sync-",
					ShardID:    "453e29079a2d30ac8122692ac4a1d1c9390acadf",
					NodeFile:   "/data/nodes.conf",
				},
			},
			want: []byte(`f89575a0d78cdc25b5ea1886bb88f1d979026c7d 192.168.132.183:32295@31500 slave c8b765997335f66f892ca6840f7f0b6df8200638 0 1709546093510 1 connected
c8b765997335f66f892ca6840f7f0b6df8200638 192.168.132.208:30471@30566 master - 0 1709546095505 1 connected 10923-16383
4cc7fd15a841f081f8c956b0432f75baa170ea97 192.168.132.208:30969@30670 slave a8fa11eb3c3cc0c8115b8fe191d1e8ce92b857a5 0 1709546094000 2 connected
e8cd1219f9d712f7a1002962625f4f6ab46e4a69 192.168.132.209:31741@30771,,tls-port=0,shard-id=453e29079a2d30ac8122692ac4a1d1c9390acadf myself,master - 0 1709546091000 3 connected 0-5461
c4db03ea65954e1c2ced6135b8622360b5bf6ca7 192.168.132.183:31176@32087,,tls-port=0,shard-id=453e29079a2d30ac8122692ac4a1d1c9390acadf slave e8cd1219f9d712f7a1002962625f4f6ab46e4a69 0 1709546092000 3 connected
a8fa11eb3c3cc0c8115b8fe191d1e8ce92b857a5 192.168.132.183:30550@31597 master - 0 1709546094511 2 connected 5462-10922
vars currentEpoch 5 lastVoteEpoch 0`),
			wantErr: false,
		},
		{
			name: "fix 7 nodes",
			args: args{
				ctx: context.Background(),
				data: []byte(`14c4e058a702f5f3d8f1cd8b70cc3dd450f531ec 192.168.132.210:30541@32045,,tls-port=0,shard-id=78524c978001da077a73c07e1bde14f0eea5eb7d slave be0453515af6a6b9f6dbf96643604cbfc9517792 0 1709542562827 1 connected
c426f0d5a3986d926497f3e925887e4db9ea4e12 192.168.132.183:32304@31460,,tls-port=0,shard-id=da7d249987641fb2379cce47636469153c5e7e58 slave 41631ee411f0c6b1c25742f867ebbb1a63856772 0 1709542563825 2 connected
a1127158a63694800ac4e1187cc9fec7cae95dda 192.168.132.209:32287@31795,,tls-port=0,shard-id=a262f15901feff7a44f73a8d64210708909db3a5 slave cce647d1c72bcac30aee07128c3aa1493405630e 0 1709542563000 0 connected
cce647d1c72bcac30aee07128c3aa1493405630e 192.168.132.208:31513@30183,,tls-port=0,shard-id=a262f15901feff7a44f73a8d64210708909db3a5 master - 0 1709542561000 0 connected 5462-10922
41631ee411f0c6b1c25742f867ebbb1a63856772 192.168.132.209:30707@30244,,tls-port=0,shard-id=da7d249987641fb2379cce47636469153c5e7e58 master - 0 1709542563000 2 connected 10923-16383
be0453515af6a6b9f6dbf96643604cbfc9517792 192.168.132.183:31200@32418,,tls-port=0,shard-id=78524c978001da077a73c07e1bde14f0eea5eb7d myself,master - 0 1709542561000 1 connected 0-5461
vars currentEpoch 5 lastVoteEpoch 0`),
				logger: logr.Discard(),
				opts: &HealOptions{
					Namespace:  "default",
					PodName:    "drc-c7-0-0",
					Workspace:  "/data",
					TargetName: "nodes.conf",
					Prefix:     "sync-",
					ShardID:    "453e29079a2d30ac8122692ac4a1d1c9390acadf",
					NodeFile:   "/data/nodes.conf",
				},
			},
			want: []byte(`14c4e058a702f5f3d8f1cd8b70cc3dd450f531ec 192.168.132.210:30541@32045,,tls-port=0,shard-id=453e29079a2d30ac8122692ac4a1d1c9390acadf slave be0453515af6a6b9f6dbf96643604cbfc9517792 0 1709542562827 1 connected
c426f0d5a3986d926497f3e925887e4db9ea4e12 192.168.132.183:32304@31460,,tls-port=0,shard-id=da7d249987641fb2379cce47636469153c5e7e58 slave 41631ee411f0c6b1c25742f867ebbb1a63856772 0 1709542563825 2 connected
a1127158a63694800ac4e1187cc9fec7cae95dda 192.168.132.209:32287@31795,,tls-port=0,shard-id=a262f15901feff7a44f73a8d64210708909db3a5 slave cce647d1c72bcac30aee07128c3aa1493405630e 0 1709542563000 0 connected
cce647d1c72bcac30aee07128c3aa1493405630e 192.168.132.208:31513@30183,,tls-port=0,shard-id=a262f15901feff7a44f73a8d64210708909db3a5 master - 0 1709542561000 0 connected 5462-10922
41631ee411f0c6b1c25742f867ebbb1a63856772 192.168.132.209:30707@30244,,tls-port=0,shard-id=da7d249987641fb2379cce47636469153c5e7e58 master - 0 1709542563000 2 connected 10923-16383
be0453515af6a6b9f6dbf96643604cbfc9517792 192.168.132.183:31200@32418,,tls-port=0,shard-id=453e29079a2d30ac8122692ac4a1d1c9390acadf myself,master - 0 1709542561000 1 connected 0-5461
vars currentEpoch 5 lastVoteEpoch 0`),
			wantErr: false,
		},
		{
			name: "fix 7 myself,slave",
			args: args{
				ctx: context.Background(),
				data: []byte(`14c4e058a702f5f3d8f1cd8b70cc3dd450f531ec 192.168.132.210:30541@32045,,tls-port=0,shard-id=78524c978001da077a73c07e1bde14f0eea5eb7d myself,slave be0453515af6a6b9f6dbf96643604cbfc9517792 0 1709542562827 1 connected
c426f0d5a3986d926497f3e925887e4db9ea4e12 192.168.132.183:32304@31460,,tls-port=0,shard-id=da7d249987641fb2379cce47636469153c5e7e58 slave 41631ee411f0c6b1c25742f867ebbb1a63856772 0 1709542563825 2 connected
a1127158a63694800ac4e1187cc9fec7cae95dda 192.168.132.209:32287@31795,,tls-port=0,shard-id=a262f15901feff7a44f73a8d64210708909db3a5 slave cce647d1c72bcac30aee07128c3aa1493405630e 0 1709542563000 0 connected
cce647d1c72bcac30aee07128c3aa1493405630e 192.168.132.208:31513@30183,,tls-port=0,shard-id=a262f15901feff7a44f73a8d64210708909db3a5 master - 0 1709542561000 0 connected 5462-10922
41631ee411f0c6b1c25742f867ebbb1a63856772 192.168.132.209:30707@30244,,tls-port=0,shard-id=da7d249987641fb2379cce47636469153c5e7e58 master - 0 1709542563000 2 connected 10923-16383
be0453515af6a6b9f6dbf96643604cbfc9517792 192.168.132.183:31200@32418,,tls-port=0,shard-id=78524c978001da077a73c07e1bde14f0eea5eb7d master - 0 1709542561000 1 connected 0-5461
vars currentEpoch 5 lastVoteEpoch 0`),
				logger: logr.Discard(),
				opts: &HealOptions{
					Namespace:  "default",
					PodName:    "drc-c7-0-0",
					Workspace:  "/data",
					TargetName: "nodes.conf",
					Prefix:     "sync-",
					ShardID:    "453e29079a2d30ac8122692ac4a1d1c9390acadf",
					NodeFile:   "/data/nodes.conf",
				},
			},
			want: []byte(`14c4e058a702f5f3d8f1cd8b70cc3dd450f531ec 192.168.132.210:30541@32045,,tls-port=0,shard-id=453e29079a2d30ac8122692ac4a1d1c9390acadf myself,slave be0453515af6a6b9f6dbf96643604cbfc9517792 0 1709542562827 1 connected
c426f0d5a3986d926497f3e925887e4db9ea4e12 192.168.132.183:32304@31460,,tls-port=0,shard-id=da7d249987641fb2379cce47636469153c5e7e58 slave 41631ee411f0c6b1c25742f867ebbb1a63856772 0 1709542563825 2 connected
a1127158a63694800ac4e1187cc9fec7cae95dda 192.168.132.209:32287@31795,,tls-port=0,shard-id=a262f15901feff7a44f73a8d64210708909db3a5 slave cce647d1c72bcac30aee07128c3aa1493405630e 0 1709542563000 0 connected
cce647d1c72bcac30aee07128c3aa1493405630e 192.168.132.208:31513@30183,,tls-port=0,shard-id=a262f15901feff7a44f73a8d64210708909db3a5 master - 0 1709542561000 0 connected 5462-10922
41631ee411f0c6b1c25742f867ebbb1a63856772 192.168.132.209:30707@30244,,tls-port=0,shard-id=da7d249987641fb2379cce47636469153c5e7e58 master - 0 1709542563000 2 connected 10923-16383
be0453515af6a6b9f6dbf96643604cbfc9517792 192.168.132.183:31200@32418,,tls-port=0,shard-id=453e29079a2d30ac8122692ac4a1d1c9390acadf master - 0 1709542561000 1 connected 0-5461
vars currentEpoch 5 lastVoteEpoch 0`),
			wantErr: false,
		},
		{
			name: "fix shard-id conflict with other shard",
			args: args{
				ctx: context.Background(),
				data: []byte(`14c4e058a702f5f3d8f1cd8b70cc3dd450f531ec 192.168.132.210:30541@32045,,tls-port=0,shard-id=78524c978001da077a73c07e1bde14f0eea5eb7d slave be0453515af6a6b9f6dbf96643604cbfc9517792 0 1709542562827 1 connected
c426f0d5a3986d926497f3e925887e4db9ea4e12 192.168.132.183:32304@31460,,tls-port=0,shard-id=da7d249987641fb2379cce47636469153c5e7e58 slave 41631ee411f0c6b1c25742f867ebbb1a63856772 0 1709542563825 2 connected
a1127158a63694800ac4e1187cc9fec7cae95dda 192.168.132.209:32287@31795,,tls-port=0,shard-id=a262f15901feff7a44f73a8d64210708909db3a5 slave cce647d1c72bcac30aee07128c3aa1493405630e 0 1709542563000 0 connected
cce647d1c72bcac30aee07128c3aa1493405630e 192.168.132.208:31513@30183,,tls-port=0,shard-id=a262f15901feff7a44f73a8d64210708909db3a5 master - 0 1709542561000 0 connected 5462-10922
41631ee411f0c6b1c25742f867ebbb1a63856772 192.168.132.209:30707@30244,,tls-port=0,shard-id=1a7d249987641fb2379cce47636469153c5e7e58 master - 0 1709542563000 2 connected 10923-16383
be0453515af6a6b9f6dbf96643604cbfc9517792 192.168.132.183:31200@32418,,tls-port=0,shard-id=78524c978001da077a73c07e1bde14f0eea5eb7d myself,master - 0 1709542561000 1 connected 0-5461
vars currentEpoch 5 lastVoteEpoch 0`),
				logger: logr.Discard(),
				opts: &HealOptions{
					Namespace:  "default",
					PodName:    "drc-c7-0-0",
					Workspace:  "/data",
					TargetName: "nodes.conf",
					Prefix:     "sync-",
					ShardID:    "453e29079a2d30ac8122692ac4a1d1c9390acadf",
					NodeFile:   "/data/nodes.conf",
				},
			},
			want: []byte(`14c4e058a702f5f3d8f1cd8b70cc3dd450f531ec 192.168.132.210:30541@32045,,tls-port=0,shard-id=453e29079a2d30ac8122692ac4a1d1c9390acadf slave be0453515af6a6b9f6dbf96643604cbfc9517792 0 1709542562827 1 connected
a1127158a63694800ac4e1187cc9fec7cae95dda 192.168.132.209:32287@31795,,tls-port=0,shard-id=a262f15901feff7a44f73a8d64210708909db3a5 slave cce647d1c72bcac30aee07128c3aa1493405630e 0 1709542563000 0 connected
cce647d1c72bcac30aee07128c3aa1493405630e 192.168.132.208:31513@30183,,tls-port=0,shard-id=a262f15901feff7a44f73a8d64210708909db3a5 master - 0 1709542561000 0 connected 5462-10922
be0453515af6a6b9f6dbf96643604cbfc9517792 192.168.132.183:31200@32418,,tls-port=0,shard-id=453e29079a2d30ac8122692ac4a1d1c9390acadf myself,master - 0 1709542561000 1 connected 0-5461
vars currentEpoch 5 lastVoteEpoch 0`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := portClusterNodesConf(tt.args.ctx, tt.args.data, tt.args.logger, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("portClusterNodesConf() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			gotVals := strings.Split(string(got), "\n")
			wantVals := strings.Split(string(tt.want), "\n")
			sort.Strings(gotVals)
			sort.Strings(wantVals)
			if !reflect.DeepEqual(gotVals, wantVals) {
				t.Errorf("portClusterNodesConf() = (%d)\n%s\n## want (%d)\n%s", len(got), strings.Join(gotVals, "\n"), len(tt.want), strings.Join(wantVals, "\n"))
			}
		})
	}
}
