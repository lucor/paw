// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fmt"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

const (
	passwordGeneratorDialogFrameWidth = 32
	randomPasswordFormats             = paw.LowercaseFormat | paw.UppercaseFormat | paw.DigitsFormat | paw.SymbolsFormat
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

func NewPasswordGenerator(key *paw.Key, ps paw.PasswordSettings) *pwgenDialog {
	return &pwgenDialog{
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
}

func (pd *pwgenDialog) ShowPasswordGenerator(bind binding.String, password *paw.Password, w fyne.Window) {
	working := *password
	if working.Mode == paw.CustomPassword {
		working.Mode = paw.RandomPassword
	}
	lengths := map[paw.PasswordMode]int{
		paw.RandomPassword:     pd.options.RandomPasswordOptions.DefaultLength,
		paw.PassphrasePassword: pd.options.PassphrasePasswordOptions.DefaultLength,
		paw.PinPassword:        pd.options.PinPasswordOptions.DefaultLength,
	}
	if min, max, ok := pd.lengthBounds(working.Mode); ok && working.Length >= min && working.Length <= max {
		lengths[working.Mode] = working.Length
	} else {
		working.Length = lengths[working.Mode]
	}
	pd.setModeDefaults(&working)

	passwordBind := binding.NewString()
	passwordEntry := widget.NewEntryWithData(passwordBind)
	passwordEntry.Validator = nil
	regenerate := func() {
		secret, err := pwgen(pd.key, &working)
		if err != nil {
			log.Println(err)
			return
		}
		if err := passwordBind.Set(secret); err != nil {
			log.Println(err)
		}
	}
	refreshButton := widget.NewButtonWithIcon("", icons.Generate, regenerate)
	resultRow := container.NewBorder(nil, nil, nil, refreshButton, passwordEntry)

	typeOptions := []string{
		paw.RandomPassword.String(),
		paw.PassphrasePassword.String(),
		paw.PinPassword.String(),
	}
	options := container.NewVBox()
	typeList := widget.NewSelect(typeOptions, func(selected string) {
		mode, ok := passwordModeFromLabel(selected)
		if !ok {
			return
		}
		if working.Mode != mode {
			if min, max, valid := pd.lengthBounds(working.Mode); valid && working.Length >= min && working.Length <= max {
				lengths[working.Mode] = working.Length
			}
			working.Mode = mode
			working.Length = lengths[mode]
			switch mode {
			case paw.RandomPassword:
				working.Format = pd.randomDefaultFormat()
			case paw.PinPassword:
				working.Format = paw.DigitsFormat
			}
		}
		options.RemoveAll()
		options.Add(pd.optionsView(&working, lengths, regenerate))
		options.Refresh()
		regenerate()
	})
	typeList.SetSelected(working.Mode.String())
	typeControl := typeList

	content := container.NewVBox(
		resultRow,
		newText("Type", captionStyle),
		typeControl,
		options,
	)
	randomOptionsPassword := working
	randomOptionsPassword.Mode = paw.RandomPassword
	randomOptionsPassword.Length = lengths[paw.RandomPassword]
	randomOptionsPassword.Format = pd.randomDefaultFormat()
	randomOptions := pd.optionsView(&randomOptionsPassword, lengths, func() {})
	maxContent := container.NewVBox(
		resultRow,
		newText("Type", captionStyle),
		typeControl,
		randomOptions,
	)
	maxContentSize := maxContent.MinSize()
	contentWidth := maxContentSize.Width
	if canvasWidth := w.Canvas().Size().Width; canvasWidth > 0 {
		availableWidth := canvasWidth - 2*theme.Size(theme.SizeNamePadding) - passwordGeneratorDialogFrameWidth
		if availableWidth > contentWidth {
			contentWidth = availableWidth
		}
	}
	content = container.NewGridWrap(fyne.NewSize(contentWidth, maxContentSize.Height), content)
	confirmDialog := dialog.NewCustomConfirm("Generate password", "Use", "Cancel", content, func(use bool) {
		if !use {
			return
		}
		value, err := passwordBind.Get()
		if err != nil {
			log.Println(err)
			return
		}
		password.Mode = working.Mode
		password.Length = working.Length
		password.Format = working.Format
		if err := bind.Set(value); err != nil {
			log.Println(err)
		}
	}, w)
	confirmDialog.Show()
}

func (pd *pwgenDialog) optionsView(password *paw.Password, lengths map[paw.PasswordMode]int, regenerate func()) fyne.CanvasObject {
	min, max, _ := pd.lengthBounds(password.Mode)
	length := newNumericStepper(&password.Length, min, max, 1, func() {
		lengths[password.Mode] = password.Length
		regenerate()
	})
	lengthRow := container.NewBorder(nil, nil, nil, length, newText("Length", captionStyle))
	if password.Mode != paw.RandomPassword {
		return lengthRow
	}
	checks := randomFormatChecks(password, regenerate)
	return container.NewVBox(lengthRow, container.NewGridWithColumns(2, checks...))
}

func randomFormatChecks(password *paw.Password, regenerate func()) []fyne.CanvasObject {
	options := []struct {
		label  string
		format paw.Format
	}{
		{"Lowercase", paw.LowercaseFormat},
		{"Uppercase", paw.UppercaseFormat},
		{"Numbers", paw.DigitsFormat},
		{"Symbols", paw.SymbolsFormat},
	}
	checks := make([]fyne.CanvasObject, 0, len(options))
	for _, option := range options {
		option := option
		check := widget.NewCheck(option.label, nil)
		check.Checked = password.Format&option.format != 0
		check.OnChanged = func(enabled bool) {
			format := password.Format
			if enabled {
				format |= option.format
			} else {
				format &^= option.format
			}
			if format&randomPasswordFormats == 0 {
				check.SetChecked(true)
				return
			}
			password.Format = format
			regenerate()
		}
		checks = append(checks, check)
	}
	return checks
}

func (pd *pwgenDialog) lengthBounds(mode paw.PasswordMode) (int, int, bool) {
	switch mode {
	case paw.RandomPassword:
		return pd.options.RandomPasswordOptions.MinLength, pd.options.RandomPasswordOptions.MaxLength, true
	case paw.PassphrasePassword:
		return pd.options.PassphrasePasswordOptions.MinLength, pd.options.PassphrasePasswordOptions.MaxLength, true
	case paw.PinPassword:
		return pd.options.PinPasswordOptions.MinLength, pd.options.PinPasswordOptions.MaxLength, true
	default:
		return 0, 0, false
	}
}

func (pd *pwgenDialog) setModeDefaults(password *paw.Password) {
	if password.Mode == paw.RandomPassword {
		if password.Format&randomPasswordFormats == 0 {
			password.Format = pd.randomDefaultFormat()
		}
		return
	}
	if password.Mode == paw.PinPassword {
		password.Format = paw.DigitsFormat
	}
}

func (pd *pwgenDialog) randomDefaultFormat() paw.Format {
	if pd.options.RandomPasswordOptions.DefaultFormat != 0 {
		return pd.options.RandomPasswordOptions.DefaultFormat
	}
	return paw.RandomPasswordDefaultFormat
}

func passwordModeFromLabel(label string) (paw.PasswordMode, bool) {
	for _, mode := range []paw.PasswordMode{paw.RandomPassword, paw.PassphrasePassword, paw.PinPassword} {
		if mode.String() == label {
			return mode, true
		}
	}
	return paw.CustomPassword, false
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
