package sync

import (
	"fmt"
	"net"
	"os"

	"github.com/libp2p/zeroconf/v2"
	"go.lucor.dev/paw/internal/netif"
)

// discoveryInterfaces returns multicast interfaces, publishable addresses, and the local host name.
func discoveryInterfaces() ([]net.Interface, []string, string, error) {
	snapshot, err := netif.Snapshot()
	if err != nil {
		return nil, nil, "", fmt.Errorf("enumerate network interfaces: %w", err)
	}
	ifaces, ips := selectDiscoveryAddresses(snapshot)
	if len(ifaces) == 0 || len(ips) == 0 {
		return nil, nil, "", fmt.Errorf("no usable multicast interface address is available")
	}
	host, err := os.Hostname()
	if err != nil {
		return nil, nil, "", fmt.Errorf("get hostname: %w", err)
	}
	if host == "" {
		return nil, nil, "", fmt.Errorf("hostname is empty")
	}
	return ifaces, ips, host, nil
}

// registerService publishes Paw with addresses from the same interface snapshot.
func registerService(instance, service, domain string, port int, text []string) (discoveryServer, error) {
	ifaces, ips, host, err := discoveryInterfaces()
	if err != nil {
		return nil, err
	}
	// RegisterProxy accepts the Android snapshot directly and avoids Interface.Addrs,
	// which falls back to the restricted standard-library Netlink enumeration.
	return zeroconf.RegisterProxy(instance, service, domain, port, host, ips, text, ifaces)
}

// selectDiscoveryAddresses filters interfaces and deduplicates addresses suitable for DNS records.
func selectDiscoveryAddresses(snapshot []netif.Info) ([]net.Interface, []string) {
	var ifaces []net.Interface
	var ips []string
	seen := make(map[string]struct{})
	for _, item := range snapshot {
		iface := item.Interface
		if !isMulticastInterface(iface) {
			continue
		}
		ifaces = append(ifaces, iface)
		for _, addr := range item.Addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || !ipnet.IP.IsGlobalUnicast() || ipnet.IP.IsLoopback() || ipnet.IP.IsLinkLocalUnicast() {
				continue
			}
			// Link-local IPv6 needs an interface zone for TCP dialing; DNS AAAA records cannot carry it.
			ip := ipnet.IP.String()
			if _, ok := seen[ip]; !ok {
				seen[ip] = struct{}{}
				ips = append(ips, ip)
			}
		}
	}
	return ifaces, ips
}

// isMulticastInterface reports whether iface can carry multicast traffic to LAN peers.
func isMulticastInterface(iface net.Interface) bool {
	return iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagMulticast != 0 && iface.Flags&net.FlagLoopback == 0
}
