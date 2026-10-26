// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package migration_test

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/json"
	"hash"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/migration"
	"go.lucor.dev/paw/internal/otp"
	"go.lucor.dev/paw/internal/paw"
)

// TestMigrateTOTPMapsAlgorithms checks each legacy hash keeps its OTP output.
func TestMigrateTOTPMapsAlgorithms(t *testing.T) {
	tests := []struct {
		name      string
		hash      paw.TOTPHash
		algorithm string
		newHash   func() hash.Hash
		secret    string
		wantValue string
	}{
		{
			name: "SHA1", hash: paw.SHA1, algorithm: cxf.OTPHashAlgorithmSHA1, newHash: sha1.New,
			secret: "12345678901234567890", wantValue: "94287082",
		},
		{
			name: "default hash", algorithm: cxf.OTPHashAlgorithmSHA1, newHash: sha1.New,
			secret: "12345678901234567890", wantValue: "94287082",
		},
		{
			name: "SHA256", hash: paw.SHA256, algorithm: cxf.OTPHashAlgorithmSHA256, newHash: sha256.New,
			secret: "12345678901234567890123456789012", wantValue: "46119246",
		},
		{
			name: "SHA512", hash: paw.SHA512, algorithm: cxf.OTPHashAlgorithmSHA512, newHash: sha512.New,
			secret:    "1234567890123456789012345678901234567890123456789012345678901234",
			wantValue: "90693936",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			legacy := &paw.TOTP{
				Hash: test.hash, Secret: base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(test.secret)),
				Interval: 30, Digits: 8,
			}
			credential, err := migration.MigrateTOTP(legacy)
			require.NoError(t, err)
			require.Equal(t, cxf.CredentialTypeTOTP, credential.Type)
			require.Equal(t, legacy.Secret, credential.Secret)
			require.Equal(t, uint16(legacy.Interval), credential.Period)
			require.Equal(t, uint16(legacy.Digits), credential.Digits)
			require.Equal(t, test.algorithm, credential.Algorithm)

			encoded, err := json.Marshal(credential)
			require.NoError(t, err)
			var decoded cxf.TOTP
			require.NoError(t, json.Unmarshal(encoded, &decoded))
			require.Equal(t, *credential, decoded)

			value, err := otp.TOTPFromBase32(test.newHash, legacy.Secret, unixTime(59), legacy.Interval, legacy.Digits)
			require.NoError(t, err)
			require.Equal(t, test.wantValue, value)
		})
	}
}

// TestMigrateTOTPWithoutSecret checks unconfigured settings do not create a credential.
func TestMigrateTOTPWithoutSecret(t *testing.T) {
	for _, legacy := range []*paw.TOTP{
		nil,
		{},
		{Hash: paw.SHA512, Interval: -1, Digits: 0},
		{Hash: paw.TOTPHash("invalid"), Interval: 1 << 20, Digits: -1},
	} {
		credential, err := migration.MigrateTOTP(legacy)
		require.NoError(t, err)
		require.Nil(t, credential)
	}
}

// TestMigrateTOTPRejectsInvalidData checks configured invalid values fail without secrets.
func TestMigrateTOTPRejectsInvalidData(t *testing.T) {
	const secret = "NOT-A-SECRET-VALUE"
	validSecret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("test secret"))
	tests := []struct {
		name string
		old  *paw.TOTP
	}{
		{name: "invalid algorithm", old: &paw.TOTP{Hash: "invalid", Secret: validSecret, Interval: 30, Digits: 6}},
		{name: "zero period", old: &paw.TOTP{Hash: paw.SHA1, Secret: validSecret, Interval: 0, Digits: 6}},
		{name: "negative period", old: &paw.TOTP{Hash: paw.SHA1, Secret: validSecret, Interval: -1, Digits: 6}},
		{name: "period overflow", old: &paw.TOTP{Hash: paw.SHA1, Secret: validSecret, Interval: 1 << 16, Digits: 6}},
		{name: "zero digits", old: &paw.TOTP{Hash: paw.SHA1, Secret: validSecret, Interval: 30, Digits: 0}},
		{name: "negative digits", old: &paw.TOTP{Hash: paw.SHA1, Secret: validSecret, Interval: 30, Digits: -1}},
		{name: "digits overflow", old: &paw.TOTP{Hash: paw.SHA1, Secret: validSecret, Interval: 30, Digits: 1 << 16}},
		{name: "invalid secret", old: &paw.TOTP{Hash: paw.SHA1, Secret: secret, Interval: 30, Digits: 6}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			credential, err := migration.MigrateTOTP(test.old)
			require.Error(t, err)
			require.Nil(t, credential)
			require.NotContains(t, err.Error(), secret)
			require.NotContains(t, err.Error(), validSecret)
		})
	}
}

// unixTime returns a UTC timestamp used by the TOTP vector.
func unixTime(seconds int64) time.Time {
	return time.Unix(seconds, 0).UTC()
}
