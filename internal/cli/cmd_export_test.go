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

	"charm.land/huh/v2"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/agent"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

func TestExportCommandRejectsInvalidConfirmation(t *testing.T) {
	for _, values := range [][]string{{"", ""}, {"secret", "different"}} {
		p := newPrompter(strings.NewReader(""), io.Discard)
		index := 0
		p.readPassword = func() ([]byte, error) { value := values[index]; index++; return []byte(value), nil }
		path := filepath.Join(t.TempDir(), "credentials.cxf.age")
		_, _, err := executeSubcommand(t, []string{"export", "personal", path}, newExportCommand, &command{prompter: p}, "")
		require.Error(t, err)
		_, err = os.Stat(path)
		require.ErrorIs(t, err, os.ErrNotExist)
	}
}

func TestExportCommandPlaintextRequiresConsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.cxf")
	prompter := testInteractivePrompter(t, "n\n")
	var promptOutput bytes.Buffer
	prompter.output = &promptOutput
	base := &command{prompter: prompter}
	_, warning, err := executeSubcommand(t, []string{"export", "--plaintext", "personal", path}, newExportCommand, base, "")
	require.ErrorIs(t, err, errCanceled)
	require.Contains(t, promptOutput.String(), "credentials in plaintext")
	require.Empty(t, warning)
	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestExportCommandDefaultEncryptedCXFIsSilent(t *testing.T) {
	_, _, _, base := historyCLIFixture(t)
	passwords := []string{"export-password", "export-password", "synthetic-password"}
	base.prompter.readPassword = func() ([]byte, error) {
		password := passwords[0]
		passwords = passwords[1:]
		return []byte(password), nil
	}
	destination := filepath.Join(t.TempDir(), "credentials.cxf.age")
	stdout, stderr, err := executeSubcommand(t, []string{"export", "Personal", destination}, newExportCommand, base, "")
	require.NoError(t, err)
	require.Empty(t, stdout)
	require.Empty(t, stderr)
	data, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.NotContains(t, string(data), "current-secret")
	backup, err := storage.ReadPawBackup(context.Background(), bytes.NewReader(data), "export-password")
	require.NoError(t, err)
	require.Equal(t, 1, backup.Credentials)
	require.NoError(t, backup.Close())
	_, err = storage.ReadPawBackup(context.Background(), bytes.NewReader(data), "wrong-password")
	require.ErrorContains(t, err, "incorrect password")
}

func TestExportCommandPlaintextAndYesAreSilent(t *testing.T) {
	_, _, _, base := historyCLIFixture(t)
	base.prompter.readPassword = func() ([]byte, error) { return []byte("synthetic-password"), nil }
	base.prompter.runForm = func(*huh.Form) error {
		t.Fatal("--yes must skip the plaintext confirmation")
		return nil
	}
	destination := filepath.Join(t.TempDir(), "credentials.cxf")
	stdout, stderr, err := executeSubcommand(t, []string{"export", "--plaintext", "--yes", "Personal", destination}, newExportCommand, base, "")
	require.NoError(t, err)
	require.Empty(t, stdout)
	require.Empty(t, stderr)
	data, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Contains(t, string(data), "current-secret")
}

func TestExportCommandPlaintextConfirmation(t *testing.T) {
	_, _, _, base := historyCLIFixture(t)
	base.prompter = historyPrompter(t, "y\n")
	destination := filepath.Join(t.TempDir(), "credentials.cxf")
	stdout, stderr, err := executeSubcommand(t, []string{"export", "--plaintext", "Personal", destination}, newExportCommand, base, "")
	require.NoError(t, err)
	require.Empty(t, stdout)
	require.Empty(t, stderr)
	require.FileExists(t, destination)
}

func TestExportCommandPasswordStdinReadsOneLineWithSession(t *testing.T) {
	_, vault, _, base := historyCLIFixture(t)
	base.agentClient = func(string) (agent.PawAgent, error) {
		return addTestAgent{key: vault.Key()}, nil
	}
	base.prompter.readPassword = func() ([]byte, error) {
		t.Fatal("password-stdin must not ask for another secret when the session is valid")
		return nil, nil
	}
	destination := filepath.Join(t.TempDir(), "credentials.cxf.age")
	stdout, stderr, err := executeSubcommand(t, []string{"export", "--password-stdin", "--session", "active", "Personal", destination}, newExportCommand, base, "export-password\nunused-input\n")
	require.NoError(t, err)
	require.Empty(t, stdout)
	require.Empty(t, stderr)
	data, err := os.ReadFile(destination)
	require.NoError(t, err)
	backup, err := storage.ReadPawBackup(context.Background(), bytes.NewReader(data), "export-password")
	require.NoError(t, err)
	require.NoError(t, backup.Close())
}

func TestExportCommandPasswordStdinAllowsPawArchive(t *testing.T) {
	store, vault, item, base := historyCLIFixture(t)
	attachment := []byte("archive payload")
	_, err := store.AddAttachment(vault, item.ID, "source.bin", bytes.NewReader(attachment))
	require.NoError(t, err)
	base.agentClient = func(string) (agent.PawAgent, error) {
		return addTestAgent{key: vault.Key()}, nil
	}
	destination := filepath.Join(t.TempDir(), "credentials.paw")
	stdout, stderr, err := executeSubcommand(t, []string{"export", "--paw", "--password-stdin", "--session", "active", "Personal", destination}, newExportCommand, base, "archive-password\n")
	require.NoError(t, err)
	require.Empty(t, stdout)
	require.Empty(t, stderr)
	archive, err := os.Open(destination)
	require.NoError(t, err)
	backup, err := storage.ReadPawArchive(context.Background(), archive, "archive-password")
	require.NoError(t, err)
	require.NoError(t, archive.Close())
	require.NoError(t, backup.Close())
}

func TestExportCommandPasswordStdinRejectsEmptyInput(t *testing.T) {
	_, vault, _, base := historyCLIFixture(t)
	base.agentClient = func(string) (agent.PawAgent, error) {
		return addTestAgent{key: vault.Key()}, nil
	}
	destination := filepath.Join(t.TempDir(), "credentials.cxf.age")
	_, _, err := executeSubcommand(t, []string{"export", "--password-stdin", "--session", "active", "Personal", destination}, newExportCommand, base, "\n")
	require.ErrorContains(t, err, "export password is required")
	require.NoFileExists(t, destination)
}

func TestExportCommandPasswordStdinDoesNotPromptWithoutTTY(t *testing.T) {
	_, _, _, base := historyCLIFixture(t)
	piped := strings.NewReader("vault-password\n")
	base.prompter = newPrompter(piped, io.Discard)
	destination := filepath.Join(t.TempDir(), "credentials.cxf.age")
	stdout, stderr, err := executeSubcommand(t, []string{"export", "--password-stdin", "Personal", destination}, newExportCommand, base, "export-password\n")
	require.ErrorContains(t, err, "requires stdin to be a terminal")
	require.Equal(t, len("vault-password\n"), piped.Len())
	require.Empty(t, stdout)
	require.Empty(t, stderr)
	require.NoFileExists(t, destination)
}

func TestExportCommandRejectsWrongVaultPassword(t *testing.T) {
	_, _, _, base := historyCLIFixture(t)
	passwords := []string{"export-password", "export-password", "wrong-vault-password"}
	base.prompter.readPassword = func() ([]byte, error) {
		password := passwords[0]
		passwords = passwords[1:]
		return []byte(password), nil
	}
	destination := filepath.Join(t.TempDir(), "credentials.cxf.age")
	stdout, stderr, err := executeSubcommand(t, []string{"export", "Personal", destination}, newExportCommand, base, "")
	require.Error(t, err)
	require.Empty(t, stdout)
	require.Empty(t, stderr)
	require.NoFileExists(t, destination)
}

func TestExportCommandExistingDestinationIsNotOverwritten(t *testing.T) {
	_, _, _, base := historyCLIFixture(t)
	passwords := []string{"export-password", "export-password", "synthetic-password"}
	base.prompter.readPassword = func() ([]byte, error) {
		password := passwords[0]
		passwords = passwords[1:]
		return []byte(password), nil
	}
	destination := filepath.Join(t.TempDir(), "existing.cxf.age")
	require.NoError(t, os.WriteFile(destination, []byte("keep this file"), 0o600))
	_, _, err := executeSubcommand(t, []string{"export", "Personal", destination}, newExportCommand, base, "")
	require.ErrorContains(t, err, "export destination already exists")
	content, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, "keep this file", string(content))
}

func TestExportCommandFlagValidationAndHelp(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"export", "--plaintext", "--password-stdin", "Personal", "backup"}, "cannot be combined"},
		{[]string{"export", "--yes", "Personal", "backup"}, "only valid with --plaintext"},
		{[]string{"export", "--plaintext", "--paw", "Personal", "backup"}, "cannot be combined"},
		{[]string{"export", "--encrypted", "Personal", "backup"}, "flag provided but not defined"},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			stdout, stderr, err := executeWithInput(t, test.args, nil, "")
			require.ErrorContains(t, err, test.want)
			require.Empty(t, stdout)
			require.Empty(t, stderr)
		})
	}
	stdout, _, err := executeWithInput(t, []string{"export", "--help"}, nil, "")
	require.NoError(t, err)
	require.Contains(t, stdout, "--password-stdin")
	require.Contains(t, stdout, "--yes")
	require.NotContains(t, stdout, "--encrypted")
}

func TestExportPasswordPromptRequiresTTY(t *testing.T) {
	_, _, _, base := historyCLIFixture(t)
	piped := strings.NewReader("should not be consumed\n")
	base.prompter = newPrompter(piped, io.Discard)
	destination := filepath.Join(t.TempDir(), "credentials.cxf.age")
	_, _, err := executeSubcommand(t, []string{"export", "Personal", destination}, newExportCommand, base, "")
	require.ErrorContains(t, err, "requires stdin to be a terminal")
	require.Equal(t, len("should not be consumed\n"), piped.Len())
	require.NoFileExists(t, destination)
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
	output, diagnostics, err := executeSubcommand(t, []string{"export", "--paw", "Personal", archivePath}, newExportCommand, base, "")
	require.NoError(t, err)
	require.Empty(t, output)
	require.Empty(t, diagnostics)
	require.FileExists(t, archivePath)

	passwords = []string{"archive-password", "new-master", "new-master"}
	base.prompter = historyPrompter(t, "y\n")
	base.prompter.readPassword = func() ([]byte, error) {
		value := passwords[0]
		passwords = passwords[1:]
		return []byte(value), nil
	}
	restoredID, diagnostics, err := executeSubcommand(t, []string{"restore", "--name", "Imported", archivePath}, newRestoreCommand, base, "y\n")
	require.NoError(t, err)
	require.Contains(t, diagnostics, "Vault: Personal\nCollections: 0\nCredentials: 1\n")
	require.Contains(t, diagnostics, "The original vault ID is already in use. An independent copy was created.")
	vaults, err := store.Vaults()
	require.NoError(t, err)
	require.Len(t, vaults, 2)
	importedInfo, err := model.FindVaultInfo(vaults, "Imported")
	require.NoError(t, err)
	require.NotEqual(t, vault.Accounts[0].ID, importedInfo.ID)
	require.Equal(t, storage.FormatVaultID(importedInfo.ID)+"\n", restoredID)
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
