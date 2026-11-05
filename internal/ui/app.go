// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package ui implements Paw's Fyne user interface.
package ui

import (
	"context"
	"fmt"
	"log"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/godbus/dbus/v5"

	"go.lucor.dev/paw/internal/agent"
	"go.lucor.dev/paw/internal/icon"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

const (
	// AppID represents the application ID
	AppID = "dev.lucor.paw"
	// AppTitle represents the application title
	AppTitle = "Paw"
	// DefaultWindowWidth is the default desktop window width in logical Fyne units.
	DefaultWindowWidth = 400
	// DefaultWindowHeight is the default desktop window height in logical Fyne units.
	DefaultWindowHeight = 640
)

type app struct {
	win     fyne.Window
	main    *container.Scroll
	state   *model.AppState
	storage *storage.FileStorage

	unlockedVault map[string]*model.Vault

	vault           *model.Vault
	vaultID         string
	vaultSection    int
	settingsVaultID string
	passwordChange  *masterPasswordView
	deletingVault   bool

	filter       map[string]*vaultFilterOptions
	audit        *PasswordAudit
	detailCancel context.CancelFunc
	codesCancel  context.CancelFunc
	codesView    *codesView
	sshEditor    *SSHKeyEditor
	autoLock     *backgroundAutoLock
	detailClear  func()

	// Paw agent client
	client agent.PawAgent
}

// MakeApp builds Paw's main Fyne interface with the supplied storage and window.
func MakeApp(s *storage.FileStorage, w fyne.Window) fyne.CanvasObject {
	appState, err := s.LoadAppState()
	if err != nil {
		dialog.NewError(err, w)
	}

	a := &app{
		state:         appState,
		filter:        make(map[string]*vaultFilterOptions),
		storage:       s,
		unlockedVault: make(map[string]*model.Vault),
		win:           w,
	}
	if a.state == nil {
		a.state = &model.AppState{Settings: model.NewDefaultSettings()}
	}
	if a.state.Settings == nil {
		a.state.Settings = model.NewDefaultSettings()
	}
	a.autoLock = newBackgroundAutoLock(a.lockAllVaults)
	a.configureAutoLock(a.state.Settings.AutoLock)
	fyne.CurrentApp().Lifecycle().SetOnExitedForeground(func() { fyne.Do(a.autoLock.exitedForeground) })
	fyne.CurrentApp().Lifecycle().SetOnEnteredForeground(func() { fyne.Do(a.autoLock.enteredForeground) })
	fyne.CurrentApp().Lifecycle().SetOnStopped(func() { fyne.Do(a.autoLock.close) })

	a.win.SetMainMenu(a.makeMainMenu())
	if a.state != nil && a.state.Settings != nil {
		installPawTheme(a.state.Settings.Theme)
	}

	a.main = a.makeApp()
	a.makeSysTray()

	return a.main
}

// agentClient returns the configured agent client when it is available.
func (a *app) agentClient() agent.PawAgent {
	if a.client != nil {
		return a.client
	}
	c, err := agent.NewClient(a.storage.SocketAgentPath())
	if err != nil {
		log.Println("agent not available: %w", err)
		return nil
	}
	return c
}

// makeSysTray configures the system tray when the desktop supports it.
func (a *app) makeSysTray() {
	if desk, ok := fyne.CurrentApp().(desktop.App); ok {
		if err := checkStatusNotifierWatcher(); err != nil {
			log.Println("systray not available: %w", err)
			return
		}
		a.win.SetCloseIntercept(a.win.Hide) // don't close the window if system tray used
		menu := fyne.NewMenu("Vaults", a.makeVaultMenuItems()...)
		desk.SetSystemTrayMenu(menu)
		desk.SetSystemTrayIcon(icon.PawSystray)
	}
}

// makeApp selects the initial vault screen.
func (a *app) makeApp() *container.Scroll {
	vaults, err := a.storage.Vaults()
	if err != nil {
		log.Fatal(err)
	}

	var o fyne.CanvasObject
	var chooseAnother func()
	if len(vaults) >= 2 {
		chooseAnother = func() { a.showVaultSelector(vaults) }
	}

	switch len(vaults) {
	case 0:
		o = NewAddVault(nil, a.showCreateVaultView, a.showRestoreVaultView)
	case 1:
		o = a.makeUnlockVaultView(vaults[0], nil, nil)
	default:
		if a.state != nil && a.state.Settings != nil && a.state.Settings.StartupVault == model.StartupVaultLastUsed && a.state.Settings.LastVaultID != "" {
			if info, err := model.FindVaultInfo(vaults, a.state.Settings.LastVaultID); err == nil {
				o = a.makeUnlockVaultView(info, nil, chooseAnother)
				break
			}
		}
		o = a.makeSelectVaultView(vaults)
	}
	return container.NewVScroll(o)
}

// setVaultViewByInfo selects the unlocked vault or displays its unlock screen.
func (a *app) setVaultViewByInfo(info model.VaultInfo, back, chooseAnother func()) {
	if a.settingsVaultID != "" {
		return
	}
	a.closeItemDetail()
	a.closePasswordAudit()
	a.closeCodesView()
	vault, ok := a.unlockedVault[info.ID]
	if !ok {
		a.vault = nil
		a.main.Content = a.makeUnlockVaultView(info, back, chooseAnother)
		a.main.Refresh()
		a.setWindowTitle()
		a.showCurrentVaultView()
		return
	}
	a.setVaultView(vault, info.ID)
	a.showCurrentVaultView()
}

// addSSHKeyToAgent adds a configured SSH item to the agent.
func (a *app) addSSHKeyToAgent(item *model.Item) error {
	if item.Metadata.Type != model.SSHKeyItemType || !item.Metadata.AddToAgent {
		return nil
	}
	credential, err := item.SSHKey()
	if err != nil {
		return err
	}
	key, _, err := model.ParseSSHKey(credential)
	if err != nil {
		return err
	}
	if c := a.agentClient(); c != nil {
		return c.AddSSHKey(key, credential.KeyComment)
	}
	return nil
}

// removeSSHKeyFromAgent removes an SSH item's public key from the agent.
func (a *app) removeSSHKeyFromAgent(item *model.Item) error {
	if item.Metadata.Type != model.SSHKeyItemType {
		return nil
	}
	credential, err := item.SSHKey()
	if err != nil {
		return err
	}
	_, publicKey, err := model.ParseSSHKey(credential)
	if err != nil {
		return err
	}
	if c := a.agentClient(); c != nil {
		return c.RemoveSSHKey(publicKey)
	}
	return nil
}

// addSSHKeysToAgent adds configured SSH items from the current vault.
func (a *app) addSSHKeysToAgent() {
	summaries, err := a.vault.FilterItems("", model.SSHKeyItemType)
	if err != nil {
		log.Println(err)
		return
	}
	for _, summary := range summaries {
		item, err := a.storage.LoadItem(a.vault, summary.ID)
		if err != nil {
			log.Println(err)
			continue
		}
		err = a.addSSHKeyToAgent(item)
		if err != nil {
			log.Println("unable to add SSH Key to agent:", err)
		}
	}
}

// setVaultView selects the vault and remembers it as unlocked.
func (a *app) setVaultView(vault *model.Vault, vaultID string) {
	a.closeItemDetail()
	a.closePasswordAudit()
	a.vault = vault
	a.vaultID = vaultID
	a.unlockedVault[vaultID] = vault
	if a.state != nil && a.state.Settings != nil && a.state.Settings.LastVaultID != vaultID {
		a.state.Settings.LastVaultID = vaultID
		if err := a.storage.StoreAppState(a.state); err != nil {
			log.Println("unable to save last used vault:", err)
		}
	}
	a.main.Content = a.makeCurrentVaultView()
	a.main.Refresh()
	a.setWindowTitle()
}

// showAuditPasswordView opens the password audit screen.
func (a *app) showAuditPasswordView() {
	a.closeItemDetail()
	if a.vault == nil {
		return
	}
	a.closePasswordAudit()
	a.win.SetContent(a.makeAuditPasswordView())
}

// showCurrentVaultView displays the selected vault or its unlock screen.
func (a *app) showCurrentVaultView() {
	a.closeMasterPasswordView()
	a.closeItemDetail()
	a.closePasswordAudit()
	a.win.SetContent(a.main)
}

// setWindowTitle updates the title to match the current vault state.
func (a *app) setWindowTitle() {
	title := "Paw"
	if a.vault != nil {
		name, _ := a.vault.Name()
		title = name + " - " + title
	}
	a.win.SetTitle(title)
}

// showItemView displays the selected item's details.
func (a *app) showItemView(item *model.Item) {
	a.closeItemDetail()
	a.win.SetContent(a.makeShowItemView(item))
}

// showEditItemView opens the selected item's editor.
func (a *app) showEditItemView(item *model.Item, isNew bool) {
	a.closeItemDetail()
	a.win.SetContent(a.makeEditItemView(item, isNew))
	if isNew {
		a.win.Canvas().FocusNext()
	}
}

// showSettingsView opens the application settings.
func (a *app) showSettingsView() {
	a.clearUnlockPassword()
	a.closeItemDetail()
	a.win.SetContent(a.makeSettingsView())
}

func (a *app) clearUnlockPassword() {
	if a.main == nil {
		return
	}
	if unlock, ok := a.main.Content.(*UnlockVault); ok {
		unlock.ClearPassword()
	}
}

// lockVault removes the current vault from the unlocked views.
func (a *app) lockVault() {
	a.closeMasterPasswordView()
	a.settingsVaultID = ""
	a.closeItemDetail()
	a.closePasswordAudit()
	a.closeCodesView()
	delete(a.unlockedVault, a.vaultID)
	a.vault = nil
	a.vaultID = ""
	a.win.SetContent(a.main)
}

func (a *app) configureAutoLock(key string) {
	if a.autoLock != nil {
		a.autoLock.configure(key)
	}
}

func (a *app) lockAllVaults() {
	a.clearUnlockPassword()
	log.Printf("auto-lock: locking %d unlocked vaults", len(a.unlockedVault))
	a.closeMasterPasswordView()
	a.settingsVaultID = ""
	a.closeItemDetail()
	a.closePasswordAudit()
	a.closeCodesView()
	a.unlockedVault = make(map[string]*model.Vault)
	a.vault = nil
	a.vaultID = ""
	a.main.Content = a.makeApp().Content
	a.main.Refresh()
	a.win.SetContent(a.main)
}

// closeCodesView stops countdowns and clears transient TOTP values.
func (a *app) closeCodesView() {
	if a.codesCancel != nil {
		a.codesCancel()
		a.codesCancel = nil
	}
	if a.codesView != nil {
		a.codesView.Clear()
		a.codesView = nil
	}
}

// closeItemDetail stops updates when leaving an entry or locking its vault.
func (a *app) closeItemDetail() {
	if a.detailCancel != nil {
		a.detailCancel()
		a.detailCancel = nil
	}
	if a.detailClear != nil {
		a.detailClear()
		a.detailClear = nil
	}
	if a.sshEditor != nil {
		a.sshEditor.ClearSensitive()
		a.sshEditor = nil
	}
}

// closePasswordAudit discards the active vault's transient audit session.
func (a *app) closePasswordAudit() {
	if a.audit != nil {
		a.audit.Close()
		a.audit = nil
	}
}

// refreshCurrentView redraws the current vault screen.
func (a *app) refreshCurrentView() {
	a.main.Content = a.makeCurrentVaultView()
	a.main.Refresh()
}

// checkStatusNotifierWatcher checks if the StatusNotifierWatcher is available on the supported unix system
func checkStatusNotifierWatcher() error {
	// systrayUnixSupportedOSes list the supported unix system by https://github.com/fyne-io/systray
	// see: https://github.com/fyne-io/systray/blob/master/systray_unix.go#L1
	systrayUnixSupportedOSes := map[string]bool{
		"linux":   true,
		"freebsd": true,
		"openbsd": true,
		"netbsd":  true,
	}
	if !systrayUnixSupportedOSes[runtime.GOOS] {
		return nil
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("failed to connect to session bus: %w", err)
	}
	defer conn.Close()

	obj := conn.Object("org.kde.StatusNotifierWatcher", "/StatusNotifierWatcher")
	call := obj.Call("org.kde.StatusNotifierWatcher.RegisterStatusNotifierItem", 0, "/StatusNotifierItem")
	return call.Err
}
