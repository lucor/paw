// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"crypto/mldsa"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"go.lucor.dev/paw/internal/model"
)

const syncPeersFileName = "sync-peers.age"

// SyncPeer is a locally trusted Sync device identity.
type SyncPeer struct {
	PublicKey []byte `json:"public_key"`
	Name      string `json:"name"`
}

// ListSyncPeers loads the trusted devices for the unlocked vault.
func (s *FileStorage) ListSyncPeers(vault *model.Vault) ([]SyncPeer, error) {
	root, err := s.syncVaultRoot(vault)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	data, err := root.ReadFile(syncPeersFileName)
	if errors.Is(err, os.ErrNotExist) {
		return []SyncPeer{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read trusted Sync devices: %w", err)
	}
	plain, err := readDecrypted(vault.Key(), bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decrypt trusted Sync devices: %w", err)
	}
	defer clear(plain)
	var peers []SyncPeer
	if err := json.Unmarshal(plain, &peers); err != nil {
		return nil, fmt.Errorf("decode trusted Sync devices: %w", err)
	}
	for i, peer := range peers {
		if err := validateSyncPeer(peer); err != nil {
			return nil, fmt.Errorf("trusted Sync device %d is invalid: %w", i, err)
		}
		for j := 0; j < i; j++ {
			if bytes.Equal(peers[j].PublicKey, peer.PublicKey) {
				return nil, fmt.Errorf("trusted Sync devices contain a duplicate identity")
			}
		}
	}
	return peers, nil
}

// AddSyncPeer stores a verified peer, without replacing an existing identity.
func (s *FileStorage) AddSyncPeer(vault *model.Vault, peer SyncPeer) error {
	if err := validateSyncPeer(peer); err != nil {
		return err
	}
	s.syncPeersMu.Lock()
	defer s.syncPeersMu.Unlock()
	peers, err := s.ListSyncPeers(vault)
	if err != nil {
		return err
	}
	for _, existing := range peers {
		if bytes.Equal(existing.PublicKey, peer.PublicKey) {
			return nil
		}
	}
	peers = append(peers, peer)
	return s.writeSyncPeers(vault, peers)
}

func validateSyncPeer(peer SyncPeer) error {
	if len(peer.PublicKey) != mldsa.MLDSA44PublicKeySize {
		return fmt.Errorf("ML-DSA-44 public key is required")
	}
	if peer.Name == "" {
		return fmt.Errorf("device name is required")
	}
	if len(peer.Name) > 255 {
		return fmt.Errorf("device name is too long")
	}
	if strings.TrimSpace(peer.Name) != peer.Name {
		return fmt.Errorf("device name has surrounding whitespace")
	}
	if _, err := mldsa.NewPublicKey(mldsa.MLDSA44(), peer.PublicKey); err != nil {
		return fmt.Errorf("invalid ML-DSA-44 public key")
	}
	return nil
}

// RemoveSyncPeer removes a trusted peer by its public key.
func (s *FileStorage) RemoveSyncPeer(vault *model.Vault, publicKey []byte) error {
	s.syncPeersMu.Lock()
	defer s.syncPeersMu.Unlock()
	peers, err := s.ListSyncPeers(vault)
	if err != nil {
		return err
	}
	filtered := peers[:0]
	for _, peer := range peers {
		if !bytes.Equal(peer.PublicKey, publicKey) {
			filtered = append(filtered, peer)
		}
	}
	if len(filtered) == len(peers) {
		return nil
	}
	return s.writeSyncPeers(vault, filtered)
}

func (s *FileStorage) writeSyncPeers(vault *model.Vault, peers []SyncPeer) error {
	root, err := s.syncVaultRoot(vault)
	if err != nil {
		return err
	}
	defer root.Close()
	return atomicWrite(root, syncPeersFileName, func(w io.Writer) error {
		return encrypt(vault.Key(), w, peers)
	})
}

func (s *FileStorage) syncVaultRoot(vault *model.Vault) (*os.Root, error) {
	if vault.Key() == nil {
		return nil, fmt.Errorf("unlocked vault is required")
	}
	return s.openVaultRoot(vault.Accounts[0].ID)
}
