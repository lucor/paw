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

	totpHASH     = paw.DefaultTOTPHash
	totpDigits   = paw.DefaultTOTPDigits
	totpInterval = paw.DefaultTOTPInterval
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

func TOTPDigits() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("totp_digits", totpDigits)
}

func SetTOTPDigits(digits int) {
	fyne.CurrentApp().Preferences().SetInt("totp_digits", totpDigits)
}

func TOTPHash() string {
	return fyne.CurrentApp().Preferences().StringWithFallback("totp_string", string(totpHASH))
}

func SetTOTPHash(len int) {
	fyne.CurrentApp().Preferences().SetString("totp_string", string(totpHASH))
}

func TOTPInverval() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("totp_interval", totpInterval)
}

func SetTOTPInverval(interval int) {
	fyne.CurrentApp().Preferences().SetInt("totp_interval", totpInterval)
}
