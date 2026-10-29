// SPDX-FileCopyrightText: 2022-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fmt"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// newItemTemplate creates an empty CXF item for a supported category.
func newItemTemplate(itemType paw.ItemType) (*model.Item, error) {
	settings := model.PasswordSettings{Mode: paw.CustomPassword}
	switch itemType {
	case paw.NoteItemType:
		return model.NewNote("", "")
	case paw.PasswordItemType:
		return model.NewPassword("", "", settings)
	case paw.LoginItemType:
		return model.NewLogin("", "", "", settings)
	case paw.SSHKeyItemType:
		credential, err := GenerateSSHKey()
		if err != nil {
			return nil, err
		}
		credential.KeyComment = "SSH key"
		return model.NewSSHKey("", credential, "", settings)
	default:
		return nil, fmt.Errorf("unsupported item type %d", itemType)
	}
}
