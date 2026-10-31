// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// VaultChoice describes a vault available in the selector.
type VaultChoice struct {
	Name       string
	Appearance storage.VaultAppearance
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
			row := NewActionRow("Vault", "Locked", icons.Lock, nil)
			return container.New(layout.NewCustomPaddedLayout(0, 0, theme.Size(theme.SizeNameInnerPadding), theme.Size(theme.SizeNameInnerPadding)), row)
		},
		func(id widget.ListItemID, object fyne.CanvasObject) {
			vault := vaults[id]
			status := "Locked"
			if vault.Unlocked {
				status = "Unlocked"
			}
			row := object.(*fyne.Container).Objects[0].(*ActionRow)
			row.SetContent(vault.Name, status, vaultIconResource(vault.Appearance.Icon))
			row.SetLeadingObject(newVaultIcon(vault.Appearance, vaultIconMedium))
			row.SetTapped(func() { list.Select(id) })
		},
	)
	list.HideSeparators = true
	list.OnSelected = func(id widget.ListItemID) {
		list.UnselectAll()
		vaults[id].Open()
	}
	header := container.NewVBox(
		container.NewBorder(nil, nil, nil, newVaultOptions(addVault), newText("Your vaults", headingStyle)),
		newText("Select a vault to open.", bodyStyle),
	)
	return container.New(layout.NewCustomPaddedLayout(24, 24, 24, 24),
		container.NewBorder(header, nil, nil, nil, list))
}
