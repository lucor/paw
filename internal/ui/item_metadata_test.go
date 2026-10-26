// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"

	"go.lucor.dev/paw/internal/icon"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// TestMetadataIconUsesFaviconForLogins checks other item types use their standard icon.
func TestMetadataIconUsesFaviconForLogins(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)

	favicon := paw.NewFavicon("example.com", []byte("icon"), "png")
	tests := []struct {
		name     string
		typeID   paw.ItemType
		standard fyne.Resource
	}{
		{name: "note", typeID: paw.NoteItemType, standard: icon.NoteOutlinedIconThemed},
		{name: "password", typeID: paw.PasswordItemType, standard: icon.PasswordOutlinedIconThemed},
		{name: "login", typeID: paw.LoginItemType, standard: icon.WorldWWWOutlinedIconThemed},
		{name: "ssh key", typeID: paw.SSHKeyItemType, standard: icon.KeyOutlinedIconThemed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := &Metadata{Item: &model.Item{Metadata: model.ItemMetadata{Type: tt.typeID, Favicon: favicon}}}
			if tt.typeID == paw.LoginItemType {
				require.Same(t, favicon, metadata.Icon())
				return
			}
			require.Equal(t, tt.standard, metadata.Icon())
		})
	}
}
