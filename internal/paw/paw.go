// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package paw provides shared helpers for keys, URLs, passwords, and TOTP.
package paw

import (
	json "encoding/json/v2"
	"fmt"
	"io"
	"runtime/debug"

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
	ageIdentity age.Identity
	migrated    bool
}

// MakeOneTimeKey generates a one-time hybrid age key for encrypted temporary data.
func MakeOneTimeKey() (*Key, error) {
	identity, err := age.GenerateHybridIdentity()
	if err != nil {
		return nil, fmt.Errorf("paw: generate key: %w", err)
	}
	return &Key{ageIdentity: identity}, nil
}

// MakeKey generates a hybrid age secret key protected by the provided password.
func MakeKey(password string, w io.Writer) (key *Key, err error) {
	key, err = MakeOneTimeKey()
	if err != nil {
		return nil, err
	}
	if err := key.WriteWrapped(w, password); err != nil {
		return nil, fmt.Errorf("paw: protect key: %w", err)
	}
	return key, nil
}

// LoadKey decrypts a hybrid age identity from the reader using the provided password.
func LoadKey(password string, r io.Reader) (key *Key, err error) {
	return loadKey(password, r, true)
}

// LoadLegacyKey decrypts a legacy X25519 age identity for automatic migration.
func LoadLegacyKey(password string, r io.Reader) (*Key, error) {
	return loadKey(password, r, false)
}

func loadKey(password string, r io.Reader, hybrid bool) (key *Key, err error) {

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

	ageIdentities, ierr := age.ParseIdentities(d)
	if ierr != nil {
		err = wrapErr(ierr)
		return
	}

	if len(ageIdentities) != 1 {
		err = wrapErr(fmt.Errorf("only one identity per file is supported, found %d", len(ageIdentities)))
		return
	}
	switch identity := ageIdentities[0].(type) {
	case *age.HybridIdentity:
		if hybrid {
			return &Key{
				ageIdentity: identity,
			}, nil
		}
	case *age.X25519Identity:
		if !hybrid {
			return &Key{
				ageIdentity: identity,
			}, nil
		}
	}
	if hybrid {
		return nil, wrapErr(fmt.Errorf("hybrid age identity required"))
	}
	return nil, wrapErr(fmt.Errorf("legacy X25519 age identity required"))
}

// Decrypt decrypts the message
func (k *Key) Decrypt(src io.Reader) (io.Reader, error) {
	return age.Decrypt(src, k.ageIdentity)
}

// Encrypt a message
func (k *Key) Encrypt(dst io.Writer) (io.WriteCloser, error) {
	switch identity := k.ageIdentity.(type) {
	case *age.HybridIdentity:
		return age.Encrypt(dst, identity.Recipient())
	case *age.X25519Identity:
		return age.Encrypt(dst, identity.Recipient())
	default:
		return nil, fmt.Errorf("unsupported age identity type")
	}
}

func (k *Key) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String(), json.Deterministic(true))
}

func (k *Key) UnmarshalJSON(data []byte) error {
	var v string
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}
	if identity, err := age.ParseHybridIdentity(v); err == nil {
		*k = Key{
			ageIdentity: identity,
		}
		return nil
	}
	identity, err := age.ParseX25519Identity(v)
	if err != nil {
		return fmt.Errorf("invalid native age identity")
	}
	*k = Key{ageIdentity: identity}
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
	if _, err = io.WriteString(encrypted, k.String()+"\n"); err != nil {
		return err
	}
	if err = encrypted.Close(); err != nil {
		return err
	}
	return armored.Close()
}

func (k *Key) String() string {
	switch identity := k.ageIdentity.(type) {
	case *age.HybridIdentity:
		return identity.String()
	case *age.X25519Identity:
		return identity.String()
	default:
		return "unsupported age identity"
	}
}

// Migrated reports whether loading this key automatically migrated its vault.
func (k *Key) Migrated() bool { return k.migrated }

// MarkMigrated records that loading this key completed a vault identity migration.
func (k *Key) MarkMigrated() { k.migrated = true }

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
