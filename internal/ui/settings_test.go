// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func TestSettingsViewEditsAndPersistsSettings(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Settings")
	t.Cleanup(window.Close)
	state := &model.AppState{Settings: model.NewDefaultSettings()}
	saved := 0
	store := &storage.StorageMock{}
	store.OnStoreAppState = func(got *model.AppState) error {
		require.Same(t, state, got)
		saved++
		return nil
	}
	view := NewSettingsView(state, store, window)
	window.SetContent(view.View)
	window.Resize(fyne.NewSize(430, 926))

	var favicon *widget.Check
	var selects []*widget.Select
	var steppers []*widget.Button
	for _, object := range test.LaidOutObjects(view.View) {
		switch object := object.(type) {
		case *widget.Check:
			favicon = object
		case *widget.Select:
			selects = append(selects, object)
		case *widget.Button:
			if object.Icon == icons.Plus {
				steppers = append(steppers, object)
			}
		}
	}
	require.NotNil(t, favicon)
	require.Len(t, selects, 3)
	require.Len(t, steppers, 3)
	require.NotNil(t, view.BackButton)

	test.Tap(favicon)
	require.True(t, state.Settings.FaviconDownloader.Disabled)
	for _, selectWidget := range selects {
		if len(selectWidget.Options) == 2 {
			selectWidget.SetSelected(startupVaultChooseLabel)
		}
	}
	test.Tap(steppers[0])
	require.Equal(t, model.StartupVaultChooseAtStartup, state.Settings.StartupVault)
	require.Equal(t, paw.PassphrasePasswordDefaultLength+1, state.Settings.Password.Passphrase.DefaultLength)
	require.Equal(t, 3, saved)

	texts := auditTexts(view.View)
	require.Contains(t, texts, "Settings")
	require.Contains(t, texts, "Download favicons")
	require.Contains(t, texts, "Download website icons automatically")
	require.NotContains(t, texts, "TOTP defaults")
	require.NotContains(t, texts, "Algorithm")
	require.NotContains(t, texts, "Interval")
}

func TestSettingsViewPersistsStableSelectionKeys(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	state := &model.AppState{Settings: model.NewDefaultSettings()}
	require.Equal(t, model.DefaultTheme, state.Settings.Theme)
	require.Equal(t, model.DefaultStartupVault, state.Settings.StartupVault)
	store := &storage.StorageMock{}
	store.OnStoreAppState = func(*model.AppState) error { return nil }
	window := app.NewWindow("Settings")
	t.Cleanup(window.Close)
	view := NewSettingsView(state, store, window)

	var selects []*widget.Select
	for _, object := range test.LaidOutObjects(view.View) {
		if selectWidget, ok := object.(*widget.Select); ok {
			selects = append(selects, selectWidget)
		}
	}
	require.Len(t, selects, 3)

	var startup, appearance *widget.Select
	for _, selectWidget := range selects {
		switch selectWidget.Selected {
		case startupVaultLastUsedLabel:
			startup = selectWidget
		case settingsThemeSystemLabel:
			appearance = selectWidget
		}
	}
	require.NotNil(t, startup)
	require.NotNil(t, appearance)
	startup.SetSelected(startupVaultChooseLabel)
	appearance.SetSelected(settingsThemeDarkLabel)
	require.Equal(t, model.StartupVaultChooseAtStartup, state.Settings.StartupVault)
	require.Equal(t, model.ThemeDark, state.Settings.Theme)
}

func TestSettingsViewPersistsCanonicalAutoLockKey(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	state := &model.AppState{Settings: model.NewDefaultSettings()}
	store := &storage.StorageMock{}
	store.OnStoreAppState = func(*model.AppState) error { return nil }
	window := app.NewWindow("Settings")
	t.Cleanup(window.Close)
	view := NewSettingsView(state, store, window)

	var autoLock *widget.Select
	for _, object := range test.LaidOutObjects(view.View) {
		selectWidget, ok := object.(*widget.Select)
		if ok && len(selectWidget.Options) == len(autoLockChoices) {
			autoLock = selectWidget
			break
		}
	}
	require.NotNil(t, autoLock)
	autoLock.SetSelected("1 minute")
	require.Equal(t, model.AutoLockOneMinute, state.Settings.AutoLock)
}

func TestSettingsTypographyFollowsSelectedTheme(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	installPawTheme(model.ThemeLight)
	window := app.NewWindow("Settings")
	t.Cleanup(window.Close)
	state := &model.AppState{Settings: model.NewDefaultSettings()}
	store := &storage.StorageMock{}
	store.OnStoreAppState = func(*model.AppState) error { return nil }
	view := NewSettingsView(state, store, window)
	window.SetContent(view.View)
	window.Resize(fyne.NewSize(430, 926))
	window.Show()

	var appearance *widget.Select
	for _, object := range test.LaidOutObjects(view.View) {
		selectWidget, ok := object.(*widget.Select)
		if ok && len(selectWidget.Options) == 3 && selectWidget.Options[0] == settingsThemeSystemLabel {
			appearance = selectWidget
			break
		}
	}
	require.NotNil(t, appearance)

	findRenderedLabel := func(label string) *canvas.Text {
		for _, object := range test.LaidOutObjects(view.View) {
			if text, ok := object.(*canvas.Text); ok && text.Text == label {
				return text
			}
		}
		return nil
	}
	labels := []string{"Settings", "General", "Startup vault", "Download website icons automatically"}
	for _, selection := range []string{settingsThemeSystemLabel, settingsThemeDarkLabel, settingsThemeLightLabel} {
		appearance.SetSelected(selection)
		want := app.Settings().Theme().Color(theme.ColorNameForeground, app.Settings().ThemeVariant())
		for _, label := range labels {
			text := findRenderedLabel(label)
			require.NotNil(t, text, label)
			require.Equal(t, want, text.Color, selection+": "+label)
		}
	}
}
