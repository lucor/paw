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
