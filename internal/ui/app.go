package ui

import (
	"errors"
	"fmt"
	"log"
	"runtime"

	"filippo.io/age"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
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
	s, err = paw.NewFyneStorage(fyne.CurrentApp().Storage())
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
	return a.mainView()
}

func (a *app) mainView() fyne.CanvasObject {
	vaults, err := a.storage.Vaults()
	if err != nil {
		log.Fatal(err)
	}
	switch len(vaults) {
	case 0:
		return a.makeInitVaultView()
	case 1:
		return a.makeUnlockVaultView(vaults[0])
	}
	return a.makeSelectVaultView()
}

func (a *app) lockVault() {
	delete(a.unlockedVault, a.vault.Name)
	a.vault = nil
	a.win.SetContent(a.mainView())
}

const (
	tabHomeIndex = iota
	tabAddIndex
)

func (a *app) setContent(c fyne.CanvasObject) {
	a.win.SetContent(c)
}

func (a *app) setVaultView(vault *paw.Vault) {
	a.vault = vault

	at := container.NewAppTabs(
		container.NewTabItemWithIcon("Home", icon.PawIcon, a.makeVaultView(vault)),
		container.NewTabItemWithIcon("Add", theme.ContentAddIcon(), a.makeAddItemView()),
	)
	at.SetTabLocation(container.TabLocationBottom)

	a.appTabs = at
	a.setContent(at)
}

func (a *app) makeInitVaultView() fyne.CanvasObject {
	logo := pawLogo()

	heading := headingText("Welcome to Paw")
	heading.Alignment = fyne.TextAlignCenter

	name := widget.NewEntry()
	name.SetPlaceHolder("Name")

	password := widget.NewPasswordEntry()
	password.SetPlaceHolder("Password")

	btn := widget.NewButton("Create Vault", func() {
		key, err := a.storage.CreateVaultKey(name.Text, password.Text)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		vault, err := a.storage.CreateVault(name.Text, key)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		a.unlockedVault[name.Text] = vault

		a.setVaultView(vault)
	})
	btn.Importance = widget.HighImportance

	return container.NewCenter(container.NewVBox(logo, heading, name, password, btn))

}

func (a *app) makeUnlockVaultView(vaultName string) fyne.CanvasObject {
	logo := pawLogo()

	msg := fmt.Sprintf("Vault %q is locked", vaultName)
	heading := headingText(msg)

	password := widget.NewPasswordEntry()
	password.SetPlaceHolder("Password")

	unlockBtn := widget.NewButtonWithIcon("Unlock", icon.LockOpenOutlinedIconThemed, func() {
		vault, err := a.storage.LoadVault(vaultName, password.Text)
		if err != nil {
			var invalidPasswordError *age.NoIdentityMatchError
			if errors.As(err, &invalidPasswordError) {
				err = errors.New("the password is incorrect")
			}
			dialog.ShowError(err, a.win)
			return
		}
		a.unlockedVault[vaultName] = vault
		a.setVaultView(vault)
	})

	return container.NewCenter(container.NewVBox(logo, heading, password, unlockBtn))
}

func (a *app) makeSelectVaultView() fyne.CanvasObject {

	heading := headingText("Select a Vault")
	heading.Alignment = fyne.TextAlignCenter

	logo := pawLogo()

	c := container.NewVBox(logo, heading)

	vaults, err := a.storage.Vaults()
	if err != nil {
		log.Fatal(err)
	}
	for _, v := range vaults {
		name := v
		resource := icon.LockOpenOutlinedIconThemed
		if _, ok := a.unlockedVault[name]; !ok {
			resource = icon.LockOutlinedIconThemed
		}
		btn := widget.NewButtonWithIcon(name, resource, func() {
			// TODO show appTabs and select first tab
			vault, ok := a.unlockedVault[name]
			if !ok {
				a.setContent(a.makeUnlockVaultView(name))
				return
			}
			a.setVaultView(vault)
			return
		})
		btn.Alignment = widget.ButtonAlignLeading
		c.Add(btn)
	}

	return container.NewCenter(c)
}

func (a *app) makeNavigationHeader(title string, parentView int) fyne.CanvasObject {
	var left, right fyne.CanvasObject
	if fyne.CurrentDevice().IsMobile() {
		right = widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
			a.appTabs.SelectIndex(parentView)
			a.setContent(a.appTabs)
		})
	} else {
		left = widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
			a.appTabs.SelectIndex(parentView)
			a.setContent(a.appTabs)
		})
	}
	return container.NewBorder(nil, nil, left, right, widget.NewLabel(title))
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
