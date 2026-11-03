// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package legacy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVaultItems(t *testing.T) {
	vault := NewVault(nil, "Personal")
	note := NewNote()
	note.Name = "Example note"
	password := NewPassword()
	password.Name = "Example password"
	require.NoError(t, vault.AddItem(note))
	require.NoError(t, vault.AddItem(password))

	items := vault.Items()
	require.Len(t, items, 2)
	require.Contains(t, items, VaultItem{ID: note.ID(), Metadata: note.Metadata})
	require.Contains(t, items, VaultItem{ID: password.ID(), Metadata: password.Metadata})
}
