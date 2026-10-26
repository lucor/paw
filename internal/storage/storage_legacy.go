// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"fmt"
	"path/filepath"
	"strings"
)

// legacyVaultRootPath returns the directory used by an unmigrated vault.
func legacyVaultRootPath(s Storage, vaultName string) string {
	return filepath.Join(storageRootPath(s), vaultName)
}

// legacyKeyPath returns the key file path used by an unmigrated vault.
func legacyKeyPath(s Storage, vaultName string) string {
	return filepath.Join(legacyVaultRootPath(s, vaultName), keyFileName)
}

// legacyVaultPath returns the index file path used by an unmigrated vault.
func legacyVaultPath(s Storage, vaultName string) string {
	return filepath.Join(legacyVaultRootPath(s, vaultName), vaultFileName)
}

// legacyItemPath returns the item file path used by an unmigrated vault.
func legacyItemPath(s Storage, vaultName string, itemID string) string {
	return filepath.Join(legacyVaultRootPath(s, vaultName), fmt.Sprintf("%s.age", itemID))
}

// isLegacyVaultName reports whether name is a single safe path component.
func isLegacyVaultName(name string) bool {
	return name != "" && name != "." && name != ".." && filepath.Base(name) == name && !strings.HasSuffix(name, vaultBackupSuffix)
}
