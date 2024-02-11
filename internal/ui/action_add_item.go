// Copyright 2022 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

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
		fyneItem := NewFyneItem(i, a.config)
		o := widget.NewButtonWithIcon(metadata.Type.String(), fyneItem.Icon(), func() {
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
		Digits:   a.config.TOTP.Digits,
		Hash:     a.config.TOTP.Hash,
		Interval: a.config.TOTP.Interval,
	}
	sshkey := paw.NewSSHKey()

	return []paw.Item{
		note,
		password,
		website,
		sshkey,
	}
}
