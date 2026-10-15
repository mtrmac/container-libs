package netavark

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/common/libnetwork/types"
)

func makeNetworkStatus(ips []net.IP) types.StatusBlock {
	addresses := make([]types.NetAddress, 0, len(ips))
	for _, ip := range ips {
		mask := net.IPv4Mask(255, 255, 255, 0)
		if ip.To4() == nil {
			mask = net.CIDRMask(64, 128)
		}
		addresses = append(addresses, types.NetAddress{IPNet: types.IPNet{IPNet: net.IPNet{IP: ip, Mask: mask}}})
	}

	return types.StatusBlock{
		Interfaces: map[string]types.NetInterface{"eth0": {Subnets: addresses}},
	}
}

func makeNetwork(internal bool) *types.Network {
	return &types.Network{
		Internal: internal,
	}
}

func TestFirstIPsFromStatus(t *testing.T) {
	tests := []struct {
		name         string
		status       map[string]types.StatusBlock
		networkOrder []string
		networks     map[string]*types.Network
		wantIPv4     string
		wantIPv4Net  string
		wantIPv6     string
		wantIPv6Net  string
	}{
		{
			name: "none",
		},
		{
			name:         "network order without status return nothing",
			networkOrder: []string{"net1", "net2"},
		},
		{
			name:         "ipv4 only",
			networkOrder: []string{"net1"},
			status: map[string]types.StatusBlock{
				"net1": makeNetworkStatus([]net.IP{net.IPv4(192, 168, 0, 1)}),
			},
			networks: map[string]*types.Network{
				"net1": makeNetwork(false),
			},
			wantIPv4:    "192.168.0.1",
			wantIPv4Net: "net1",
		},
		{
			name:         "ipv4 only internal",
			networkOrder: []string{"net1"},
			status: map[string]types.StatusBlock{
				"net1": makeNetworkStatus([]net.IP{net.IPv4(192, 168, 0, 1)}),
			},
			networks: map[string]*types.Network{
				"net1": makeNetwork(true),
			},
		},
		{
			name:         "ipv4 first match",
			networkOrder: []string{"net1", "net2"},
			status: map[string]types.StatusBlock{
				"net1": makeNetworkStatus([]net.IP{net.IPv4(192, 168, 0, 1)}),
				"net2": makeNetworkStatus([]net.IP{net.IPv4(1, 1, 1, 1)}),
			},
			networks: map[string]*types.Network{
				"net1": makeNetwork(false),
				"net2": makeNetwork(false),
			},
			wantIPv4:    "192.168.0.1",
			wantIPv4Net: "net1",
		},
		{
			name:         "ipv6 only",
			networkOrder: []string{"netv6"},
			status: map[string]types.StatusBlock{
				"netv6": makeNetworkStatus([]net.IP{net.IPv6loopback}),
			},
			networks: map[string]*types.Network{
				"netv6": makeNetwork(false),
			},
			wantIPv6:    "::1",
			wantIPv6Net: "netv6",
		},
		{
			name:         "dual stack",
			networkOrder: []string{"net"},
			status: map[string]types.StatusBlock{
				"net": makeNetworkStatus([]net.IP{net.IPv4(10, 0, 0, 1), net.IPv6loopback}),
			},
			networks: map[string]*types.Network{
				"net": makeNetwork(false),
			},
			wantIPv4:    "10.0.0.1",
			wantIPv4Net: "net",
			wantIPv6:    "::1",
			wantIPv6Net: "net",
		},
		{
			name:         "across two networks",
			networkOrder: []string{"net4", "net6"},
			status: map[string]types.StatusBlock{
				"net4": makeNetworkStatus([]net.IP{net.IPv4(10, 0, 0, 1)}),
				"net6": makeNetworkStatus([]net.IP{net.IPv6loopback}),
			},
			networks: map[string]*types.Network{
				"net4": makeNetwork(false),
				"net6": makeNetwork(false),
			},
			wantIPv4:    "10.0.0.1",
			wantIPv4Net: "net4",
			wantIPv6:    "::1",
			wantIPv6Net: "net6",
		},
		{
			name:         "mixed internal and normal networks",
			networkOrder: []string{"int", "net4", "int2", "net6"},
			status: map[string]types.StatusBlock{
				"int":  makeNetworkStatus([]net.IP{net.IPv4(1, 0, 0, 1)}),
				"int2": makeNetworkStatus([]net.IP{net.IPv6linklocalallnodes}),
				"net4": makeNetworkStatus([]net.IP{net.IPv4(10, 0, 0, 1)}),
				"net6": makeNetworkStatus([]net.IP{net.IPv6loopback}),
			},
			networks: map[string]*types.Network{
				"int":  makeNetwork(true),
				"int2": makeNetwork(true),
				"net4": makeNetwork(false),
				"net6": makeNetwork(false),
			},
			wantIPv4:    "10.0.0.1",
			wantIPv4Net: "net4",
			wantIPv6:    "::1",
			wantIPv6Net: "net6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ipv4, ipv4Net, ipv6, ipv6Net := firstIPsFromStatus(tt.status, tt.networkOrder, tt.networks)
			assert.Equal(t, tt.wantIPv4, ipv4, "ipv4")
			assert.Equal(t, tt.wantIPv4Net, ipv4Net, "ipv4Net")
			assert.Equal(t, tt.wantIPv6, ipv6, "ipv6")
			assert.Equal(t, tt.wantIPv6Net, ipv6Net, "ipv6Net")
		})
	}
}
