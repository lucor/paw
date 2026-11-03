package model

import (
	"fmt"

	"go.lucor.dev/cxf"
)

// FindItem finds one summary by its title and Paw item category.
func (vault *Vault) FindItem(title string, itemType ItemType) (*cxf.Item, error) {
	if len(vault.Accounts) != 1 {
		return nil, fmt.Errorf("invalid vault account structure")
	}
	var found *cxf.Item
	for index := range vault.Accounts[0].Items {
		candidate := &vault.Accounts[0].Items[index]
		if candidate.Title != title {
			continue
		}
		candidateType, err := ItemTypeOf(candidate)
		if err != nil {
			return nil, err
		}
		if candidateType != itemType {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("item name %q is ambiguous", title)
		}
		found = candidate
	}
	if found == nil {
		return nil, fmt.Errorf("item %q does not exist", title)
	}
	return found, nil
}
