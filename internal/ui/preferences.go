// Copyright 2022 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ui

import (
	"log"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"lucor.dev/paw/internal/paw"
)

func (a *app) makePreferencesView() fyne.CanvasObject {
	content := container.NewVScroll(
		container.NewVBox(
			a.makePasswordPreferencesCard(),
			a.makeTOTPPreferencesCard(),
		),
	)

	return container.NewBorder(a.makeCancelHeaderButton(), nil, nil, nil, content)
}

func (a *app) storePreferences() {
	err := a.storage.StoreConfig(a.config)
	if err != nil {
		dialog.ShowError(err, a.win)
	}
}

func (a *app) makePasswordPreferencesCard() fyne.CanvasObject {
	passphraseCard := widget.NewCard(
		"Passphrase",
		"",
		a.makePreferenceLenghtWidget(&a.config.Password.Passphrase.DefaultLength, a.config.Password.Passphrase.MinLength, a.config.Password.Passphrase.MaxLength),
	)
	pinCard := widget.NewCard(
		"Pin",
		"",
		a.makePreferenceLenghtWidget(&a.config.Password.Pin.DefaultLength, a.config.Password.Pin.MinLength, a.config.Password.Pin.MaxLength),
	)
	randomCard := widget.NewCard(
		"Random Password",
		"",
		a.makePreferenceLenghtWidget(&a.config.Password.Random.DefaultLength, a.config.Password.Random.MinLength, a.config.Password.Random.MaxLength),
	)
	return container.NewVBox(passphraseCard, pinCard, randomCard)
}

func (a *app) makeTOTPPreferencesCard() fyne.CanvasObject {
	form := container.New(layout.NewFormLayout())

	hashOptions := []string{string(paw.SHA1), string(paw.SHA256), string(paw.SHA512)}
	hashSelect := widget.NewSelect(hashOptions, func(selected string) {
		a.config.TOTP.Hash = paw.TOTPHash(selected)
		a.storePreferences()
	})
	hashSelect.Selected = string(a.config.TOTP.Hash)
	form.Add(labelWithStyle("Hash Algorithm"))
	form.Add(hashSelect)

	digitsOptions := []string{"5", "6", "7", "8", "9", "10"}
	digitsSelect := widget.NewSelect(digitsOptions, func(selected string) {
		a.config.TOTP.Digits, _ = strconv.Atoi(selected)
		a.storePreferences()
	})
	digitsSelect.Selected = strconv.Itoa(a.config.TOTP.Digits)
	form.Add(labelWithStyle("Digits"))
	form.Add(digitsSelect)

	intervalBind := binding.BindInt(&a.config.TOTP.Interval)
	intervalSlider := widget.NewSlider(5, 60)
	intervalSlider.Step = 5
	intervalSlider.OnChanged = func(f float64) {
		intervalBind.Set(int(f))
		a.storePreferences()
	}
	intervalSlider.Value = float64(a.config.TOTP.Interval)
	intervalEntry := widget.NewLabelWithData(binding.IntToString(intervalBind))
	form.Add(labelWithStyle("Interval"))
	form.Add(container.NewBorder(nil, nil, nil, intervalEntry, intervalSlider))

	return widget.NewCard(
		"TOTP",
		"",
		form,
	)
}

func (a *app) makePreferenceLenghtWidget(lenght *int, min, max int) fyne.CanvasObject {
	lengthBind := binding.BindInt(lenght)
	lengthEntry := widget.NewEntryWithData(binding.IntToString(lengthBind))
	lengthEntry.Disabled()
	lengthEntry.Validator = nil
	lengthEntry.OnChanged = func(value string) {
		if value == "" {
			return
		}
		l, err := strconv.Atoi(value)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		if l < min || l > max {
			log.Printf("lenght must be between %d and %d, got %d", min, max, l)
			return
		}
		lengthBind.Set(l)
		a.storePreferences()
	}

	lengthSlider := widget.NewSlider(float64(min), float64(max))
	lengthSlider.OnChanged = func(f float64) {
		lengthBind.Set(int(f))
		a.storePreferences()
	}
	lengthSlider.SetValue(float64(*lenght))
	return container.NewBorder(nil, nil, widget.NewLabel("Default lenght"), lengthEntry, lengthSlider)
}
