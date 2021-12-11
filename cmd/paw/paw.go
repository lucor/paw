package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/ui"
)

func main() {
	a := app.NewWithID("dev.lucor.paw")
	a.SetIcon(icon.PawIcon)

	w := a.NewWindow("Paw")
	w.SetMaster()

	w.Resize(fyne.NewSize(800, 600))
	w.SetContent(ui.Make(a, w))
	w.ShowAndRun()
}
