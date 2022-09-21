package main

import (
	"runtime/debug"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/ui"
)

// Version allow to set the version at link time
var Version string

func main() {
	a := app.NewWithID("dev.lucor.paw")
	a.SetIcon(icon.PawIcon)

	w := a.NewWindow("Paw")
	w.SetMaster()
	w.Resize(fyne.NewSize(400, 600))
	w.SetContent(ui.MakeApp(w, version()))
	w.ShowAndRun()
}

func version() string {
	if Version != "" {
		return Version
	}

	info, ok := debug.ReadBuildInfo()
	if ok {
		return info.Main.Version
	}
	return "(unknown)"
}
