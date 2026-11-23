// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

type attachmentFailingWriter struct{ err error }

func (writer attachmentFailingWriter) Write([]byte) (int, error) { return 0, writer.err }

func TestAttachmentCLIAddListSaveRemove(t *testing.T) {
	store, vault, item, base := historyCLIFixture(t)
	path := vault.Metadata.VaultName + "/note/" + item.Title
	source := filepath.Join(t.TempDir(), "source file.bin")
	payload := []byte{0, 255, 7, 0}
	require.NoError(t, os.WriteFile(source, payload, 0600))
	added, stderr, err := runAttachmentCLI(t, base, []string{"attachment", "add", path, source})
	require.NoError(t, err)
	require.Regexp(t, `^[A-Za-z0-9_-]+\n$`, added)
	require.Empty(t, stderr)
	listed, stderr, err := runAttachmentCLI(t, base, []string{"attachment", "list", path})
	require.NoError(t, err)
	require.Empty(t, stderr)
	require.Contains(t, listed, "ID")
	require.Contains(t, listed, "Name")
	require.Contains(t, listed, "Size")
	require.Contains(t, listed, "source file.bin")
	key, err := store.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	id, err := store.ListAttachments(key, item.ID)
	require.NoError(t, err)
	require.Len(t, id, 1)
	destinationDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	destination := filepath.Join(destinationDir, "saved file.bin")
	stdout, stderr, err := runAttachmentCLI(t, base, []string{"attachment", "save", "--output", destination, path, id[0].ID})
	require.NoError(t, err)
	require.Empty(t, stdout)
	require.Empty(t, stderr)
	got, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, payload, got)
	stdout, stderr, err = runAttachmentCLI(t, base, []string{"attachment", "remove", path, id[0].ID})
	require.NoError(t, err)
	require.Empty(t, stdout)
	require.Empty(t, stderr)
	listed, stderr, err = runAttachmentCLI(t, base, []string{"attachment", "list", path})
	require.NoError(t, err)
	require.Empty(t, listed)
	require.Empty(t, stderr)
}

func TestAttachmentCLIAddsBinaryStdinAndRequiresName(t *testing.T) {
	store, vault, item, base := historyCLIFixture(t)
	path := vault.Metadata.VaultName + "/note/" + item.Title
	payload := []byte{0, 255, 1, 0, 128}
	_, _, err := runAttachmentCLIInput(t, base, []string{"attachment", "add", path, "-"}, string(payload))
	require.ErrorContains(t, err, "--name is required")
	stdout, stderr, err := runAttachmentCLIInput(t, base, []string{"attachment", "add", path, "-", "--name", "binary\x1b[31m.bin"}, string(payload))
	require.NoError(t, err)
	require.Regexp(t, `^[A-Za-z0-9_-]+\n$`, stdout)
	require.Empty(t, stderr)
	vault, err = store.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	files, err := store.ListAttachments(vault, item.ID)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "binary\x1b[31m.bin", files[0].Name)
	require.EqualValues(t, payload, readAttachmentForTest(t, store, vault, item.ID, files[0].ID))
	listed, stderr, err := runAttachmentCLI(t, base, []string{"attachment", "list", path})
	require.NoError(t, err)
	require.Empty(t, stderr)
	require.NotContains(t, listed, "\x1b[31m")
	require.Contains(t, listed, `\u001b[31m`)
}

func TestAttachmentCLIAddsRelativePathAndRejectsMissingFile(t *testing.T) {
	_, vault, item, base := historyCLIFixture(t)
	directory := t.TempDir()
	file := filepath.Join(directory, "relative.bin")
	require.NoError(t, os.WriteFile(file, []byte("contents"), 0o600))
	previousDirectory, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(directory))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previousDirectory)) })
	path := vault.Metadata.VaultName + "/note/" + item.Title
	_, _, err = runAttachmentCLI(t, base, []string{"attachment", "add", path, "relative.bin"})
	require.NoError(t, err)
	_, _, err = runAttachmentCLI(t, base, []string{"attachment", "add", path, filepath.Join(t.TempDir(), "missing")})
	require.ErrorContains(t, err, "open attachment source")
}

func TestAttachmentCLIPropagatesOutputErrors(t *testing.T) {
	_, vault, item, base := historyCLIFixture(t)
	wantErr := errors.New("stdout unavailable")
	base.out = attachmentFailingWriter{err: wantErr}
	add := &AttachmentCmd{
		command:   base,
		operation: attachmentAdd,
		itemPath: itemPath{
			vaultName: vault.Metadata.VaultName,
			itemType:  model.NoteItemType,
			itemName:  item.Title,
		},
		filePath: "-",
		name:     "stdin.bin",
		input:    strings.NewReader("attachment data"),
	}
	require.ErrorIs(t, add.Run(t.Context(), nil), wantErr)
	list := &AttachmentCmd{
		command: base,
		itemPath: itemPath{
			vaultName: vault.Metadata.VaultName,
			itemType:  model.NoteItemType,
			itemName:  item.Title,
		},
		sessionID: "",
	}
	list.operation = attachmentList
	require.ErrorIs(t, list.Run(t.Context(), nil), wantErr)
}

func TestAttachmentCLIRequiresOutputForSave(t *testing.T) {
	_, _, _, base := historyCLIFixture(t)

	_, _, err := runAttachmentCLI(t, base, []string{"attachment", "save", "personal/note/current", "file-id"})
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
			_, _, err := runAttachmentCLI(t, base, test.args)
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

func runAttachmentCLI(t *testing.T, base *command, args []string) (string, string, error) {
	return runAttachmentCLIInput(t, base, args, "")
}

func runAttachmentCLIInput(t *testing.T, base *command, args []string, input string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	commandBase := *base
	commandBase.out = &stdout
	commandBase.errOut = &stderr
	cmd := &urfave.Command{
		Name:           "paw cli",
		Commands:       []*urfave.Command{newAttachmentCommand(&commandBase)},
		ExitErrHandler: leaveExitToCaller,
	}
	cmd.Writer = &stdout
	cmd.ErrWriter = &stderr
	cmd.Reader = strings.NewReader(input)
	err := cmd.Run(t.Context(), append([]string{"paw cli"}, args...))
	return stdout.String(), stderr.String(), err
}

func readAttachmentForTest(t *testing.T, store *storage.FileStorage, vault *model.Vault, itemID, fileID string) []byte {
	t.Helper()
	directory, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	path := filepath.Join(directory, "attachment.bin")
	require.NoError(t, store.SaveAttachment(vault, itemID, fileID, path))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}
