// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"encoding/json"
	"fmt"
	"io"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// decryptVault decrypts and decodes a complete CXF vault.
func decryptVault(key *paw.Key, r io.Reader) (*model.Vault, error) {
	data, err := readDecrypted(key, r)
	if err != nil {
		return nil, fmt.Errorf("could not read and decrypt vault: %w", err)
	}
	vault, err := model.DecodeVault(data, key)
	if err != nil {
		return nil, fmt.Errorf("could not decode vault: %w", err)
	}
	return vault, nil
}

// decryptItem decrypts and decodes a complete CXF item with the expected ID.
func decryptItem(key *paw.Key, r io.Reader, expectedID string) (*model.Item, error) {
	data, err := readDecrypted(key, r)
	if err != nil {
		return nil, fmt.Errorf("could not read and decrypt item: %w", err)
	}
	var item model.Item
	if err := json.Unmarshal(data, &item); err != nil {
		return nil, fmt.Errorf("could not decode item: %w", err)
	}
	if item.ID != expectedID {
		return nil, fmt.Errorf("item ID %q does not match expected ID %q", item.ID, expectedID)
	}
	return &item, nil
}

// readDecrypted reads the full age stream so authentication errors are returned.
func readDecrypted(key *paw.Key, r io.Reader) ([]byte, error) {
	if key == nil {
		return nil, fmt.Errorf("key is nil")
	}
	decrypted, err := key.Decrypt(r)
	if err != nil {
		return nil, fmt.Errorf("could not decrypt content: %w", err)
	}
	data, err := io.ReadAll(decrypted)
	if err != nil {
		return nil, fmt.Errorf("could not read decrypted content: %w", err)
	}
	return data, nil
}
