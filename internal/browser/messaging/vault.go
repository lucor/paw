// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package messaging

import (
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

// resolveVault finds a vault from a display name or filesystem ID.
func resolveVault(s *storage.FileStorage, value string) (model.VaultInfo, error) {
	vaults, err := s.Vaults()
	if err != nil {
		return model.VaultInfo{}, err
	}
	return model.FindVaultInfo(vaults, value)
}
