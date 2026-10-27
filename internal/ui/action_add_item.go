// SPDX-FileCopyrightText: 2022-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"fmt"

	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/sshkey"
)

// newItemTemplate creates an empty CXF item for a supported category.
func newItemTemplate(itemType paw.ItemType) (*model.Item, error) {
	settings := model.PasswordSettings{Mode: paw.CustomPassword}
	switch itemType {
	case paw.NoteItemType:
		return model.NewNote("Untitled note", "")
	case paw.PasswordItemType:
		return model.NewPassword("Untitled password", "", settings)
	case paw.LoginItemType:
		return model.NewLogin("Untitled login", "", "", settings)
	case paw.SSHKeyItemType:
		key, err := sshkey.GenerateKey()
		if err != nil {
			return nil, err
		}
		privateKey := key.PrivateKey()
		if raw, ok := privateKey.(*ed25519.PrivateKey); ok {
			privateKey = *raw
		}
		der, err := x509.MarshalPKCS8PrivateKey(privateKey)
		if err != nil {
			return nil, fmt.Errorf("cannot encode SSH key")
		}
		credential := cxf.SSHKey{Type: cxf.CredentialTypeSSHKey, KeyType: key.PublicKey().Type(), KeyComment: "SSH key", PrivateKey: base64.RawURLEncoding.EncodeToString(der)}
		return model.NewSSHKey("Untitled SSH key", credential, "", settings)
	default:
		return nil, fmt.Errorf("unsupported item type %d", itemType)
	}
}
