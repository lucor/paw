// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
	"image/color"
)

const (
	settingsThemeSystemLabel    = "System"
	settingsThemeLightLabel     = "Light"
	settingsThemeDarkLabel      = "Dark"
	startupVaultLastUsedLabel   = "Last used"
	startupVaultChooseLabel     = "Choose at startup"
	settingsAutoLockNever       = "Never"
	settingsAutoLockDescription = "Lock vaults after Paw is in the background"
)

// SettingsView displays the application's immediately saved settings.
type SettingsView struct {
	View              fyne.CanvasObject
	BackButton        *iconButton
	state             *paw.AppState
	storage           storage.Storage
	window            fyne.Window
	configureAutoLock func(string)
	openAbout         func()
}

// NewSettingsView builds the application's Settings view.
func NewSettingsView(state *paw.AppState, store storage.Storage, window fyne.Window) *SettingsView {
	if state == nil {
		state = &paw.AppState{}
	}
	if state.Settings == nil {
		state.Settings = paw.NewDefaultSettings()
	}
	e := &SettingsView{state: state, storage: store, window: window}
	e.BackButton = newIconButton("Back", icons.Back, nil)
	e.BackButton.Importance = widget.LowImportance
	p := state.Settings
	autoLock := widget.NewSelect(autoLockLabels(), nil)
	autoLock.Selected = autoLockLabel(p.AutoLock)
	autoLock.OnChanged = func(label string) {
		p.AutoLock = autoLockKey(label)
		e.save()
		if e.configureAutoLock != nil {
			e.configureAutoLock(p.AutoLock)
		}
	}
	startup := widget.NewSelect([]string{startupVaultLastUsedLabel, startupVaultChooseLabel}, nil)
	startup.Selected = startupVaultLastUsedLabel
	if p.StartupVault == paw.StartupVaultChooseAtStartup || p.StartupVault == startupVaultChooseLabel {
		startup.Selected = startupVaultChooseLabel
	}
	startup.OnChanged = func(value string) {
		p.StartupVault = paw.StartupVaultLastUsed
		if value == startupVaultChooseLabel {
			p.StartupVault = paw.StartupVaultChooseAtStartup
		}
		e.save()
	}
	appearance := widget.NewSelect([]string{settingsThemeSystemLabel, settingsThemeLightLabel, settingsThemeDarkLabel}, nil)
	appearance.Selected = settingsThemeSystemLabel
	switch p.Theme {
	case paw.ThemeLight, settingsThemeLightLabel:
		appearance.Selected = settingsThemeLightLabel
	case paw.ThemeDark, settingsThemeDarkLabel:
		appearance.Selected = settingsThemeDarkLabel
	}
	appearance.OnChanged = func(value string) {
		switch value {
		case settingsThemeLightLabel:
			p.Theme = paw.ThemeLight
		case settingsThemeDarkLabel:
			p.Theme = paw.ThemeDark
		default:
			p.Theme = paw.ThemeSystem
		}
		applySettingsTheme(p.Theme)
		e.save()
	}
	favicon := widget.NewCheck("", nil)
	favicon.Checked = !p.FaviconDownloader.Disabled
	favicon.OnChanged = func(enabled bool) { p.FaviconDownloader.Disabled = !enabled; e.save() }
	about := newNavigationRow(aboutPageTitle, navigationRowIcon(icons.Info), func() {
		if e.openAbout != nil {
			e.openAbout()
		}
	})
	content := container.NewVBox(
		settingsSection("Security", e.row("Auto-lock", settingsAutoLockDescription, icons.Timer, autoLock)),
		settingsSection("General",
			e.row("Startup vault", "", icons.Play, startup),
			e.row("Download favicons", "Download website icons automatically", icons.Image, favicon)),
		settingsSection("Password generator",
			e.row("Passphrase", "Words", icons.WholeWord, e.stepper(&p.Password.Passphrase.DefaultLength, p.Password.Passphrase.MinLength, p.Password.Passphrase.MaxLength, 1)),
			e.row("PIN", "Digits", icons.Hash, e.stepper(&p.Password.Pin.DefaultLength, p.Password.Pin.MinLength, p.Password.Pin.MaxLength, 1)),
			e.row("Random password", "Characters", icons.Password, e.stepper(&p.Password.Random.DefaultLength, p.Password.Random.MinLength, p.Password.Random.MaxLength, 1))),
		settingsSection("Appearance", e.row("Theme", "", icons.SunMoon, appearance)),
		settingsSection("About", about),
	)
	e.View = newSecondaryPage(newPageHeader(e.BackButton, newText("Settings", titleStyle), nil), nil, container.NewVScroll(content))
	return e
}

func settingsSection(title string, rows ...fyne.CanvasObject) fyne.CanvasObject {
	heading := newText(title, captionStyle)
	heading.TextStyle.Bold = true
	objects := []fyne.CanvasObject{heading}
	objects = append(objects, rows...)
	return container.NewVBox(widget.NewSeparator(), container.New(layout.NewCustomPaddedVBoxLayout(theme.Size(theme.SizeNameInnerPadding)), objects...))
}

// settingsTheme keeps the native theme while selecting its color variant.
type settingsTheme struct {
	fyne.Theme
	variant fyne.ThemeVariant
}

func (t settingsTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return t.Theme.Color(name, t.variant)
}

func applySettingsTheme(value string) {
	var selected fyne.Theme = theme.DefaultTheme()
	if value == paw.ThemeLight || value == settingsThemeLightLabel {
		selected = settingsTheme{selected, theme.VariantLight}
	}
	if value == paw.ThemeDark || value == settingsThemeDarkLabel {
		selected = settingsTheme{selected, theme.VariantDark}
	}
	fyne.CurrentApp().Settings().SetTheme(selected)
}

func (e *SettingsView) save() {
	if err := e.storage.StoreAppState(e.state); err != nil {
		dialog.ShowError(err, e.window)
	}
}

func (e *SettingsView) row(title, description string, icon fyne.Resource, control fyne.CanvasObject) fyne.CanvasObject {
	label := newText(title, bodyStyle)
	label.TextStyle.Bold = true
	textContent := container.New(layout.NewCustomPaddedVBoxLayout(0), label)
	if description != "" {
		textContent.Add(newText(description, captionStyle))
	}
	text := container.NewThemeOverride(textContent, compactTextTheme{Theme: theme.Current()})
	leading := container.NewGridWrap(fyne.NewSquareSize(40), container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(20), widget.NewIcon(icon))))
	return container.NewBorder(nil, nil, leading, container.NewCenter(control), container.NewVBox(layout.NewSpacer(), text, layout.NewSpacer()))
}

func (e *SettingsView) stepper(value *int, min, max, step int) fyne.CanvasObject {
	return newNumericStepper(value, min, max, step, e.save)
}
