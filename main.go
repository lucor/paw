// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"lucor.dev/paw/internal/agent"
	"lucor.dev/paw/internal/browser"
	"lucor.dev/paw/internal/cli"
	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/paw"
	"lucor.dev/paw/internal/ui"
)

// appType detects the application type from the command line arguments and the runtime
type appType struct {
	args []string
}

// IsCLI returns true if the application is a CLI app
func (a *appType) IsCLI() bool {
	return len(a.args) > 1 && a.args[1] == "cli"
}

// IsGUI returns true if the application is a GUI app
func (a *appType) IsGUI() bool {
	return !a.IsCLI()
}

// IsMessageFromBrowserExtension returns true if the application is a message from the browser extension
func (a *appType) IsMessageFromBrowserExtension() bool {
	return len(a.args) > 1 && browser.MessageFromExtension(a.args[1:])
}

// IsMobile returns true if the application is running on a mobile device
func (a *appType) IsMobile() bool {
	return runtime.GOOS == "android" || runtime.GOOS == "ios"
}

// IsWindowsOS returns true if the application is running on Windows
func (a *appType) IsWindowsOS() bool {
	return runtime.GOOS == "windows"
}

func main() {

	at := &appType{args: os.Args}

	// handle application start: CLI, GUI
	if at.IsCLI() && at.IsMobile() {
		fmt.Fprintln(os.Stderr, "CLI app is unsupported on this OS")
		os.Exit(1)
	}

	if !at.IsCLI() && at.IsWindowsOS() {
		// On Windows, to ship a single binary for GUI and CLI we need to build as
		// "console binary" and detach the console when running as GUI
		ui.DetachConsole()
	}

	fyneApp := app.NewWithID(ui.AppID)
	fyneApp.SetIcon(icon.PawIcon)
	s, err := makeStorage(at, fyneApp)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Write the native manifests to support browser native messaging for the current OS
	// TODO: this should be once at installation time
	browser.WriteNativeManifests()

	// Handle message from browser extension
	if at.IsMessageFromBrowserExtension() {
		browser.HandleNativeMessage(s)
		return
	}

	if at.IsCLI() {
		// Run the CLI app
		cli.Run(os.Args, s)
		return
	}

	// check for running instance looking at the health service
	if ui.HealthServiceCheck(s.LockFilePath()) {
		fmt.Fprintln(os.Stderr, "Paw GUI is already running")
		os.Exit(1)
	}
	// start the health service
	go ui.HealthService(s.LockFilePath())

	// agent could be already running (e.g. from CLI)
	// if not, start it
	var agentType agent.Type
	c, err := agent.NewClient(s.SocketAgentPath())
	if err == nil {
		agentType, _ = c.Type()
	}

	// start the GUI agent if not already running
	if agentType.IsZero() {
		go agent.Run(agent.NewGUI(), s.SocketAgentPath())
	}

	// create window and run the app
	w := fyneApp.NewWindow(ui.AppTitle)
	w.SetMaster()
	w.Resize(fyne.NewSize(400, 600))
	w.SetContent(ui.MakeApp(s, w))
	w.ShowAndRun()
}

// makeStorage create the storage
func makeStorage(at *appType, fyneApp fyne.App) (paw.Storage, error) {
	if at.IsMobile() {
		// Mobile app returns the Fyne storage
		return paw.NewFyneStorage(fyneApp.Storage())
	}
	// Otherwise returns the OS storage
	return paw.NewOSStorage()
}
