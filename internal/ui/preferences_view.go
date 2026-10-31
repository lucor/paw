// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// PreferencesView displays and persists application preferences.
type PreferencesView struct {
	View       fyne.CanvasObject
	BackButton *iconButton

	state   *paw.AppState
	storage storage.Storage
	window  fyne.Window
}

// NewPreferencesView builds the application settings screen.
func NewPreferencesView(state *paw.AppState, store storage.Storage, window fyne.Window) *PreferencesView {
	if state == nil {
		state = &paw.AppState{}
	}
	if state.Preferences == nil {
		state.Preferences = paw.NewDefaultPreferences()
	}
	e := &PreferencesView{state: state, storage: store, window: window}
	e.BackButton = newIconButton("Back", icons.Back, nil)
	e.BackButton.Importance = widget.LowImportance
	header := newPageHeader(e.BackButton, newText("Settings", titleStyle), nil)
	content := container.NewVScroll(container.New(layout.NewCustomPaddedVBoxLayout(16),
		e.faviconPreferences(), e.passwordPreferences(), e.totpPreferences(),
	))
	e.View = newSecondaryPage(header, nil, content)
	return e
}

func (e *PreferencesView) save() {
	if err := e.storage.StoreAppState(e.state); err != nil {
		dialog.ShowError(err, e.window)
	}
}

func (e *PreferencesView) faviconPreferences() fyne.CanvasObject {
	checkbox := widget.NewCheck("Disabled", func(disabled bool) {
		e.state.Preferences.FaviconDownloader.Disabled = disabled
		e.save()
	})
	checkbox.SetChecked(e.state.Preferences.FaviconDownloader.Disabled)
	return widget.NewCard("Favicon downloader", "", checkbox)
}

func (e *PreferencesView) passwordPreferences() fyne.CanvasObject {
	return container.NewVBox(
		widget.NewCard("Passphrase", "", e.lengthControl(&e.state.Preferences.Password.Passphrase.DefaultLength, e.state.Preferences.Password.Passphrase.MinLength, e.state.Preferences.Password.Passphrase.MaxLength)),
		widget.NewCard("PIN", "", e.lengthControl(&e.state.Preferences.Password.Pin.DefaultLength, e.state.Preferences.Password.Pin.MinLength, e.state.Preferences.Password.Pin.MaxLength)),
		widget.NewCard("Random password", "", e.lengthControl(&e.state.Preferences.Password.Random.DefaultLength, e.state.Preferences.Password.Random.MinLength, e.state.Preferences.Password.Random.MaxLength)),
	)
}

func (e *PreferencesView) lengthControl(value *int, min, max int) fyne.CanvasObject {
	bindingValue := binding.BindInt(value)
	length := widget.NewEntryWithData(binding.IntToString(bindingValue))
	length.Disable()
	slider := widget.NewSlider(float64(min), float64(max))
	slider.SetValue(float64(*value))
	slider.OnChanged = func(current float64) {
		_ = bindingValue.Set(int(current))
		e.save()
	}
	return container.NewBorder(nil, nil, widget.NewLabel("Default length"), length, slider)
}

func (e *PreferencesView) totpPreferences() fyne.CanvasObject {
	preferences := &e.state.Preferences.TOTP
	form := container.New(layout.NewFormLayout())
	hashOptions := []string{string(paw.SHA1), string(paw.SHA256), string(paw.SHA512)}
	hash := widget.NewSelect(hashOptions, func(selected string) {
		preferences.Hash = paw.TOTPHash(selected)
		e.save()
	})
	hash.Selected = string(preferences.Hash)
	form.Add(newText("Hash algorithm", captionStyle))
	form.Add(hash)
	digitsOptions := []string{"5", "6", "7", "8", "9", "10"}
	digits := widget.NewSelect(digitsOptions, func(selected string) {
		preferences.Digits, _ = strconv.Atoi(selected)
		e.save()
	})
	digits.Selected = strconv.Itoa(preferences.Digits)
	form.Add(newText("Digits", captionStyle))
	form.Add(digits)
	interval := binding.BindInt(&preferences.Interval)
	slider := widget.NewSlider(5, 60)
	slider.Step = 5
	slider.SetValue(float64(preferences.Interval))
	slider.OnChanged = func(value float64) {
		_ = interval.Set(int(value))
		e.save()
	}
	form.Add(newText("Interval", captionStyle))
	form.Add(container.NewBorder(nil, nil, nil, widget.NewLabelWithData(binding.IntToString(interval)), slider))
	return widget.NewCard("Two factor authentication (TOTP)", "", form)
}
