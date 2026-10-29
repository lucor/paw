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

	"filippo.io/age"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
)

func TestExportCommandDefaultEncryptionAndConfirmation(t *testing.T) {
	for _, values := range [][]string{{"export-secret", "export-secret"}, {"", ""}, {"secret", "different"}} {
		f := newCommandFixture(t)
		f.base.prompter = newPrompter(strings.NewReader(""), io.Discard)
		index := 0
		f.base.prompter.readPassword = func() ([]byte, error) { value := values[index]; index++; return []byte(value), nil }
		path := filepath.Join(t.TempDir(), "credentials.cxf.age")
		_, _, err := executeWithInput(t, []string{"export", "--session", "session", "personal", path}, newExportCommand(f.base), "")
		if values[0] == "" || values[0] != values[1] {
			require.Error(t, err)
			_, err = os.Stat(path)
			require.True(t, os.IsNotExist(err))
			continue
		}
		require.NoError(t, err)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		identity, err := age.NewScryptIdentity(values[0])
		require.NoError(t, err)
		reader, err := age.Decrypt(bytes.NewReader(data), identity)
		require.NoError(t, err)
		_, err = cxf.Decode(reader)
		require.NoError(t, err)
	}
}

func TestExportCommandPlaintextRequiresConsent(t *testing.T) {
	for _, answer := range []string{"n\n", "y\n"} {
		f := newCommandFixture(t)
		path := filepath.Join(t.TempDir(), "credentials.cxf")
		_, warning, err := executeWithInput(t, []string{"export", "--plaintext", "--session", "session", "personal", path}, newExportCommand(f.base), answer)
		if answer == "n\n" {
			require.ErrorIs(t, err, errCanceled)
			_, err = os.Stat(path)
			require.True(t, os.IsNotExist(err))
			continue
		}
		require.NoError(t, err)
		require.Contains(t, warning, "credentials in plaintext")
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = cxf.Decode(bytes.NewReader(data))
		require.NoError(t, err)
	}
}
