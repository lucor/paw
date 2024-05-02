// Copyright 2022 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/paw"
)

// Declare conformity to Item interface
var _ paw.Item = (*Metadata)(nil)

// Item represents the basic paw identity
type Metadata struct {
	*paw.Metadata
}

func (m *Metadata) Item() paw.Item {
	return m.Metadata
}

func (m *Metadata) Icon() fyne.Resource {
	if m.Favicon != nil {
		return m.Favicon
	}
	switch m.Type {
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

func ShowMetadata(m *paw.Metadata) fyne.CanvasObject {
	return container.New(
		layout.NewFormLayout(),
		widget.NewLabel("Modified"),
		widget.NewLabel(m.Modified.Local().Format(time.RFC1123)),
		widget.NewLabel("Created"),
		widget.NewLabel(m.Created.Local().Format(time.RFC1123)),
	)
}
