// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/storage"
)

func TestRestoreCommandUsesNewMasterPassword(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		f := newCommandFixture(t)
		var encoded bytes.Buffer
		if encrypted {
			require.NoError(t, storage.WriteEncryptedCXF(context.Background(), &encoded, f.storage, f.vault, "backup-secret"))
		} else {
			require.NoError(t, storage.WriteCXF(context.Background(), &encoded, f.storage, f.vault))
		}
		path := filepath.Join(t.TempDir(), "backup.without-extension")
		require.NoError(t, os.WriteFile(path, encoded.Bytes(), 0600))
		store, err := storage.NewOSStorageRooted(t.TempDir())
		require.NoError(t, err)
		f.base.storage = store
		f.base.prompter = newPrompter(strings.NewReader("y\n"), io.Discard)
		passwords := []string{"new-master", "new-master"}
		if encrypted {
			passwords = append([]string{"backup-secret"}, passwords...)
		}
		index := 0
		f.base.prompter.readPassword = func() ([]byte, error) { value := passwords[index]; index++; return []byte(value), nil }
		out, _, err := executeWithInput(t, []string{"restore", "--name", "Restored", path}, newRestoreCommand(f.base), "")
		require.NoError(t, err)
		require.Contains(t, out, "Vault restored successfully")
		require.NotContains(t, out, "new-master")
		require.NotContains(t, out, "backup-secret")
		vaults, err := store.Vaults()
		require.NoError(t, err)
		require.Len(t, vaults, 1)
		require.Equal(t, "Restored", vaults[0].Name)
		_, err = store.LoadVaultKey(vaults[0].ID, "new-master")
		require.NoError(t, err)
		_, err = store.LoadVaultKey(vaults[0].ID, "backup-secret")
		require.Error(t, err)
	}
}
