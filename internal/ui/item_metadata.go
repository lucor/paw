// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"go.lucor.dev/paw/internal/icon"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// Declare conformity to Item interface
var _ = (*Metadata)(nil)

// Item represents the basic paw identity
type Metadata struct {
	Item *model.Item
}

// Icon returns the icon for the item's Paw category.
func (m *Metadata) Icon() fyne.Resource {
	if m.Item.Metadata.Type == paw.LoginItemType && m.Item.Metadata.Favicon != nil {
		return m.Item.Metadata.Favicon
	}
	switch m.Item.Metadata.Type {
	case paw.NoteItemType:
		return icon.NoteOutlinedIconThemed
	case paw.PasswordItemType:
		return icon.PasswordOutlinedIconThemed
	case paw.LoginItemType:
		return icon.WorldWWWOutlinedIconThemed
	case paw.SSHKeyItemType:
		return icon.KeyOutlinedIconThemed
	}
	return icon.PawIcon
}

// ShowMetadata displays the item timestamps.
func ShowMetadata(m *model.Item) fyne.CanvasObject {
	created, modified := "", ""
	if m.CreationAt != nil {
		created = time.Unix(int64(*m.CreationAt), 0).Local().Format(time.RFC1123)
	}
	if m.ModifiedAt != nil {
		modified = time.Unix(int64(*m.ModifiedAt), 0).Local().Format(time.RFC1123)
	}
	return container.New(
		layout.NewFormLayout(),
		widget.NewLabel("Modified"),
		widget.NewLabel(modified),
		widget.NewLabel("Created"),
		widget.NewLabel(created),
	)
}
