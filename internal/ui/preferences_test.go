// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
)

func TestPreferencesViewEditsAndPersistsSettings(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Settings")
	t.Cleanup(window.Close)
	state := &paw.AppState{Preferences: paw.NewDefaultPreferences()}
	saved := 0
	store := &storage.StorageMock{}
	store.OnStoreAppState = func(got *paw.AppState) error {
		require.Same(t, state, got)
		saved++
		return nil
	}
	view := NewPreferencesView(state, store, window)
	window.SetContent(view.View)
	window.Resize(fyne.NewSize(430, 926))

	var favicon *widget.Check
	var selects []*widget.Select
	var sliders []*widget.Slider
	for _, object := range test.LaidOutObjects(view.View) {
		switch object := object.(type) {
		case *widget.Check:
			favicon = object
		case *widget.Select:
			selects = append(selects, object)
		case *widget.Slider:
			sliders = append(sliders, object)
		}
	}
	require.NotNil(t, favicon)
	require.Len(t, selects, 2)
	require.Len(t, sliders, 4)
	require.NotNil(t, view.BackButton)

	test.Tap(favicon)
	require.True(t, state.Preferences.FaviconDownloader.Disabled)
	selects[0].SetSelected(string(paw.SHA256))
	selects[1].SetSelected("8")
	sliders[0].OnChanged(6)
	sliders[3].OnChanged(45)
	require.Equal(t, paw.SHA256, state.Preferences.TOTP.Hash)
	require.Equal(t, 8, state.Preferences.TOTP.Digits)
	require.Equal(t, 6, state.Preferences.Password.Passphrase.DefaultLength)
	require.Equal(t, 45, state.Preferences.TOTP.Interval)
	require.Equal(t, 5, saved)

	texts := auditTexts(view.View)
	require.Contains(t, texts, "Settings")
	require.Contains(t, texts, "Hash algorithm")
	require.Contains(t, texts, "Interval")
}
