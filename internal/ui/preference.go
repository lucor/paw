package ui

import (
	"fyne.io/fyne/v2"

	"lucor.dev/paw/internal/paw"
)

const (
	passwordLength    = 16
	passwordMinLength = 8
	passwordMaxLength = 120
	passwordFormat    = paw.LowercaseFormat | paw.DigitsFormat | paw.SymbolsFormat | paw.UppercaseFormat
)

func PasswordLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("password_length", passwordLength)
}

func SetPasswordLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("password_length", passwordLength)
}

func PasswordFormat() paw.Format {
	f := fyne.CurrentApp().Preferences().Int("password_format")
	if f == 0 {
		return passwordFormat
	}
	return paw.Format(f)
}

func SetPasswordFormat(format paw.Format) {
	fyne.CurrentApp().Preferences().SetInt("password_format", int(format))
}

func PasswordMinLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("password_min_length", passwordMinLength)
}

func SetPasswordMinLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("password_min_length", passwordMinLength)
}

func PasswordMaxLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("password_max_length", passwordMaxLength)
}

func SetPasswordMaxLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("password_max_length", passwordMaxLength)
}

func defaultPasswordOptions() paw.PasswordOptions {
	return paw.PasswordOptions{
		DefaultFormat: PasswordFormat(),
		DefaultMode:   paw.CustomPassword,
		DefaultLength: PasswordLength(),
		MinLength:     PasswordMinLength(),
		MaxLength:     PasswordMaxLength(),
	}
}
