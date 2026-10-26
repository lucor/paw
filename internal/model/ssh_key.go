// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"go.lucor.dev/cxf"

	"lucor.dev/paw/internal/paw"
)

// NewSSHKey creates a Paw SSH-key item with a new ID and current dates.
func NewSSHKey(title string, key cxf.SSHKey, passphrase string, settings PasswordSettings) (*Item, error) {
	if !utf8.ValidString(title) || !utf8.ValidString(key.KeyComment) {
		return nil, fmt.Errorf("cannot create SSH key: invalid UTF-8")
	}
	if err := key.Validate(); err != nil {
		return nil, fmt.Errorf("cannot create SSH key: invalid CXF credential")
	}
	passphraseField, err := newPasswordField(passphrase, settings)
	if err != nil {
		return nil, fmt.Errorf("cannot create SSH key passphrase: %w", err)
	}
	id, err := newID()
	if err != nil {
		return nil, fmt.Errorf("cannot generate SSH key ID: %w", err)
	}
	keyCredential, err := json.Marshal(key)
	if err != nil {
		return nil, fmt.Errorf("cannot encode SSH key credential")
	}
	passphraseCredential, err := json.Marshal(cxf.BasicAuth{
		Type:     cxf.CredentialTypeBasicAuth,
		Password: passphraseField,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot encode SSH key passphrase")
	}
	created := uint64(time.Now().Unix())
	modified := created
	metadata := ItemMetadata{Type: paw.SSHKeyItemType}
	extension, err := encodeItemExtension(nil, metadata)
	if err != nil {
		return nil, fmt.Errorf("cannot encode SSH key metadata: %w", err)
	}
	item := &Item{
		Item: cxf.Item{
			ID:          id,
			Title:       title,
			CreationAt:  &created,
			ModifiedAt:  &modified,
			Credentials: []json.RawMessage{keyCredential, passphraseCredential},
			Extensions:  []json.RawMessage{extension},
		},
		Metadata: metadata,
	}
	if err := item.Validate(); err != nil {
		return nil, fmt.Errorf("cannot create SSH key: %w", err)
	}
	return item, nil
}
