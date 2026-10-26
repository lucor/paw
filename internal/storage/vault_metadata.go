// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// FindVaultInfo resolves an ID or an unambiguous display name.
func FindVaultInfo(vaults []VaultInfo, value string) (VaultInfo, error) {
	var match VaultInfo
	for _, vault := range vaults {
		if vault.ID == value {
			return vault, nil
		}
		if strings.EqualFold(vault.Name, value) {
			if match.ID != "" {
				return VaultInfo{}, fmt.Errorf("vault name %q is ambiguous", value)
			}
			match = vault
		}
	}
	if match.ID == "" {
		return VaultInfo{}, fmt.Errorf("vault %q does not exist", value)
	}
	return match, nil
}

type vaultMetadata struct {
	Name string `json:"name"`
}

// encodeVaultMetadata encodes the public name used to list a CXF vault.
func encodeVaultMetadata(name string) ([]byte, error) {
	if name == "" || !utf8.ValidString(name) {
		return nil, fmt.Errorf("vault name is empty or invalid UTF-8")
	}
	data, err := json.Marshal(vaultMetadata{Name: name})
	if err != nil {
		return nil, fmt.Errorf("could not encode vault metadata")
	}
	return data, nil
}

// decodeVaultMetadata decodes a public vault name from its metadata file.
func decodeVaultMetadata(data []byte) (string, error) {
	if !utf8.Valid(data) {
		return "", fmt.Errorf("vault metadata is invalid UTF-8")
	}
	var metadata vaultMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return "", fmt.Errorf("could not decode vault metadata: %w", err)
	}
	if metadata.Name == "" || !utf8.ValidString(metadata.Name) {
		return "", fmt.Errorf("vault metadata has an empty or invalid name")
	}
	return metadata.Name, nil
}

// isVaultID reports whether value is a canonical Paw 128-bit Base64url ID.
func isVaultID(value string) bool {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(value)
	return err == nil && len(decoded) == 16 && base64.RawURLEncoding.EncodeToString(decoded) == value
}

// vaultInfoFromFilesystemName decodes a vault directory name and its metadata.
func vaultInfoFromFilesystemName(name string, data []byte) (VaultInfo, error) {
	id, ok := idFromFilesystemName(name)
	if !ok {
		return VaultInfo{}, fmt.Errorf("invalid vault directory ID")
	}
	return vaultInfoFromMetadata(id, data)
}

// vaultInfoFromMetadata decodes a CXF vault's listing metadata.
func vaultInfoFromMetadata(id string, data []byte) (VaultInfo, error) {
	if !isVaultID(id) {
		return VaultInfo{}, fmt.Errorf("invalid CXF vault ID")
	}
	name, err := decodeVaultMetadata(data)
	if err != nil {
		return VaultInfo{}, err
	}
	return VaultInfo{ID: id, Name: name}, nil
}

// writeVaultMetadata writes a vault's public listing name.
func writeVaultMetadata(w io.Writer, info VaultInfo) error {
	if !isVaultID(info.ID) {
		return fmt.Errorf("invalid CXF vault ID")
	}
	data, err := encodeVaultMetadata(info.Name)
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
