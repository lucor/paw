package ui

import (
	"context"
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lucor.dev/paw/internal/haveibeenpwned"
	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/paw"
)

type vaultView struct {
	widget.BaseWidget

	mainView      *mainView
	keyring       *keyring
	vault         *paw.Vault
	filterOptions *paw.VaultFilterOptions

	// view is a container used to split the vault view in two areas: navbar and content.
	// The navbar area allows to switch between the vault's item along with the possibility to filter by name, type and add new items.
	// The content area contains the views that allow to perform action on the item (i.e. read, edit, delete)
	view *fyne.Container

	// content is the container that represents the content area
	content *fyne.Container

	// the objects below are all parts of the navbar area
	searchEntry     *widget.Entry
	typeSelectEntry *widget.Select
	addItemButton   fyne.CanvasObject
	itemsWidget     *itemsWidget
}

func newVaultView(name string, mw *mainView, kr *keyring) *vaultView {
	vault, _ := kr.LoadVault(name)
	vw := &vaultView{
		mainView:      mw,
		keyring:       kr,
		filterOptions: &paw.VaultFilterOptions{},
		vault:         vault,
	}
	vw.ExtendBaseWidget(vw)

	vw.searchEntry = vw.makeSearchEntry()
	vw.addItemButton = vw.makeAddItemButton()

	vw.itemsWidget = newItemsWidget(vw.vault, vw.filterOptions)
	vw.itemsWidget.OnSelected = func(i paw.Item) {
		vw.setContent(vw.itemView(i))
	}
	vw.typeSelectEntry = vw.makeTypeSelectEntry()
	vw.content = container.NewMax(vw.defaultContent())

	vw.view = container.NewMax(vw.makeView())
	return vw
}

func (vw *vaultView) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(vw.view)
}

// Reload reloads the widget according the specified options
func (vw *vaultView) Reload() {
	vw.view.Objects[0] = vw.makeView()
}

// emptyVaultContent returns the content to display when the vault has no items
func (vw *vaultView) emptyVaultContent() fyne.CanvasObject {
	msg := fmt.Sprintf("Vault %q is empty", vw.vault.Name())
	t := headingText(msg)
	b := vw.makeAddItemButton()
	return container.NewCenter(container.NewVBox(t, b))
}

// defaultContent returns the object to display for default states
func (vw *vaultView) defaultContent() fyne.CanvasObject {
	if vw.itemsWidget.Length() == 0 {
		return vw.emptyVaultContent()
	}
	img := canvas.NewImageFromResource(icon.PawIcon)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(64, 64))
	return container.NewCenter(img)
}

// setContent sets the content view with the provided object and refresh
func (vw *vaultView) setContent(o fyne.CanvasObject) {
	vw.content.Objects = []fyne.CanvasObject{o}
	vw.content.Refresh()
}

// makeView returns the view container
func (vw *vaultView) makeView() fyne.CanvasObject {
	if vw.itemsWidget.Length() == 0 {
		vw.setContent(vw.defaultContent())
		return vw.content
	}

	left := container.NewBorder(container.NewVBox(vw.makeVaultMenu(), vw.searchEntry, vw.typeSelectEntry), vw.addItemButton, nil, nil, vw.itemsWidget)
	split := container.NewHSplit(left, vw.content)
	split.Offset = 0.3
	return split
}

func (vw *vaultView) makeVaultMenu() fyne.CanvasObject {
	d := fyne.CurrentApp().Driver()

	switchVault := fyne.NewMenuItem("Switch Vault", func() {
		vw.mainView.Reload()
	})
	if len(vw.keyring.Vaults()) == 1 {
		switchVault.Disabled = true
	}

	lockVault := fyne.NewMenuItem("Lock Vault", func() {
		vw.keyring.LockVault(vw.vault.Name())
		vw.mainView.Reload()
	})

	passwordAudit := fyne.NewMenuItem("Password Audit", func() {
		vw.setContent(vw.auditPasswordView())
	})

	menuItems := []*fyne.MenuItem{
		passwordAudit,
		fyne.NewMenuItemSeparator(),
		switchVault,
		lockVault,
	}
	popUpMenu := widget.NewPopUpMenu(fyne.NewMenu("", menuItems...), vw.mainView.Window.Canvas())

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

	label := widget.NewLabel(vw.vault.Name())
	return container.NewBorder(nil, nil, nil, button, label)
}

// makeSearchEntry returns the search entry used to filter the item list by name
func (vw *vaultView) makeSearchEntry() *widget.Entry {
	search := widget.NewEntry()
	search.SetPlaceHolder("Search")
	search.SetText(vw.filterOptions.Title)
	search.OnChanged = func(s string) {
		vw.filterOptions.Title = s
		vw.itemsWidget.Reload(nil, vw.filterOptions)
	}
	return search
}

// makeTypeSelectEntry returns the select entry used to filter the item list by type
func (vw *vaultView) makeTypeSelectEntry() *widget.Select {

	options := []string{"All items"}

	itemTypeMap := map[string]paw.ItemType{}
	for _, item := range vw.makeItems() {
		i := item
		name := i.Type().String()
		options = append(options, name)
		itemTypeMap[name] = i.Type()
	}

	filter := widget.NewSelect(options, func(s string) {
		var v paw.ItemType
		if s == "All items" {
			v = paw.ItemType(0) // No item type will be selected
		} else {
			v = itemTypeMap[s]
		}

		vw.filterOptions.ItemType = v
		vw.itemsWidget.Reload(nil, vw.filterOptions)
	})

	filter.SetSelectedIndex(0)
	return filter
}

// makeItems returns a slice of empty paw.Item ready to use as template for
// item's creation
func (vw *vaultView) makeItems() []paw.Item {
	secretMaker := vw.vault.Key()
	password := paw.NewPassword(secretMaker, defaultPasswordOptions())
	return []paw.Item{
		paw.NewNote(),
		password,
		paw.NewWebsite(password),
	}
}

// makeAddItemButton returns the button used to add an item to the vault
func (vw *vaultView) makeAddItemButton() fyne.CanvasObject {

	button := widget.NewButtonWithIcon("Add Item", theme.ContentAddIcon(), func() {
		var modal *widget.PopUp

		c := container.NewVBox()
		for _, item := range vw.makeItems() {
			i := item
			o := widget.NewButtonWithIcon(i.Type().String(), i.(paw.FyneObject).Icon().Resource, func() {
				vw.setContent(vw.editItemView(i))
				modal.Hide()
			})
			o.Alignment = widget.ButtonAlignLeading
			c.Add(o)
		}
		c.Add(widget.NewLabel(""))
		cancelButton := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
			modal.Hide()
		})
		c.Add(cancelButton)

		modal = widget.NewModalPopUp(c, vw.mainView.Window.Canvas())
		modal.Show()
	})
	button.Importance = widget.HighImportance
	return button
}

// itemView returns the view that displays the item's content along with the allowed actions
func (vw *vaultView) itemView(id paw.Item) fyne.CanvasObject {

	editBtn := widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), func() {
		vw.setContent(vw.editItemView(id))
	})
	top := container.NewBorder(nil, nil, nil, editBtn, widget.NewLabel(""))

	content := id.(paw.FyneObject).Show(vw.mainView.Window)
	bottom := id.(paw.FyneObject).InfoUI()

	return container.NewBorder(top, bottom, nil, nil, content)
}

// editItemView returns the view that allow to edit an item
func (vw *vaultView) editItemView(id paw.Item) fyne.CanvasObject {

	cancelBtn := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		if id.GetMetadata().Created.IsZero() {
			vw.setContent(vw.defaultContent())
			return
		}
		vw.setContent(vw.itemView(id))
	})

	var fo paw.FyneObject
	switch v := id.(type) {
	case (*paw.Password):
		v.SetOptions(defaultPasswordOptions())
		v.SetSecretMaker(vw.vault.Key())
		fo = v
	case (*paw.Website):
		v.Password.SetOptions(defaultPasswordOptions())
		v.Password.SetSecretMaker(vw.vault.Key())
		fo = v
	default:
		fo = v.(paw.FyneObject)
	}

	content, editID := fo.Edit(vw.mainView.Window)
	saveBtn := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		metadata := editID.GetMetadata()

		// TODO: update to use the built-in entry validation
		if metadata.Title == "" {
			d := dialog.NewInformation("", "The title cannot be emtpy", vw.mainView.Window)
			d.Show()
			return
		}
		if metadata.Created.IsZero() && vw.vault.Item(editID.ID()) != nil {
			msg := fmt.Sprintf("An item with the name %q already exists", metadata.Title)
			d := dialog.NewInformation("", msg, vw.mainView.Window)
			d.Show()
			return
		}

		metadata.Modified = time.Now()
		if metadata.Created.IsZero() || id.ID() != editID.ID() {
			metadata.Created = time.Now()
		}

		vw.vault.SetItem(editID)
		vw.keyring.StoreVault(vw.vault)

		if id.ID() != editID.ID() {
			vw.itemsWidget.Reload(editID, vw.filterOptions)
		}

		id = editID
		vw.setContent(vw.itemView(id))
		vw.Reload()

	})
	saveBtn.Importance = widget.HighImportance

	top := container.NewBorder(nil, nil, cancelBtn, saveBtn, widget.NewLabel(""))

	// elements should not be displayed on create but only on edit
	var bottomContent fyne.CanvasObject
	var deleteBtn fyne.CanvasObject
	if !id.GetMetadata().Created.IsZero() {
		bottomContent = id.(paw.FyneObject).InfoUI()
		button := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
			msg := widget.NewLabel(fmt.Sprintf("Are you sure you want to delete %q?", id.String()))
			d := dialog.NewCustomConfirm("", "Delete", "Cancel", msg, func(b bool) {
				if b {
					vw.vault.DeleteItem(editID)
					vw.keyring.StoreVault(vw.vault)
					vw.itemsWidget.Reload(nil, vw.filterOptions)
					vw.setContent(vw.defaultContent())
					vw.Reload()
				}
			}, vw.mainView.Window)
			d.Show()
		})
		deleteBtn = container.NewMax(canvas.NewRectangle(color.NRGBA{0xd0, 0x17, 0x2d, 0xff}), button)
	}

	bottom := container.NewBorder(bottomContent, nil, nil, deleteBtn, widget.NewLabel(""))
	return container.NewBorder(top, bottom, nil, nil, content)
}

// auditPasswordView returns a view with the list of available vaults
func (vw *vaultView) auditPasswordView() fyne.CanvasObject {

	image := imageFromResource(icon.FactCheckOutlinedIconThemed)

	heading := headingText("Password Audit")
	heading.Alignment = fyne.TextAlignCenter

	text := widget.NewLabel("Check Vault passwords against existing data breaches")
	text.Wrapping = fyne.TextWrapWord
	text.Alignment = fyne.TextAlignCenter

	auditBtn := widget.NewButtonWithIcon("Audit", icon.FactCheckOutlinedIconThemed, func() {
		items := vw.vault.FilterItems(&paw.VaultFilterOptions{ItemType: paw.PasswordItemType | paw.WebsiteItemType})

		progressBind := binding.NewFloat()
		progressbar := widget.NewProgressBarWithData(progressBind)
		progressbar.TextFormatter = func() string {
			return fmt.Sprintf("%.0f of %d", progressbar.Value, len(items))
		}

		ctx := context.Background()
		d := dialog.NewCustom("Checking passwords...", "Cancel", progressbar, vw.mainView.Window)
		d.SetOnClosed(func() {
			ctx.Done()
		})
		d.Show()
		pwend, err := haveibeenpwned.Search(ctx, items, progressBind)
		if err != nil {
			dialog.ShowError(err, vw.mainView.Window)
			return
		}
		d.Hide()

		num := len(pwend)
		if num == 0 {
			image = imageFromResource(icon.CheckCircleOutlinedIconThemed)
			text.SetText("No password found in data breaches")
			vw.setContent(container.NewVBox(image, heading, text))
			return
		}

		image = imageFromResource(theme.WarningIcon())
		text.SetText("Passwords of the items below have been found in a data breaches and should not be used")
		list := widget.NewList(
			func() int {
				return len(pwend)
			},
			func() fyne.CanvasObject {
				return container.NewBorder(nil, nil, widget.NewIcon(icon.PasswordOutlinedIconThemed), widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), nil), widget.NewLabel("item label"))
			},
			func(lii widget.ListItemID, co fyne.CanvasObject) {
				v := pwend[lii]
				co.(*fyne.Container).Objects[0].(*widget.Label).SetText(fmt.Sprintf("%s (found %d times)", v.Item.GetMetadata().Title, v.Count))
				co.(*fyne.Container).Objects[1].(*widget.Icon).SetResource(v.Item.(paw.FyneObject).Icon().Resource)
				co.(*fyne.Container).Objects[2].(*widget.Button).OnTapped = func() { vw.setContent(vw.editItemView(v.Item)) }
			},
		)
		list.OnSelected = func(id widget.ListItemID) {
			v := pwend[id]
			vw.setContent(vw.itemView(v.Item))
		}

		c := container.NewBorder(container.NewVBox(image, heading, text), nil, nil, nil, list)
		vw.setContent(c)
	})
	auditBtn.Resize(auditBtn.MinSize())

	empty := widget.NewLabel("")
	return container.NewVBox(image, heading, text, container.NewGridWithColumns(3, empty, auditBtn, empty))
}
