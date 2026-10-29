// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/godbus/dbus/v5"
	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/agent"
	"go.lucor.dev/paw/internal/icon"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
	"golang.org/x/crypto/ssh"
)

const (
	// AppID represents the application ID
	AppID = "dev.lucor.paw"
	// AppTitle represents the application title
	AppTitle = "Paw"
)

type app struct {
	win     fyne.Window
	main    *container.Scroll
	state   *paw.AppState
	storage storage.Storage

	unlockedVault map[string]*model.Vault

	vault   *model.Vault
	vaultID string

	filter       map[string]*paw.VaultFilterOptions
	audit        *PasswordAudit
	detailCancel context.CancelFunc
	codesCancel  context.CancelFunc
	codesView    *codesView
	sshEditor    *SSHKeyEditor

	// Paw agent client
	client agent.PawAgent
}

// MakeApp builds Paw's main Fyne interface with the supplied storage and window.
func MakeApp(s storage.Storage, w fyne.Window) fyne.CanvasObject {
	appState, err := s.LoadAppState()
	if err != nil {
		dialog.NewError(err, w)
	}

	a := &app{
		state:         appState,
		filter:        make(map[string]*paw.VaultFilterOptions),
		storage:       s,
		unlockedVault: make(map[string]*model.Vault),
		win:           w,
	}

	a.win.SetMainMenu(a.makeMainMenu())

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

	switch len(vaults) {
	case 0:
		showSection := func(title string, resource fyne.Resource) {
			a.win.SetContent(NewPlaceholder(title, resource, func() {
				a.win.SetContent(a.main)
			}))
		}
		o = NewWelcome(pawLogo(), WelcomeActions{
			CreateVault: a.showCreateVaultView,
			Restore:     a.showRestoreVaultView,
			Sync:        func() { showSection("Connect via Sync", icons.Sync) },
		})
	case 1:
		o = a.makeUnlockVaultView(vaults[0], nil)
	default:
		o = a.makeSelectVaultView(vaults)
	}
	return container.NewVScroll(o)
}

// setVaultViewByInfo selects the unlocked vault or displays its unlock screen.
func (a *app) setVaultViewByInfo(info storage.VaultInfo, back func()) {
	a.closeItemDetail()
	a.closePasswordAudit()
	a.closeCodesView()
	vault, ok := a.unlockedVault[info.ID]
	if !ok {
		a.vault = nil
		a.main.Content = a.makeUnlockVaultView(info, back)
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
	if item.Metadata.Type != paw.SSHKeyItemType || !item.Metadata.AddToAgent {
		return nil
	}
	credential, err := uiSSHCredential(item)
	if err != nil {
		return err
	}
	der, err := base64.RawURLEncoding.DecodeString(credential.PrivateKey)
	if err != nil {
		return fmt.Errorf("unable to decode SSH private key")
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return fmt.Errorf("unable to parse SSH private key")
	}
	if c := a.agentClient(); c != nil {
		return c.AddSSHKey(key, credential.KeyComment)
	}
	return nil
}

// removeSSHKeyFromAgent removes an SSH item's public key from the agent.
func (a *app) removeSSHKeyFromAgent(item *model.Item) error {
	if item.Metadata.Type != paw.SSHKeyItemType {
		return nil
	}
	credential, err := uiSSHCredential(item)
	if err != nil {
		return err
	}
	der, err := base64.RawURLEncoding.DecodeString(credential.PrivateKey)
	if err != nil {
		return fmt.Errorf("unable to decode SSH private key")
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return fmt.Errorf("unable to parse SSH private key")
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return fmt.Errorf("unsupported SSH private key")
	}
	if c := a.agentClient(); c != nil {
		return c.RemoveSSHKey(signer.PublicKey())
	}
	return nil
}

// uiSSHCredential reads the SSH key credential from an item.
func uiSSHCredential(item *model.Item) (cxf.SSHKey, error) {
	for _, raw := range item.Credentials {
		typeName, err := cxf.CredentialTypeOf(raw)
		if err != nil {
			return cxf.SSHKey{}, err
		}
		if typeName == cxf.CredentialTypeSSHKey {
			var credential cxf.SSHKey
			if err := json.Unmarshal(raw, &credential); err != nil {
				return cxf.SSHKey{}, err
			}
			return credential, nil
		}
	}
	return cxf.SSHKey{}, fmt.Errorf("SSH key credential is missing")
}

// addSSHKeysToAgent adds configured SSH items from the current vault.
func (a *app) addSSHKeysToAgent() {
	summaries, err := a.vault.FilterItems("", paw.SSHKeyItemType)
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

// showPreferencesView opens the preferences screen.
func (a *app) showPreferencesView() {
	a.closeItemDetail()
	a.win.SetContent(a.makePreferencesView())
}

// lockVault removes the current vault from the unlocked views.
func (a *app) lockVault() {
	a.closeItemDetail()
	a.closePasswordAudit()
	a.closeCodesView()
	delete(a.unlockedVault, a.vaultID)
	a.vault = nil
	a.vaultID = ""
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
	if a.sshEditor != nil {
		a.sshEditor.ClearSensitive()
		a.sshEditor = nil
	}
	if a.detailCancel != nil {
		a.detailCancel()
		a.detailCancel = nil
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

// logo returns the Paw logo as a canvas image with the specified dimensions
func pawLogo() *canvas.Image {
	return imageFromResource(icon.PawIcon)
}

// imageFromResource creates a canvas image from a Fyne resource.
func imageFromResource(resource fyne.Resource) *canvas.Image {
	img := canvas.NewImageFromResource(resource)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(64, 64))
	return img
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
