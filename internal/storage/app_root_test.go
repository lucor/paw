// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"path/filepath"
	"testing"

	fynetest "fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/paw"
)

func TestNewFileStorageUsesDesktopHomeOverride(t *testing.T) {
	root := t.TempDir()
	t.Setenv(paw.ENV_HOME, root)

	store, err := NewFileStorage("")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.Equal(t, filepath.Join(root, ".paw"), store.Root())
}

func TestAppStorageRootUsesFyneMobileURI(t *testing.T) {
	app := fynetest.NewApp()
	t.Cleanup(app.Quit)

	root, err := appStorageRoot(app, "android")
	require.NoError(t, err)
	require.Equal(t, app.Storage().RootURI().Path(), root)
}

func TestAppStorageRootRejectsMissingMobileApp(t *testing.T) {
	_, err := appStorageRoot(nil, "android")
	require.EqualError(t, err, "mobile application storage is unavailable")
}
