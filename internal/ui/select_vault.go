// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/model"
)

// VaultChoice describes a vault available in the selector.
type VaultChoice struct {
	Name       string
	Appearance model.VaultAppearance
	Unlocked   bool
	Active     bool
	Open       func()
}

// NewSelectVault displays the available vaults in a native scrolling list.
func NewSelectVault(vaults []VaultChoice, addVault func()) fyne.CanvasObject {
	var list *widget.List
	list = widget.NewList(
		func() int { return len(vaults) },
		func() fyne.CanvasObject {
			row := newNavigationRowWithSubtitle(
				"Vault",
				"Locked",
				newVaultIcon(model.DefaultVaultAppearance(), vaultIconMedium),
				nil,
			)
			return container.NewStack(row)
		},
		func(id widget.ListItemID, object fyne.CanvasObject) {
			vault := vaults[id]
			status := "Locked"
			if vault.Unlocked {
				status = "Unlocked"
			}
			cell := object.(*fyne.Container)
			row := newNavigationRowWithSubtitle(
				vault.Name,
				status,
				newVaultIcon(vault.Appearance, vaultIconMedium),
				func() { list.Select(id) },
			)
			cell.Objects[0] = row
			cell.Refresh()
		},
	)
	list.HideSeparators = false
	list.OnSelected = func(id widget.ListItemID) {
		list.UnselectAll()
		vaults[id].Open()
	}
	header := container.NewVBox(
		container.NewBorder(nil, nil, nil, newVaultOptions(addVault), newText("Your vaults", headingStyle)),
		newText("Select a vault to open.", bodyStyle),
	)
	pagePadding := pawSize(pawSizeNamePagePadding)
	return container.New(layout.NewCustomPaddedLayout(pagePadding, pagePadding, pagePadding, pagePadding),
		container.NewBorder(header, nil, nil, nil, list))
}
