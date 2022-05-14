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
		a.setContent(a.makeEditItemView(fyneItem))
	})

	metadata := fyneItem.Item().GetMetadata()
	itemContent := fyneItem.Show(context.TODO(), a.win)
	metaContent := ShowMetadata(metadata)

	top := a.makeNavigationHeader("", tabHomeIndex)
	content := container.NewBorder(nil, metaContent, nil, nil, itemContent)

	return container.NewBorder(top, editBtn, nil, nil, content)

}
