package ui

import "go.lucor.dev/paw/internal/model"

// vaultFilterOptions stores the current item-list filters for one vault.
type vaultFilterOptions struct {
	Name     string
	ItemType model.ItemType
}
