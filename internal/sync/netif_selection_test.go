package sync

import (
	"net"
	"testing"

	"go.lucor.dev/paw/internal/netif"
)

func TestSelectDiscoveryAddresses(t *testing.T) {
	snapshot := []netif.Info{
		{
			Interface: net.Interface{
				Index: 1,
				Flags: net.FlagUp | net.FlagLoopback | net.FlagMulticast,
			},
			Addrs: []net.Addr{mustCIDR(t, "127.0.0.1/8")},
		},
		{
			Interface: net.Interface{
				Index: 2,
				Flags: net.FlagMulticast,
			},
			Addrs: []net.Addr{mustCIDR(t, "192.168.1.3/24")},
		},
		{
			Interface: net.Interface{
				Index: 3,
				Flags: net.FlagUp | net.FlagMulticast,
			},
			Addrs: []net.Addr{
				mustCIDR(t, "192.168.1.3/24"),
				mustCIDR(t, "169.254.1.2/16"),
				mustCIDR(t, "2001:db8::2/64"),
				mustCIDR(t, "fe80::2/64"),
			},
		},
		{
			Interface: net.Interface{
				Index: 4,
				Flags: net.FlagUp | net.FlagMulticast,
			},
			Addrs: []net.Addr{mustCIDR(t, "192.168.1.3/24")},
		},
	}
	ifaces, ips := selectDiscoveryAddresses(snapshot)
	if len(ifaces) != 2 || ifaces[0].Index != 3 || ifaces[1].Index != 4 {
		t.Fatalf("unexpected interfaces: %#v", ifaces)
	}
	if len(ips) != 2 || ips[0] != "192.168.1.3" || ips[1] != "2001:db8::2" {
		t.Fatalf("unexpected addresses: %#v", ips)
	}
}

func TestSelectDiscoveryAddressesEmpty(t *testing.T) {
	ifaces, ips := selectDiscoveryAddresses(nil)
	if len(ifaces) != 0 || len(ips) != 0 {
		t.Fatalf("got %v, %v", ifaces, ips)
	}
}

func TestSelectDiscoveryAddressesRequiresUpMulticastNonLoopback(t *testing.T) {
	snapshot := []netif.Info{
		{
			Interface: net.Interface{
				Index: 1,
				Flags: net.FlagUp | net.FlagLoopback | net.FlagMulticast,
			},
			Addrs: []net.Addr{mustCIDR(t, "192.0.2.1/24")},
		},
		{
			Interface: net.Interface{
				Index: 2,
				Flags: net.FlagUp,
			},
			Addrs: []net.Addr{mustCIDR(t, "192.0.2.2/24")},
		},
		{
			Interface: net.Interface{
				Index: 3,
				Flags: net.FlagMulticast,
			},
			Addrs: []net.Addr{mustCIDR(t, "192.0.2.3/24")},
		},
	}
	ifaces, ips := selectDiscoveryAddresses(snapshot)
	if len(ifaces) != 0 || len(ips) != 0 {
		t.Fatalf("got %v, %v", ifaces, ips)
	}
}

func mustCIDR(t *testing.T, value string) net.Addr {
	t.Helper()
	ip, address, err := net.ParseCIDR(value)
	if err != nil {
		t.Fatal(err)
	}
	address.IP = ip
	return address
}
