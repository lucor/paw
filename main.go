// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	urfave "github.com/urfave/cli/v3"
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
	// handle application start: CLI, GUI
	if at.IsCLI() && at.IsMobile() {
		fmt.Fprintln(os.Stderr, "Error: CLI app is unsupported on this OS")
		return cli.ExitCodeApplicationError
	}

	if !at.IsCLI() && at.IsWindowsOS() {
		// On Windows, to ship a single binary for GUI and CLI we need to build as
		// "console binary" and detach the console when running as GUI
		ui.DetachConsole()
	}

	fyneApp := app.NewWithID(ui.AppID)
	fyneApp.SetIcon(icon.PawIcon)
	s, err := storage.NewAppStorage(fyneApp)
	if err != nil {
		slog.Error("could not initialize storage", "component", "app", "error", err)
		fmt.Fprintf(os.Stderr, "Paw could not start: %v\n", err)
		return cli.ExitCodeApplicationError
	}
	defer s.Close()
	if logFile := configureLogger(s, at.IsMobile(), os.Stderr); logFile != nil {
		defer logFile.Close()
	}

	// Write the native manifests to support browser native messaging for the current OS
	// TODO: this should be once at installation time
	if err := browser.WriteNativeManifests(s); err != nil {
		slog.Warn("could not install browser manifests", "component", "app", "error", err)
	}

	// Handle message from browser extension
	if at.IsMessageFromBrowserExtension() {
		browser.HandleNativeMessage(s)
		return cli.ExitCodeSuccess
	}

	if at.IsCLI() {
		// Run the CLI app
		if err := cli.Run(os.Args, s); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return cliErrorExitCode(err)
		}
		return cli.ExitCodeSuccess
	}

	// check for running instance looking at the health service
	if ui.HealthServiceCheck(s) {
		fmt.Fprintln(os.Stderr, "Paw GUI is already running")
		return cli.ExitCodeApplicationError
	}
	// start the health service
	go ui.HealthService(s)

	// agent could be already running (e.g. from CLI)
	// if not, start it
	var agentType agent.Type
	c, err := agent.NewClient(s.SocketAgentPath())
	if err == nil {
		agentType, _ = c.Type()
	}

	// start the GUI agent if not already running
	if agentType.IsZero() {
		socketPath, err := s.PrepareAgentSocket()
		if err != nil {
			slog.Error("could not prepare agent socket", "component", "app", "error", err)
		} else {
			go agent.Run(agent.NewGUI(), socketPath)
		}
	}

	// create window and run the app
	w := fyneApp.NewWindow(ui.AppTitle)
	w.SetMaster()
	w.Resize(fyne.NewSize(ui.DefaultWindowWidth, ui.DefaultWindowHeight))
	w.SetContent(ui.MakeApp(s, w))
	w.ShowAndRun()
	return cli.ExitCodeSuccess
}

// cliErrorExitCode returns the exit code carried by a CLI error.
func cliErrorExitCode(err error) int {
	var exitCoder urfave.ExitCoder
	if errors.As(err, &exitCoder) {
		return exitCoder.ExitCode()
	}
	return cli.ExitCodeApplicationError
}

func configureLogger(store *storage.FileStorage, mobile bool, stderr io.Writer) io.Closer {
	if mobile {
		if err := logging.Configure(stderr); err != nil {
			configureFallbackLogger(stderr)
			slog.Warn("could not configure logger", "component", "app", "error", err)
		}
		return nil
	}

	logFile, err := store.OpenLogFile(os.Getenv("PAW_LOG_FILE"))
	if err != nil {
		configureFallbackLogger(stderr)
		slog.Warn("could not initialize file logging", "component", "app", "error", err)
		return nil
	}
	if err := logging.Configure(logFile); err != nil {
		_ = logFile.Close()
		configureFallbackLogger(stderr)
		slog.Warn("could not configure logger", "component", "app", "error", err)
		return nil
	}
	return logFile
}

func configureFallbackLogger(stderr io.Writer) {
	slog.SetDefault(slog.New(slog.NewTextHandler(stderr, nil)))
}
