// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/paw"
)

const totpSettingsDialogTitle = "Authenticator settings"

const (
	totpIntervalMinimum = 5
	totpIntervalMaximum = 60
	totpIntervalStep    = 5
)

var totpHashOptions = []string{string(paw.SHA1), string(paw.SHA256), string(paw.SHA512)}
var totpDigitOptions = []string{"5", "6", "7", "8", "9", "10"}

// showTOTPSettingsDialog edits TOTP parameters before they are saved with the item.
func showTOTPSettingsDialog(editor *LoginEditor, window fyne.Window) {
	working := editor.totpSettings
	algorithm := widget.NewSelect(totpHashOptions, nil)
	algorithm.SetSelected(strings.ToUpper(working.Algorithm))
	digits := widget.NewSelect(totpDigitOptions, nil)
	digits.SetSelected(strconv.Itoa(int(working.Digits)))
	interval := int(working.Period)
	intervalStepper := newNumericStepper(&interval, totpIntervalMinimum, totpIntervalMaximum, totpIntervalStep, nil)
	errorText := newErrorText("")

	intervalLabel := newText("Interval", captionStyle)
	intervalLabel.Wrapping = fyne.TextWrapOff
	intervalRow := container.NewHBox(intervalLabel, layout.NewSpacer(), intervalStepper)
	content := container.New(layout.NewCustomPaddedVBoxLayout(theme.Size(theme.SizeNameInnerPadding)),
		newText("Algorithm", captionStyle), algorithm,
		newText("Digits", captionStyle), digits,
		intervalRow,
		errorText,
	)
	contentSize := content.MinSize()
	if rowWidth := intervalRow.MinSize().Width; rowWidth > contentSize.Width {
		contentSize.Width = rowWidth
	}
	content = container.NewGridWrap(contentSize, content)

	d := dialog.NewCustomWithoutButtons(totpSettingsDialogTitle, content, window)
	cancel := widget.NewButton("Cancel", d.Hide)
	apply := widget.NewButton("Apply", func() {
		selected, err := validatedTOTPSettings(working, algorithm.Selected, digits.Selected, interval)
		if err != nil {
			errorText.SetText(err.Error())
			errorText.Show()
			return
		}
		editor.totpSettings = selected
		d.Hide()
	})
	apply.Importance = widget.HighImportance
	d.SetButtons([]fyne.CanvasObject{container.NewHBox(layout.NewSpacer(), cancel, apply)})
	d.Show()
}

func validatedTOTPSettings(current cxf.TOTP, algorithm, digits string, interval int) (cxf.TOTP, error) {
	if !containsOption(totpHashOptions, algorithm) {
		return current, fmt.Errorf("select a supported TOTP algorithm")
	}
	digitCount, err := strconv.Atoi(digits)
	if err != nil || !containsOption(totpDigitOptions, digits) {
		return current, fmt.Errorf("select a supported number of TOTP digits")
	}
	if interval < totpIntervalMinimum || interval > totpIntervalMaximum {
		return current, fmt.Errorf("interval must be between %d and %d seconds", totpIntervalMinimum, totpIntervalMaximum)
	}
	current.Type = cxf.CredentialTypeTOTP
	current.Algorithm = strings.ToLower(algorithm)
	current.Digits = uint16(digitCount)
	current.Period = uint16(interval)
	return current, nil
}

func containsOption(options []string, value string) bool {
	for _, option := range options {
		if option == value {
			return true
		}
	}
	return false
}
