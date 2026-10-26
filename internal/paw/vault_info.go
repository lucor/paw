// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package paw

import "fmt"

// VaultInfo describes a vault's filesystem ID and display name.
type VaultInfo struct {
	// ID is the vault directory name used for storage operations.
	ID string
	// Name is the label shown to the user.
	Name string
}

// FindVaultInfo finds a vault by its ID or unambiguous display name.
func FindVaultInfo(vaults []VaultInfo, value string) (VaultInfo, error) {
	for _, vault := range vaults {
		if vault.ID == value {
			return vault, nil
		}
	}

	var match VaultInfo
	for _, vault := range vaults {
		if vault.Name != value {
			continue
		}
		if match.ID != "" {
			return VaultInfo{}, fmt.Errorf("vault name %q is ambiguous", value)
		}
		match = vault
	}
	if match.ID == "" {
		return VaultInfo{}, fmt.Errorf("vault %q does not exist", value)
	}
	return match, nil
}
