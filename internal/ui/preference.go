package ui

import (
	"fyne.io/fyne/v2"

	"lucor.dev/paw/internal/paw"
)

const (
	randomPasswordDefaultLength     = 16
	randomPasswordMinLength         = 8
	randomPasswordMaxLength         = 120
	randomPasswordFormat            = paw.LowercaseFormat | paw.DigitsFormat | paw.SymbolsFormat | paw.UppercaseFormat
	pinPasswordDefaultLength        = 4
	pinPasswordMinLength            = 3
	pinPasswordMaxLength            = 10
	passphrasePasswordDefaultLength = 4
	passphrasePasswordMinLength     = 3
	passphrasePasswordMaxLength     = 12

	totpHASH     = paw.DefaultTOTPHash
	totpDigits   = paw.DefaultTOTPDigits
	totpInterval = paw.DefaultTOTPInterval
)

// Random Password options

func RandomPasswordDefaultFormat() paw.Format {
	f := fyne.CurrentApp().Preferences().Int("random_password_default_format")
	if f == 0 {
		return randomPasswordFormat
	}
	return paw.Format(f)
}

func SetRandomPasswordDefaultFormat(format paw.Format) {
	fyne.CurrentApp().Preferences().SetInt("random_password_default_format", int(format))
}

func RandomPasswordDefaultLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("random_password_default_length", randomPasswordDefaultLength)
}

func SetRandomPasswordDefaultLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("random_password_default_length", randomPasswordDefaultLength)
}

func RandomPasswordMinLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("random_password_min_length", randomPasswordMinLength)
}

func SetRandomPasswordMinLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("random_password_min_length", randomPasswordMinLength)
}

func RandomPasswordMaxLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("random_password_max_length", randomPasswordMaxLength)
}

func SetRandomPasswordMaxLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("random_password_max_length", randomPasswordMaxLength)
}

// Pin Password options

func PinPasswordDefaultLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("pin_password_default_length", pinPasswordDefaultLength)
}

func SetPinPasswordDefaultLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("pin_password_default_length", pinPasswordDefaultLength)
}

func PinPasswordMinLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("pin_password_min_length", pinPasswordMinLength)
}

func SetPinPasswordMinLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("pin_password_min_length", pinPasswordMinLength)
}

func PinPasswordMaxLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("pin_password_max_length", pinPasswordMaxLength)
}

func SetPinPasswordMaxLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("pin_password_max_length", pinPasswordMaxLength)
}

// Passphrase Password options

func PassphrasePasswordDefaultLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("passphrase_password_default_length", passphrasePasswordDefaultLength)
}

func SetPassphrasePasswordDefaultLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("passphrase_password_default_length", passphrasePasswordDefaultLength)
}

func PassphrasePasswordMinLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("passphrase_password_min_length", passphrasePasswordMinLength)
}

func SetPassphrasePasswordMinLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("passphrase_password_min_length", passphrasePasswordMinLength)
}

func PassphrasePasswordMaxLength() int {
	return fyne.CurrentApp().Preferences().IntWithFallback("passphrase_password_max_length", passphrasePasswordMaxLength)
}

func SetPassphrasePasswordMaxLength(len int) {
	fyne.CurrentApp().Preferences().SetInt("passphrase_password_max_length", passphrasePasswordMaxLength)
}

func defaultPasswordOptions() paw.PasswordOptions {
	return paw.PasswordOptions{
		RandomPasswordOptions: paw.RandomPasswordOptions{
			DefaultFormat: RandomPasswordDefaultFormat(),
			DefaultMode:   paw.CustomPassword,
			DefaultLength: RandomPasswordDefaultLength(),
			MinLength:     RandomPasswordMinLength(),
			MaxLength:     RandomPasswordMaxLength(),
		},
		PinPasswordOptions: paw.PinPasswordOptions{
			DefaultLength: PinPasswordDefaultLength(),
			MinLength:     PinPasswordMinLength(),
			MaxLength:     PinPasswordMaxLength(),
		},
		PassphrasePasswordOptions: paw.PassphrasePasswordOptions{
			DefaultLength: PassphrasePasswordDefaultLength(),
			MinLength:     PassphrasePasswordMinLength(),
			MaxLength:     PassphrasePasswordMaxLength(),
		},
	}
}

// TOTP Password options

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
