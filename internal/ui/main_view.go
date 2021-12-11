package ui

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"runtime/debug"

	"filippo.io/age"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lucor.dev/paw/internal/icon"
)

// mainView represents the Paw main view
type mainView struct {
	fyne.Window

	keyring *keyring

	view *fyne.Container
}

// Make returns the fyne user interface
func Make(a fyne.App, w fyne.Window) fyne.CanvasObject {
	kr, err := newKeyring(a.Storage())
	if err != nil {
		log.Fatal(err)
	}

	mw := &mainView{
		Window:  w,
		keyring: kr,
	}

	mw.view = container.NewMax(mw.buildMainView())
	mw.SetMainMenu(mw.makeMainMenu())
	return mw.view
}

func (mw *mainView) makeMainMenu() *fyne.MainMenu {
	// a Quit item will is appended automatically by Fyne to the first menu item
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("New Vault", func() {
			mw.view.Objects[0] = mw.createVaultView()
		}),
	)
	switchItem := fyne.NewMenuItem("Switch Vault", func() {
		mw.Reload()
	})
	if len(mw.keyring.Vaults()) <= 1 {
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
					pawLogo(64, 64),
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
	mw.view.Objects[0] = mw.buildMainView()
	mw.SetMainMenu(mw.makeMainMenu())
}

func (mw *mainView) buildMainView() fyne.CanvasObject {
	var view fyne.CanvasObject
	vaults := mw.keyring.Vaults()
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

	logo := pawLogo(64, 64)

	heading := headingText("Welcome to Paw")
	heading.Alignment = fyne.TextAlignCenter

	name := widget.NewEntry()
	name.SetPlaceHolder("Name")

	secret := widget.NewPasswordEntry()
	secret.SetPlaceHolder("Password")

	btn := widget.NewButton("Create Vault", func() {
		vault, err := mw.keyring.CreateVault(name.Text, secret.Text)
		if err != nil {
			dialog.ShowError(err, mw.Window)
			return
		}
		mw.view.Objects[0] = mw.vaultView(vault.Name())
	})
	btn.Importance = widget.HighImportance

	return container.NewCenter(container.NewVBox(logo, heading, name, secret, btn))
}

// initVaultView returns the view used to create the first vault
func (mw *mainView) createVaultView() fyne.CanvasObject {
	heading := headingText("Create a new Vault")
	heading.Alignment = fyne.TextAlignCenter

	logo := pawLogo(64, 64)

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
		vault, err := mw.keyring.CreateVault(name.Text, password.Text)
		if err != nil {
			dialog.ShowError(err, mw.Window)
			return
		}
		mw.view.Objects[0] = mw.vaultView(vault.Name())
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

	logo := pawLogo(64, 64)

	c := container.NewVBox(logo, heading)

	for _, v := range mw.keyring.Vaults() {
		name := v
		resource := icon.LockOpenOutlinedIconThemed
		if mw.keyring.IsLockedVault(name) {
			resource = icon.LockOutlinedIconThemed
		}
		btn := widget.NewButtonWithIcon(name, resource, func() {
			mw.view.Objects[0] = mw.vaultView(name)
		})
		btn.Alignment = widget.ButtonAlignLeading
		c.Add(btn)
	}

	return container.NewCenter(c)
}

// unlockVaultView returns the view used to unlock a vault
func (mw *mainView) unlockVaultView(name string) fyne.CanvasObject {
	logo := pawLogo(64, 64)

	msg := fmt.Sprintf("Vault %q is locked", name)
	heading := headingText(msg)

	secret := widget.NewPasswordEntry()
	secret.SetPlaceHolder("Password")

	unlockBtn := widget.NewButtonWithIcon("Unlock", icon.LockOpenOutlinedIconThemed, func() {
		_, err := mw.keyring.UnlockVault(name, secret.Text)
		if err != nil {
			var invalidPasswordError *age.NoIdentityMatchError
			if errors.As(err, &invalidPasswordError) {
				err = errors.New("the password is incorrect")
			}
			dialog.ShowError(err, mw.Window)
			return
		}
		mw.view.Objects[0] = mw.vaultView(name)
	})

	return container.NewCenter(container.NewVBox(logo, heading, secret, unlockBtn))
}

// vaultView returns the view used to handle a vault
func (mw *mainView) vaultView(name string) fyne.CanvasObject {
	if mw.keyring.IsLockedVault(name) {
		return mw.unlockVaultView(name)
	}
	return newVaultView(name, mw, mw.keyring)
}

// headingText returns a text formatted as heading
func headingText(text string) *canvas.Text {
	t := canvas.NewText(text, theme.ForegroundColor())
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.TextSize = theme.TextSubHeadingSize()
	return t
}

// logo returns the Paw logo as a canvas image with the specified dimensions
func pawLogo(width float32, height float32) *canvas.Image {
	img := canvas.NewImageFromResource(icon.PawIcon)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(width, height))
	return img
}
