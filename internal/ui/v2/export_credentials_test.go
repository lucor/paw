// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package v2

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestExportDialogDefaultsAndPlaintextConsent(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, widget.NewLabel("Vault"))
	window.Resize(fyne.NewSize(480, 650))
	ShowExportCredentials(window, nil, nil)
	var mode *widget.RadioGroup
	var password *PasswordField
	var submit *widget.Button
	for _, object := range test.LaidOutObjects(window.Canvas().Overlays().Top()) {
		switch object := object.(type) {
		case *widget.RadioGroup:
			mode = object
		case *PasswordField:
			password = object
		case *widget.Button:
			if object.Text == "Choose destination" {
				submit = object
			}
		}
	}
	require.NotNil(t, mode)
	require.NotNil(t, password)
	require.NotNil(t, submit)
	require.Equal(t, "Encrypted (recommended)", mode.Selected)
	require.True(t, password.Entry.Password)
	test.Tap(submit)
	require.Contains(t, auditTexts(window.Canvas().Overlays().Top()), "Enter an export password and matching confirmation.")
	mode.SetSelected("Plaintext")
	test.Tap(submit)
	require.Contains(t, auditTexts(window.Canvas().Overlays().Top()), "Confirm that you understand the plaintext warning.")
}
