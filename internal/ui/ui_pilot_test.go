// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func TestUIPilotCreateVaultFlow(t *testing.T) {
	outputDir := os.Getenv("PAW_UI_SNAPSHOT_DIR")
	if outputDir == "" {
		outputDir = t.TempDir()
	}
	require.NoError(t, os.MkdirAll(outputDir, 0o755))
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			runUIPilotCreateVaultFlow(t, outputDir, variant)
		})
	}
	t.Logf("UI pilot screenshots: %s", outputDir)
}

func runUIPilotCreateVaultFlow(t *testing.T, outputDir string, variant fyne.ThemeVariant) {
	t.Helper()
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	driver.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
	window := driver.NewWindow("Paw UI pilot")
	t.Cleanup(window.Close)

	var createdVault *model.Vault
	store := &storage.StorageMock{}
	store.OnVaults = func() ([]storage.VaultInfo, error) {
		if createdVault == nil {
			return nil, nil
		}
		return []storage.VaultInfo{{ID: createdVault.Accounts[0].ID, Name: "Personal"}}, nil
	}
	store.OnCreateVault = func(name, password string) (*model.Vault, error) {
		key, err := paw.MakeOneTimeKey()
		if err != nil {
			return nil, err
		}
		createdVault, err = model.NewVault(key, name)
		return createdVault, err
	}

	a := &app{
		win:           window,
		storage:       store,
		state:         &paw.AppState{Settings: paw.NewDefaultSettings()},
		filter:        make(map[string]*paw.VaultFilterOptions),
		unlockedVault: make(map[string]*model.Vault),
	}
	a.main = a.makeApp()
	window.SetContent(a.main)
	window.Resize(fyne.NewSize(440, 650))
	window.Show()
	for _, object := range test.LaidOutObjects(window.Content()) {
		if button, ok := object.(*widget.Button); ok {
			require.NotEqual(t, icons.Back, button.Icon, "first-run Add vault has no Back button")
		}
	}
	require.NotContains(t, unlockViewText(t, window.Content()), "Connect via Sync")
	captureUIPilotScreen(t, window, outputDir, "first-run-add-vault-"+variantName(variant)+".png")

	createVault := findNavigationRowButton(t, window.Content(), "Create new vault")
	test.Tap(createVault)
	captureUIPilotScreen(t, window, outputDir, "create-vault-"+variantName(variant)+".png")

	var entries []*widget.Entry
	var submit *widget.Button
	for _, object := range test.LaidOutObjects(window.Content()) {
		switch control := object.(type) {
		case *widget.Entry:
			entries = append(entries, control)
		case *widget.Button:
			if control.Text == "Create vault" {
				submit = control
			}
		}
	}
	require.Len(t, entries, 3)
	require.NotNil(t, submit)
	entries[0].SetText("Personal")
	entries[1].SetText("example master password")
	entries[2].SetText("example master password")
	require.False(t, submit.Disabled())
	captureUIPilotScreen(t, window, outputDir, "create-vault-filled-"+variantName(variant)+".png")
	test.Tap(submit)
	require.NotNil(t, createdVault)
	require.Same(t, createdVault, a.vault, "successful creation opens the new vault")
	require.NotContains(t, unlockViewText(t, a.main.Content), "Add vault")
	captureUIPilotScreen(t, window, outputDir, "vault-list-"+variantName(variant)+".png")
}

func captureUIPilotScreen(t *testing.T, window fyne.Window, outputDir, name string) {
	t.Helper()
	test.LaidOutObjects(window.Content())
	window.Canvas().Refresh(window.Content())
	if overlay := window.Canvas().Overlays().Top(); overlay != nil {
		test.LaidOutObjects(overlay)
		window.Canvas().Refresh(overlay)
	}
	file, err := os.Create(filepath.Join(outputDir, name))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, file.Close()) })
	require.NoError(t, png.Encode(file, window.Canvas().Capture()))
}
