// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package paw provides shared helpers for keys, URLs, passwords, and TOTP.
package paw

import (
	"bytes"
	json "encoding/json/v2"
	"fmt"
	"io"
	"runtime/debug"
	"time"

	"filippo.io/age"
	"filippo.io/age/armor"
)

const (
	ID            = "dev.lucor.paw"
	ServicePrefix = "paw/"
	// UserAgentFaviconDownloader is the UserAgent user by the favicon downloader
	UserAgentFaviconDownloader = "Paw/1.0 (+https://paw.pm/paw-favicon-downloader)"
)

var (
	// BuildVersion allow to set the version at link time
	BuildVersion string
)

type Key struct {
	ageIdentity *age.X25519Identity
}

// MakeOneTimeKey generates a one time age secret key.
// The key can be used to encrypt and decrypt vault data.
func MakeOneTimeKey() (key *Key, err error) {
	wrapErr := func(err error) error {
		return fmt.Errorf("paw: makekey error: %w", err)
	}

	// Generate the age X25519 Identity
	ageIdentity, ierr := age.GenerateX25519Identity()
	if ierr != nil {
		err = wrapErr(ierr)
		return
	}
	key = &Key{
		ageIdentity: ageIdentity,
	}
	return
}

// MakeKey generates an age secret key. The key is encrypted to w and protect using the provided password
func MakeKey(password string, w io.Writer) (key *Key, err error) {

	wrapErr := func(err error) error {
		return fmt.Errorf("paw: makekey error: %w", err)
	}

	// Generate the age X25519 Identity
	ageIdentity, ierr := age.GenerateX25519Identity()
	if ierr != nil {
		err = wrapErr(ierr)
		return
	}

	ageScryptRecipient, ierr := age.NewScryptRecipient(password)
	if ierr != nil {
		err = wrapErr(ierr)
		return
	}

	a := armor.NewWriter(w)
	defer func() {
		// make sure to handle the error, if any
		if ierr := a.Close(); ierr != nil {
			err = wrapErr(ierr)
			return
		}
	}()
	e, err := age.Encrypt(a, ageScryptRecipient)
	if err != nil {
		err = wrapErr(ierr)
		return
	}

	data := &bytes.Buffer{}
	fmt.Fprintf(data, "# created: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(data, "# public key: %s\n", ageIdentity.Recipient())
	fmt.Fprintf(data, "%s\n", ageIdentity)

	_, err = e.Write(data.Bytes())
	if err != nil {
		err = wrapErr(ierr)
		return
	}
	err = e.Close()
	if err != nil {
		err = wrapErr(ierr)
		return
	}

	key = &Key{
		ageIdentity: ageIdentity,
	}
	return
}

// LoadKey decrypts an age secret key from the reader r using the provided password
func LoadKey(password string, r io.Reader) (key *Key, err error) {

	wrapErr := func(err error) error {
		return fmt.Errorf("paw: loadkey error: %w", err)
	}

	ageScryptIdentity, ierr := age.NewScryptIdentity(password)
	if ierr != nil {
		err = wrapErr(ierr)
		return
	}

	a := armor.NewReader(r)
	d, ierr := age.Decrypt(a, ageScryptIdentity)
	if ierr != nil {
		err = wrapErr(ierr)
		return
	}

	// Generate the age X25519 Identity
	ageIdentities, ierr := age.ParseIdentities(d)
	if ierr != nil {
		err = wrapErr(ierr)
		return
	}

	if len(ageIdentities) > 1 {
		err = wrapErr(fmt.Errorf("only one identity per file is supported, found %d", len(ageIdentities)))
		return
	}

	ageIdentity, ok := ageIdentities[0].(*age.X25519Identity)
	if !ok {
		err = wrapErr(fmt.Errorf("only *age.X25519Identity are supported, got %T", ageIdentities[0]))
		return
	}

	key = &Key{
		ageIdentity: ageIdentity,
	}
	return
}

// Decrypt decrypts the message
func (k *Key) Decrypt(src io.Reader) (io.Reader, error) {
	return age.Decrypt(src, k.ageIdentity)
}

// Encrypt a message
func (k *Key) Encrypt(dst io.Writer) (io.WriteCloser, error) {
	return age.Encrypt(dst, k.ageIdentity.Recipient())
}

func (k *Key) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.ageIdentity.String(), json.Deterministic(true))
}

func (k *Key) UnmarshalJSON(data []byte) error {
	var v string
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}
	ageIdentity, err := age.ParseX25519Identity(v)
	if err != nil {
		return err
	}
	k.ageIdentity = ageIdentity
	return nil
}

// WriteWrapped protects the existing identity with a password without generating a new key.
func (k *Key) WriteWrapped(w io.Writer, password string) error {
	if password == "" {
		return fmt.Errorf("master password is required")
	}
	recipient, err := age.NewScryptRecipient(password)
	if err != nil {
		return err
	}
	armored := armor.NewWriter(w)
	encrypted, err := age.Encrypt(armored, recipient)
	if err != nil {
		return err
	}
	if _, err = io.WriteString(encrypted, k.ageIdentity.String()+"\n"); err != nil {
		return err
	}
	if err = encrypted.Close(); err != nil {
		return err
	}
	return armored.Close()
}

func (k *Key) String() string {
	return k.ageIdentity.String()
}

// Version returns the Paw's version
func Version() string {
	if BuildVersion != "" {
		return BuildVersion
	}

	info, ok := debug.ReadBuildInfo()
	if ok {
		return info.Main.Version
	}
	return "(unknown)"
}

// ServiceVersion returns the Paw's service version
func ServiceVersion() string {
	return ServicePrefix + Version()
}
