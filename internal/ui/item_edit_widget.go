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
	"lucor.dev/paw/internal/paw"
)

// itemEditWidget is a custom widget that edits an paw item
type itemEditWidget struct {
	widget.BaseWidget

	ctx        context.Context
	key        *paw.Key
	itemWidget FyneItemWidget
	win        fyne.Window

	deleteBtn *widget.Button
	saveBtn   *widget.Button

	OnDelete func()
	OnSave   func()
}

func newItemEditWidget(ctx context.Context, key *paw.Key, itemWidget FyneItemWidget, win fyne.Window) *itemEditWidget {
	saveBtn := &widget.Button{
		Text: "Save",
		Icon: theme.DocumentSaveIcon(),
	}
	deleteBtn := &widget.Button{
		Text:       "Delete",
		Icon:       theme.DeleteIcon(),
		Importance: widget.DangerImportance,
	}
	iew := &itemEditWidget{
		ctx:        ctx,
		key:        key,
		itemWidget: itemWidget,
		win:        win,

		deleteBtn: deleteBtn,
		saveBtn:   saveBtn,
	}
	iew.ExtendBaseWidget(iew)
	iew.deleteBtn.OnTapped = func() {
		iew.OnDelete()
	}
	iew.saveBtn.OnTapped = func() {
		iew.OnSave()
	}
	return iew
}

func (iew *itemEditWidget) CreateRenderer() fyne.WidgetRenderer {
	metadata := iew.itemWidget.Item().GetMetadata()
	itemContent := iew.itemWidget.Edit(iew.ctx, iew.key, iew.win)
	if metadata.IsEmpty() {
		iew.deleteBtn.Hide()
	}
	bottom := container.NewBorder(nil, nil, iew.deleteBtn, iew.saveBtn, widget.NewLabel(""))

	c := container.NewBorder(nil, bottom, nil, nil, itemContent)
	return widget.NewSimpleRenderer(c)
}
