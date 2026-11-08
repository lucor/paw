// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
)

func TestAttachmentCLIAddListSaveRemove(t *testing.T) {
	store, vault, item, base := historyCLIFixture(t)
	path := vault.Metadata.VaultName + "/note/" + item.Title
	source := filepath.Join(t.TempDir(), "source file.bin")
	payload := []byte{0, 255, 7, 0}
	require.NoError(t, os.WriteFile(source, payload, 0600))
	added, err := runAttachmentCLI(t, base, []string{"attachment", "add", path, source})
	require.NoError(t, err)
	require.Contains(t, added, "source file.bin")
	listed, err := runAttachmentCLI(t, base, []string{"attachment", "list", path})
	require.NoError(t, err)
	require.Contains(t, listed, "source file.bin")
	key, err := store.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	id, err := store.ListAttachments(key, item.ID)
	require.NoError(t, err)
	require.Len(t, id, 1)
	destinationDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	destination := filepath.Join(destinationDir, "saved file.bin")
	_, err = runAttachmentCLI(t, base, []string{"attachment", "save", "--output", destination, path, id[0].ID})
	require.NoError(t, err)
	got, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, payload, got)
	_, err = runAttachmentCLI(t, base, []string{"attachment", "remove", path, id[0].ID})
	require.NoError(t, err)
	listed, err = runAttachmentCLI(t, base, []string{"attachment", "list", path})
	require.NoError(t, err)
	require.Empty(t, listed)
}

func TestAttachmentCLIRequiresOutputForSave(t *testing.T) {
	_, _, _, base := historyCLIFixture(t)

	_, err := runAttachmentCLI(t, base, []string{"attachment", "save", "personal/note/current", "file-id"})
	require.ErrorContains(t, err, "output")
}

func TestAttachmentCLIRejectsInvalidArguments(t *testing.T) {
	_, _, _, base := historyCLIFixture(t)

	tests := []struct {
		name string
		args []string
	}{
		{name: "missing item", args: []string{"attachment", "list"}},
		{name: "missing source", args: []string{"attachment", "add", "personal/note/current"}},
		{name: "missing file id", args: []string{"attachment", "remove", "personal/note/current"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := runAttachmentCLI(t, base, test.args)
			require.Error(t, err)
		})
	}
}

func TestShowIncludesAttachmentMetadataOnlyWhenPresent(t *testing.T) {
	_, _, item, _ := historyCLIFixture(t)
	var output bytes.Buffer
	show := &ShowCmd{command: &command{out: &output}, itemPath: itemPath{itemType: model.NoteItemType}}
	require.NoError(t, show.showItem(t.Context(), item))
	require.NotContains(t, output.String(), "Attachments:")
	require.NotContains(t, output.String(), "History:")

	fileID, err := model.NewFileID()
	require.NoError(t, err)
	file := cxf.File{
		Type:          cxf.CredentialTypeFile,
		ID:            fileID,
		Name:          "document.pdf",
		DecryptedSize: 24 * 1024,
		IntegrityHash: base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
	}
	require.NoError(t, item.AddFile(file))
	output.Reset()
	show.command = &command{out: &output}
	require.NoError(t, show.showItem(t.Context(), item))
	require.Contains(t, output.String(), "Attachments: 1")
	require.NotContains(t, output.String(), "document.pdf", "show reports a count, not a file listing")
	history, err := item.History()
	require.NoError(t, err)
	require.NoError(t, history.AddSnapshot(item, 123))
	require.NoError(t, item.SetHistory(history))
	output.Reset()
	require.NoError(t, show.showItem(t.Context(), item))
	require.Contains(t, output.String(), "Attachments: 1")
	require.Contains(t, output.String(), "History: 1 versions")
}

func runAttachmentCLI(t *testing.T, base *command, args []string) (string, error) {
	t.Helper()
	var output bytes.Buffer
	cmd := &urfave.Command{Name: "paw cli", Commands: []*urfave.Command{newAttachmentCommand(base)}}
	cmd.Writer = &output
	cmd.ErrWriter = &output
	err := cmd.Run(t.Context(), append([]string{"paw cli"}, args...))
	return output.String(), err
}
