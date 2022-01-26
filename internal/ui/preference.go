package ui

import (
	"fyne.io/fyne/v2"

	"lucor.dev/paw/internal/paw"
)

// Random Password options

func RandomPasswordDefaultFormat() paw.Format {
	f := fyne.CurrentApp().Preferences().Int("random_password_default_format")
	if f == 0 {
		return paw.RandomPasswordDefaultFormat
	}
	return paw.Format(f)
}

func SetRandomPasswordDefaultFormat(format paw.Format) {
	fyne.CurrentApp().Preferences().SetInt("random_password_default_format", int(format))
}

func RandomPasswordDefaultLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("random_password_default_length", paw.RandomPasswordDefaultLength)
}

func SetRandomPasswordDefaultLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("random_password_default_length", paw.RandomPasswordDefaultLength)
}

func RandomPasswordMinLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("random_password_min_length", paw.RandomPasswordMinLength)
}

func SetRandomPasswordMinLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("random_password_min_length", paw.RandomPasswordMinLength)
}

func RandomPasswordMaxLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("random_password_max_length", paw.RandomPasswordMaxLength)
}

func SetRandomPasswordMaxLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("random_password_max_length", paw.RandomPasswordMaxLength)
}

// Pin Password options

func PinPasswordDefaultLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("pin_password_default_length", paw.PinPasswordDefaultLength)
}

func SetPinPasswordDefaultLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("pin_password_default_length", paw.PinPasswordDefaultLength)
}

func PinPasswordMinLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("pin_password_min_length", paw.PinPasswordMinLength)
}

func SetPinPasswordMinLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("pin_password_min_length", paw.PinPasswordMinLength)
}

func PinPasswordMaxLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("pin_password_max_length", paw.PinPasswordMaxLength)
}

func SetPinPasswordMaxLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("pin_password_max_length", paw.PinPasswordMaxLength)
}

// Passphrase Password options

func PassphrasePasswordDefaultLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("passphrase_password_default_length", paw.PassphrasePasswordDefaultLength)
}

func SetPassphrasePasswordDefaultLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("passphrase_password_default_length", paw.PassphrasePasswordDefaultLength)
}

func PassphrasePasswordMinLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("passphrase_password_min_length", paw.PassphrasePasswordMinLength)
}

func SetPassphrasePasswordMinLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("passphrase_password_min_length", paw.PassphrasePasswordMinLength)
}

func PassphrasePasswordMaxLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("passphrase_password_max_length", paw.PassphrasePasswordMaxLength)
}

func SetPassphrasePasswordMaxLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("passphrase_password_max_length", paw.PassphrasePasswordMaxLength)
}

// TOTP Password options

func TOTPDigits() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("totp_digits", paw.TOTPDigitsDefault)
}

func SetTOTPDigits(digits int) {
	fyne.CurrentApp().Preferences().SetInt("totp_digits", paw.TOTPDigitsDefault)
}

func TOTPHash() string {
	return fyne.CurrentApp().Preferences().StringWithFallback("totp_string", string(paw.TOTPHashDefault))
}

func SetTOTPHash(len int) {
	fyne.CurrentApp().Preferences().SetString("totp_string", string(paw.TOTPHashDefault))
}

func TOTPInverval() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("totp_interval", paw.TOTPIntervalDefault)
}

func SetTOTPInverval(interval int) {
	fyne.CurrentApp().Preferences().SetInt("totp_interval", paw.TOTPIntervalDefault)
}
