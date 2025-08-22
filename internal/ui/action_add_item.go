// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later


package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"lucor.dev/paw/internal/paw"
)

func (a *app) makeAddItemView() fyne.CanvasObject {
	c := container.NewVBox()
	for _, item := range a.makeEmptyItems() {
		i := item
		metadata := i.GetMetadata()
		fyneItem := NewFyneItemWidget(i, a.state.Preferences)
		o := widget.NewButtonWithIcon(metadata.Type.Label(), fyneItem.Icon(), func() {
			a.showEditItemView(fyneItem)
		})
		o.Alignment = widget.ButtonAlignLeading
		c.Add(o)
	}

	return container.NewBorder(a.makeCancelHeaderButton(), nil, nil, nil, container.NewCenter(c))
}

// makeEmptyItems returns a slice of empty paw.Item ready to use as template for
// item's creation
func (a *app) makeEmptyItems() []paw.Item {
	note := paw.NewNote()
	password := paw.NewPassword()
	website := paw.NewLogin()
	website.TOTP = &paw.TOTP{
		Digits:   a.state.Preferences.TOTP.Digits,
		Hash:     a.state.Preferences.TOTP.Hash,
		Interval: a.state.Preferences.TOTP.Interval,
	}
	sshkey := paw.NewSSHKey()

	return []paw.Item{
		note,
		password,
		website,
		sshkey,
	}
}
