// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"go.lucor.dev/paw/internal/legacy"
	"go.lucor.dev/paw/internal/migration"
	"go.lucor.dev/paw/internal/model"
)

// legacyItems returns vault items in a stable order for predictable reports.
func legacyItems(vault *legacy.Vault) []legacy.VaultItem {
	items := vault.Items()
	slices.SortFunc(items, func(a, b legacy.VaultItem) int {
		return strings.Compare(a.ID, b.ID)
	})
	return items
}

// convertLegacyItem converts a decoded Paw item into the current model.
func convertLegacyItem(item legacy.Item) (*model.Item, error) {
	switch value := item.(type) {
	case *legacy.Note:
		return migration.MigrateNote(value)
	case *legacy.Password:
		return migration.MigratePassword(value)
	case *legacy.Login:
		return migration.MigrateLogin(value)
	case *legacy.SSHKey:
		return migration.MigrateSSHKey(value)
	default:
		return nil, fmt.Errorf("unsupported legacy item type")
	}
}

// migrationVault creates a new CXF vault while retaining the legacy creation time.
func migrationVault(legacyVault *legacy.Vault) (*model.Vault, error) {
	if !legacyVault.Created.IsZero() && legacyVault.Created.Before(time.Unix(0, 0)) {
		return nil, fmt.Errorf("legacy vault creation date predates Unix time")
	}
	vault, err := model.NewVault(legacyVault.Key(), legacyVault.Name)
	if err != nil {
		return nil, err
	}
	if !legacyVault.Created.IsZero() {
		created := uint64(legacyVault.Created.Unix())
		vault.Metadata.CreationAt = &created
	}
	return vault, nil
}

// verifyMigratedItem checks a staged item decrypts and matches its source value.
func verifyMigratedItem(expected, actual *model.Item) error {
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		return fmt.Errorf("could not encode expected item for verification")
	}
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		return fmt.Errorf("could not encode staged item for verification")
	}
	if string(expectedJSON) != string(actualJSON) {
		return fmt.Errorf("staged item does not match the converted item")
	}
	return nil
}
