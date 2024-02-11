// Copyright 2022 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ui

import (
	"context"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (a *app) makeShowItemView(fyneItem FyneItem) fyne.CanvasObject {

	editBtn := widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), func() {
		a.showEditItemView(fyneItem)
	})

	metadata := fyneItem.Item().GetMetadata()
	itemContent := fyneItem.Show(context.TODO(), a.win)
	metaContent := ShowMetadata(metadata)

	content := container.NewBorder(nil, metaContent, nil, nil, itemContent)

	return container.NewBorder(a.makeCancelHeaderButton(), editBtn, nil, nil, content)
}
