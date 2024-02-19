// Copyright 2021 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"io"
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

	var fyneApp fyne.App
	// handle application start: CLI, GUI
	args := len(os.Args)
	if args > 1 && os.Args[1] == "cli" {
		if runtime.GOOS == "android" || runtime.GOOS == "ios" {
			fmt.Println("CLI app is unsupported on this OS")
			os.Exit(1)
		}
	} else {
		fyneApp = app.NewWithID(ui.AppID)
		fyneApp.SetIcon(icon.PawIcon)
		if runtime.GOOS == "windows" {
			// On Windows, to ship a single binary for GUI and CLI we need to build as
			// "console binary" and detach the console when running as GUI
			ui.DetachConsole()
		}
	}

	s, err := makeStorage(fyneApp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Enable debugging log, if set
	wc := initDebugLog(s)
	defer wc.Close()

	if fyneApp == nil {
		// make and run the CLI app
		cmd, err := cli.New(s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[✗] %s\n", err)
			os.Exit(1)
		}
		err = cmd.Run(s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[✗] %s\n", err)
			os.Exit(1)
		}
		return
	}

	// check for running instance
	if ui.HealthServiceCheck(s.LockFilePath()) {
		msg := "Paw GUI is already running\n"
		wc.Write([]byte(msg))
		os.Exit(1)
	}
	go ui.HealthService(s.LockFilePath())

	// agent could be already running (e.g. from CLI)
	// if not, start it
	var agentType agent.Type
	c, err := agent.NewClient(s.SocketAgentPath())
	if err == nil {
		agentType, _ = c.Type()
	}

	if agentType.IsZero() {
		go agent.Run(agent.NewGUI(), s.SocketAgentPath())
	}

	// create window and run the app
	w := fyneApp.NewWindow(ui.AppTitle)
	w.SetMaster()
	w.Resize(fyne.NewSize(400, 600))
	w.SetContent(ui.MakeApp(w))
	w.ShowAndRun()
}

// makeStorage create the storage
func makeStorage(fyneApp fyne.App) (paw.Storage, error) {
	if fyneApp == nil {
		// CLI app returns the OS storage
		return paw.NewOSStorage()
	}
	device := fyneApp.Driver().Device()
	if device.IsMobile() {
		// Fyne Mobile app returns the Fyne storage
		return paw.NewFyneStorage(fyneApp.Storage())
	}
	// Fyne Desktop app returns the OS storage
	return paw.NewOSStorage()
}

// initDebugLog create the debug log, if enabled
func initDebugLog(s paw.Storage) io.WriteCloser {
	w := os.Stderr
	// Enable log debugging
	if !paw.IsDebug() {
		return w
	}
	// init logger
	if fyne.CurrentDevice().IsMobile() {
		w := os.Stdout
		log.SetOutput(w)
		log.SetPrefix("[paw] ")
		return w
	}
	w, err := os.OpenFile(s.LogFilePath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintln(w, "paw: error writing log file %w", err)
		return w
	}
	log.SetOutput(w)
	return w
}
