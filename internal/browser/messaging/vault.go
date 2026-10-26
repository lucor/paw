// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package messaging

import "lucor.dev/paw/internal/storage"

// resolveVault finds a vault from a display name or filesystem ID.
func resolveVault(s storage.Storage, value string) (storage.VaultInfo, error) {
	vaults, err := s.Vaults()
	if err != nil {
		return storage.VaultInfo{}, err
	}
	return storage.FindVaultInfo(vaults, value)
}
