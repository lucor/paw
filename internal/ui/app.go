package ui

import (
	"fmt"
	"log"
	"os"
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

// maxWorkers represents the max number of workers to use in parallel processing
var maxWorkers = runtime.NumCPU()

type app struct {
	win     fyne.Window
	config  *paw.Config
	main    *container.Scroll
	storage paw.Storage

	unlockedVault map[string]*paw.Vault // this act as cache

	vault *paw.Vault

	filter map[string]*paw.VaultFilterOptions

	version string

	// Paw agent client
	client agent.PawAgent
}

func MakeApp(w fyne.Window, ver string) fyne.CanvasObject {
	var s paw.Storage
	var err error

	if fyne.CurrentDevice().IsMobile() {
		s, err = paw.NewFyneStorage(fyne.CurrentApp().Storage())
	} else {
		s, err = paw.NewOSStorage()
	}
	if err != nil {
		log.Fatal(err)
	}

	if ver == "" {
		ver = "(unknown)"
	}

	// check for running instance
	cliAgentRunning := false
	c, err := agent.NewClient(s.SocketAgentPath())
	if err == nil {
		t, _ := c.Type()
		switch t {
		case agent.GUI:
			// a GUI instance is already running, exit
			os.Exit(1)
		case agent.CLI:
			cliAgentRunning = true
		}
	}

	config, err := s.LoadConfig()
	if err != nil {
		dialog.NewError(err, w)
	}

	a := &app{
		win:           w,
		storage:       s,
		config:        config,
		unlockedVault: make(map[string]*paw.Vault),
		version:       ver,
		filter:        make(map[string]*paw.VaultFilterOptions),
	}

	a.win.SetMainMenu(a.makeMainMenu())

	a.main = a.makeApp()
	a.makeSysTray()
	if !cliAgentRunning {
		go agent.Run(agent.NewGUI(), s.SocketAgentPath())
	}

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
		return
	}
	a.setVaultView(vault)
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

func (a *app) addSSHKeysToAgent(vault *paw.Vault) {
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

func (a *app) showItemView(fyneItem FyneItem) {
	a.win.SetContent(a.makeShowItemView(fyneItem))
}

func (a *app) showEditItemView(fyneItem FyneItem) {
	a.win.SetContent(a.makeEditItemView(fyneItem))
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
