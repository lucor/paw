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
	// Items represents the list of the item names available into the vault grouped by ItemType
	ItemMetadata map[string]*Metadata //slice of item names
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
		ItemMetadata: make(map[string]*Metadata),
		Created:      time.Now(),
		Modified:     time.Now(),
	}
}

// Size return the total number of items into the vault
func (v *Vault) Size() int {
	return len(v.ItemMetadata)
}

func (v *Vault) Key() *Key {
	return v.key
}

func (v *Vault) HasItem(item Item) bool {
	meta := item.GetMetadata()
	if meta == nil {
		return false
	}
	_, ok := v.ItemMetadata[item.ID()]
	return ok
}

func (v *Vault) AddItem(item Item) error {
	meta := item.GetMetadata()
	if meta == nil {
		return fmt.Errorf("item metadata is nil")
	}
	v.ItemMetadata[item.ID()] = meta
	return nil
}

func (v *Vault) DeleteItem(item Item) {
	meta := item.GetMetadata()
	if meta == nil {
		return
	}

	delete(v.ItemMetadata, item.ID())
}

func (v *Vault) FilterItemMetadata(opts *VaultFilterOptions) []*Metadata {
	metadata := []*Metadata{}
	nameFilter := opts.Name

	for _, itemMetadata := range v.ItemMetadata {
		if opts.ItemType != 0 && (opts.ItemType&itemMetadata.Type) == 0 {
			continue
		}

		if nameFilter != "" && !strings.Contains(itemMetadata.Name, nameFilter) {
			continue
		}
		metadata = append(metadata, itemMetadata)
	}

	sort.Sort(ByString(metadata))

	return metadata
}

type VaultFilterOptions struct {
	Name     string
	ItemType ItemType
}
