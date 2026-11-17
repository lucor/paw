//go:build !android

// Package netif enumerates network interfaces and their assigned IP addresses.
// On non-Android systems, it delegates enumeration to the Go standard library.
package netif

import (
	"net"
)

// Info pairs an interface with its assigned IP addresses from one snapshot.
type Info struct {
	// Interface contains the interface index, name, MTU, and flags.
	Interface net.Interface
	// Addrs contains the interface IP addresses with their network prefixes.
	Addrs []net.Addr
}

// Snapshot returns interfaces and their assigned IP addresses from one enumeration.
func Snapshot() ([]Info, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	result := make([]Info, 0, len(ifaces))
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}
		result = append(result, Info{
			Interface: iface,
			Addrs:     addrs,
		})
	}
	return result, nil
}

// Interfaces returns the network interfaces visible to the current process.
func Interfaces() ([]net.Interface, error) { return net.Interfaces() }

// InterfaceAddrs returns the IP addresses assigned to visible interfaces.
func InterfaceAddrs() ([]net.Addr, error) { return net.InterfaceAddrs() }

// Addrs returns the IP addresses assigned to ifi, matched by interface index.
func Addrs(ifi net.Interface) ([]net.Addr, error) { return ifi.Addrs() }
