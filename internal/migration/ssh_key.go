// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package migration

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"golang.org/x/crypto/ssh"

	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/sshkey"
)

// MigrateSSHKey converts a legacy SSH key to a CXF-backed Paw item.
func MigrateSSHKey(old *paw.SSHKey) (*model.Item, error) {
	if old == nil || old.Metadata == nil || old.Type != paw.SSHKeyItemType {
		return nil, fmt.Errorf("cannot migrate SSH key: invalid metadata")
	}
	if !utf8.ValidString(old.Name) || !utf8.ValidString(old.Metadata.Subtitle) || !utf8.ValidString(old.Comment) {
		return nil, fmt.Errorf("cannot migrate SSH key: invalid UTF-8")
	}
	created, err := timestamp(old.Created)
	if err != nil {
		return nil, fmt.Errorf("cannot migrate SSH key creation date: %w", err)
	}
	modified, err := timestamp(old.Modified)
	if err != nil {
		return nil, fmt.Errorf("cannot migrate SSH key modification date: %w", err)
	}
	parsed, err := sshkey.ParseKey([]byte(old.PrivateKey))
	if err != nil {
		var missing *ssh.PassphraseMissingError
		if !errors.As(err, &missing) {
			return nil, fmt.Errorf("cannot migrate SSH key: invalid or unsupported private key")
		}
		if old.Passphrase == nil {
			return nil, fmt.Errorf("cannot migrate SSH key: private key requires a passphrase")
		}
		parsed, err = sshkey.ParseKeyWithPassphrase([]byte(old.PrivateKey), []byte(old.Passphrase.Value))
		if err != nil {
			return nil, fmt.Errorf("cannot migrate SSH key: invalid passphrase or private key")
		}
	}
	privateKey := parsed.PrivateKey()
	if key, ok := privateKey.(*ed25519.PrivateKey); ok {
		privateKey = *key
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("cannot migrate SSH key: unsupported private key")
	}
	publicKey := parsed.PublicKey()
	if old.PublicKey != "" {
		stored, _, _, _, err := ssh.ParseAuthorizedKey([]byte(old.PublicKey))
		if err != nil || !equalPublicKey(stored, publicKey) {
			return nil, fmt.Errorf("cannot migrate SSH key: stored public key does not match private key")
		}
	}
	fingerprint := ssh.FingerprintSHA256(publicKey)
	if old.Fingerprint != "" && old.Fingerprint != fingerprint {
		return nil, fmt.Errorf("cannot migrate SSH key: stored fingerprint does not match private key")
	}
	settings := model.PasswordSettings{Mode: paw.CustomPassword}
	passphrase := ""
	if old.Passphrase != nil {
		passphrase = old.Passphrase.Value
		settings = model.PasswordSettings{
			Mode:   old.Passphrase.Mode,
			Length: old.Passphrase.Length,
			Format: old.Passphrase.Format,
		}
	}
	credential := cxf.SSHKey{
		Type:       cxf.CredentialTypeSSHKey,
		KeyType:    publicKey.Type(),
		PrivateKey: base64.RawURLEncoding.EncodeToString(privateDER),
		KeyComment: old.Comment,
	}
	item, err := model.NewSSHKey(old.Name, credential, passphrase, settings)
	if err != nil {
		return nil, fmt.Errorf("cannot migrate SSH key: %w", err)
	}
	item.CreationAt = created
	item.ModifiedAt = modified
	item.Metadata.AddToAgent = old.AddToAgent
	if old.Metadata.Subtitle != "" {
		subtitle := old.Metadata.Subtitle
		item.Subtitle = &subtitle
	}
	if old.Note != nil && old.Note.Value != "" {
		if !utf8.ValidString(old.Note.Value) {
			return nil, fmt.Errorf("cannot migrate SSH key note: invalid UTF-8")
		}
		note, err := json.Marshal(cxf.Note{
			Type: cxf.CredentialTypeNote,
			Content: cxf.EditableField{
				FieldType: cxf.FieldTypeString,
				Value:     old.Note.Value,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("cannot encode SSH key note")
		}
		item.Credentials = append(item.Credentials, note)
	}
	if err := item.Validate(); err != nil {
		return nil, fmt.Errorf("cannot migrate SSH key: %w", err)
	}
	return item, nil
}

// equalPublicKey compares SSH public-key wire encodings.
func equalPublicKey(first, second ssh.PublicKey) bool {
	return first != nil && second != nil && bytes.Equal(first.Marshal(), second.Marshal())
}
