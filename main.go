// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"go.lucor.dev/paw/internal/agent"
	"go.lucor.dev/paw/internal/browser"
	"go.lucor.dev/paw/internal/cli"
	"go.lucor.dev/paw/internal/icon"
	"go.lucor.dev/paw/internal/logging"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui"
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
	os.Exit(run())
}

func run() int {
	at := &appType{args: os.Args}
	if logger := makeLogger(); logger != nil {
		defer logger.Close()
	}

	// handle application start: CLI, GUI
	if at.IsCLI() && at.IsMobile() {
		fmt.Fprintln(os.Stderr, "CLI app is unsupported on this OS")
		return 1
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
		slog.Error("could not initialize storage", "component", "app", "error", err)
		fmt.Fprintf(os.Stderr, "Paw could not start: %v\n", err)
		return 1
	}
	defer s.Close()

	// Write the native manifests to support browser native messaging for the current OS
	// TODO: this should be once at installation time
	browser.WriteNativeManifests()

	// Handle message from browser extension
	if at.IsMessageFromBrowserExtension() {
		browser.HandleNativeMessage(s)
		return 0
	}

	if at.IsCLI() {
		// Run the CLI app
		if err := cli.Run(os.Args, s); err != nil {
			fmt.Fprintf(os.Stderr, "[✗] %s\n", err)
			return 1
		}
		return 0
	}

	// check for running instance looking at the health service
	if ui.HealthServiceCheck(s.LockFilePath()) {
		fmt.Fprintln(os.Stderr, "Paw GUI is already running")
		return 1
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
	w.Resize(fyne.NewSize(ui.DefaultWindowWidth, ui.DefaultWindowHeight))
	w.SetContent(ui.MakeApp(s, w))
	w.ShowAndRun()
	return 0
}

// makeLogger configures application logging and returns the log file closer.
// When file logging cannot start it falls back to stderr, so startup never fails.
func makeLogger() io.Closer {
	closer, err := logging.Init()
	if err == nil {
		return closer
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	slog.Warn("could not initialize file logging", "component", "app", "error", err)
	return nil
}

// makeStorage create the storage
func makeStorage(at *appType, fyneApp fyne.App) (*storage.FileStorage, error) {
	root := ""
	if !at.IsMobile() {
		return storage.NewFileStorage(root)
	}
	// Fyne selects the private application directory on mobile.
	uri := fyneApp.Storage().RootURI()
	if uri == nil || uri.Scheme() != "file" || uri.Authority() != "" || uri.Query() != "" || uri.Fragment() != "" {
		return nil, fmt.Errorf("application storage must provide a local file URI")
	}
	root = uri.Path()
	return storage.NewFileStorage(root)
}
