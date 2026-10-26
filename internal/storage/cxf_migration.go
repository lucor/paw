// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"go.lucor.dev/paw/internal/migration"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

type legacyItemRef struct {
	id       string
	metadata *paw.Metadata
}

// legacyItems returns vault items in a stable order for predictable reports.
func legacyItems(vault *paw.Vault) []legacyItemRef {
	items := make([]legacyItemRef, 0, vault.Size())
	vault.Range(func(id string, metadata *paw.Metadata) bool {
		items = append(items, legacyItemRef{id: id, metadata: metadata})
		return true
	})
	slices.SortFunc(items, func(a, b legacyItemRef) int {
		return strings.Compare(a.id, b.id)
	})
	return items
}

// convertLegacyItem converts a decoded Paw item into the current model.
func convertLegacyItem(item paw.Item) (*model.Item, error) {
	switch value := item.(type) {
	case *paw.Note:
		return migration.MigrateNote(value)
	case *paw.Password:
		return migration.MigratePassword(value)
	case *paw.Login:
		return migration.MigrateLogin(value)
	case *paw.SSHKey:
		return migration.MigrateSSHKey(value)
	default:
		return nil, fmt.Errorf("unsupported legacy item type")
	}
}

// migrationVault creates a new CXF vault while retaining the legacy creation time.
func migrationVault(legacy *paw.Vault) (*model.Vault, error) {
	if !legacy.Created.IsZero() && legacy.Created.Before(time.Unix(0, 0)) {
		return nil, fmt.Errorf("legacy vault creation date predates Unix time")
	}
	vault, err := model.NewVault(legacy.Key(), legacy.Name)
	if err != nil {
		return nil, err
	}
	if !legacy.Created.IsZero() {
		created := uint64(legacy.Created.Unix())
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
