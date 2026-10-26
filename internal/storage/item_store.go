// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"fmt"
	"slices"

	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/model"
)

// prepareItemVault returns a separate vault with the item's derived summary.
func prepareItemVault(vault *model.Vault, item *model.Item) (*model.Vault, error) {
	if err := vault.Validate(); err != nil {
		return nil, fmt.Errorf("could not prepare vault: %w", err)
	}
	if vault.Key() == nil {
		return nil, fmt.Errorf("could not store item: vault key is missing")
	}
	summary, err := item.Summary()
	if err != nil {
		return nil, fmt.Errorf("could not prepare item summary: %w", err)
	}

	updated := *vault
	updated.Accounts = slices.Clone(vault.Accounts)
	account := &updated.Accounts[0]
	account.Items = slices.Clone(account.Items)
	// Vault marshaling replaces its Paw extension, so isolate that slice too.
	account.Extensions = slices.Clone(account.Extensions)
	index := slices.IndexFunc(account.Items, func(candidate cxf.Item) bool {
		return candidate.ID == item.ID
	})
	if index < 0 {
		account.Items = append(account.Items, *summary)
	} else {
		account.Items[index] = *summary
	}
	if err := updated.Validate(); err != nil {
		return nil, fmt.Errorf("could not prepare updated vault: %w", err)
	}
	return &updated, nil
}

// prepareItemDeleteVault returns a separate vault without the selected item summary.
func prepareItemDeleteVault(vault *model.Vault, itemID string) (*model.Vault, error) {
	if err := vault.Validate(); err != nil {
		return nil, fmt.Errorf("could not prepare vault: %w", err)
	}
	if vault.Key() == nil {
		return nil, fmt.Errorf("could not delete item: vault key is missing")
	}

	updated := *vault
	updated.Accounts = slices.Clone(vault.Accounts)
	account := &updated.Accounts[0]
	account.Items = slices.Clone(account.Items)
	account.Extensions = slices.Clone(account.Extensions)
	index := slices.IndexFunc(account.Items, func(candidate cxf.Item) bool {
		return candidate.ID == itemID
	})
	if index < 0 {
		return nil, fmt.Errorf("could not delete item: item %q is not in the vault", itemID)
	}
	account.Items = slices.Delete(account.Items, index, index+1)
	if err := updated.Validate(); err != nil {
		return nil, fmt.Errorf("could not prepare updated vault: %w", err)
	}
	return &updated, nil
}
