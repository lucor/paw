// Copyright 2021 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

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
	return &Vault{
		key:          key,
		Name:         name,
		ItemMetadata: make(map[ItemType]map[string]*Metadata),
		Created:      time.Now(),
		Modified:     time.Now(),
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
