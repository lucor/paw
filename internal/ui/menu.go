package ui

import (
	"log"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (a *app) makeMainMenu() *fyne.MainMenu {
	// a Quit item will is appended automatically by Fyne to the first menu item
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("New Vault", func() {
			a.setContent(a.makeInitVaultView())
		}),
	)

	switchItem := fyne.NewMenuItem("Switch Vault", func() {
		a.setContent(a.makeSelectVaultView())
	})

	vaults, err := a.storage.Vaults()
	if err != nil {
		log.Fatal(err)
	}
	if len(vaults) <= 1 {
		switchItem.Disabled = true
	}
	fileMenu.Items = append(fileMenu.Items, switchItem)

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

	switchVault := fyne.NewMenuItem("Switch Vault", func() {
		a.setContent(a.makeSelectVaultView())
		return
	})

	vaults, err := a.storage.Vaults()
	if err != nil {
		log.Fatal(err)
	}
	if len(vaults) == 1 {
		switchVault.Disabled = true
	}

	lockVault := fyne.NewMenuItem("Lock Vault", func() {
		a.lockVault()
	})

	passwordAudit := fyne.NewMenuItem("Password Audit", func() {
		a.setContent(a.makeAuditPasswordView())
	})

	importFromFile := fyne.NewMenuItem("Import From File", a.importFromFile)

	exportToFile := fyne.NewMenuItem("Export To File", a.exportToFile)

	menuItems := []*fyne.MenuItem{
		passwordAudit,
		importFromFile,
		exportToFile,
		fyne.NewMenuItemSeparator(),
		switchVault,
		lockVault,
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

	label := widget.NewLabel(a.vault.Name)
	return container.NewBorder(nil, nil, nil, button, label)
}
