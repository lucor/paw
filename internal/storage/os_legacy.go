// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"fmt"
	"os"

	"lucor.dev/paw/internal/paw"
)

// loadLegacyVault reads a vault using the legacy Paw format.
func (s *OSStorage) loadLegacyVault(vaultID string, key *paw.Key) (*paw.Vault, error) {
	data, err := s.readLegacyVaultData(vaultID, key)
	if err != nil {
		return nil, err
	}
	return paw.DecodeVault(data, key)
}

// readLegacyVaultData reads and decrypts the index at its legacy path.
func (s *OSStorage) readLegacyVaultData(vaultID string, key *paw.Key) ([]byte, error) {
	r, err := os.Open(legacyVaultPath(s, vaultID))
	if err != nil {
		return nil, fmt.Errorf("could not create reader: %w", err)
	}
	defer r.Close()

	data, err := readDecrypted(key, r)
	if err != nil {
		return nil, fmt.Errorf("could not read and decrypt the vault: %w", err)
	}
	return data, nil
}

// loadLegacyItem reads an item using the legacy Paw format.
func (s *OSStorage) loadLegacyItem(vault *paw.Vault, itemMetadata *paw.Metadata) (paw.Item, error) {
	r, err := os.Open(legacyItemPath(s, vault.Name, itemMetadata.ID()))
	if err != nil {
		return nil, fmt.Errorf("could not create reader: %w", err)
	}
	defer r.Close()

	data, err := readDecrypted(vault.Key(), r)
	if err != nil {
		return nil, fmt.Errorf("could not read and decrypt the item: %w", err)
	}
	return paw.DecodeItem(data, itemMetadata.Type)
}
