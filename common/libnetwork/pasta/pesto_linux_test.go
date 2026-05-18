package pasta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/common/libnetwork/types"
)

func Test_portMappingsToPestoArgs(t *testing.T) {
	tests := []struct {
		name    string
		ports   []types.PortMapping
		want    []string
		wantErr string
	}{
		{
			name:  "no ports returns nil",
			ports: nil,
			want:  nil,
		},
		{
			name:  "empty slice same as nil",
			ports: []types.PortMapping{},
			want:  nil,
		},
		{
			name: "single tcp port dual-stack",
			ports: []types.PortMapping{
				{HostPort: 8080, ContainerPort: 80, Protocol: "tcp", Range: 1},
			},
			want: []string{"-t", "0.0.0.0/8080", "-t", "[::]/8080"},
		},
		{
			name: "single udp port dual-stack",
			ports: []types.PortMapping{
				{HostPort: 53, ContainerPort: 53, Protocol: "udp", Range: 1},
			},
			want: []string{"-u", "0.0.0.0/53", "-u", "[::]/53"},
		},
		{
			name: "tcp and udp port dual-stack",
			ports: []types.PortMapping{
				{HostPort: 80, ContainerPort: 80, Protocol: "tcp", Range: 1},
				{HostPort: 53, ContainerPort: 53, Protocol: "udp", Range: 1},
			},
			want: []string{"-t", "0.0.0.0/80", "-t", "[::]/80", "-u", "0.0.0.0/53", "-u", "[::]/53"},
		},
		{
			name: "dual protocol on single mapping",
			ports: []types.PortMapping{
				{HostPort: 80, ContainerPort: 80, Protocol: "tcp,udp", Range: 1},
			},
			want: []string{"-t", "0.0.0.0/80", "-t", "[::]/80", "-u", "0.0.0.0/80", "-u", "[::]/80"},
		},
		{
			name: "port range expands to host port range",
			ports: []types.PortMapping{
				{HostPort: 8000, ContainerPort: 80, Protocol: "tcp", Range: 5},
			},
			want: []string{"-t", "0.0.0.0/8000-8004", "-t", "[::]/8000-8004"},
		},
		{
			name: "range of zero treated as single port",
			ports: []types.PortMapping{
				{HostPort: 80, ContainerPort: 80, Protocol: "tcp", Range: 0},
			},
			want: []string{"-t", "0.0.0.0/80", "-t", "[::]/80"},
		},
		{
			name: "range of two",
			ports: []types.PortMapping{
				{HostPort: 3000, ContainerPort: 3000, Protocol: "tcp", Range: 2},
			},
			want: []string{"-t", "0.0.0.0/3000-3001", "-t", "[::]/3000-3001"},
		},
		{
			name: "explicit IPv4 host IP",
			ports: []types.PortMapping{
				{HostIP: "127.0.0.1", HostPort: 443, ContainerPort: 443, Protocol: "tcp", Range: 1},
			},
			want: []string{"-t", "127.0.0.1/443"},
		},
		{
			name: "IPv6 host IP gets brackets",
			ports: []types.PortMapping{
				{HostIP: "::1", HostPort: 8080, ContainerPort: 80, Protocol: "tcp", Range: 1},
			},
			want: []string{"-t", "[::1]/8080"},
		},
		{
			name: "full-form IPv6 host IP",
			ports: []types.PortMapping{
				{HostIP: "fd00::1", HostPort: 80, ContainerPort: 80, Protocol: "udp", Range: 1},
			},
			want: []string{"-u", "[fd00::1]/80"},
		},
		{
			name: "multiple tcp ports dual-stack",
			ports: []types.PortMapping{
				{HostPort: 80, ContainerPort: 80, Protocol: "tcp", Range: 1},
				{HostPort: 443, ContainerPort: 443, Protocol: "tcp", Range: 1},
			},
			want: []string{"-t", "0.0.0.0/80", "-t", "[::]/80", "-t", "0.0.0.0/443", "-t", "[::]/443"},
		},
		{
			name: "unsupported protocol returns error",
			ports: []types.PortMapping{
				{HostPort: 80, ContainerPort: 80, Protocol: "sctp", Range: 1},
			},
			wantErr: "pesto: unsupported protocol sctp",
		},
		{
			name: "unsupported protocol mixed with valid returns error",
			ports: []types.PortMapping{
				{HostPort: 80, ContainerPort: 80, Protocol: "tcp", Range: 1},
				{HostPort: 90, ContainerPort: 90, Protocol: "sctp", Range: 1},
			},
			wantErr: "pesto: unsupported protocol sctp",
		},
		{
			name: "explicit host IP on udp",
			ports: []types.PortMapping{
				{HostIP: "10.0.0.1", HostPort: 3000, ContainerPort: 3000, Protocol: "udp", Range: 1},
			},
			want: []string{"-u", "10.0.0.1/3000"},
		},
		{
			name: "container port does not appear in args",
			ports: []types.PortMapping{
				{HostPort: 9090, ContainerPort: 3000, Protocol: "tcp", Range: 1},
			},
			want: []string{"-t", "0.0.0.0/9090", "-t", "[::]/9090"},
		},
		{
			name: "host IP with range",
			ports: []types.PortMapping{
				{HostIP: "10.0.0.1", HostPort: 3000, ContainerPort: 3000, Protocol: "udp", Range: 3},
			},
			want: []string{"-u", "10.0.0.1/3000-3002"},
		},
		{
			name: "range with dual protocol",
			ports: []types.PortMapping{
				{HostPort: 5000, ContainerPort: 5000, Protocol: "tcp,udp", Range: 3},
			},
			want: []string{"-t", "0.0.0.0/5000-5002", "-t", "[::]/5000-5002", "-u", "0.0.0.0/5000-5002", "-u", "[::]/5000-5002"},
		},
		{
			name: "IPv6 host IP with range",
			ports: []types.PortMapping{
				{HostIP: "::1", HostPort: 5000, ContainerPort: 5000, Protocol: "tcp", Range: 4},
			},
			want: []string{"-t", "[::1]/5000-5003"},
		},
		{
			name: "mixed explicit and default host IPs",
			ports: []types.PortMapping{
				{HostIP: "10.0.0.1", HostPort: 80, ContainerPort: 80, Protocol: "tcp", Range: 1},
				{HostPort: 443, ContainerPort: 443, Protocol: "tcp", Range: 1},
			},
			want: []string{"-t", "10.0.0.1/80", "-t", "0.0.0.0/443", "-t", "[::]/443"},
		},
		{
			name: "triple protocol with unsupported in middle returns error",
			ports: []types.PortMapping{
				{HostPort: 80, ContainerPort: 80, Protocol: "tcp,sctp,udp", Range: 1},
			},
			wantErr: "pesto: unsupported protocol sctp",
		},
		{
			name: "dual protocol with explicit IPv4",
			ports: []types.PortMapping{
				{HostIP: "192.168.1.1", HostPort: 80, ContainerPort: 80, Protocol: "tcp,udp", Range: 1},
			},
			want: []string{"-t", "192.168.1.1/80", "-u", "192.168.1.1/80"},
		},
		{
			name: "dual protocol with explicit IPv6",
			ports: []types.PortMapping{
				{HostIP: "fd00::1", HostPort: 80, ContainerPort: 80, Protocol: "tcp,udp", Range: 1},
			},
			want: []string{"-t", "[fd00::1]/80", "-u", "[fd00::1]/80"},
		},
		{
			name: "all unsupported protocols returns error",
			ports: []types.PortMapping{
				{HostPort: 80, ContainerPort: 80, Protocol: "sctp,dccp", Range: 1},
			},
			wantErr: "pesto: unsupported protocol sctp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := portMappingsToPestoArgs(tt.ports)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
