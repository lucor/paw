package ui

import (
	"log"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/paw"
)

// maxWorkers represents the max number of workers to use in parallel processing
var maxWorkers = runtime.NumCPU()

type app struct {
	win     fyne.Window
	main    *container.Scroll
	storage paw.Storage

	unlockedVault map[string]*paw.Vault // this act as cache

	vault *paw.Vault

	filter map[string]*paw.VaultFilterOptions

	version string
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

	a := &app{
		win:           w,
		storage:       s,
		unlockedVault: make(map[string]*paw.Vault),
		version:       ver,
		filter:        make(map[string]*paw.VaultFilterOptions),
	}

	a.win.SetMainMenu(a.makeMainMenu())

	a.main = a.makeApp()
	a.makeSysTray()

	return a.main
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
		a.main.Content = a.makeUnlockVaultView(name)
		a.main.Refresh()
		return
	}
	a.setVaultView(vault)
}

func (a *app) setVaultView(vault *paw.Vault) {
	a.vault = vault
	a.unlockedVault[vault.Name] = vault
	a.main.Content = a.makeCurrentVaultView()
	a.main.Refresh()
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

func (a *app) showAddItemView() {
	a.win.SetContent(a.makeAddItemView())
}

func (a *app) showItemView(fyneItem FyneItem) {
	a.win.SetContent(a.makeShowItemView(fyneItem))
}

func (a *app) showEditItemView(fyneItem FyneItem) {
	a.win.SetContent(a.makeEditItemView(fyneItem))
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
