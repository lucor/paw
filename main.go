package main

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"lucor.dev/paw/internal/agent"
	"lucor.dev/paw/internal/cli"
	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/paw"
	"lucor.dev/paw/internal/ui"
)

func main() {
	var isCLI bool
	// handle application start: CLI, GUI
	args := len(os.Args)
	if args > 1 && os.Args[1] == "cli" {
		if runtime.GOOS == "android" || runtime.GOOS == "ios" {
			fmt.Println("CLI app is unsupported on this OS")
			os.Exit(1)
		}
		isCLI = true
	}

	if !isCLI && runtime.GOOS == "windows" {
		// On Windows, to ship a single binary for GUI and CLI we need to build as
		// "console binary" and detach the console when running as GUI
		ui.DetachConsole()
	}

	s, err := paw.NewOSStorage()
	if err != nil {
		log.Fatal(err)
	}

	// check for running instance
	var agentType agent.Type
	c, err := agent.NewClient(s.SocketAgentPath())
	if err == nil {
		agentType, _ = c.Type()
	}

	// handle application start: CLI, GUI
	if isCLI {
		// make CLI app
		cli.New(s)
		return
	}

	if ui.HealthServiceCheck(s.LockFilePath()) {
		fmt.Println("Paw GUI is already running")
		os.Exit(1)
	}

	go ui.HealthService(s.LockFilePath())

	if agentType.IsZero() {
		go agent.Run(agent.NewGUI(), s.SocketAgentPath())
	}

	a := app.NewWithID("dev.lucor.paw")
	a.SetIcon(icon.PawIcon)

	w := a.NewWindow("Paw")
	w.SetMaster()
	w.Resize(fyne.NewSize(400, 600))
	w.SetContent(ui.MakeApp(w))
	w.ShowAndRun()
}
