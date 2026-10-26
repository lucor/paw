// SPDX-FileCopyrightText: 2022-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/icon"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/sshkey"
)

// makeAddItemView displays the supported item categories for creation.
func (a *app) makeAddItemView() fyne.CanvasObject {
	content := container.NewVBox()
	for _, itemType := range a.makeEmptyItems() {
		itemType := itemType
		button := widget.NewButtonWithIcon(itemType.Label(), newItemIcon(itemType), func() {
			item, err := newItemTemplate(itemType)
			if err != nil {
				dialog.ShowError(err, a.win)
				return
			}
			a.showEditItemView(newModelItemWidget(item, a.state.Preferences, true))
		})
		button.Alignment = widget.ButtonAlignLeading
		content.Add(button)
	}
	return container.NewBorder(a.makeCancelHeaderButton(), nil, nil, nil, container.NewCenter(content))
}

// newItemIcon returns the icon used for an item category.
func newItemIcon(itemType paw.ItemType) fyne.Resource {
	switch itemType {
	case paw.NoteItemType:
		return icon.NoteOutlinedIconThemed
	case paw.PasswordItemType:
		return icon.PasswordOutlinedIconThemed
	case paw.LoginItemType:
		return icon.WorldWWWOutlinedIconThemed
	case paw.SSHKeyItemType:
		return icon.KeyOutlinedIconThemed
	}
	return icon.PawIcon
}

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
