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
	args := len(os.Args)
	if args > 1 {
		if os.Args[1] == "cli" {
			if runtime.GOOS == "android" || runtime.GOOS == "ios" {
				fmt.Println("cli is unsupported on this OS")
				os.Exit(1)
			}
			// make CLI app
			cli.New(s)
			return
		}
	}

	if ui.HealthServiceCheck() {
		fmt.Println("paw GUI is already running, exits")
		os.Exit(1)
	}

	go ui.HealthService()

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
