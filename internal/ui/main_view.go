package ui

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"runtime"
	"runtime/debug"

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

// mainView represents the Paw main view
type mainView struct {
	fyne.Window

	storage *paw.Storage

	unlockedVault map[string]*paw.Vault // this act as cache

	view *fyne.Container
}

// Make returns the fyne user interface
func Make(a fyne.App, w fyne.Window) fyne.CanvasObject {
	s, err := paw.NewStorage(a.Storage())
	if err != nil {
		log.Fatal(err)
	}

	mw := &mainView{
		Window:        w,
		storage:       s,
		unlockedVault: make(map[string]*paw.Vault),
	}

	mw.view = container.NewMax(mw.buildMainView())
	mw.SetMainMenu(mw.makeMainMenu())
	return mw.view
}

func (mw *mainView) setView(v fyne.CanvasObject) {
	mw.view.Objects[0] = v
	mw.view.Refresh()
}

func (mw *mainView) makeMainMenu() *fyne.MainMenu {
	// a Quit item will is appended automatically by Fyne to the first menu item
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("New Vault", func() {
			mw.setView(mw.createVaultView())
		}),
	)
	switchItem := fyne.NewMenuItem("Switch Vault", func() {
		mw.Reload()
	})

	vaults, err := mw.storage.Vaults()
	if err != nil {
		log.Fatal(err)
	}
	if len(vaults) <= 1 {
		switchItem.Disabled = true
	}
	fileMenu.Items = append(fileMenu.Items, switchItem)

	helpMenu := fyne.NewMenu("Help",
		fyne.NewMenuItem("About", func() {
			version := "devel"
			info, ok := debug.ReadBuildInfo()
			if ok {
				version = info.Main.Version
			}

			u, _ := url.Parse("https://lucor.dev/paw")
			l := widget.NewLabel("Paw - " + version)
			l.Alignment = fyne.TextAlignCenter
			link := widget.NewHyperlink("https://lucor.dev/paw", u)
			link.Alignment = fyne.TextAlignCenter
			co := container.NewCenter(
				container.NewVBox(
					pawLogo(),
					l,
					link,
				),
			)
			d := dialog.NewCustom("About Paw", "Ok", co, mw.Window)
			d.Show()
		}),
	)
	return fyne.NewMainMenu(
		fileMenu,

		helpMenu,
	)
}

func (mw *mainView) Reload() {
	mw.setView(mw.buildMainView())
	mw.SetMainMenu(mw.makeMainMenu())
}

func (mw *mainView) buildMainView() fyne.CanvasObject {
	var view fyne.CanvasObject
	vaults, err := mw.storage.Vaults()
	if err != nil {
		log.Fatal(err)
	}
	switch len(vaults) {
	case 0:
		view = mw.initVaultView()
	case 1:
		view = mw.unlockVaultView(vaults[0])
	default:
		view = mw.vaultListView()
	}
	return view
}

// initVaultView returns the view used to create the first vault
func (mw *mainView) initVaultView() fyne.CanvasObject {

	logo := pawLogo()

	heading := headingText("Welcome to Paw")
	heading.Alignment = fyne.TextAlignCenter

	name := widget.NewEntry()
	name.SetPlaceHolder("Name")

	password := widget.NewPasswordEntry()
	password.SetPlaceHolder("Password")

	btn := widget.NewButton("Create Vault", func() {
		vault, err := mw.storage.CreateVault(name.Text, password.Text)
		if err != nil {
			dialog.ShowError(err, mw.Window)
			return
		}
		mw.unlockedVault[name.Text] = vault
		mw.setView(newVaultView(mw, vault))
	})
	btn.Importance = widget.HighImportance

	return container.NewCenter(container.NewVBox(logo, heading, name, password, btn))
}

// initVaultView returns the view used to create the first vault
func (mw *mainView) createVaultView() fyne.CanvasObject {
	heading := headingText("Create a new Vault")
	heading.Alignment = fyne.TextAlignCenter

	logo := pawLogo()

	name := widget.NewEntry()
	name.SetPlaceHolder("Name")

	password := widget.NewPasswordEntry()
	password.SetPlaceHolder("Password")

	createButton := widget.NewButtonWithIcon("Create", theme.ContentAddIcon(), func() {
		// TODO: update to use the built-in entry validation
		if name.Text == "" {
			d := dialog.NewInformation("", "The Vault name cannot be emtpy", mw.Window)
			d.Show()
			return
		}
		if password.Text == "" {
			d := dialog.NewInformation("", "The Vault password cannot be emtpy", mw.Window)
			d.Show()
			return
		}
		vault, err := mw.storage.CreateVault(name.Text, password.Text)
		if err != nil {
			dialog.ShowError(err, mw.Window)
			return
		}
		mw.unlockedVault[name.Text] = vault
		mw.setView(newVaultView(mw, vault))
		mw.SetMainMenu(mw.makeMainMenu())
	})
	createButton.Importance = widget.HighImportance

	cancelButton := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		mw.Reload()
	})

	return container.NewCenter(container.NewVBox(logo, heading, name, password, container.NewHBox(cancelButton, createButton)))
}

// vaultListView returns a view with the list of available vaults
func (mw *mainView) vaultListView() fyne.CanvasObject {

	heading := headingText("Choose a Vault")
	heading.Alignment = fyne.TextAlignCenter

	logo := pawLogo()

	c := container.NewVBox(logo, heading)

	vaults, err := mw.storage.Vaults()
	if err != nil {
		log.Fatal(err)
	}
	for _, v := range vaults {
		name := v
		resource := icon.LockOpenOutlinedIconThemed
		if _, ok := mw.unlockedVault[name]; !ok {
			resource = icon.LockOutlinedIconThemed
		}
		btn := widget.NewButtonWithIcon(name, resource, func() {
			mw.setView(mw.vaultViewByName(name))
		})
		btn.Alignment = widget.ButtonAlignLeading
		c.Add(btn)
	}

	return container.NewCenter(c)
}

// unlockVaultView returns the view used to unlock a vault
func (mw *mainView) unlockVaultView(name string) fyne.CanvasObject {
	logo := pawLogo()

	msg := fmt.Sprintf("Vault %q is locked", name)
	heading := headingText(msg)

	password := widget.NewPasswordEntry()
	password.SetPlaceHolder("Password")

	unlockBtn := widget.NewButtonWithIcon("Unlock", icon.LockOpenOutlinedIconThemed, func() {
		vault, err := mw.storage.LoadVault(name, password.Text)
		if err != nil {
			var invalidPasswordError *age.NoIdentityMatchError
			if errors.As(err, &invalidPasswordError) {
				err = errors.New("the password is incorrect")
			}
			dialog.ShowError(err, mw.Window)
			return
		}
		mw.unlockedVault[name] = vault
		mw.setView(newVaultView(mw, vault))
	})

	return container.NewCenter(container.NewVBox(logo, heading, password, unlockBtn))
}

// vaultView returns the view used to handle a vault
func (mw *mainView) vaultViewByName(name string) fyne.CanvasObject {
	vault, ok := mw.unlockedVault[name]
	if !ok {
		return mw.unlockVaultView(name)
	}
	return newVaultView(mw, vault)
}

func (mw *mainView) LockVault(name string) {
	delete(mw.unlockedVault, name)
	mw.Reload()
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
