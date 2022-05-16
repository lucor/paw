package ui

import (
	"log"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/paw"
)

// maxWorkers represents the max number of workers to use in parallel processing
var maxWorkers = runtime.NumCPU()

type app struct {
	win     fyne.Window
	appTabs *container.AppTabs
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

	a.appTabs = a.makeAppTabs()

	if len(a.appTabs.Items) == 0 {
		return a.makeCreateVaultView()
	}
	return a.appTabs
}

func (a *app) makeAppTabs() *container.AppTabs {
	vaults, err := a.storage.Vaults()
	if err != nil {
		log.Fatal(err)
	}

	// init the application tabs
	at := container.NewAppTabs()
	for _, vaultName := range vaults {
		at.Append(container.NewTabItemWithIcon(vaultName, icon.PawIcon, a.makeUnlockVaultView(vaultName)))
	}
	at.SetTabLocation(container.TabLocationTop)
	at.OnSelected = func(ti *container.TabItem) {
		vaultName := ti.Text
		var vault *paw.Vault
		v, ok := a.unlockedVault[vaultName]
		if ok {
			vault = v
		}
		a.vault = vault
	}
	return at
}

// addVaultView adds a vault view to app tabs and set to default
func (a *app) addVaultView(vault *paw.Vault) {
	a.vault = vault
	a.unlockedVault[vault.Name] = vault

	a.appTabs.Append(container.NewTabItemWithIcon(vault.Name, icon.PawIcon, a.makeCurrentVaultView()))
	index := len(a.appTabs.Items)

	a.appTabs.SelectIndex(index)
}

func (a *app) setCurrentVaultView(vault *paw.Vault) {
	a.vault = vault
	a.unlockedVault[vault.Name] = vault
	a.appTabs.Selected().Content = a.makeCurrentVaultView()
	a.appTabs.Refresh()
}

func (a *app) showAuditPasswordView() {
	a.win.SetContent(a.makeAuditPasswordView())
}

func (a *app) showCreateVaultView() {
	a.win.SetContent(a.makeCreateVaultView())
}

func (a *app) showCurrentVaultView() {
	a.win.SetContent(a.appTabs)
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
	a.appTabs.Selected().Content = a.makeCurrentVaultView()
	a.appTabs.Refresh()
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
