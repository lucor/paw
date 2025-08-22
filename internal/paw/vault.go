// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later


package paw

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Vault struct {
	key *Key

	Name string
	// Items represents the list of the item IDs available into the vault grouped by ItemType
	ItemMetadata map[ItemType]map[string]*Metadata //map[ItemType]map[<ID>]
	// Version represents the specification version
	Version string
	// Created represents the creation date
	Created time.Time
	// Modified represents the modification date
	Modified time.Time
}

func NewVault(key *Key, name string) *Vault {
	now := time.Now().UTC()
	return &Vault{
		key:          key,
		Name:         name,
		ItemMetadata: make(map[ItemType]map[string]*Metadata),
		Created:      now,
		Modified:     now,
	}
}

// Size return the total number of items into the vault
func (v *Vault) Size() int {
	size := 0
	for _, itemMetadataByType := range v.ItemMetadata {
		size += len(itemMetadataByType)
	}
	return size
}

func (v *Vault) SizeByType(itemType ItemType) int {
	return len(v.ItemMetadata[itemType])
}

func (v *Vault) Key() *Key {
	return v.key
}

// HasItem returns true if a item with the same ID is present into the vault
func (v *Vault) HasItem(item Item) bool {
	meta := item.GetMetadata()
	if meta == nil {
		return false
	}

	metaByType, ok := v.ItemMetadata[meta.Type]
	if !ok {
		return false
	}

	_, ok = metaByType[item.ID()]
	return ok
}

func (v *Vault) AddItem(item Item) error {
	meta := item.GetMetadata()
	if meta == nil {
		return fmt.Errorf("item metadata is nil")
	}
	if v.ItemMetadata[meta.Type] == nil {
		v.ItemMetadata[meta.Type] = make(map[string]*Metadata)
	}
	v.ItemMetadata[meta.Type][item.ID()] = meta
	v.Modified = time.Now().UTC()
	return nil
}

func (v *Vault) DeleteItem(item Item) {
	meta := item.GetMetadata()
	if meta == nil {
		return
	}
	_, ok := v.ItemMetadata[meta.Type]
	if !ok {
		return
	}

	delete(v.ItemMetadata[meta.Type], item.ID())
	v.Modified = time.Now().UTC()
}

// Range calls f sequentially for each key and value present in the vault. If f
// returns false, range stops the iteration.
func (v *Vault) Range(f func(id string, meta *Metadata) bool) {
	for _, itemMetadataByType := range v.ItemMetadata {
		for id, itemMetadata := range itemMetadataByType {
			if !f(id, itemMetadata) {
				break
			}
		}
	}
}

func (v *Vault) FilterItemMetadata(opts *VaultFilterOptions) []*Metadata {
	metadata := []*Metadata{}
	nameFilter := strings.ToLower(opts.Name)

	for t, itemMetadataByType := range v.ItemMetadata {
		if opts.ItemType != 0 && (opts.ItemType&t) == 0 {
			continue
		}

		for _, itemMetadata := range itemMetadataByType {
			if nameFilter != "" && !strings.Contains(strings.ToLower(itemMetadata.Name), nameFilter) {
				continue
			}
			metadata = append(metadata, itemMetadata)
		}
	}

	sort.Sort(ByString(metadata))

	return metadata
}

type VaultFilterOptions struct {
	Name     string
	ItemType ItemType
}
