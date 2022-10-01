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

func storeConfig(s paw.Storage, config *paw.Config, w fyne.Window) {
	err := s.StoreConfig(config)
	if err != nil {
		dialog.ShowError(err, w)
	}
}

func makePasswordSettings(s paw.Storage, config *paw.Config, w fyne.Window) fyne.CanvasObject {
	passphraseCard := widget.NewCard(
		"Passphrase settings",
		"",
		configureLenght(&config.Password.Passphrase.DefaultLength, config.Password.Passphrase.MinLength, config.Password.Passphrase.MaxLength, s, config, w),
	)
	pinCard := widget.NewCard(
		"Pin settings",
		"",
		configureLenght(&config.Password.Pin.DefaultLength, config.Password.Pin.MinLength, config.Password.Pin.MaxLength, s, config, w),
	)
	randomCard := widget.NewCard(
		"Random settings",
		"",
		configureLenght(&config.Password.Random.DefaultLength, config.Password.Random.MinLength, config.Password.Random.MaxLength, s, config, w),
	)
	return container.NewVBox(passphraseCard, pinCard, randomCard)
}

func makeTOTPSettings(s paw.Storage, config *paw.Config, w fyne.Window) fyne.CanvasObject {
	form := container.New(layout.NewFormLayout())

	hashOptions := []string{string(paw.SHA1), string(paw.SHA256), string(paw.SHA512)}
	hashSelect := widget.NewSelect(hashOptions, func(selected string) {
		config.TOTP.Hash = paw.TOTPHash(selected)
		storeConfig(s, config, w)
	})
	hashSelect.Selected = string(config.TOTP.Hash)
	form.Add(labelWithStyle("Hash Algorithm"))
	form.Add(hashSelect)

	digitsOptions := []string{"5", "6", "7", "8", "9", "10"}
	digitsSelect := widget.NewSelect(digitsOptions, func(selected string) {
		config.TOTP.Digits, _ = strconv.Atoi(selected)
		storeConfig(s, config, w)
	})
	digitsSelect.Selected = strconv.Itoa(config.TOTP.Digits)
	form.Add(labelWithStyle("Digits"))
	form.Add(digitsSelect)

	intervalBind := binding.BindInt(&config.TOTP.Interval)
	intervalSlider := widget.NewSlider(5, 60)
	intervalSlider.Step = 5
	intervalSlider.OnChanged = func(f float64) {
		intervalBind.Set(int(f))
		storeConfig(s, config, w)
	}
	intervalSlider.Value = float64(config.TOTP.Interval)
	intervalEntry := widget.NewLabelWithData(binding.IntToString(intervalBind))
	form.Add(labelWithStyle("Interval"))
	form.Add(container.NewBorder(nil, nil, nil, intervalEntry, intervalSlider))

	return container.NewBorder(widget.NewLabel("TOTP settings"), nil, nil, nil, form)
}

func configureLenght(lenght *int, min, max int, s paw.Storage, config *paw.Config, w fyne.Window) fyne.CanvasObject {
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
		storeConfig(s, config, w)
	}

	lengthSlider := widget.NewSlider(float64(min), float64(max))
	lengthSlider.OnChanged = func(f float64) {
		lengthBind.Set(int(f))
		storeConfig(s, config, w)
	}
	lengthSlider.SetValue(float64(*lenght))
	return container.NewBorder(nil, nil, widget.NewLabel("Default lenght"), lengthEntry, lengthSlider)
}
