// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/storage"
)

func TestRestoreEncryptedCXFWithExplicitNameAndYes(t *testing.T) {
	store, source, _, base := historyCLIFixture(t)
	backupPath := filepath.Join(t.TempDir(), "encrypted.cxf.age")
	require.NoError(t, storage.ExportFile(context.Background(), store, source, backupPath, "backup-password", false))
	passwords := []string{"backup-password", "new-master", "new-master"}
	base.prompter.readPassword = func() ([]byte, error) {
		password := passwords[0]
		passwords = passwords[1:]
		return []byte(password), nil
	}
	base.prompter.runForm = func(*huh.Form) error {
		t.Fatal("--yes with --name must skip only the restore confirmation")
		return nil
	}
	stdout, stderr, err := executeSubcommand(t, []string{"restore", "--name", "Imported", "--yes", backupPath}, newRestoreCommand, base, "")
	require.NoError(t, err)
	require.Empty(t, passwords)
	require.Contains(t, stderr, "Vault: Personal\nCollections: 0\nCredentials: 1\n")
	require.Contains(t, stderr, "The original vault ID is already in use. An independent copy was created.")
	require.NotContains(t, stderr, "new-master")
	vaults, err := store.Vaults()
	require.NoError(t, err)
	require.Len(t, vaults, 2)
	imported, err := base.resolveVaultInfo("Imported")
	require.NoError(t, err)
	require.NotEqual(t, source.Accounts[0].ID, imported.ID)
	require.Equal(t, storage.FormatVaultID(imported.ID)+"\n", stdout)
	key, err := store.LoadVaultKey(imported.ID, "new-master")
	require.NoError(t, err)
	_, err = store.LoadVault(imported.ID, key)
	require.NoError(t, err)
}

func TestRestorePlaintextPromptsForNameAndConfirmation(t *testing.T) {
	store, source, _, base := historyCLIFixture(t)
	backupPath := filepath.Join(t.TempDir(), "plaintext.cxf")
	require.NoError(t, storage.ExportFile(context.Background(), store, source, backupPath, "", true))
	passwords := []string{"new-master", "new-master"}
	base.prompter = testInteractivePrompter(t, "Imported name\ny\n")
	base.prompter.readPassword = func() ([]byte, error) {
		password := passwords[0]
		passwords = passwords[1:]
		return []byte(password), nil
	}
	stdout, stderr, err := executeSubcommand(t, []string{"restore", backupPath}, newRestoreCommand, base, "")
	require.NoError(t, err)
	require.Empty(t, passwords)
	require.Contains(t, stderr, "Vault: Personal\n")
	vaults, err := store.Vaults()
	require.NoError(t, err)
	imported, err := base.resolveVaultInfo("Imported name")
	require.NoError(t, err)
	require.Equal(t, storage.FormatVaultID(imported.ID)+"\n", stdout)
	require.NotEqual(t, source.Accounts[0].ID, imported.ID)
	require.Len(t, vaults, 2)
}

func TestRestoreYesStillPromptsForNameAndPasswords(t *testing.T) {
	store, source, _, base := historyCLIFixture(t)
	backupPath := filepath.Join(t.TempDir(), "plaintext.cxf")
	require.NoError(t, storage.ExportFile(context.Background(), store, source, backupPath, "", true))
	passwords := []string{"new-master", "new-master"}
	base.prompter = testInteractivePrompter(t, "Imported with yes\n")
	base.prompter.readPassword = func() ([]byte, error) {
		password := passwords[0]
		passwords = passwords[1:]
		return []byte(password), nil
	}
	stdout, stderr, err := executeSubcommand(t, []string{"restore", "--yes", backupPath}, newRestoreCommand, base, "")
	require.NoError(t, err)
	require.Empty(t, passwords)
	require.Contains(t, stderr, "Vault: Personal")
	imported, err := base.resolveVaultInfo("Imported with yes")
	require.NoError(t, err)
	require.Equal(t, storage.FormatVaultID(imported.ID)+"\n", stdout)
	vaults, err := store.Vaults()
	require.NoError(t, err)
	require.Len(t, vaults, 2)
}

func TestRestoreNoAndCancellationDoNotCreateVault(t *testing.T) {
	for _, test := range []struct {
		name string
		form func(*prompter)
	}{
		{name: "no", form: func(p *prompter) { p.runForm = func(form *huh.Form) error { return form.WithAccessible(true).Run() } }},
		{name: "cancel", form: func(p *prompter) { p.runForm = func(*huh.Form) error { return errCanceled } }},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, source, _, base := historyCLIFixture(t)
			backupPath := filepath.Join(t.TempDir(), "plaintext.cxf")
			require.NoError(t, storage.ExportFile(context.Background(), store, source, backupPath, "", true))
			base.prompter = testInteractivePrompter(t, "n\n")
			test.form(base.prompter)
			base.prompter.readPassword = func() ([]byte, error) {
				t.Fatal("restore confirmation must happen before the new master password prompt")
				return nil, nil
			}
			stdout, stderr, err := executeSubcommand(t, []string{"restore", "--name", "Cancelled", backupPath}, newRestoreCommand, base, "")
			require.ErrorIs(t, err, errCanceled)
			require.Empty(t, stdout)
			require.Contains(t, stderr, "Vault: Personal")
			vaults, err := store.Vaults()
			require.NoError(t, err)
			require.Len(t, vaults, 1)
		})
	}
}

func TestRestoreRejectsWrongBackupPasswordAndMasterMismatch(t *testing.T) {
	t.Run("wrong backup password", func(t *testing.T) {
		store, source, _, base := historyCLIFixture(t)
		backupPath := filepath.Join(t.TempDir(), "encrypted.paw")
		require.NoError(t, storage.ExportPawFile(context.Background(), store, source, backupPath, "backup-password"))
		base.prompter.readPassword = func() ([]byte, error) { return []byte("wrong-password"), nil }
		stdout, stderr, err := executeSubcommand(t, []string{"restore", "--name", "Imported", backupPath}, newRestoreCommand, base, "")
		require.Error(t, err)
		require.Empty(t, stdout)
		require.Empty(t, stderr)
		vaults, err := store.Vaults()
		require.NoError(t, err)
		require.Len(t, vaults, 1)
	})

	t.Run("master password confirmation mismatch", func(t *testing.T) {
		store, source, _, base := historyCLIFixture(t)
		backupPath := filepath.Join(t.TempDir(), "plaintext.cxf")
		require.NoError(t, storage.ExportFile(context.Background(), store, source, backupPath, "", true))
		passwords := []string{"new-master", "different-master"}
		base.prompter.readPassword = func() ([]byte, error) {
			password := passwords[0]
			passwords = passwords[1:]
			return []byte(password), nil
		}
		stdout, stderr, err := executeSubcommand(t, []string{"restore", "--name", "Imported", "--yes", backupPath}, newRestoreCommand, base, "")
		require.ErrorContains(t, err, "master passwords do not match")
		require.Empty(t, stdout)
		require.Contains(t, stderr, "Vault: Personal")
		vaults, err := store.Vaults()
		require.NoError(t, err)
		require.Len(t, vaults, 1)
	})
}

func TestRestoreYesStillRequiresTTYForPasswords(t *testing.T) {
	store, source, _, base := historyCLIFixture(t)
	backupPath := filepath.Join(t.TempDir(), "encrypted.paw")
	require.NoError(t, storage.ExportPawFile(context.Background(), store, source, backupPath, "backup-password"))
	piped := strings.NewReader("backup-password\n")
	base.prompter = newPrompter(piped, io.Discard)
	stdout, stderr, err := executeSubcommand(t, []string{"restore", "--name", "Imported", "--yes", backupPath}, newRestoreCommand, base, "")
	require.ErrorContains(t, err, "requires stdin to be a terminal")
	require.Equal(t, len("backup-password\n"), piped.Len())
	require.Empty(t, stdout)
	require.Empty(t, stderr)
	vaults, err := store.Vaults()
	require.NoError(t, err)
	require.Len(t, vaults, 1)
}

func TestRestoreValidationAndNoImplicitStdinMode(t *testing.T) {
	for _, args := range [][]string{
		{"restore"},
		{"restore", "backup", "extra"},
		{"restore", "--password-stdin", "backup"},
	} {
		stdout, stderr, err := executeWithInput(t, args, nil, "secret\n")
		require.Error(t, err)
		require.Empty(t, stdout)
		require.Empty(t, stderr)
	}
	stdout, _, err := executeWithInput(t, []string{"restore", "--help"}, nil, "")
	require.NoError(t, err)
	require.Contains(t, stdout, "--name")
	require.Contains(t, stdout, "--yes")
	require.NotContains(t, stdout, "--password-stdin")
	require.NotContains(t, stdout, "--password-file")
}

func TestRestorePropagatesOutputErrors(t *testing.T) {
	writeErr := errors.New("restore output unavailable")
	store, source, _, base := historyCLIFixture(t)
	backupPath := filepath.Join(t.TempDir(), "plaintext.cxf")
	require.NoError(t, storage.ExportFile(context.Background(), store, source, backupPath, "", true))
	base.prompter.readPassword = func() ([]byte, error) { return []byte("new-master"), nil }

	t.Run("stderr summary", func(t *testing.T) {
		cmd := &RestoreCmd{
			command:    &command{storage: store, prompter: base.prompter, out: io.Discard, errOut: restoreFailWriter{err: writeErr}},
			backupPath: backupPath,
			name:       "Summary failure",
			yes:        true,
		}
		require.ErrorIs(t, cmd.Run(context.Background(), nil), writeErr)
		vaults, err := store.Vaults()
		require.NoError(t, err)
		require.Len(t, vaults, 1)
	})

	t.Run("stdout id", func(t *testing.T) {
		cmd := &RestoreCmd{
			command:    &command{storage: store, prompter: base.prompter, out: restoreFailWriter{err: writeErr}, errOut: io.Discard},
			backupPath: backupPath,
			name:       "ID failure",
			yes:        true,
		}
		require.ErrorIs(t, cmd.Run(context.Background(), nil), writeErr)
		vaults, err := store.Vaults()
		require.NoError(t, err)
		require.Len(t, vaults, 2)
	})
}

type restoreFailWriter struct {
	err error
}

// Write returns the configured failure for restore output tests.
func (writer restoreFailWriter) Write([]byte) (int, error) {
	return 0, writer.err
}
