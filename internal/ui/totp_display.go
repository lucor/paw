// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/otp"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// TOTPDisplay keeps the shared code, countdown, and copy presentation.
type TOTPDisplay struct {
	settings  cxf.TOTP
	code      *widget.Label
	progress  *widget.ProgressBar
	copy      *iconButton
	codeValue string
	remaining int
}

// newTOTPDisplay builds the compact TOTP row used by Login Detail and Codes.
func newTOTPDisplay(credential cxf.TOTP) (*TOTPDisplay, bool) {
	if credential.Secret == "" || credential.Period == 0 || credential.Digits == 0 {
		return nil, false
	}
	switch credential.Algorithm {
	case "sha1", "sha256", "sha512":
	default:
		return nil, false
	}
	display := &TOTPDisplay{
		settings: credential,
		code:     newText("", headingStyle),
		progress: widget.NewProgressBar(),
	}
	display.progress.TextFormatter = func() string { return fmt.Sprintf("%ds", display.remaining) }
	if !display.Update() {
		return nil, false
	}
	display.copy = newIconButton("Copy verification code", icons.Copy, func() {
		if display.Update() {
			detailCopy("Verification code", display.Value())
		}
	})
	return display, true
}

// View returns the code and countdown on a single expanding row.
func (d *TOTPDisplay) View() fyne.CanvasObject {
	progress := container.New(layout.NewCustomPaddedVBoxLayout(0), layout.NewSpacer(), d.progress, layout.NewSpacer())
	return container.NewBorder(nil, nil, d.code, container.NewCenter(d.copy), progress)
}

// Update calculates the current code and remaining period seconds.
func (d *TOTPDisplay) Update() bool {
	if d.settings.Secret == "" {
		return false
	}
	hasher := paw.TOTPHash(strings.ToUpper(d.settings.Algorithm)).Hasher()
	value, err := otp.TOTPFromBase32(hasher, d.settings.Secret, time.Now().UTC(), int(d.settings.Period), int(d.settings.Digits))
	if err != nil {
		d.codeValue = ""
		d.code.SetText("")
		return false
	}
	d.codeValue = value
	d.code.SetText(formatTOTPCode(value))
	d.remaining = int(d.settings.Period) - int(time.Now().Unix()%int64(d.settings.Period))
	d.progress.SetValue(float64(d.remaining) / float64(d.settings.Period))
	d.progress.Refresh()
	return true
}

// Value returns the current unformatted code for clipboard use.
func (d *TOTPDisplay) Value() string { return d.codeValue }

// Run refreshes the display once a second until its view is closed.
func (d *TOTPDisplay) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fyne.Do(func() {
				if ctx.Err() == nil {
					d.Update()
				}
			})
		}
	}
}

// Clear removes the current code and secret from the display state.
func (d *TOTPDisplay) Clear() {
	d.settings.Secret = ""
	d.codeValue = ""
	d.remaining = 0
	d.code.SetText("")
	d.progress.SetValue(0)
	d.progress.Refresh()
}

func formatTOTPCode(value string) string {
	if len(value) < 6 || len(value)%2 != 0 {
		return value
	}
	middle := len(value) / 2
	return value[:middle] + " " + value[middle:]
}
