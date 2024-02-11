// Copyright 2021 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package otp

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestTOTPRFC6238 tests the HOTP implementation with values as per rfc6238
// See https://datatracker.ietf.org/doc/html/rfc6238#appendix-B
func TestTOTPRFC6238(t *testing.T) {
	const (
		secret   = "12345678901234567890"
		interval = DefaultInterval
		digits   = 8
	)
	type args struct {
		h      func() hash.Hash
		secret string
		t      string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{

		{
			args: args{
				h:      sha1.New,
				secret: "12345678901234567890",
				t:      "1970-01-01 00:00:59",
			},
			want: "94287082",
		},
		{
			args: args{
				h:      sha256.New,
				secret: "12345678901234567890123456789012",
				t:      "1970-01-01 00:00:59",
			},
			want: "46119246",
		},
		{
			args: args{
				h:      sha512.New,
				secret: "1234567890123456789012345678901234567890123456789012345678901234",
				t:      "1970-01-01 00:00:59",
			},
			want: "90693936",
		},
		{
			args: args{
				h:      sha1.New,
				secret: "12345678901234567890",
				t:      "2005-03-18 01:58:29",
			},
			want: "07081804",
		},
		{
			args: args{
				h:      sha256.New,
				secret: "12345678901234567890123456789012",
				t:      "2005-03-18 01:58:29",
			},
			want: "68084774",
		},
		{
			args: args{
				h:      sha512.New,
				secret: "1234567890123456789012345678901234567890123456789012345678901234",
				t:      "2005-03-18 01:58:29",
			},
			want: "25091201",
		},
		{
			args: args{
				h:      sha1.New,
				secret: "12345678901234567890",
				t:      "2005-03-18 01:58:31",
			},
			want: "14050471",
		},
		{
			args: args{
				h:      sha256.New,
				secret: "12345678901234567890123456789012",
				t:      "2005-03-18 01:58:31",
			},
			want: "67062674",
		},
		{
			args: args{
				h:      sha512.New,
				secret: "1234567890123456789012345678901234567890123456789012345678901234",
				t:      "2005-03-18 01:58:31",
			},
			want: "99943326",
		},
		{
			args: args{
				h:      sha1.New,
				secret: "12345678901234567890",
				t:      "2009-02-13 23:31:30",
			},
			want: "89005924",
		},
		{
			args: args{
				h:      sha256.New,
				secret: "12345678901234567890123456789012",
				t:      "2009-02-13 23:31:30",
			},
			want: "91819424",
		},
		{
			args: args{
				h:      sha512.New,
				secret: "1234567890123456789012345678901234567890123456789012345678901234",
				t:      "2009-02-13 23:31:30",
			},
			want: "93441116",
		},
		{
			args: args{
				h:      sha1.New,
				secret: "12345678901234567890",
				t:      "2033-05-18 03:33:20",
			},
			want: "69279037",
		},
		{
			args: args{
				h:      sha256.New,
				secret: "12345678901234567890123456789012",
				t:      "2033-05-18 03:33:20",
			},
			want: "90698825",
		},
		{
			args: args{
				h:      sha512.New,
				secret: "1234567890123456789012345678901234567890123456789012345678901234",
				t:      "2033-05-18 03:33:20",
			},
			want: "38618901",
		},
		{
			args: args{
				h:      sha1.New,
				secret: "12345678901234567890",
				t:      "2603-10-11 11:33:20",
			},
			want: "65353130",
		},
		{
			args: args{
				h:      sha256.New,
				secret: "12345678901234567890123456789012",
				t:      "2603-10-11 11:33:20",
			},
			want: "77737706",
		},
		{
			args: args{
				h:      sha512.New,
				secret: "1234567890123456789012345678901234567890123456789012345678901234",
				t:      "2603-10-11 11:33:20",
			},
			want: "47863826",
		},
	}
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			v, _ := time.Parse("2006-01-02 15:04:05", tt.args.t)
			got, err := TOTP(tt.args.h, []byte(tt.args.secret), v, interval, digits)
			if (err != nil) != tt.wantErr {
				t.Errorf("TOTP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("TOTP() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNumTimeStepsRFC6238 tests the number of time step values as per rfc6238
// See https://datatracker.ietf.org/doc/html/rfc6238#appendix-B
func TestNumTimeStepsRFC6238(t *testing.T) {
	type args struct {
		t string
	}
	tests := []struct {
		args args
		want uint64
	}{

		{
			args: args{
				t: "1970-01-01 00:00:59",
			},
			want: 0x0000000000000001,
		},
		{
			args: args{
				t: "2005-03-18 01:58:29",
			},
			want: 0x00000000023523EC,
		},
		{
			args: args{
				t: "2005-03-18 01:58:31",
			},
			want: 0x00000000023523ED,
		},
		{
			args: args{
				t: "2009-02-13 23:31:30",
			},
			want: 0x000000000273EF07,
		},
		{
			args: args{
				t: "2033-05-18 03:33:20",
			},
			want: 0x0000000003F940AA,
		},
		{
			args: args{
				t: "2603-10-11 11:33:20",
			},
			want: 0x0000000027BC86AA,
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			v, _ := time.Parse("2006-01-02 15:04:05", tt.args.t)
			got := numTimeSteps(v, DefaultInterval, t0)

			if got != tt.want {
				t.Errorf("TOTP() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHOTPRFC4226 tests the HOTP implementation with values as per rfc4226
// See https://datatracker.ietf.org/doc/html/rfc4226#page-32
func TestHOTPRFC422TestHOTPRFC42266(t *testing.T) {
	const (
		secret = "12345678901234567890"
	)
	tests := []struct {
		count   uint64
		want    string
		wantErr bool
	}{
		{
			count: 0,
			want:  "755224",
		},
		{
			count: 1,
			want:  "287082",
		},
		{
			count: 2,
			want:  "359152",
		},
		{
			count: 3,
			want:  "969429",
		},
		{
			count: 4,
			want:  "338314",
		},
		{
			count: 5,
			want:  "254676",
		},
		{
			count: 6,
			want:  "287922",
		},
		{
			count: 7,
			want:  "162583",
		},
		{
			count: 8,
			want:  "399871",
		},
		{
			count: 9,
			want:  "520489",
		},
	}
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			got, err := HOTP(sha1.New, []byte(secret), tt.count, DefaultDigits)
			if (err != nil) != tt.wantErr {
				t.Errorf("Totp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("HOTP() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHmacGenerator tests the intermediate HMAC value as per rfc4226
// See https://datatracker.ietf.org/doc/html/rfc4226#page-32
func TestHmacGeneratorRFC4226(t *testing.T) {
	const (
		secret = "12345678901234567890"
	)
	tests := []struct {
		count uint64
		want  string
	}{
		{
			count: 0,
			want:  "cc93cf18508d94934c64b65d8ba7667fb7cde4b0",
		},
		{
			count: 1,
			want:  "75a48a19d4cbe100644e8ac1397eea747a2d33ab",
		},
		{
			count: 2,
			want:  "0bacb7fa082fef30782211938bc1c5e70416ff44",
		},
		{
			count: 3,
			want:  "66c28227d03a2d5529262ff016a1e6ef76557ece",
		},
		{
			count: 4,
			want:  "a904c900a64b35909874b33e61c5938a8e15ed1c",
		},
		{
			count: 5,
			want:  "a37e783d7b7233c083d4f62926c7a25f238d0316",
		},
		{
			count: 6,
			want:  "bc9cd28561042c83f219324d3c607256c03272ae",
		},
		{
			count: 7,
			want:  "a4fb960c0bc06e1eabb804e5b397cdc4b45596fa",
		},
		{
			count: 8,
			want:  "1b3c89f65e6c9e883012052823443f048b4332db",
		},
		{
			count: 9,
			want:  "1637409809a679dc698207310c8c7fc07290d9e5",
		},
	}
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			got := hmacGenerator(sha1.New, []byte(secret), tt.count)
			if fmt.Sprintf("%x", got) != tt.want {
				t.Errorf("hmacGenerator() = %x, want %v", got, tt.want)
			}
		})
	}
}

func TestTOTPFromBase32(t *testing.T) {
	key := "OBQXO"
	v, err := TOTPFromBase32(sha1.New, key, time.Now(), DefaultInterval, DefaultDigits)
	require.NoError(t, err)
	require.Len(t, v, DefaultDigits)
}

func TestTOTPFromBase32InvalidKey(t *testing.T) {
	key := "A"
	_, err := TOTPFromBase32(sha1.New, key, time.Now(), DefaultInterval, DefaultDigits)
	require.Error(t, err)
}

func TestTOTPDigitsOutput(t *testing.T) {
	key := "OBQXO"
	now, err := time.Parse(time.DateTime, "2024-02-09 23:03:59")
	require.NoError(t, err)
	v, err := TOTPFromBase32(sha1.New, key, now, DefaultInterval, DefaultDigits)
	require.NoError(t, err)
	require.Equal(t, "003475", v)
}
