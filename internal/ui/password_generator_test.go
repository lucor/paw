// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func TestPasswordGeneratorUsesGeneratedPassword(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	window.Resize(fyne.NewSize(440, 800))
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	value := binding.NewString()
	settings := paw.NewCustomPassword()
	NewPasswordGenerator(key, model.NewDefaultSettings().Password).ShowPasswordGenerator(value, settings, window)
	popup := window.Canvas().Overlays().Top()
	require.NotNil(t, popup)
	result := passwordGeneratorEntry(t, popup)
	var use *widget.Button
	generated := result.Text
	for _, object := range test.LaidOutObjects(popup) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Use" {
			use = button
		}
	}
	require.NotEmpty(t, generated)
	require.NotNil(t, use)
	const edited = "user-edited-password"
	result.SetText(edited)
	test.Tap(use)
	selected, err := value.Get()
	require.NoError(t, err)
	require.NotEqual(t, generated, selected)
	require.Equal(t, edited, selected)
	require.Nil(t, window.Canvas().Overlays().Top())
}

func TestPasswordGeneratorModesUseTheirDefaults(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	window.Resize(fyne.NewSize(440, 800))
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	settings := model.NewDefaultSettings().Password
	password := &paw.PasswordSpec{Mode: paw.CustomPassword}
	value := binding.NewString()
	require.NoError(t, value.Set("existing value"))
	NewPasswordGenerator(key, settings).ShowPasswordGenerator(value, password, window)
	popup := window.Canvas().Overlays().Top()
	require.NotNil(t, popup)
	initialSize := popup.Size()
	require.Greater(t, popup.MinSize().Width, float32(300))
	texts := auditTexts(popup)
	require.NotContains(t, texts, "Password")
	require.NotContains(t, texts, "Generated password")
	require.Contains(t, texts, "Type")
	require.Contains(t, texts, "Length")

	selectPasswordGeneratorMode(t, popup, paw.PassphrasePassword.String())
	require.Equal(t, initialSize, popup.Size())
	require.Equal(t, paw.CustomPassword, password.Mode, "mode selection remains local until Use")
	require.Equal(t, settings.Passphrase.DefaultLength, passwordGeneratorLength(t, popup))
	require.Empty(t, passwordGeneratorChecks(popup), "passphrase only shows its length control")
	test.Tap(passwordGeneratorButton(t, popup, icons.Plus))
	require.Equal(t, settings.Passphrase.DefaultLength+1, passwordGeneratorLength(t, popup))
	require.Len(t, strings.Split(passwordGeneratorEntry(t, popup).Text, "-"), settings.Passphrase.DefaultLength+1)

	selectPasswordGeneratorMode(t, popup, paw.PinPassword.String())
	require.Equal(t, initialSize, popup.Size())
	require.Equal(t, settings.Pin.DefaultLength, passwordGeneratorLength(t, popup))
	require.Empty(t, passwordGeneratorChecks(popup), "PIN only shows its length control")
	pin := passwordGeneratorEntry(t, popup).Text
	require.Len(t, pin, settings.Pin.DefaultLength)
	for _, digit := range pin {
		require.Contains(t, "0123456789", string(digit))
	}
	selectPasswordGeneratorMode(t, popup, paw.PassphrasePassword.String())
	require.Equal(t, settings.Passphrase.DefaultLength+1, passwordGeneratorLength(t, popup), "each type retains its local length")

	selectPasswordGeneratorMode(t, popup, paw.RandomPassword.String())
	require.Equal(t, initialSize, popup.Size())
	require.Equal(t, settings.Random.DefaultLength, passwordGeneratorLength(t, popup))
	require.Equal(t, []string{"Lowercase", "Uppercase", "Numbers", "Symbols"}, passwordGeneratorCheckLabels(popup))
	require.Len(t, passwordGeneratorEntry(t, popup).Text, settings.Random.DefaultLength)
	selected, err := value.Get()
	require.NoError(t, err)
	require.Equal(t, "existing value", selected, "generation does not update the editor before Use")
}

func TestPasswordGeneratorDialogKeepsSizeWhenOpenedInEachMode(t *testing.T) {
	for _, mode := range []paw.PasswordMode{paw.RandomPassword, paw.PassphrasePassword, paw.PinPassword} {
		t.Run(mode.String(), func(t *testing.T) {
			test.NewTempApp(t)
			window := test.NewTempWindow(t, nil)
			window.Resize(fyne.NewSize(440, 800))
			key, err := paw.MakeOneTimeKey()
			require.NoError(t, err)
			settings := model.NewDefaultSettings().Password
			password := &paw.PasswordSpec{Mode: mode}
			switch mode {
			case paw.RandomPassword:
				password.Length = settings.Random.DefaultLength
				password.Format = settings.Random.DefaultFormat
			case paw.PassphrasePassword:
				password.Length = settings.Passphrase.DefaultLength
			case paw.PinPassword:
				password.Length = settings.Pin.DefaultLength
				password.Format = paw.DigitsFormat
			}
			NewPasswordGenerator(key, settings).ShowPasswordGenerator(binding.NewString(), password, window)
			popup := window.Canvas().Overlays().Top()
			size := popup.Size()
			minSize := popup.MinSize()
			for _, next := range []paw.PasswordMode{paw.RandomPassword, paw.PassphrasePassword, paw.PinPassword} {
				selectPasswordGeneratorMode(t, popup, next.String())
				require.Equal(t, size, popup.Size())
				require.Equal(t, minSize, popup.MinSize())
			}
		})
	}
}

func TestPasswordGeneratorDialogFitsNarrowWindow(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	window.Resize(fyne.NewSize(320, 540))
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	settings := model.NewDefaultSettings().Password
	password := &paw.PasswordSpec{Mode: paw.CustomPassword}
	NewPasswordGenerator(key, settings).ShowPasswordGenerator(binding.NewString(), password, window)
	popup := window.Canvas().Overlays().Top()
	test.LaidOutObjects(popup)
	require.LessOrEqual(t, popup.MinSize().Width, window.Canvas().Size().Width)
	for _, mode := range []paw.PasswordMode{paw.PassphrasePassword, paw.PinPassword, paw.RandomPassword} {
		selectPasswordGeneratorMode(t, popup, mode.String())
		require.NotEmpty(t, passwordGeneratorEntry(t, popup).Text)
		require.NotNil(t, passwordGeneratorAction(t, popup, "Use"))
		require.NotNil(t, passwordGeneratorAction(t, popup, "Cancel"))
	}
}

func TestPasswordGeneratorStepperHonorsEachTypeBounds(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	settings := model.NewDefaultSettings().Password
	settings.Random.MinLength, settings.Random.DefaultLength, settings.Random.MaxLength = 15, 15, 16
	settings.Passphrase.MinLength, settings.Passphrase.DefaultLength, settings.Passphrase.MaxLength = 3, 3, 4
	settings.Pin.MinLength, settings.Pin.DefaultLength, settings.Pin.MaxLength = 5, 5, 6
	password := &paw.PasswordSpec{Mode: paw.RandomPassword, Length: settings.Random.DefaultLength, Format: settings.Random.DefaultFormat}
	generator := NewPasswordGenerator(key, settings)
	generator.ShowPasswordGenerator(binding.NewString(), password, window)
	popup := window.Canvas().Overlays().Top()
	assertGeneratorStepperBounds(t, popup, 15, 16)

	selectPasswordGeneratorMode(t, popup, paw.PassphrasePassword.String())
	assertGeneratorStepperBounds(t, popup, 3, 4)
	selectPasswordGeneratorMode(t, popup, paw.PinPassword.String())
	assertGeneratorStepperBounds(t, popup, 5, 6)
}

func assertGeneratorStepperBounds(t *testing.T, popup fyne.CanvasObject, min, max int) {
	t.Helper()
	minus := passwordGeneratorButton(t, popup, icons.Minus)
	plus := passwordGeneratorButton(t, popup, icons.Plus)
	require.True(t, minus.Disabled())
	require.False(t, plus.Disabled())
	test.Tap(plus)
	require.Equal(t, min+1, passwordGeneratorLength(t, popup))
	test.Tap(plus)
	require.Equal(t, max, passwordGeneratorLength(t, popup))
	require.True(t, plus.Disabled())
	require.False(t, minus.Disabled())
}

func TestPasswordGeneratorCancelDoesNotChangeEditorAndUseAppliesResult(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	settings := model.NewDefaultSettings().Password
	password := &paw.PasswordSpec{Mode: paw.RandomPassword, Length: settings.Random.DefaultLength, Format: settings.Random.DefaultFormat}
	original := *password
	value := binding.NewString()
	require.NoError(t, value.Set("existing value"))
	generator := NewPasswordGenerator(key, settings)
	generator.ShowPasswordGenerator(value, password, window)
	popup := window.Canvas().Overlays().Top()
	selectPasswordGeneratorMode(t, popup, paw.PassphrasePassword.String())
	test.Tap(passwordGeneratorButton(t, popup, icons.Plus))
	require.Equal(t, original, *password)
	test.Tap(passwordGeneratorAction(t, popup, "Cancel"))
	selected, err := value.Get()
	require.NoError(t, err)
	require.Equal(t, "existing value", selected)
	require.Equal(t, original, *password, "Cancel leaves editor generator settings unchanged")
	require.Nil(t, window.Canvas().Overlays().Top())

	generator.ShowPasswordGenerator(value, password, window)
	popup = window.Canvas().Overlays().Top()
	selectPasswordGeneratorMode(t, popup, paw.PassphrasePassword.String())
	test.Tap(passwordGeneratorButton(t, popup, icons.Plus))
	generated := passwordGeneratorEntry(t, popup).Text
	require.NotEmpty(t, generated)
	selected, err = value.Get()
	require.NoError(t, err)
	require.Equal(t, "existing value", selected)
	require.Equal(t, paw.RandomPassword, password.Mode, "Use has not been selected yet")
	test.Tap(passwordGeneratorAction(t, popup, "Use"))
	selected, err = value.Get()
	require.NoError(t, err)
	require.Equal(t, generated, selected)
	require.Equal(t, paw.PassphrasePassword, password.Mode)
	require.Equal(t, settings.Passphrase.DefaultLength+1, password.Length)
	require.Nil(t, window.Canvas().Overlays().Top())
}

func TestPasswordGeneratorKeepsAtLeastOneCharacterSetSelected(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	settings := model.NewDefaultSettings().Password
	password := &paw.PasswordSpec{Mode: paw.RandomPassword, Length: settings.Random.DefaultLength, Format: settings.Random.DefaultFormat}
	NewPasswordGenerator(key, settings).ShowPasswordGenerator(binding.NewString(), password, window)
	popup := window.Canvas().Overlays().Top()
	checks := passwordGeneratorChecks(popup)
	require.Len(t, checks, 4)
	for _, label := range []string{"Lowercase", "Uppercase", "Numbers"} {
		test.Tap(checks[label])
	}
	test.Tap(checks["Symbols"])
	require.True(t, checks["Symbols"].Checked)
	require.False(t, checks["Numbers"].Checked)
	symbols, err := paw.NewRule(1, paw.SymbolsFormat)
	require.NoError(t, err)
	generated := passwordGeneratorEntry(t, popup).Text
	require.NotEmpty(t, generated)
	for _, character := range generated {
		require.Contains(t, string(symbols.Tpl), string(character))
	}
	test.Tap(passwordGeneratorButton(t, popup, icons.Generate))
	for _, character := range passwordGeneratorEntry(t, popup).Text {
		require.Contains(t, string(symbols.Tpl), string(character))
	}
}

func selectPasswordGeneratorMode(t *testing.T, popup fyne.CanvasObject, mode string) {
	t.Helper()
	for _, object := range test.LaidOutObjects(popup) {
		if selectMode, ok := object.(*widget.Select); ok && len(selectMode.Options) == 3 {
			selectMode.SetSelected(mode)
			return
		}
	}
	t.Fatalf("password generator type selector is missing")
}

func passwordGeneratorLength(t *testing.T, popup fyne.CanvasObject) int {
	t.Helper()
	for _, object := range test.LaidOutObjects(popup) {
		if label, ok := object.(*widget.Label); ok && label.TextStyle.Monospace && label.Text != "" {
			if value, err := strconv.Atoi(label.Text); err == nil {
				return value
			}
		}
	}
	t.Fatal("password generator length value is missing")
	return 0
}

func passwordGeneratorEntry(t *testing.T, popup fyne.CanvasObject) *widget.Entry {
	t.Helper()
	for _, object := range test.LaidOutObjects(popup) {
		if entry, ok := object.(*widget.Entry); ok {
			return entry
		}
	}
	t.Fatal("password generator entry is missing")
	return nil
}

func passwordGeneratorChecks(popup fyne.CanvasObject) map[string]*widget.Check {
	checks := make(map[string]*widget.Check)
	for _, object := range test.LaidOutObjects(popup) {
		if check, ok := object.(*widget.Check); ok {
			checks[check.Text] = check
		}
	}
	return checks
}

func passwordGeneratorCheckLabels(popup fyne.CanvasObject) []string {
	checks := passwordGeneratorChecks(popup)
	return []string{checks["Lowercase"].Text, checks["Uppercase"].Text, checks["Numbers"].Text, checks["Symbols"].Text}
}

func passwordGeneratorButton(t *testing.T, popup fyne.CanvasObject, icon fyne.Resource) *widget.Button {
	t.Helper()
	for _, object := range test.LaidOutObjects(popup) {
		if button, ok := object.(*widget.Button); ok && button.Icon == icon {
			return button
		}
	}
	t.Fatalf("password generator button %q is missing", icon.Name())
	return nil
}

func passwordGeneratorAction(t *testing.T, popup fyne.CanvasObject, title string) *widget.Button {
	t.Helper()
	for _, object := range test.LaidOutObjects(popup) {
		if button, ok := object.(*widget.Button); ok && button.Text == title {
			return button
		}
	}
	t.Fatalf("password generator action %q is missing", title)
	return nil
}
