package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func ShowErrorDialog(title string, err error, w fyne.Window) {
	content := &widget.Label{
		Text:     fmt.Sprintf("Error: %s", err),
		Wrapping: fyne.TextWrapBreak,
	}

	d := dialog.NewCustom(title, "Ok", content, w)
	d.Show()
}
