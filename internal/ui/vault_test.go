// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/v2/assets/icons"
)

// TestVaultEntryListWiring checks summary selection loads the requested item.
func TestVaultEntryListWiring(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	vault.Accounts[0].Items = []cxf.Item{{ID: "item-id", Title: "Example"}}
	store := &storage.StorageMock{}
	store.OnVaults = func() ([]storage.VaultInfo, error) {
		return []storage.VaultInfo{{ID: "vault-id", Name: "Personal"}}, nil
	}
	loaded := ""
	store.OnLoadItem = func(gotVault *model.Vault, id string) (*model.Item, error) {
		require.Same(t, vault, gotVault)
		loaded = id
		return nil, errors.New("example load failure")
	}
	a := &app{win: window, storage: store, vault: vault, vaultID: "vault-id", unlockedVault: map[string]*model.Vault{"vault-id": vault}, filter: map[string]*paw.VaultFilterOptions{}}
	view := a.makeCurrentVaultView()
	a.main = container.NewVScroll(view)
	window.SetContent(a.main)
	window.Resize(fyne.NewSize(440, 650))
	for _, object := range test.LaidOutObjects(view) {
		if list, ok := object.(*widget.List); ok {
			require.Equal(t, 1, list.Length())
			list.Select(0)
			require.Equal(t, "item-id", loaded)
			require.NotNil(t, window.Canvas().Overlays().Top())
			return
		}
	}
	t.Fatal("entry list was not rendered")
}

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

	view := a.makeSelectVaultView([]storage.VaultInfo{info})
	a.win.SetContent(view)
	a.win.Show()
	for _, object := range test.LaidOutObjects(view) {
		if list, ok := object.(*widget.List); ok {
			list.Select(0)
			break
		}
	}

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

	test.Tap(unlocker.button)
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

// TestSingleVaultUnlockControls checks required input and password visibility controls.
func TestSingleVaultUnlockControls(t *testing.T) {
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	window := driver.NewWindow("Paw")
	t.Cleanup(window.Close)
	store := &storage.StorageMock{}
	store.OnVaults = func() ([]storage.VaultInfo, error) {
		return []storage.VaultInfo{{ID: "personal", Name: "Personal"}}, nil
	}
	a := &app{win: window, storage: store}
	a.main = a.makeApp()
	window.SetContent(a.main)
	window.Resize(fyne.NewSize(440, 650))
	window.Show()
	unlocker := a.main.Content.(*unlockerVaultWidget)
	require.Nil(t, unlocker.back)
	require.Empty(t, unlocker.password.PlaceHolder)
	require.True(t, unlocker.password.Password)
	require.NotNil(t, unlocker.password.ActionItem)
	require.True(t, unlocker.button.Disabled())
	unlocker.password.SetText("password")
	require.False(t, unlocker.button.Disabled())
	unlocker.password.SetText("")
	require.True(t, unlocker.button.Disabled())
	require.Greater(t, unlocker.button.Size().Width, float32(350))
	foundHeading := false
	for _, object := range test.LaidOutObjects(unlocker) {
		if label, ok := object.(*widget.Label); ok {
			require.NotEqual(t, "Master password", label.Text)
		}
		if label, ok := object.(*widget.Label); ok && label.Text == "Unlock Personal" {
			foundHeading = true
			require.Equal(t, fyne.TextAlignCenter, label.Alignment)
		}
		_, checkbox := object.(*widget.Check)
		require.False(t, checkbox, "remembering the last vault belongs in Settings")
		if button, ok := object.(*widget.Button); ok {
			require.NotEqual(t, icons.Back, button.Icon, "single-vault unlock must not show Back")
			require.NotEqual(t, "Add vault", button.Text, "Add vault belongs in the overflow menu")
		}
	}
	require.True(t, foundHeading)
}

// TestUnlockAddVaultOptions checks adding a vault is reachable without unlocking.
func TestUnlockAddVaultOptions(t *testing.T) {
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	window := driver.NewWindow("Paw")
	t.Cleanup(window.Close)
	store := &storage.StorageMock{}
	store.OnVaults = func() ([]storage.VaultInfo, error) {
		return []storage.VaultInfo{{ID: "personal", Name: "Personal"}}, nil
	}
	a := &app{win: window, storage: store}
	a.main = a.makeApp()
	window.SetContent(a.main)
	window.Resize(fyne.NewSize(440, 650))
	window.Show()
	var more *widget.Button
	for _, object := range test.LaidOutObjects(a.main) {
		if button, ok := object.(*widget.Button); ok && button.Icon == icons.More {
			more = button
		}
	}
	require.NotNil(t, more)
	for index, choice := range []string{"Create new vault", "Restore from backup", "Connect via Sync"} {
		test.Tap(more)
		menu := window.Canvas().Overlays().Top()
		require.NotNil(t, menu)
		var item fyne.Tappable
		for _, object := range test.LaidOutObjects(menu) {
			control, ok := object.(fyne.Widget)
			if !ok {
				continue
			}
			for _, child := range test.WidgetRenderer(control).Objects() {
				if text, ok := child.(*canvas.Text); ok && text.Text == "Add vault" {
					item, _ = object.(fyne.Tappable)
				}
			}
		}
		require.NotNil(t, item)
		test.Tap(item)
		require.Nil(t, window.Canvas().Overlays().Top(), "Add vault must open a page, not a dialog")
		var actions []*widget.Button
		for _, object := range test.LaidOutObjects(window.Content()) {
			if button, ok := object.(*widget.Button); ok && button.Text == "" && button.Icon == nil {
				actions = append(actions, button)
			}
		}
		require.Len(t, actions, 3)
		test.Tap(actions[index])
		var back *widget.Button
		foundTitle := false
		for _, object := range test.LaidOutObjects(window.Content()) {
			if label, ok := object.(*widget.Label); ok && label.Text == choice {
				foundTitle = true
			}
			if button, ok := object.(*widget.Button); ok && button.Icon == icons.Back && button.Visible() {
				back = button
			}
		}
		require.True(t, foundTitle)
		require.NotNil(t, back)
		test.Tap(back)
		back = nil
		foundAddTitle := false
		for _, object := range test.LaidOutObjects(window.Content()) {
			if label, ok := object.(*widget.Label); ok && label.Text == "Add vault" {
				foundAddTitle = true
			}
			if button, ok := object.(*widget.Button); ok && button.Icon == icons.Back && button.Visible() {
				back = button
			}
		}
		require.True(t, foundAddTitle)
		require.NotNil(t, back)
		test.Tap(back)
		require.Same(t, a.main, window.Content())
	}
}

// TestSelectedVaultUnlockReturnsToList checks Back preserves the available vaults.
func TestSelectedVaultUnlockReturnsToList(t *testing.T) {
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	window := driver.NewWindow("Paw")
	t.Cleanup(window.Close)
	store := &storage.StorageMock{}
	store.OnVaults = func() ([]storage.VaultInfo, error) {
		return []storage.VaultInfo{{ID: "personal", Name: "Personal"}, {ID: "work", Name: "Work"}}, nil
	}
	a := &app{win: window, storage: store}
	a.main = a.makeApp()
	window.SetContent(a.main)
	window.Show()
	for _, object := range test.LaidOutObjects(a.main) {
		if list, ok := object.(*widget.List); ok {
			list.Select(0)
			break
		}
	}
	unlocker, ok := a.main.Content.(*unlockerVaultWidget)
	require.True(t, ok)
	require.Equal(t, "personal", unlocker.vaultInfo.ID)
	require.NotNil(t, unlocker.back)
	var back *widget.Button
	for _, object := range test.LaidOutObjects(unlocker) {
		if button, ok := object.(*widget.Button); ok && button.Icon == icons.Back && button.Visible() {
			back = button
		}
	}
	require.NotNil(t, back)
	test.Tap(back)
	var names []string
	for _, object := range test.LaidOutObjects(a.main) {
		if label, ok := object.(*widget.Label); ok && (label.Text == "Personal" || label.Text == "Work") {
			names = append(names, label.Text)
		}
	}
	require.ElementsMatch(t, []string{"Personal", "Work"}, names)
}
