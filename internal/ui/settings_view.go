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
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
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
	state             *model.AppState
	storage           storage.Storage
	window            fyne.Window
	configureAutoLock func(string)
	openAbout         func()
}

// NewSettingsView builds the application's Settings view.
func NewSettingsView(state *model.AppState, store storage.Storage, window fyne.Window) *SettingsView {
	if state == nil {
		state = &model.AppState{}
	}
	if state.Settings == nil {
		state.Settings = model.NewDefaultSettings()
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
	if p.StartupVault == model.StartupVaultChooseAtStartup || p.StartupVault == startupVaultChooseLabel {
		startup.Selected = startupVaultChooseLabel
	}
	startup.OnChanged = func(value string) {
		p.StartupVault = model.StartupVaultLastUsed
		if value == startupVaultChooseLabel {
			p.StartupVault = model.StartupVaultChooseAtStartup
		}
		e.save()
	}
	appearance := widget.NewSelect([]string{settingsThemeSystemLabel, settingsThemeLightLabel, settingsThemeDarkLabel}, nil)
	appearance.Selected = settingsThemeSystemLabel
	switch p.Theme {
	case model.ThemeLight, settingsThemeLightLabel:
		appearance.Selected = settingsThemeLightLabel
	case model.ThemeDark, settingsThemeDarkLabel:
		appearance.Selected = settingsThemeDarkLabel
	}
	appearance.OnChanged = func(value string) {
		switch value {
		case settingsThemeLightLabel:
			p.Theme = model.ThemeLight
		case settingsThemeDarkLabel:
			p.Theme = model.ThemeDark
		default:
			p.Theme = model.ThemeSystem
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
	heading := newText(title, sectionTitleStyle)
	objects := []fyne.CanvasObject{heading}
	objects = append(objects, rows...)
	return container.NewVBox(widget.NewSeparator(), container.New(layout.NewCustomPaddedVBoxLayout(theme.Size(theme.SizeNameInnerPadding)), objects...))
}

func installPawTheme(value string) {
	selected := newPawTheme()
	setPawThemeVariant(selected, value)
	fyne.CurrentApp().Settings().SetTheme(selected)
}

func applySettingsTheme(value string) {
	settings := fyne.CurrentApp().Settings()
	selected, ok := settings.Theme().(*pawTheme)
	if !ok {
		selected = newPawTheme()
	}
	setPawThemeVariant(selected, value)
	settings.SetTheme(selected)
}

func setPawThemeVariant(selected *pawTheme, value string) {
	if value == model.ThemeLight || value == settingsThemeLightLabel {
		selected.setVariant(theme.VariantLight, true)
		return
	}
	if value == model.ThemeDark || value == settingsThemeDarkLabel {
		selected.setVariant(theme.VariantDark, true)
		return
	}
	selected.setVariant(theme.VariantLight, false)
}

func (e *SettingsView) save() {
	if err := e.storage.StoreAppState(e.state); err != nil {
		dialog.ShowError(err, e.window)
	}
}

func (e *SettingsView) row(title, description string, icon fyne.Resource, control fyne.CanvasObject) fyne.CanvasObject {
	label := newText(title, itemTitleStyle)
	textContent := container.New(layout.NewCustomPaddedVBoxLayout(0), label)
	if description != "" {
		textContent.Add(newText(description, captionStyle))
	}
	text := container.NewThemeOverride(textContent, compactTextTheme{Theme: theme.Current()})
	leading := navigationRowIcon(icon)
	return container.NewBorder(nil, nil, leading, container.NewCenter(control), container.NewVBox(layout.NewSpacer(), text, layout.NewSpacer()))
}

func (e *SettingsView) stepper(value *int, min, max, step int) fyne.CanvasObject {
	return newNumericStepper(value, min, max, step, e.save)
}
