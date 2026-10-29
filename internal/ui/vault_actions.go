// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// NewVaultActions connects the implemented vault management operations to native menu items.
func NewVaultActions(audit, exportFile, lock func()) []*fyne.MenuItem {
	items := []*fyne.MenuItem{
		fyne.NewMenuItem("Password audit", audit),
		fyne.NewMenuItem("Export credentials", exportFile),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Lock vault", lock),
	}
	items[0].Icon = icons.Audit
	items[1].Icon = icons.Export
	items[3].Icon = icons.LockVault
	return items
}

// iconButton adds a spoken label to Fyne's native icon-only button.
type iconButton struct {
	widget.Button
	label string
}

func newIconButton(label string, resource fyne.Resource, tapped func()) *iconButton {
	button := &iconButton{label: label}
	button.Icon = resource
	button.OnTapped = tapped
	button.ExtendBaseWidget(button)
	return button
}

// AccessibilityLabel describes the action without relying on an icon filename.
func (b *iconButton) AccessibilityLabel() string { return b.label }
