// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
)

func TestExportCommandRejectsInvalidConfirmation(t *testing.T) {
	for _, values := range [][]string{{"", ""}, {"secret", "different"}} {
		p := newPrompter(strings.NewReader(""), io.Discard)
		index := 0
		p.readPassword = func() ([]byte, error) { value := values[index]; index++; return []byte(value), nil }
		path := filepath.Join(t.TempDir(), "credentials.cxf.age")
		_, _, err := executeWithInput(t, []string{"export", "personal", path}, newExportCommand(&command{prompter: p}), "")
		require.Error(t, err)
		_, err = os.Stat(path)
		require.ErrorIs(t, err, os.ErrNotExist)
	}
}

func TestExportCommandPlaintextRequiresConsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.cxf")
	_, warning, err := executeWithInput(t, []string{"export", "--plaintext", "personal", path}, newExportCommand(&command{}), "n\n")
	require.ErrorIs(t, err, errCanceled)
	require.Contains(t, warning, "credentials in plaintext")
	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestPawArchiveExportAndRestoreIncludesAttachments(t *testing.T) {
	store, vault, item, base := historyCLIFixture(t)
	payload := []byte{0, 255, 9, 0}
	_, err := store.AddAttachment(vault, item.ID, "source.bin", bytes.NewReader(payload))
	require.NoError(t, err)
	archivePath := filepath.Join(t.TempDir(), "credentials.paw")
	passwords := []string{"archive-password", "archive-password", "synthetic-password"}
	base.prompter.readPassword = func() ([]byte, error) {
		value := passwords[0]
		passwords = passwords[1:]
		return []byte(value), nil
	}
	output, _, err := executeWithInput(t, []string{"export", "--paw", "Personal", archivePath}, newExportCommand(base), "")
	require.NoError(t, err)
	require.Contains(t, output, "Paw archive exported successfully.")
	require.FileExists(t, archivePath)

	passwords = []string{"archive-password", "new-master", "new-master"}
	base.prompter = historyPrompter("y\n")
	base.prompter.readPassword = func() ([]byte, error) {
		value := passwords[0]
		passwords = passwords[1:]
		return []byte(value), nil
	}
	_, _, err = executeWithInput(t, []string{"restore", "--name", "Imported", archivePath}, newRestoreCommand(base), "y\n")
	require.NoError(t, err)
	vaults, err := store.Vaults()
	require.NoError(t, err)
	importedInfo, err := model.FindVaultInfo(vaults, "Imported")
	require.NoError(t, err)
	key, err := store.LoadVaultKey(importedInfo.ID, "new-master")
	require.NoError(t, err)
	importedVault, err := store.LoadVault(importedInfo.ID, key)
	require.NoError(t, err)
	importedItem, err := store.LoadItem(importedVault, item.ID)
	require.NoError(t, err)
	files, err := importedItem.Files()
	require.NoError(t, err)
	require.Len(t, files, 1)
	destinationDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	destination := filepath.Join(destinationDir, "restored.bin")
	require.NoError(t, store.SaveAttachment(importedVault, item.ID, files[0].ID, destination))
	content, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, payload, content)
}
