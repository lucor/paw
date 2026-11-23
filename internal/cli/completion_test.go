// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/test"
)

// TestVaultCompletion checks candidates without unlocking a vault.
func TestVaultCompletion(t *testing.T) {
	store, err := storage.NewFileStorage(filepath.Join(t.TempDir(), "data"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	test.CreateVaultWithPassword(t, store, "Personal Vault", "password")

	for _, tc := range []struct {
		args []string
		want string
	}{
		{args: []string{"unlock"}, want: "Personal Vault\n"},
		{args: []string{"show"}, want: "Personal Vault/login/\n"},
		{args: []string{"edit"}, want: "Personal Vault/note/\n"},
		{args: []string{"add", "login"}, want: "Personal Vault\n"},
		{args: []string{"unlock", "--life"}, want: "--lifetime"},
		{args: []string{"unlock", "Personal Vault"}},
		{args: []string{"add", "login", "Personal Vault"}},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			var out, stderr bytes.Buffer
			cmd := newCommand(store, strings.NewReader(""), &out, &stderr)
			args := append([]string{"paw cli"}, tc.args...)
			args = append(args, "--generate-shell-completion")
			require.NoError(t, cmd.Run(context.Background(), args))
			if tc.want == "" {
				require.Empty(t, out.String())
			} else {
				require.Contains(t, out.String(), tc.want)
			}
			require.Empty(t, stderr.String())
		})
	}
}
