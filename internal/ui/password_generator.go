// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fmt"
	"log"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lucor.dev/paw/internal/paw"
)

type pwgenOptions struct {
	DefaultMode paw.PasswordMode
	PassphrasePasswordOptions
	PinPasswordOptions
	RandomPasswordOptions
}

type PassphrasePasswordOptions struct {
	DefaultLength int
	MinLength     int
	MaxLength     int
}

type PinPasswordOptions struct {
	DefaultLength int
	MinLength     int
	MaxLength     int
}

type RandomPasswordOptions struct {
	DefaultFormat paw.Format
	DefaultMode   paw.PasswordMode
	DefaultLength int
	MinLength     int
	MaxLength     int
}

type pwgenDialog struct {
	key     *paw.Key
	options pwgenOptions
}

func NewPasswordGenerator(key *paw.Key, ps paw.PasswordPreferences) *pwgenDialog {
	pd := &pwgenDialog{
		key: key,
		options: pwgenOptions{
			RandomPasswordOptions: RandomPasswordOptions{
				DefaultFormat: ps.Random.DefaultFormat,
				DefaultMode:   paw.CustomPassword,
				DefaultLength: ps.Random.DefaultLength,
				MinLength:     ps.Random.MinLength,
				MaxLength:     ps.Random.MaxLength,
			},
			PinPasswordOptions: PinPasswordOptions{
				DefaultLength: ps.Pin.DefaultLength,
				MinLength:     ps.Pin.MinLength,
				MaxLength:     ps.Pin.MaxLength,
			},
			PassphrasePasswordOptions: PassphrasePasswordOptions{
				DefaultLength: ps.Passphrase.DefaultLength,
				MinLength:     ps.Passphrase.MinLength,
				MaxLength:     ps.Passphrase.MaxLength,
			},
		},
	}

	return pd
}

func (pd *pwgenDialog) ShowPasswordGenerator(bind binding.String, password *paw.Password, w fyne.Window) {

	passwordBind := binding.NewString()
	passwordEntry := widget.NewEntryWithData(passwordBind)
	passwordEntry.Validator = nil
	refreshButton := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		secret, err := pwgen(pd.key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		passwordBind.Set(secret)
	})

	content := container.NewStack(widget.NewLabel(""))
	typeOptions := []string{
		paw.RandomPassword.String(),
		paw.PassphrasePassword.String(),
		paw.PinPassword.String(),
	}
	typeList := widget.NewSelect(typeOptions, func(s string) {
		switch s {
		case paw.PassphrasePassword.String():
			content.Objects[0] = passphraseOptions(pd.key, passwordBind, password, pd.options.PassphrasePasswordOptions)
		case paw.PinPassword.String():
			content.Objects[0] = pinOptions(pd.key, passwordBind, password, pd.options.PinPasswordOptions)
		default:
			content.Objects[0] = randomPasswordOptions(pd.key, passwordBind, password, pd.options.RandomPasswordOptions)
		}
		content.Refresh()
	})
	switch password.Mode.String() {
	case paw.CustomPassword.String():
		password.Mode = paw.RandomPassword
		typeList.SetSelected(paw.RandomPassword.String())
	default:
		typeList.SetSelected(password.Mode.String())
	}

	form := container.New(layout.NewFormLayout())
	form.Add(labelWithStyle("Password"))
	form.Add(container.NewBorder(nil, nil, nil, refreshButton, passwordEntry))
	form.Add(labelWithStyle("Type"))
	form.Add(typeList)
	c := container.NewBorder(form, nil, nil, nil, content)

	d := dialog.NewCustomConfirm("Generate password", "Use", "Cancel", c, func(b bool) {
		if b {
			value, _ := passwordBind.Get()
			bind.Set(value)
		}
	}, w)
	d.Resize(fyne.NewSize(400, 300))
	d.Show()
}

func passphraseOptions(key *paw.Key, passwordBind binding.String, password *paw.Password, opts PassphrasePasswordOptions) fyne.CanvasObject {

	if password.Length == 0 || password.Length < opts.MinLength || password.Length > opts.MaxLength {
		password.Length = opts.DefaultLength
	}

	if password.Mode != paw.PassphrasePassword {
		password.Mode = paw.PassphrasePassword
	}

	lengthBind := binding.BindInt(&password.Length)
	lengthEntry := widget.NewEntryWithData(binding.IntToString(lengthBind))
	lengthEntry.Disabled()
	lengthEntry.Validator = nil
	lengthEntry.OnChanged = func(s string) {
		if s == "" {
			return
		}
		l, err := strconv.Atoi(s)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		if l < opts.MinLength || l > opts.MaxLength {
			log.Printf("password lenght must be between %d and %d, got %d", opts.MinLength, opts.MaxLength, l)
			return
		}
		lengthBind.Set(l)
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		passwordBind.Set(secret)
	}

	lengthSlider := widget.NewSlider(float64(opts.MinLength), float64(opts.MaxLength))
	lengthSlider.OnChanged = func(f float64) {
		lengthBind.Set(int(f))
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		passwordBind.Set(secret)
	}
	lengthSlider.SetValue(float64(password.Length))

	secret, err := pwgen(key, password)
	if err != nil {
		// TODO show dialog
		log.Println(err)
	}
	passwordBind.Set(secret)

	form := container.New(layout.NewFormLayout())
	form.Add(labelWithStyle("Length"))
	form.Add(container.NewBorder(nil, nil, nil, lengthEntry, lengthSlider))

	return form
}

func pinOptions(key *paw.Key, passwordBind binding.String, password *paw.Password, opts PinPasswordOptions) fyne.CanvasObject {

	if password.Length == 0 || password.Length < opts.MinLength || password.Length > opts.MaxLength {
		password.Length = opts.DefaultLength
	}

	// with PIN we want only digits
	password.Format = paw.DigitsFormat
	if password.Mode != paw.PinPassword {
		password.Mode = paw.PinPassword
	}

	lengthBind := binding.BindInt(&password.Length)
	if password.Length == 0 || password.Mode != paw.PinPassword {
		lengthBind.Set(opts.DefaultLength)
	}

	lengthEntry := widget.NewEntryWithData(binding.IntToString(lengthBind))
	lengthEntry.Disabled()
	lengthEntry.Validator = nil
	lengthEntry.OnChanged = func(s string) {
		if s == "" {
			return
		}
		l, err := strconv.Atoi(s)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		if l < opts.MinLength || l > opts.MaxLength {
			log.Printf("password lenght must be between %d and %d, got %d", opts.MinLength, opts.MaxLength, l)
			return
		}
		lengthBind.Set(l)
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		passwordBind.Set(secret)
	}

	lengthSlider := widget.NewSlider(float64(opts.MinLength), float64(opts.MaxLength))
	lengthSlider.OnChanged = func(f float64) {
		lengthBind.Set(int(f))
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		passwordBind.Set(secret)
	}
	lengthSlider.SetValue(float64(password.Length))

	secret, err := pwgen(key, password)
	if err != nil {
		// TODO show dialog
		log.Println(err)
	}
	passwordBind.Set(secret)

	form := container.New(layout.NewFormLayout())
	form.Add(labelWithStyle("Length"))
	form.Add(container.NewBorder(nil, nil, nil, lengthEntry, lengthSlider))

	return form
}

func randomPasswordOptions(key *paw.Key, passwordBind binding.String, password *paw.Password, opts RandomPasswordOptions) fyne.CanvasObject {

	if password.Length == 0 || password.Length < opts.MinLength || password.Length > opts.MaxLength {
		password.Length = opts.DefaultLength
	}

	if password.Format == 0 {
		password.Format = opts.DefaultFormat
	}

	if password.Mode != paw.RandomPassword {
		password.Mode = paw.RandomPassword
		password.Format = opts.DefaultFormat
	}

	lengthBind := binding.BindInt(&password.Length)
	lengthEntry := widget.NewEntryWithData(binding.IntToString(lengthBind))
	lengthEntry.Disabled()
	lengthEntry.Validator = nil
	lengthEntry.OnChanged = func(s string) {
		if s == "" {
			return
		}
		l, err := strconv.Atoi(s)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		if l < opts.MinLength || l > opts.MaxLength {
			log.Printf("password lenght must be between %d and %d, got %d", opts.MinLength, opts.MaxLength, l)
			return
		}
		lengthBind.Set(l)
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		passwordBind.Set(secret)
	}

	lengthSlider := widget.NewSlider(float64(opts.MinLength), float64(opts.MaxLength))
	lengthSlider.OnChanged = func(f float64) {
		lengthBind.Set(int(f))
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		passwordBind.Set(secret)
	}
	lengthSlider.SetValue(float64(password.Length))

	lowercaseButton := widget.NewCheck("a-z", func(isChecked bool) {
		if isChecked {
			password.Format |= paw.LowercaseFormat
		} else {
			password.Format &^= paw.LowercaseFormat
		}
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		passwordBind.Set(secret)
	})
	if (password.Format & paw.LowercaseFormat) != 0 {
		lowercaseButton.SetChecked(true)
	} else {
		lowercaseButton.SetChecked(false)
	}

	uppercaseButton := widget.NewCheck("A-Z", func(isChecked bool) {
		if isChecked {
			password.Format |= paw.UppercaseFormat
		} else {
			password.Format &^= paw.UppercaseFormat
		}
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		passwordBind.Set(secret)
	})
	if (password.Format & paw.UppercaseFormat) != 0 {
		uppercaseButton.SetChecked(true)
	} else {
		uppercaseButton.SetChecked(false)
	}

	digitsButton := widget.NewCheck("0-9", func(isChecked bool) {
		if isChecked {
			password.Format |= paw.DigitsFormat
		} else {
			password.Format &^= paw.DigitsFormat
		}
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		passwordBind.Set(secret)
	})
	if (password.Format & paw.DigitsFormat) != 0 {
		digitsButton.SetChecked(true)
	} else {
		digitsButton.SetChecked(false)
	}

	symbolsButton := widget.NewCheck("!%$", func(isChecked bool) {
		if isChecked {
			password.Format |= paw.SymbolsFormat
		} else {
			password.Format &^= paw.SymbolsFormat
		}
		secret, err := pwgen(key, password)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		passwordBind.Set(secret)
	})
	if (password.Format & paw.SymbolsFormat) != 0 {
		symbolsButton.SetChecked(true)
	} else {
		symbolsButton.SetChecked(false)
	}

	secret, err := pwgen(key, password)
	if err != nil {
		// TODO show dialog
		log.Println(err)
	}
	passwordBind.Set(secret)

	form := container.New(layout.NewFormLayout())
	form.Add(labelWithStyle("Length"))
	form.Add(container.NewBorder(nil, nil, nil, lengthEntry, lengthSlider))
	form.Add(widget.NewLabel(""))
	form.Add(container.NewGridWithColumns(4, lowercaseButton, uppercaseButton, digitsButton, symbolsButton))

	return form
}

func pwgen(key *paw.Key, password *paw.Password) (string, error) {
	if password.Mode == paw.PassphrasePassword {
		return key.Passphrase(password.Length)
	}
	secret, err := key.Secret(password)
	if err != nil {
		return "", fmt.Errorf("could not generate password: %w", err)
	}
	return secret, nil
}
