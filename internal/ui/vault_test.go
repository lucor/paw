// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"crypto/sha1"
	"errors"
	"strings"
	"sync"
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
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func TestAuditReadsPasswordsFromCXFItems(t *testing.T) {
	test.NewTempApp(t)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	settings := model.PasswordSettings{Mode: paw.CustomPassword}
	login, err := model.NewLogin("Website", "dev@example.com", "password", settings)
	require.NoError(t, err)
	password, err := model.NewPassword("WiFi", "password", settings)
	require.NoError(t, err)
	note, err := model.NewNote("Note", "password")
	require.NoError(t, err)
	items := map[string]*model.Item{}
	for _, item := range []*model.Item{login, password, note} {
		summary, err := item.Summary()
		require.NoError(t, err)
		require.Empty(t, summary.Credentials, "vault summaries do not contain passwords")
		vault.Accounts[0].Items = append(vault.Accounts[0].Items, *summary)
		items[item.ID] = item
	}
	var loaded []string
	store := &storage.StorageMock{}
	store.OnLoadItem = func(got *model.Vault, id string) (*model.Item, error) {
		require.Same(t, vault, got)
		loaded = append(loaded, id)
		return items[id], nil
	}
	checker := &auditBreachClient{}
	result := NewPasswordAuditor(vault, store, checker).Run(context.Background(), nil)
	require.ElementsMatch(t, []string{login.ID, password.ID}, loaded)
	hash := sha1.Sum([]byte("password"))
	require.Equal(t, [][sha1.Size]byte{hash, hash}, checker.values)
	require.Equal(t, 2, result.Passwords)
	require.Equal(t, 2, result.ExposedChecked)
	require.Len(t, result.Findings, 2)
	for _, finding := range result.Findings {
		require.True(t, finding.Exposed)
		require.Equal(t, 0, finding.Score)
		require.Equal(t, 2, finding.ReuseCount)
	}
}

type auditBreachClient struct {
	mu     sync.Mutex
	values [][sha1.Size]byte
}

func (c *auditBreachClient) SearchHash(_ context.Context, value [sha1.Size]byte) (bool, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values = append(c.values, value)
	return true, 1, nil
}

func TestAuditIsDiscardedOnLockAndVaultSwitch(t *testing.T) {
	for _, action := range []string{"lock", "switch"} {
		t.Run(action, func(t *testing.T) {
			test.NewTempApp(t)
			window := test.NewTempWindow(t, nil)
			key, err := paw.MakeOneTimeKey()
			require.NoError(t, err)
			vault, err := model.NewVault(key, "Personal")
			require.NoError(t, err)
			a := &app{win: window, main: container.NewVScroll(widget.NewLabel("Vault")), vault: vault, vaultID: "personal", unlockedVault: map[string]*model.Vault{"personal": vault}}
			detailContext, cancelDetail := context.WithCancel(context.Background())
			a.detailCancel = cancelDetail
			a.audit = NewPasswordAudit("Personal", nil, window.SetContent, func(string) {}, func() {})
			view := a.audit.View
			window.SetContent(view)
			if action == "lock" {
				a.makeVaultActions()[4].Action()
			} else {
				a.setVaultViewByInfo(model.VaultInfo{ID: "work", Name: "Work"}, nil, nil)
			}
			require.Nil(t, a.audit)
			require.ErrorIs(t, detailContext.Err(), context.Canceled)
			require.Empty(t, view.Objects)
			require.Same(t, a.main, window.Content())
			require.Nil(t, a.vault)
		})
	}
}

func TestSupportedEntriesUseNewEntryDetail(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	login, err := model.NewLogin("Login", "user", "secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	note, err := model.NewNote("Note", "Text")
	require.NoError(t, err)
	password, err := model.NewPassword("Password", "secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	key, err := GenerateSSHKey()
	require.NoError(t, err)
	sshItem, err := model.NewSSHKey("SSH key", key, "", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	a := &app{win: window}
	t.Cleanup(a.closeItemDetail)
	for _, item := range []*model.Item{login, note, password, sshItem} {
		view := a.makeShowItemView(item)
		window.SetContent(view)
		found := false
		for _, object := range test.LaidOutObjects(view) {
			if _, nativeButton := object.(*widget.Button); nativeButton {
				continue
			}
			if control, ok := object.(interface{ AccessibilityLabel() string }); ok && control.AccessibilityLabel() == "Entry actions" {
				found = true
			}
		}
		require.True(t, found, "%v should use the v2 entry detail", item.Metadata.Type)
	}
}

func TestLoginEditUsesV2EditorWithoutTypeOrNavigation(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	login, err := model.NewLogin("GitHub", "dev@example.com", "secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	a := &app{win: window}
	t.Cleanup(a.closeItemDetail)
	view := a.makeEditItemView(login, false)
	window.SetContent(view)

	selectFound, navigationFound := false, false
	for _, object := range test.LaidOutObjects(view) {
		_, selectFound = object.(*widget.Select)
		_, navigationFound = object.(*Navigation)
		if selectFound || navigationFound {
			break
		}
	}
	require.False(t, selectFound, "Login editor should not display an entry type control")
	require.False(t, navigationFound, "Login editor should hide bottom navigation")

	a.closeItemDetail()
	newLogin, err := newItemTemplate(model.LoginItemType)
	require.NoError(t, err)
	newView := a.makeEditItemView(newLogin, true)
	for _, object := range test.LaidOutObjects(newView) {
		_, isSelect := object.(*widget.Select)
		require.False(t, isSelect, "New Login should use the v2 editor")
	}
}

func TestSSHKeyEditUsesV2EditorWithoutTypeOrNavigation(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	sshKey, err := GenerateSSHKey()
	require.NoError(t, err)
	item, err := model.NewSSHKey("Production server", sshKey, "passphrase", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	a := &app{win: window, vault: vault}
	t.Cleanup(a.closeItemDetail)
	view := a.makeEditItemView(item, false)
	window.SetContent(view)

	buttons := map[string]bool{}
	headerFound := false
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*widget.Button); ok {
			buttons[button.Text] = true
		}
		if control, ok := object.(interface{ AccessibilityLabel() string }); ok {
			buttons[control.AccessibilityLabel()] = true
		}
		if label, ok := object.(*widget.Label); ok && label.Text == "Edit SSH key" {
			headerFound = true
		}
		_, isNavigation := object.(*Navigation)
		_, isSelect := object.(*widget.Select)
		require.False(t, isNavigation, "SSH editor should hide bottom navigation")
		require.False(t, isSelect, "SSH type is fixed by the item")
	}
	require.True(t, headerFound)
	require.True(t, buttons["Back"])
	require.True(t, buttons["Generate"])
	require.True(t, buttons["Import"])
	require.True(t, buttons["Reveal private key"])
}

func TestSSHKeyPrivateCopyWorksWhileHiddenAndVisible(t *testing.T) {
	appDriver := test.NewApp()
	t.Cleanup(appDriver.Quit)
	window := appDriver.NewWindow("Paw")
	t.Cleanup(window.Close)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	sshKey, err := GenerateSSHKey()
	require.NoError(t, err)
	item, err := model.NewSSHKey("Production server", sshKey, "", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	a := &app{win: window, vault: vault}
	t.Cleanup(a.closeItemDetail)
	window.SetContent(a.makeEditItemView(item, false))
	editor := a.sshEditor
	require.NotNil(t, editor)
	require.False(t, sshPrivateLabelVisible(editor.View))
	test.Tap(editor.CopyPrivateButton)
	require.True(t, strings.HasPrefix(appDriver.Clipboard().Content(), "-----BEGIN OPENSSH PRIVATE KEY-----\n"))
	require.True(t, strings.HasSuffix(appDriver.Clipboard().Content(), "-----END OPENSSH PRIVATE KEY-----\n"))
	test.Tap(editor.ExportPrivateButton)
	exportDialog := window.Canvas().Overlays().Top()
	require.NotNil(t, exportDialog, "export should be available while the private key is hidden")
	window.Canvas().Overlays().Remove(exportDialog)
	test.Tap(editor.RevealPrivateButton)
	require.True(t, sshPrivateLabelVisible(editor.View))
	test.Tap(editor.CopyPrivateButton)
	require.True(t, strings.HasPrefix(appDriver.Clipboard().Content(), "-----BEGIN OPENSSH PRIVATE KEY-----\n"))
	require.True(t, strings.HasSuffix(appDriver.Clipboard().Content(), "-----END OPENSSH PRIVATE KEY-----\n"))
	test.Tap(editor.ExportPrivateButton)
	exportDialog = window.Canvas().Overlays().Top()
	require.NotNil(t, exportDialog, "export should remain available while the private key is visible")
	window.Canvas().Overlays().Remove(exportDialog)
	a.closeItemDetail()
	require.False(t, sshPrivateLabelVisible(editor.View), "closing the editor should remove revealed private text")
	require.Empty(t, editor.Passphrase.Text, "closing the editor should clear the passphrase")
	require.Nil(t, editor.CopyPrivateButton.OnTapped, "closed editor actions should release key references")
}

func TestSSHKeyReplacementRequiresConfirmation(t *testing.T) {
	appDriver := test.NewApp()
	t.Cleanup(appDriver.Quit)
	window := appDriver.NewWindow("Paw")
	t.Cleanup(window.Close)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	sshKey, err := GenerateSSHKey()
	require.NoError(t, err)
	item, err := model.NewSSHKey("Production server", sshKey, "", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	a := &app{win: window, vault: vault}
	t.Cleanup(a.closeItemDetail)
	window.SetContent(a.makeEditItemView(item, false))
	editor := a.sshEditor
	test.Tap(editor.GenerateButton)
	require.True(t, editor.KeyChanged(), "generation should mark the existing key for replacement")
	test.Tap(editor.SaveButton)
	confirmation := window.Canvas().Overlays().Top()
	require.NotNil(t, confirmation)
	var cancel fyne.Tappable
	for _, object := range test.LaidOutObjects(confirmation) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Cancel" {
			cancel = button
		}
	}
	require.NotNil(t, cancel)
	test.Tap(cancel)
	require.Nil(t, window.Canvas().Overlays().Top(), "cancelling should leave the editor open")
}

func sshPrivateLabelVisible(view fyne.CanvasObject) bool {
	for _, object := range test.LaidOutObjects(view) {
		label, ok := object.(*widget.Label)
		if ok && strings.HasPrefix(label.Text, "-----BEGIN OPENSSH PRIVATE KEY-----") {
			return true
		}
	}
	return false
}

func TestLoginEditConfirmsDiscardingUnsavedChanges(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	login, err := model.NewLogin("GitHub", "dev@example.com", "secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	a := &app{win: window}
	t.Cleanup(a.closeItemDetail)
	item := login
	view := a.makeEditItemView(item, false)
	window.SetContent(view)
	var back fyne.Tappable
	for _, object := range test.LaidOutObjects(view) {
		if entry, ok := object.(*widget.Entry); ok && entry.Text == "GitHub" {
			entry.SetText("GitHub work")
		}
		if button, ok := object.(interface{ AccessibilityLabel() string }); ok && button.AccessibilityLabel() == "Back" {
			if tappable, ok := object.(fyne.Tappable); ok {
				back = tappable
			}
		}
	}
	require.NotNil(t, back)
	test.Tap(back)
	confirmation := window.Canvas().Overlays().Top()
	require.NotNil(t, confirmation, "changed fields should ask before leaving")
	var discard fyne.Tappable
	for _, object := range test.LaidOutObjects(confirmation) {
		button, ok := object.(*widget.Button)
		if ok && button.Text == "Discard" {
			discard = button
		}
	}
	require.NotNil(t, discard)
	test.Tap(discard)
	require.Nil(t, window.Canvas().Overlays().Top())
	require.NotSame(t, view, window.Content())
}

func TestPasswordAndNoteUseV2EditorsForCreateAndEdit(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	password, err := model.NewPassword("Wi-Fi", "secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	note, err := model.NewNote("Note", "Private content")
	require.NoError(t, err)
	a := &app{win: window}
	for _, item := range []*model.Item{password, note} {
		for _, isNew := range []bool{false, true} {
			view := a.makeEditItemView(item, isNew)
			window.SetContent(view)
			for _, object := range test.LaidOutObjects(view) {
				_, isNavigation := object.(*Navigation)
				_, isSelect := object.(*widget.Select)
				require.False(t, isNavigation, "edit forms should hide bottom navigation")
				require.False(t, isSelect, "entry type should not be editable")
			}
		}
	}
}

func TestCreatingVaultStopsEntryUpdates(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	a := &app{win: window, detailCancel: cancel}
	a.showCreateVaultView()
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	require.Nil(t, a.detailCancel)
}

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
	store.OnVaults = func() ([]model.VaultInfo, error) {
		return []model.VaultInfo{{ID: "vault-id", Name: "Personal"}}, nil
	}
	loaded := ""
	store.OnLoadItem = func(gotVault *model.Vault, id string) (*model.Item, error) {
		require.Same(t, vault, gotVault)
		loaded = id
		return nil, errors.New("example load failure")
	}
	a := &app{win: window, storage: store, vault: vault, vaultID: "vault-id", unlockedVault: map[string]*model.Vault{"vault-id": vault}, filter: map[string]*vaultFilterOptions{}}
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
	info := model.VaultInfo{ID: "cxf-vault-id", Name: "Personal"}
	a := &app{
		main:          container.NewVScroll(nil),
		win:           appDriver.NewWindow("Paw"),
		unlockedVault: make(map[string]*model.Vault),
	}

	view := a.makeSelectVaultView([]model.VaultInfo{info})
	a.win.SetContent(view)
	a.win.Show()
	for _, object := range test.LaidOutObjects(view) {
		if list, ok := object.(*widget.List); ok {
			list.Select(0)
			break
		}
	}

	unlocker, ok := a.main.Content.(*UnlockVault)
	require.True(t, ok)
	require.Contains(t, unlockViewText(t, unlocker), "Unlock Personal")
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
			ItemType: model.PasswordItemType,
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
	info := model.VaultInfo{ID: "legacy-vault-id", Name: "Personal", NeedsMigration: true}
	opened := false
	unlocker := NewUnlockVault(info, store, func(*model.Vault) { opened = true }, nil, nil, nil)
	window.SetContent(unlocker)
	password, button := unlockControls(t, unlocker)
	password.SetText("vault password")

	test.Tap(button)
	select {
	case <-keyLoading:
	case <-time.After(time.Second):
		t.Fatal("vault loading did not start")
	}

	require.Nil(t, window.Canvas().Overlays().Top())
	require.Contains(t, unlockViewText(t, unlocker), "Migrating vault…")
	close(continueLoading)

	waitForUnlock(t, unlocker)
	message := unlockViewText(t, unlocker)
	require.Contains(t, message, "migration failed: 1/1 items")
	require.Contains(t, message, "Unsupported password")
	require.Contains(t, message, "unsupported mode 4")
	require.NotContains(t, message, secret)
	require.False(t, opened)
}

func unlockControls(t *testing.T, view fyne.CanvasObject) (*widget.Entry, *widget.Button) {
	t.Helper()
	var password *widget.Entry
	var button *widget.Button
	for _, object := range test.LaidOutObjects(view) {
		switch control := object.(type) {
		case *widget.Entry:
			if control.Password {
				password = control
			}
		case *widget.Button:
			if control.Text == "Unlock" {
				button = control
			}
		}
	}
	require.NotNil(t, password)
	require.NotNil(t, button)
	return password, button
}

// TestSingleVaultUnlockControls checks required input and password visibility controls.
func TestSingleVaultUnlockControls(t *testing.T) {
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	window := driver.NewWindow("Paw")
	t.Cleanup(window.Close)
	store := &storage.StorageMock{}
	store.OnVaults = func() ([]model.VaultInfo, error) {
		return []model.VaultInfo{{ID: "personal", Name: "Personal"}}, nil
	}
	a := &app{win: window, storage: store}
	a.main = a.makeApp()
	window.SetContent(a.main)
	window.Resize(fyne.NewSize(440, 650))
	window.Show()
	unlocker, ok := a.main.Content.(*UnlockVault)
	require.True(t, ok)
	password, button := unlockControls(t, unlocker)
	require.Empty(t, password.PlaceHolder)
	require.True(t, password.Password)
	require.NotNil(t, password.ActionItem)
	require.True(t, button.Disabled())
	password.SetText("password")
	require.False(t, button.Disabled())
	password.SetText("")
	require.True(t, button.Disabled())
	require.Greater(t, button.Size().Width, float32(350))
	foundHeading := false
	foundPasswordLabel := false
	foundChooseAnother := false
	for _, object := range test.LaidOutObjects(unlocker) {
		if label, ok := object.(*widget.Label); ok {
			if label.Text == "Master password" {
				foundPasswordLabel = true
			}
		}
		if label, ok := object.(*widget.Label); ok && label.Text == "Unlock Personal" {
			foundHeading = true
			require.Equal(t, fyne.TextAlignCenter, label.Alignment)
		}
		_, checkbox := object.(*widget.Check)
		require.False(t, checkbox, "remembering the last vault belongs in Settings")
		if button, ok := object.(*widget.Button); ok {
			foundChooseAnother = foundChooseAnother || button.Text == "Choose another vault"
			require.NotEqual(t, icons.Back, button.Icon, "single-vault unlock must not show Back")
			require.NotEqual(t, "Add vault", button.Text, "Add vault belongs in the overflow menu")
		}
	}
	require.True(t, foundHeading)
	require.True(t, foundPasswordLabel)
	require.False(t, foundChooseAnother, "the action is hidden when only one vault exists")
}

// TestUnlockAddVaultOptions checks adding a vault is reachable without unlocking.
func TestUnlockAddVaultOptions(t *testing.T) {
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	window := driver.NewWindow("Paw")
	t.Cleanup(window.Close)
	store := &storage.StorageMock{}
	store.OnVaults = func() ([]model.VaultInfo, error) {
		return []model.VaultInfo{{ID: "personal", Name: "Personal"}}, nil
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
	for index, choice := range []string{"Create new vault", "Restore from backup"} {
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
		require.Len(t, actions, 2)
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
	store.OnVaults = func() ([]model.VaultInfo, error) {
		return []model.VaultInfo{{ID: "personal", Name: "Personal"}, {ID: "work", Name: "Work"}}, nil
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
	unlocker, ok := a.main.Content.(*UnlockVault)
	require.True(t, ok)
	require.Contains(t, unlockViewText(t, unlocker), "Unlock Personal")
	var chooseAnother *widget.Button
	for _, object := range test.LaidOutObjects(unlocker) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Choose another vault" {
			chooseAnother = button
			break
		}
	}
	require.NotNil(t, chooseAnother, "unlock from the selector should keep the choose action available")
	unlocker.password.SetText("previous vault password")
	test.Tap(chooseAnother)
	require.Empty(t, unlocker.password.Text, "changing vault clears the previous password")
	selectorCount := 0
	for _, object := range test.LaidOutObjects(a.main) {
		if _, ok := object.(*widget.List); ok {
			selectorCount++
		}
	}
	require.Equal(t, 1, selectorCount, "Choose another vault should replace the unlock view with one selector")
	listFound := false
	for _, object := range test.LaidOutObjects(a.main) {
		if list, ok := object.(*widget.List); ok {
			list.Select(0)
			listFound = true
			break
		}
	}
	require.True(t, listFound)
	unlocker, ok = a.main.Content.(*UnlockVault)
	require.True(t, ok)
	chooseAnother = nil
	for _, object := range test.LaidOutObjects(unlocker) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Choose another vault" {
			chooseAnother = button
			break
		}
	}
	require.NotNil(t, chooseAnother)
	test.Tap(chooseAnother)
	selectorCount = 0
	for _, object := range test.LaidOutObjects(a.main) {
		if _, ok := object.(*widget.List); ok {
			selectorCount++
		}
	}
	require.Equal(t, 1, selectorCount, "repeated choose actions must not stack selectors")
	listFound = false
	for _, object := range test.LaidOutObjects(a.main) {
		if list, ok := object.(*widget.List); ok {
			list.Select(0)
			listFound = true
			break
		}
	}
	require.True(t, listFound)
	unlocker, ok = a.main.Content.(*UnlockVault)
	require.True(t, ok)
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
