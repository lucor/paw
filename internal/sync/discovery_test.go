// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/zeroconf/v2"
	"github.com/stretchr/testify/require"
)

type testDiscoveryServer struct{ shutdowns atomic.Int32 }

func (s *testDiscoveryServer) Shutdown() { s.shutdowns.Add(1) }

func TestPeerFromEntry(t *testing.T) {
	validEntry := func() *zeroconf.ServiceEntry {
		return &zeroconf.ServiceEntry{
			AddrIPv4: []net.IP{net.ParseIP("192.0.2.10")},
			AddrIPv6: []net.IP{net.ParseIP("fe80::1"), net.ParseIP("2001:db8::10")},
			Port:     17417,
			Text:     []string{"name=Other", "version=" + protocolVersion, "fingerprint=remote"},
		}
	}

	for _, test := range []struct {
		name        string
		mutate      func(*zeroconf.ServiceEntry)
		own         string
		wantValid   bool
		wantAddress string
	}{
		{name: "valid peer", wantValid: true, wantAddress: "192.0.2.10"},
		{name: "missing address", mutate: func(entry *zeroconf.ServiceEntry) { entry.AddrIPv4 = nil; entry.AddrIPv6 = nil }},
		{name: "link-local IPv6 only", mutate: func(entry *zeroconf.ServiceEntry) { entry.AddrIPv4 = nil; entry.AddrIPv6 = entry.AddrIPv6[:1] }},
		{name: "missing fingerprint", mutate: func(entry *zeroconf.ServiceEntry) { entry.Text = []string{"name=Other", "version=" + protocolVersion} }},
		{name: "self", own: "remote"},
		{name: "version mismatch", mutate: func(entry *zeroconf.ServiceEntry) { entry.Text[1] = "version=old" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			entry := validEntry()
			if test.mutate != nil {
				test.mutate(entry)
			}
			peer, valid := peerFromEntry(entry, "Other device", test.own)
			require.Equal(t, test.wantValid, valid)
			if test.wantValid {
				require.Equal(t, test.wantAddress, peer.IP.String())
				require.Len(t, peer.IPs, 2)
				require.Equal(t, "Other", peer.Name)
				require.Equal(t, "remote", peer.Fingerprint)
				require.Equal(t, 17417, peer.Port)
			}
		})
	}
	_, valid := peerFromEntry(validEntry(), "", "local")
	require.False(t, valid)
}

func TestDiscoveryStartStop(t *testing.T) {
	server := &testDiscoveryServer{}
	d := NewDiscovery("test-id", "Test", 17417, "fingerprint")
	d.register = func(instance, service, domain string, port int, txt []string, ifaces []net.Interface) (discoveryServer, error) {
		require.Equal(t, "test-id", instance)
		require.Equal(t, serviceName, service)
		require.Equal(t, "local.", domain)
		require.Equal(t, 17417, port)
		require.Equal(t, []string{"name=Test", "version=" + protocolVersion, "fingerprint=fingerprint"}, txt)
		return server, nil
	}

	require.NoError(t, d.Start(context.Background()))
	require.Error(t, d.Start(context.Background()))
	require.NoError(t, d.Stop())
	require.NoError(t, d.Stop())
	require.EqualValues(t, 1, server.shutdowns.Load())
	_, open := <-d.Peers()
	require.False(t, open)
}

func TestDiscoveryStartBrowsingDoesNotAdvertise(t *testing.T) {
	d := NewDiscovery("test-id", "Clone target", 17417, "fingerprint")
	d.register = func(string, string, string, int, []string, []net.Interface) (discoveryServer, error) {
		t.Fatal("browse-only discovery must not register an mDNS service")
		return nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, d.StartBrowsing(ctx))
	cancel()
	require.NoError(t, d.Stop())
	_, open := <-d.Peers()
	require.False(t, open)
}

func TestDiscoveryStopsWhenContextIsCanceled(t *testing.T) {
	server := &testDiscoveryServer{}
	d := NewDiscovery("test-id", "Test", 17417, "fingerprint")
	d.register = func(string, string, string, int, []string, []net.Interface) (discoveryServer, error) {
		return server, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, d.Start(ctx))
	cancel()
	require.Eventually(t, func() bool { return server.shutdowns.Load() == 1 }, time.Second, time.Millisecond)
	_, open := <-d.Peers()
	require.False(t, open)
}

func TestDiscoveryPublishesMultiplePeers(t *testing.T) {
	ctx := context.Background()
	peers := make(chan Peer, 2)
	seen := make(map[string]struct{})
	for _, peer := range []Peer{{ID: "one", Name: "One"}, {ID: "two", Name: "Two"}} {
		require.True(t, publishDiscoveredPeer(ctx, peers, seen, peer))
	}
	require.True(t, publishDiscoveredPeer(ctx, peers, seen, Peer{ID: "one", Name: "One"}))
	require.Equal(t, "one", (<-peers).ID)
	require.Equal(t, "two", (<-peers).ID)
	require.Empty(t, peers)
}
