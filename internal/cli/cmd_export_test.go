// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
