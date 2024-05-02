// Copyright 2022 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ui

import (
	"fmt"
	"log"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lucor.dev/paw/internal/agent"
	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/paw"
	"lucor.dev/paw/internal/sshkey"
)

const (
	// AppID represents the application ID
	AppID = "dev.lucor.paw"
	// AppTitle represents the application title
	AppTitle = "Paw"
)

// maxWorkers represents the max number of workers to use in parallel processing
var maxWorkers = runtime.NumCPU()

type app struct {
	win     fyne.Window
	main    *container.Scroll
	state   *paw.AppState
	storage paw.Storage

	unlockedVault map[string]*paw.Vault // this act as cache

	vault *paw.Vault

	filter map[string]*paw.VaultFilterOptions

	// Paw agent client
	client agent.PawAgent
}

func MakeApp(s paw.Storage, w fyne.Window) fyne.CanvasObject {
	appState, err := s.LoadAppState()
	if err != nil {
		dialog.NewError(err, w)
	}

	a := &app{
		state:         appState,
		filter:        make(map[string]*paw.VaultFilterOptions),
		storage:       s,
		unlockedVault: make(map[string]*paw.Vault),
		win:           w,
	}

	a.win.SetMainMenu(a.makeMainMenu())

	a.main = a.makeApp()
	a.makeSysTray()

	return a.main
}

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

func (a *app) makeSysTray() {
	if desk, ok := fyne.CurrentApp().(desktop.App); ok {
		a.win.SetCloseIntercept(a.win.Hide) // don't close the window if system tray used
		menu := fyne.NewMenu("Vaults", a.makeVaultMenuItems()...)
		desk.SetSystemTrayMenu(menu)
		desk.SetSystemTrayIcon(icon.PawSystray)
	}
}

func (a *app) makeApp() *container.Scroll {
	vaults, err := a.storage.Vaults()
	if err != nil {
		log.Fatal(err)
	}

	var o fyne.CanvasObject

	switch len(vaults) {
	case 0:
		o = a.makeCreateVaultView()
	case 1:
		o = a.makeUnlockVaultView(vaults[0])
	default:
		o = a.makeSelectVaultView(vaults)
	}
	return container.NewVScroll(o)
}

func (a *app) setVaultViewByName(name string) {
	vault, ok := a.unlockedVault[name]
	if !ok {
		a.vault = nil
		a.main.Content = a.makeUnlockVaultView(name)
		a.main.Refresh()
		a.setWindowTitle()
		a.showCurrentVaultView()
		return
	}
	a.setVaultView(vault)
	a.showCurrentVaultView()
}

func (a *app) addSSHKeyToAgent(item paw.Item) error {
	if item.GetMetadata().Type != paw.SSHKeyItemType {
		return nil
	}
	v := item.(*paw.SSHKey)
	if !v.AddToAgent {
		return nil
	}
	k, err := sshkey.ParseKey([]byte(v.PrivateKey))
	if err != nil {
		return fmt.Errorf("unable to parse SSH raw key: %w", err)
	}
	if c := a.agentClient(); c != nil {
		return c.AddSSHKey(k.PrivateKey(), v.Comment)
	}
	return nil
}

func (a *app) removeSSHKeyFromAgent(item paw.Item) error {
	if item.GetMetadata().Type != paw.SSHKeyItemType {
		return nil
	}
	v := item.(*paw.SSHKey)
	k, err := sshkey.ParseKey([]byte(v.PrivateKey))
	if err != nil {
		return fmt.Errorf("unable to parse SSH raw key: %w", err)
	}
	if c := a.agentClient(); c != nil {
		return c.RemoveSSHKey(k.PublicKey())
	}
	return nil
}

func (a *app) addSSHKeysToAgent() {
	a.vault.Range(func(id string, meta *paw.Metadata) bool {
		item, err := a.storage.LoadItem(a.vault, meta)
		if err != nil {
			return false
		}
		err = a.addSSHKeyToAgent(item)
		if err != nil {
			log.Println("unable to add SSH Key to agent:", err)
		}
		return true
	})
}

func (a *app) setVaultView(vault *paw.Vault) {
	a.vault = vault
	a.unlockedVault[vault.Name] = vault
	a.main.Content = a.makeCurrentVaultView()
	a.main.Refresh()
	a.setWindowTitle()
}

func (a *app) showAuditPasswordView() {
	a.win.SetContent(a.makeAuditPasswordView())
}

func (a *app) showCreateVaultView() {
	a.win.SetContent(a.makeCreateVaultView())
}

func (a *app) showCurrentVaultView() {
	a.win.SetContent(a.main)
}

func (a *app) setWindowTitle() {
	title := "Paw"
	if a.vault != nil {
		title = a.vault.Name + " - " + title
	}
	a.win.SetTitle(title)
}

func (a *app) showAddItemView() {
	a.win.SetContent(a.makeAddItemView())
}

func (a *app) showItemView(fyneItemWidget FyneItemWidget) {
	a.win.SetContent(a.makeShowItemView(fyneItemWidget))
}

func (a *app) showEditItemView(fyneItemWidget FyneItemWidget) {
	a.win.SetContent(a.makeEditItemView(fyneItemWidget))
}

func (a *app) showPreferencesView() {
	a.win.SetContent(a.makePreferencesView())
}

func (a *app) lockVault() {
	delete(a.unlockedVault, a.vault.Name)
	a.vault = nil
}

func (a *app) refreshCurrentView() {
	a.main.Content = a.makeCurrentVaultView()
	a.main.Refresh()
}

func (a *app) makeCancelHeaderButton() fyne.CanvasObject {
	var left, right fyne.CanvasObject
	if fyne.CurrentDevice().IsMobile() {
		right = widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
			a.showCurrentVaultView()
		})
	} else {
		left = widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
			a.showCurrentVaultView()
		})
	}
	return container.NewBorder(nil, nil, left, right, widget.NewLabel(""))
}

// headingText returns a text formatted as heading
func headingText(text string) *canvas.Text {
	t := canvas.NewText(text, theme.ForegroundColor())
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.TextSize = theme.TextSubHeadingSize()
	return t
}

// logo returns the Paw logo as a canvas image with the specified dimensions
func pawLogo() *canvas.Image {
	return imageFromResource(icon.PawIcon)
}

func imageFromResource(resource fyne.Resource) *canvas.Image {
	img := canvas.NewImageFromResource(resource)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(64, 64))
	return img
}
