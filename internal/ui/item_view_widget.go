// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

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
