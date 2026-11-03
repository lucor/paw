// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package legacy

import (
	"fmt"
	"time"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// Vault is a vault index decoded from the legacy Paw storage format.
type Vault struct {
	key          *paw.Key
	Name         string
	ItemMetadata map[model.ItemType]map[string]*Metadata
	Version      string
	Created      time.Time
	Modified     time.Time
}

// VaultItem identifies one metadata record in a legacy vault index.
type VaultItem struct {
	ID       string
	Metadata *Metadata
}

// NewVault creates a legacy vault fixture.
func NewVault(key *paw.Key, name string) *Vault {
	now := time.Now().UTC()
	return &Vault{
		key: key, Name: name, ItemMetadata: make(map[model.ItemType]map[string]*Metadata),
		Created: now, Modified: now,
	}
}

// Key returns the runtime key attached when the vault was decoded.
func (v *Vault) Key() *paw.Key { return v.key }

// AddItem adds metadata to a legacy vault fixture.
func (v *Vault) AddItem(item Item) error {
	metadata := item.GetMetadata()
	if metadata == nil {
		return fmt.Errorf("item metadata is nil")
	}
	if v.ItemMetadata[metadata.Type] == nil {
		v.ItemMetadata[metadata.Type] = make(map[string]*Metadata)
	}
	v.ItemMetadata[metadata.Type][item.ID()] = metadata
	v.Modified = time.Now().UTC()
	return nil
}

// Items returns the metadata records stored in the legacy vault index.
func (v *Vault) Items() []VaultItem {
	items := make([]VaultItem, 0)
	for _, byType := range v.ItemMetadata {
		for id, metadata := range byType {
			items = append(items, VaultItem{ID: id, Metadata: metadata})
		}
	}
	return items
}
