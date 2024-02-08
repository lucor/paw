// Package provides an implementation to generate one-time password values based
// on the TOTP (Time-Based One-Time Password) and HOTP (HMAC-Based One-Time
// Password) algorithms as defined into the RFC4226 and RFC6238 specifications
package otp

import (
	"crypto/hmac"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"hash"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultInterval = 30
	DefaultDigits   = 6
	t0              = 0 // t0 is the Unix time to start counting time steps (default value is 0)
)

// TOTPFromBase32 generates a TOTP (Time-Based One-Time Password) value as
// defined into the RFC6238 from a base32 encoded secret
func TOTPFromBase32(h func() hash.Hash, decodedKey string, t time.Time, interval int, digits int) (string, error) {
	// padding the decodedKey if needed
	for len(decodedKey)%8 != 0 {
		decodedKey += "="
	}

	secret, err := base32.StdEncoding.DecodeString(strings.ToUpper(decodedKey))
	if err != nil {
		return "", err
	}
	return TOTP(h, secret, t, interval, digits)
}

// TOTP generates a TOTP (Time-Based One-Time Password) value as defined into the RFC6238
// Note: if the hash function is nil, defaults to SHA1
// Reference: https://datatracker.ietf.org/doc/html/rfc6238
func TOTP(h func() hash.Hash, key []byte, t time.Time, interval int, digits int) (string, error) {
	if interval < 1 {
		return "", errors.New("interval value must be greater than 0. RFC suggests as default 30")
	}

	count := numTimeSteps(t, interval, t0)
	return HOTP(h, key, count, digits)
}

// numTimeSteps the number of time steps between the initial counter time T0 and the current Unix time
func numTimeSteps(t time.Time, interval int, t0 int64) uint64 {
	return uint64(math.Floor(float64(t.Unix()-t0) / float64(interval)))
}

// HOTP generates an HOTP (HMAC-Based One-Time Password) value as defined into the RFC4226
// Note: if the hash function is nil, defaults to SHA1
// Reference: https://datatracker.ietf.org/doc/html/rfc4226
func HOTP(h func() hash.Hash, key []byte, count uint64, digits int) (string, error) {
	if digits < 1 {
		return "", errors.New("digits value must be greater than 0. RFC suggests it must be at least a 6-digit value")
	}

	// Generate the hash
	hash := hmacGenerator(h, key, count)

	// Generate a 4-byte string
	// offset is the lower 4 bits of the last byte
	offset := hash[len(hash)-1] & 0xf

	// the dynamic binary code is the value of the 4 bytes starting at byte "offset"
	dbc := hash[offset : offset+4]

	// masks the most significant bit of dbc to avoid confusion about signed vs.
	// unsigned modulo computations.
	dbc[0] = dbc[0] & 0x7f

	// dynamic binary code as a 31-bit, unsigned, big-endian integer
	code := binary.BigEndian.Uint32(dbc)

	// hotp is code % (10^digits)
	otp := code % uint32(math.Pow10(digits))

	// value is the string representation of otp
	value := strconv.Itoa(int(otp))
	for i := 0; i < digits-len(value); i++ {
		// otp len must be equal to digits, if shorter prepend 0
		value = "0" + value
	}
	return value, nil
}

// hmacGenerator generates the HMAC for the key and count using the specified algorithm.
// Note: the key and count are hashed high-order byte first, see rfc4226#section-5.2
func hmacGenerator(h func() hash.Hash, key []byte, count uint64) []byte {
	mac := hmac.New(h, key)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, count)
	mac.Write(buf)
	return mac.Sum(nil)
}
