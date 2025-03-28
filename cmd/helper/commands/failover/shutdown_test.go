package failover

import (
	"os"
	"testing"

	"github.com/go-logr/logr"
)

func Test_loadAnnounceAddress(t *testing.T) {
	logger := logr.Discard()

	type args struct {
		filepath string
		data     string
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{
				filepath: "/tmp/slave-abc123",
				data:     ``,
			},
			want: "",
		},
		{
			name: "slave config",
			args: args{
				filepath: "/tmp/slave-abc123",
				data: `
slave-announce-ip 192.168.138.159
slave-announce-port 31095`,
			},
			want: "192.168.138.159:31095",
		},
		{
			name: "replica config",
			args: args{
				filepath: "/tmp/replica-abc123",
				data: `
replica-announce-ip 192.168.138.159
replica-announce-port 31095`,
			},
			want: "192.168.138.159:31095",
		},
		{
			name: "slave config without lead space",
			args: args{
				filepath: "/tmp/slave-abc123",
				data: `slave-announce-ip 192.168.138.159
slave-announce-port 31095`,
			},
			want: "192.168.138.159:31095",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(tt.args.filepath, []byte(tt.args.data), 0644); err != nil {
				t.Errorf("failed to write file: %v", err)
			}
			if got := loadAnnounceAddress(tt.args.filepath, logger); got != tt.want {
				t.Errorf("loadAnnounceAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}
