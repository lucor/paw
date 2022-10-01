package ui

import (
	"fmt"
	"log"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (a *app) makeMainMenu() *fyne.MainMenu {
	// a Quit item will is appended automatically by Fyne to the first menu item
	vaultItem := fyne.NewMenuItem("Vault", nil)
	vaultItem.ChildMenu = fyne.NewMenu("", a.makeVaultMenuItems()...)

	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("New Vault", func() {
			a.showCreateVaultView()
		}),
		vaultItem,
		fyne.NewMenuItem("Preferences", func() {
			a.showPreferencesView()
		}),
	)

	helpMenu := fyne.NewMenu("Help",
		fyne.NewMenuItem("About", a.about),
	)

	return fyne.NewMainMenu(
		fileMenu,

		helpMenu,
	)
}

func (a *app) about() {
	u, _ := url.Parse("https://lucor.dev/paw")
	l := widget.NewLabel("Paw - " + a.version)
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
	d := dialog.NewCustom("About Paw", "Ok", co, a.win)
	d.Show()
}

func (a *app) makeVaultMenu() fyne.CanvasObject {
	d := fyne.CurrentApp().Driver()

	menuItems := []*fyne.MenuItem{
		fyne.NewMenuItem("Password Audit", a.showAuditPasswordView),
		fyne.NewMenuItem("Import From File", a.importFromFile),
		fyne.NewMenuItem("Export To File", a.exportToFile),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Lock Vault", func() {
			a.main.Content = a.makeUnlockVaultView(a.vault.Name)
			a.lockVault()
			a.main.Refresh()
		}),
	}

	popUpMenu := widget.NewPopUpMenu(fyne.NewMenu("", menuItems...), a.win.Canvas())

	var button *widget.Button
	button = widget.NewButtonWithIcon("", theme.MoreVerticalIcon(), func() {
		buttonPos := d.AbsolutePositionForObject(button)
		buttonSize := button.Size()
		popUpMin := popUpMenu.MinSize()

		var popUpPos fyne.Position
		popUpPos.X = buttonPos.X + buttonSize.Width - popUpMin.Width
		popUpPos.Y = buttonPos.Y + buttonSize.Height
		popUpMenu.ShowAtPosition(popUpPos)
	})

	return button
}

func (a *app) makeVaultMenuItems() []*fyne.MenuItem {
	vaults, err := a.storage.Vaults()
	if err != nil {
		log.Fatal(err)
	}

	mi := make([]*fyne.MenuItem, len(vaults))
	for i, vaultName := range vaults {
		i := i
		vaultName := vaultName
		mi[i] = fyne.NewMenuItem(vaultName, func() {})
		_, isDesktop := fyne.CurrentApp().(desktop.App)
		if isDesktop && i < 9 {
			shortcut := &desktop.CustomShortcut{KeyName: fyne.KeyName(fmt.Sprint(i + 1)), Modifier: fyne.KeyModifierControl}
			a.win.Canvas().AddShortcut(shortcut, func(shortcut fyne.Shortcut) {
				a.setVaultViewByName(vaultName)
			})
			mi[i].Shortcut = shortcut
		}
		mi[i].Action = func() {
			defer a.win.Show()
			a.setVaultViewByName(vaultName)
		}
	}
	return mi
}
