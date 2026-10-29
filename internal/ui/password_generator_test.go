// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/paw"
)

func TestPasswordGeneratorUsesGeneratedPassword(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	value := binding.NewString()
	settings := paw.NewCustomPassword()
	NewPasswordGenerator(key, paw.NewDefaultPreferences().Password).ShowPasswordGenerator(value, settings, window)
	popup := window.Canvas().Overlays().Top()
	require.NotNil(t, popup)
	var use *widget.Button
	var generated string
	for _, object := range test.LaidOutObjects(popup) {
		if entry, ok := object.(*widget.Entry); ok && len(entry.Text) > 3 {
			generated = entry.Text
		}
		if button, ok := object.(*widget.Button); ok && button.Text == "Use" {
			use = button
		}
	}
	require.NotEmpty(t, generated)
	require.NotNil(t, use)
	test.Tap(use)
	selected, err := value.Get()
	require.NoError(t, err)
	require.Equal(t, generated, selected)
	require.Nil(t, window.Canvas().Overlays().Top())
}
