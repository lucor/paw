// SPDX-FileCopyrightText: 2021-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"encoding/json"
	"fyne.io/fyne/v2"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/ui/v2/assets/icons"
)

// summaryIcon uses a stored login favicon or the entry type's Lucide icon.
func summaryIcon(item *cxf.Item) fyne.Resource {
	kind, err := model.ItemTypeOf(item)
	if err != nil {
		return icons.Vault
	}
	if kind == paw.LoginItemType {
		for _, raw := range item.Extensions {
			var extension struct {
				Name    string       `json:"name"`
				Favicon *paw.Favicon `json:"favicon"`
			}
			if json.Unmarshal(raw, &extension) == nil && extension.Name == "paw.pm/item" && extension.Favicon != nil {
				return extension.Favicon
			}
		}
	}
	switch kind {
	case paw.NoteItemType:
		return icons.Note
	case paw.LoginItemType:
		return icons.Login
	case paw.PasswordItemType:
		return icons.Password
	case paw.SSHKeyItemType:
		return icons.SSH
	}
	return icons.Vault
}
