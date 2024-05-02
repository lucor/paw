// Copyright 2024 the Paw Authors. All rights reserved.
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

// itemViewWidget is a custom widget that displays a paw item
type itemViewWidget struct {
	widget.BaseWidget

	ctx        context.Context
	itemWidget FyneItemWidget
	win        fyne.Window

	editBtn  *widget.Button
	OnSubmit func()
}

func newItemViewWidget(ctx context.Context, itemWidget FyneItemWidget, win fyne.Window) *itemViewWidget {
	editBtn := &widget.Button{
		Text: "Edit",
		Icon: theme.DocumentCreateIcon(),
	}
	ivw := &itemViewWidget{
		ctx:        ctx,
		itemWidget: itemWidget,
		win:        win,

		editBtn: editBtn,
	}
	ivw.ExtendBaseWidget(ivw)
	ivw.editBtn.OnTapped = func() {
		ivw.OnSubmit()
	}
	return ivw
}

func (ivw *itemViewWidget) CreateRenderer() fyne.WidgetRenderer {
	metadata := ivw.itemWidget.Item().GetMetadata()
	itemContent := ivw.itemWidget.Show(ivw.ctx, ivw.win)
	metaContent := ShowMetadata(metadata)
	bottom := container.NewVBox(metaContent, ivw.editBtn)
	c := container.NewBorder(nil, bottom, nil, nil, itemContent)
	return widget.NewSimpleRenderer(c)
}
