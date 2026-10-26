// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
)

// TestSelectVaultKeepsIDAndDisplayName checks selection uses the ID and shows the name.
func TestSelectVaultKeepsIDAndDisplayName(t *testing.T) {
	appDriver := test.NewApp()
	t.Cleanup(appDriver.Quit)
	info := storage.VaultInfo{ID: "cxf-vault-id", Name: "Personal"}
	a := &app{
		main:          container.NewVScroll(nil),
		win:           appDriver.NewWindow("Paw"),
		unlockedVault: make(map[string]*model.Vault),
	}

	view := a.makeSelectVaultView([]storage.VaultInfo{info}).(*fyne.Container)
	vaultList := view.Objects[0].(*fyne.Container)
	button := vaultList.Objects[2].(*widget.Button)
	require.Equal(t, info.Name, button.Text)
	test.Tap(button)

	unlocker, ok := a.main.Content.(*unlockerVaultWidget)
	require.True(t, ok)
	require.Equal(t, info, unlocker.vaultInfo)
}

// TestUnlockShowsMigrationFailure checks a safe item report is shown after storage migration fails.
func TestUnlockShowsMigrationFailure(t *testing.T) {
	appDriver := test.NewApp()
	t.Cleanup(appDriver.Quit)
	window := appDriver.NewWindow("Paw")
	t.Cleanup(window.Close)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	const secret = "SYNTHETIC_MIGRATION_SECRET"
	report := &storage.MigrationReport{
		TotalItems: 1,
		Failures: []storage.MigrationItemFailure{{
			LegacyID: "legacy-item-id",
			Title:    "Unsupported password",
			ItemType: paw.PasswordItemType,
			Phases:   []storage.MigrationPhase{storage.MigrationPhaseConvert},
			Err:      errors.New("unsupported mode 4"),
		}},
	}
	store := &storage.StorageMock{}
	keyLoading := make(chan struct{})
	continueLoading := make(chan struct{})
	store.OnLoadVaultKey = func(vaultID, password string) (*paw.Key, error) {
		close(keyLoading)
		<-continueLoading
		return key, nil
	}
	store.OnLoadVault = func(vaultID string, loadedKey *paw.Key) (*model.Vault, error) {
		return nil, report
	}
	a := &app{
		main:          container.NewVScroll(nil),
		win:           window,
		storage:       store,
		unlockedVault: make(map[string]*model.Vault),
	}
	unlocker := NewUnlockerVaultWidget(storage.VaultInfo{ID: "legacy-vault-id", Name: "Personal", NeedsMigration: true}, a)
	unlocker.password.SetText("vault password")

	unlocker.unlock()
	select {
	case <-keyLoading:
	case <-time.After(time.Second):
		t.Fatal("vault loading did not start")
	}

	progressMessage := dialogText(window.Canvas().Overlays().Top())
	require.Contains(t, progressMessage, "Migrating vault...")
	close(continueLoading)

	var message string
	require.Eventually(t, func() bool {
		message = dialogText(window.Canvas().Overlays().Top())
		return strings.Contains(message, "Migration failed: 1/1 items")
	}, time.Second, 10*time.Millisecond)
	require.Contains(t, message, "Migration failed: 1/1 items")
	require.Contains(t, message, "Unsupported password")
	require.Contains(t, message, "unsupported mode 4")
	require.NotContains(t, message, secret)
	require.Nil(t, a.vault)
}

// dialogText returns visible text from a Fyne dialog overlay.
func dialogText(overlay fyne.CanvasObject) string {
	var text []string
	for _, object := range test.LaidOutObjects(overlay) {
		switch value := object.(type) {
		case *widget.Label:
			text = append(text, value.Text)
		case *widget.RichText:
			for _, segment := range value.Segments {
				if segment, ok := segment.(*widget.TextSegment); ok {
					text = append(text, segment.Text)
				}
			}
		}
	}
	return strings.Join(text, "\n")
}
