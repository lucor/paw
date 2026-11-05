// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// decodeModelItem decodes and validates an item stored in CXF format.
func decodeModelItem(data []byte) (*model.Item, error) {
	var item model.Item
	if err := json.Unmarshal(data, &item); err != nil {
		return nil, fmt.Errorf("could not decode CXF item: %w", err)
	}
	return &item, nil
}

// encrypt writes a JSON value encrypted with the vault key.
func encrypt(key *paw.Key, w io.Writer, v any) error {
	encWriter, err := key.Encrypt(w)
	if err != nil {
		return fmt.Errorf("could not create encrypted writer for URI: %w", err)
	}

	encodeErr := json.NewEncoder(encWriter).Encode(v)
	if encodeErr != nil {
		encodeErr = fmt.Errorf("could not encode data for URI: %w", encodeErr)
	}
	closeErr := encWriter.Close()
	if closeErr != nil {
		closeErr = fmt.Errorf("could not close encrypted writer: %w", closeErr)
	}
	return errors.Join(encodeErr, closeErr)
}

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

type vaultMetadata struct {
	Name  string `json:"name"`
	Icon  string `json:"icon,omitempty"`
	Color string `json:"color,omitempty"`
}

// encodeVaultMetadata encodes the public details used to list a CXF vault.
func encodeVaultMetadata(name string, appearance model.VaultAppearance) ([]byte, error) {
	if name == "" || !utf8.ValidString(name) {
		return nil, fmt.Errorf("vault name is empty or invalid UTF-8")
	}
	appearance = model.NormalizeVaultAppearance(appearance)
	data, err := json.Marshal(vaultMetadata{Name: name, Icon: appearance.Icon, Color: appearance.Color})
	if err != nil {
		return nil, fmt.Errorf("could not encode vault metadata")
	}
	return data, nil
}

// decodeVaultMetadata decodes public vault details from its metadata file.
func decodeVaultMetadata(data []byte) (vaultMetadata, error) {
	if !utf8.Valid(data) {
		return vaultMetadata{}, fmt.Errorf("vault metadata is invalid UTF-8")
	}
	var metadata vaultMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return vaultMetadata{}, fmt.Errorf("could not decode vault metadata: %w", err)
	}
	if metadata.Name == "" || !utf8.ValidString(metadata.Name) {
		return vaultMetadata{}, fmt.Errorf("vault metadata has an empty or invalid name")
	}
	return metadata, nil
}

// writeVaultMetadata writes a vault's public listing details.
func writeVaultMetadata(w io.Writer, info model.VaultInfo) error {
	if !isVaultID(info.ID) {
		return fmt.Errorf("invalid CXF vault ID")
	}
	data, err := encodeVaultMetadata(info.Name, info.Appearance)
	if err != nil {
		return err
	}
	n, err := w.Write(data)
	if err != nil {
		return fmt.Errorf("could not write vault metadata: %w", err)
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return nil
}
