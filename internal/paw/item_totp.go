package paw

import (
	"lucor.dev/paw/internal/otp"
)

type TOTPHash string

const (
	SHA1   TOTPHash = "SHA1"
	SHA256 TOTPHash = "SHA256"
	SHA512 TOTPHash = "SHA512"
)

const (
	TOTPHashDefault     = SHA1
	TOTPDigitsDefault   = otp.DefaultDigits
	TOTPIntervalDefault = otp.DefaultInterval
)

type TOTP struct {
	Digits   int      `json:"digits,omitempty"`
	Hash     TOTPHash `json:"hash,omitempty"`
	Interval int      `json:"interval,omitempty"`
	Secret   string   `json:"secret,omitempty"`
}

func NewDefaultTOTP() *TOTP {
	return &TOTP{
		Digits:   TOTPDigitsDefault,
		Hash:     TOTPHashDefault,
		Interval: TOTPIntervalDefault,
	}
}
