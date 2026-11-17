// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package sync provides LAN discovery for Paw instances.
package sync

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/libp2p/zeroconf/v2"
	"go.lucor.dev/paw/internal/netif"
)

const serviceName = "_paw-sync._tcp"

// Peer is an instance found on the local network.
type Peer struct {
	ID          string
	Name        string
	IP          net.IP
	IPs         []net.IP
	Port        int
	Fingerprint string
	Version     string
}

// Discovery advertises this instance and reports other Paw instances.
type Discovery struct {
	id          string
	name        string
	port        int
	fingerprint string
	server      discoveryServer
	closed      bool
	peers       chan Peer
	queryDone   chan struct{}
	stop        context.CancelFunc
	mu          sync.Mutex
	register    func(string, string, string, int, []string) (discoveryServer, error)
}

type discoveryServer interface{ Shutdown() }

// NewDiscovery creates LAN discovery for an instance identifier and display name.
func NewDiscovery(id, name string, port int, fingerprint string) *Discovery {
	return &Discovery{
		id:          id,
		name:        name,
		port:        port,
		fingerprint: fingerprint,
		peers:       make(chan Peer, 8),
		register:    registerService,
	}
}

// Peers returns the channel of discovered instances.
func (d *Discovery) Peers() <-chan Peer { return d.peers }

// Start advertises this instance and searches for peers until ctx is canceled.
func (d *Discovery) Start(ctx context.Context) error {
	return d.start(ctx, true)
}

// StartBrowsing searches for Paw instances without advertising this instance.
func (d *Discovery) StartBrowsing(ctx context.Context) error {
	return d.start(ctx, false)
}

func (d *Discovery) start(ctx context.Context, advertise bool) error {
	logger := slog.With("component", "sync.discovery")
	logger.Debug("starting mDNS discovery", "device", d.name, "port", d.port)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.server != nil || d.closed {
		return fmt.Errorf("discovery already started")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var server discoveryServer
	var err error
	if advertise {
		server, err = d.register(d.id, serviceName, "local.", d.port, []string{"name=" + d.name, "version=" + protocolVersion, "fingerprint=" + d.fingerprint})
		if err != nil {
			logger.Warn("could not register mDNS service", "error", err)
			return fmt.Errorf("register mDNS service: %w", err)
		}
	}
	queryCtx, cancel := context.WithCancel(ctx)
	queryDone := make(chan struct{})
	d.server, d.stop, d.queryDone = server, cancel, queryDone
	if advertise {
		logger.Debug("mDNS service registered", "device", d.name, "port", d.port)
	}
	go func() {
		defer close(queryDone)
		defer close(d.peers)
		d.query(queryCtx)
	}()
	go func() { <-queryCtx.Done(); _ = d.Stop() }()
	return nil
}

// Stop stops discovery. It is safe to call more than once.
func (d *Discovery) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.queryDone == nil {
		return nil
	}
	d.stop()
	if d.server != nil {
		d.server.Shutdown()
	}
	slog.Debug("stopping mDNS discovery", "component", "sync.discovery", "device", d.name)
	queryDone := d.queryDone
	d.server, d.stop, d.closed = nil, nil, true
	d.queryDone = nil
	<-queryDone
	return nil
}

func (d *Discovery) query(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	logger := slog.With("component", "sync.discovery")
	logger.Debug("mDNS browser started", "device", d.name)
	defer logger.Debug("mDNS browser stopped", "device", d.name)
	seen := make(map[string]struct{})
	entries := make(chan *zeroconf.ServiceEntry, 16)
	snapshot, err := netif.Snapshot()
	if err != nil {
		logger.Warn("could not list interfaces for mDNS", "error", err)
		return
	}
	ifaces, _ := selectDiscoveryAddresses(snapshot)
	if len(ifaces) == 0 {
		logger.Warn("could not list interfaces for mDNS", "error", "no active multicast interface")
		return
	}
	logger.Debug("browsing mDNS interfaces", "count", len(ifaces))
	// Separate browsers let IPv4 continue when IPv6 sockets are unavailable.
	var browsers sync.WaitGroup
	for _, traffic := range []zeroconf.IPType{zeroconf.IPv4, zeroconf.IPv6} {
		browsers.Add(1)
		go func() {
			defer browsers.Done()
			browseFamily(ctx, traffic, ifaces, entries)
		}()
	}
	go func() {
		browsers.Wait()
		close(entries)
	}()
	defer func() {
		cancel()
		browsers.Wait()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-entries:
			if !ok {
				return
			}
			peer, ok := peerFromEntry(entry, entry.ServiceInstanceName(), d.fingerprint)
			if !ok {
				logger.Debug("filtered mDNS result", "peer_instance", peer.ID, "peer_device", peer.Name, "has_fingerprint", peer.Fingerprint != "", "same_identity", peer.Fingerprint == d.fingerprint, "version", peer.Version)
				continue
			}
			logger.Debug("Sync device discovered", "device", peer.Name, "port", peer.Port, "addresses", len(peer.IPs))
			if !publishDiscoveredPeer(ctx, d.peers, seen, peer) {
				return
			}
		}
	}
}

// browseFamily forwards one IP family's results while keeping its socket errors independent.
func browseFamily(ctx context.Context, traffic zeroconf.IPType, ifaces []net.Interface, output chan<- *zeroconf.ServiceEntry) {
	entries := make(chan *zeroconf.ServiceEntry)
	done := make(chan error, 1)
	go func() {
		done <- zeroconf.Browse(ctx, serviceName, "local.", entries,
			zeroconf.SelectIPTraffic(traffic),
			zeroconf.SelectIfaces(ifaces),
		)
	}()
	for {
		select {
		case err := <-done:
			if err != nil && ctx.Err() == nil {
				slog.Warn("mDNS browse failed", "component", "sync.discovery", "traffic", traffic, "error", err)
			}
			return
		case entry, ok := <-entries:
			if !ok {
				entries = nil
				continue
			}
			select {
			case output <- entry:
			case <-ctx.Done():
			}
		}
	}
}

func publishDiscoveredPeer(ctx context.Context, peers chan<- Peer, seen map[string]struct{}, peer Peer) bool {
	if _, ok := seen[peer.ID]; ok {
		return true
	}
	select {
	case peers <- peer:
		seen[peer.ID] = struct{}{}
		return true
	case <-ctx.Done():
		return false
	}
}

func peerFromEntry(entry *zeroconf.ServiceEntry, instance, ownFingerprint string) (Peer, bool) {
	addresses := make([]net.IP, 0, len(entry.AddrIPv4)+len(entry.AddrIPv6))
	addresses = append(addresses, entry.AddrIPv4...)
	for _, address := range entry.AddrIPv6 {
		if !address.IsLinkLocalUnicast() {
			addresses = append(addresses, address)
		}
	}
	peer := Peer{ID: instance, Port: entry.Port}
	if len(addresses) > 0 {
		peer.IP = addresses[0]
		peer.IPs = addresses
	}
	for _, field := range entry.Text {
		if key, value, ok := splitField(field); ok {
			switch key {
			case "name":
				peer.Name = value
			case "version":
				peer.Version = value
			case "fingerprint":
				peer.Fingerprint = value
			}
		}
	}
	if len(addresses) == 0 || peer.ID == "" || peer.Fingerprint == "" ||
		peer.Fingerprint == ownFingerprint || peer.Version != protocolVersion {
		return peer, false
	}
	return peer, true
}

func splitField(field string) (string, string, bool) {
	for i := 0; i < len(field); i++ {
		if field[i] == '=' {
			return field[:i], field[i+1:], true
		}
	}
	return "", "", false
}
