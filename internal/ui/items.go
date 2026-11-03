// SPDX-FileCopyrightText: 2021-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"encoding/json"

	"fyne.io/fyne/v2"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// summaryIcon uses a stored login favicon or the entry type's icon.
func summaryIcon(item *cxf.Item) fyne.Resource {
	kind, err := model.ItemTypeOf(item)
	if err != nil {
		return icons.Vault
	}
	if kind == model.LoginItemType {
		for _, raw := range item.Extensions {
			var extension struct {
				Name    string         `json:"name"`
				Favicon *model.Favicon `json:"favicon"`
			}
			if json.Unmarshal(raw, &extension) == nil && extension.Name == "paw.pm/item" && extension.Favicon != nil && len(extension.Favicon.Data) > 0 {
				return extension.Favicon
			}
		}
	}
	switch kind {
	case model.NoteItemType:
		return icons.Note
	case model.LoginItemType:
		return icons.Login
	case model.PasswordItemType:
		return icons.Password
	case model.SSHKeyItemType:
		return icons.SSH
	}
	return icons.Vault
}
