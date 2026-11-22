// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/storage"
)

func TestInitCreatesVaultWithoutExtraOutput(t *testing.T) {
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	base, stderrOutput := initTestCommand(store, []string{"master-password", "master-password"})
	stdout, diagnostics, err := executeSubcommand(t, []string{"init", "personal"}, newInitCommand, base, "")
	require.NoError(t, err)
	require.Empty(t, stdout)
	require.Empty(t, diagnostics)
	require.Empty(t, stderrOutput.String())
	vaults, err := store.Vaults()
	require.NoError(t, err)
	require.Len(t, vaults, 1)
	_, err = store.LoadVaultKey(vaults[0].ID, "master-password")
	require.NoError(t, err)
}

func TestInitRetriesMismatchedPasswordConfirmation(t *testing.T) {
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	base, stderrOutput := initTestCommand(store, []string{"first-password", "mismatch", "master-password", "master-password"})
	stdout, stderr, err := executeSubcommand(t, []string{"init", "personal"}, newInitCommand, base, "")
	require.NoError(t, err)
	require.Empty(t, stdout)
	require.Empty(t, stderr)
	require.Equal(t, "Passwords do not match\n", stderrOutput.String())
	vaults, err := store.Vaults()
	require.NoError(t, err)
	require.Len(t, vaults, 1)
	_, err = store.LoadVaultKey(vaults[0].ID, "master-password")
	require.NoError(t, err)
}

func TestInitCancellationDoesNotCreateVault(t *testing.T) {
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	var output, stderr strings.Builder
	prompter := newPrompter(strings.NewReader(""), &stderr)
	prompter.readPassword = func() ([]byte, error) { return nil, errCanceled }
	base := &command{storage: store, prompter: prompter, out: &output, errOut: &stderr}

	stdout, diagnostics, err := executeSubcommand(t, []string{"init", "personal"}, newInitCommand, base, "")
	require.ErrorIs(t, err, errCanceled)
	require.Empty(t, stdout)
	require.Empty(t, diagnostics)
	vaults, err := store.Vaults()
	require.NoError(t, err)
	require.Empty(t, vaults)
}

func TestInitRequiresTerminalAndDoesNotCreateVault(t *testing.T) {
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	var output, stderr strings.Builder
	base := &command{
		storage:  store,
		prompter: newPrompter(strings.NewReader("password\n"), &stderr),
		out:      &output,
		errOut:   &stderr,
	}

	stdout, diagnostics, err := executeSubcommand(t, []string{"init", "personal"}, newInitCommand, base, "")
	require.ErrorContains(t, err, "requires stdin to be a terminal")
	require.Empty(t, stdout)
	require.Empty(t, diagnostics)
	vaults, err := store.Vaults()
	require.NoError(t, err)
	require.Empty(t, vaults)
}

func TestInitValidatesArguments(t *testing.T) {
	for _, args := range [][]string{{"init"}, {"init", "personal", "extra"}} {
		_, _, err := executeSubcommand(t, args, newInitCommand, &command{}, "")
		require.ErrorContains(t, err, "invalid arguments")
	}
}

func initTestCommand(store *storage.FileStorage, passwords []string) (*command, *strings.Builder) {
	var stderr strings.Builder
	prompter := newPrompter(strings.NewReader(""), &stderr)
	prompter.readPassword = func() ([]byte, error) {
		password := passwords[0]
		passwords = passwords[1:]
		return []byte(password), nil
	}
	return &command{storage: store, prompter: prompter, out: &stderr, errOut: &stderr}, &stderr
}
