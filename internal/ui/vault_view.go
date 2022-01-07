package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"lucor.dev/paw/internal/haveibeenpwned"
	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/paw"
)

type vaultView struct {
	widget.BaseWidget

	cancelCtx context.CancelFunc

	mainView *mainView

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

func newVaultView(mw *mainView, vault *paw.Vault) *vaultView {
	vw := &vaultView{
		mainView:      mw,
		filterOptions: &paw.VaultFilterOptions{},
		vault:         vault,
	}
	vw.ExtendBaseWidget(vw)

	vw.searchEntry = vw.makeSearchEntry()
	vw.addItemButton = vw.makeAddItemButton()

	vw.itemsWidget = newItemsWidget(vw.vault, vw.filterOptions)
	vw.itemsWidget.OnSelected = func(meta *paw.Metadata) {
		item, _ := vw.mainView.storage.LoadItem(vw.vault, meta)
		vw.setContentItem(item, vw.itemView)
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
	msg := fmt.Sprintf("Vault %q is empty", vw.vault.Name)
	text := headingText(msg)
	addItemButton := vw.makeAddItemButton()
	importItemButton := widget.NewButton("Import From File", vw.importFromFile)
	return container.NewCenter(container.NewVBox(text, addItemButton, importItemButton))
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

// setContent sets the content view for an Item action (i.e. view or edit) and handle the context view (creation and cancellation)
func (vw *vaultView) setContentItem(item paw.Item, f func(context.Context, paw.Item) fyne.CanvasObject) {
	if vw.cancelCtx != nil {
		vw.cancelCtx()
	}
	ctx, cancel := context.WithCancel(context.Background())
	vw.cancelCtx = cancel
	o := f(ctx, item)
	vw.content.Objects = []fyne.CanvasObject{o}
	vw.content.Refresh()
}

// setContent sets the content view with the provided object and refresh
func (vw *vaultView) setContent(o fyne.CanvasObject) {
	if vw.cancelCtx != nil {
		vw.cancelCtx()
	}
	vw.cancelCtx = nil
	vw.content.Objects = []fyne.CanvasObject{o}
	vw.content.Refresh()
}

// makeView returns the view container
func (vw *vaultView) makeView() fyne.CanvasObject {
	if vw.vault.Size() == 0 {
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

	vaults, err := vw.mainView.storage.Vaults()
	if err != nil {
		log.Fatal(err)
	}
	if len(vaults) == 1 {
		switchVault.Disabled = true
	}

	lockVault := fyne.NewMenuItem("Lock Vault", func() {
		vw.mainView.LockVault(vw.vault.Name)
		vw.mainView.Reload()
	})

	passwordAudit := fyne.NewMenuItem("Password Audit", func() {
		vw.setContent(vw.auditPasswordView())
	})

	importFromFile := fyne.NewMenuItem("Import From File", vw.importFromFile)

	exportToFile := fyne.NewMenuItem("Export To File", vw.exportToFile)

	menuItems := []*fyne.MenuItem{
		passwordAudit,
		importFromFile,
		exportToFile,
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

	label := widget.NewLabel(vw.vault.Name)
	return container.NewBorder(nil, nil, nil, button, label)
}

// makeSearchEntry returns the search entry used to filter the item list by name
func (vw *vaultView) makeSearchEntry() *widget.Entry {
	search := widget.NewEntry()
	search.SetPlaceHolder("Search")
	search.SetText(vw.filterOptions.Name)
	search.OnChanged = func(s string) {
		vw.filterOptions.Name = s
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
		name := i.GetMetadata().Type.String()
		options = append(options, name)
		itemTypeMap[name] = i.GetMetadata().Type
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
	note := paw.NewNote()
	password := paw.NewPassword()
	website := paw.NewLogin()
	website.TOTP = &paw.TOTP{
		Digits:   TOTPDigits(),
		Hash:     paw.TOTPHash(TOTPHash()),
		Interval: TOTPInverval(),
	}

	return []paw.Item{
		note,
		password,
		website,
	}
}

// makeAddItemButton returns the button used to add an item to the vault
func (vw *vaultView) makeAddItemButton() fyne.CanvasObject {

	button := widget.NewButtonWithIcon("Add Item", theme.ContentAddIcon(), func() {
		var modal *widget.PopUp

		c := container.NewVBox()
		for _, item := range vw.makeItems() {
			i := item
			o := widget.NewButtonWithIcon(i.GetMetadata().Type.String(), i.(paw.FyneObject).Icon(), func() {
				vw.setContentItem(i, vw.editItemView)
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
func (vw *vaultView) itemView(ctx context.Context, item paw.Item) fyne.CanvasObject {
	editBtn := widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), func() {
		vw.setContentItem(item, vw.editItemView)
	})
	top := container.NewBorder(nil, nil, nil, editBtn, widget.NewLabel(""))

	content := item.(paw.FyneObject).Show(ctx, vw.mainView.Window)
	bottom := item.(paw.FyneObject).InfoUI()

	return container.NewBorder(top, bottom, nil, nil, content)
}

// editItemView returns the view that allow to edit an item
func (vw *vaultView) editItemView(ctx context.Context, item paw.Item) fyne.CanvasObject {
	cancelBtn := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		vw.cancelCtx()
		if item.GetMetadata().Created.IsZero() {
			vw.setContent(vw.defaultContent())
			return
		}
		vw.setContentItem(item, vw.itemView)
	})

	d := NewPasswordGenerator(vw.vault.Key())
	var fo paw.FyneObject
	switch v := item.(type) {
	case (*paw.Password):
		v.SetPasswordGenerator(d)
		fo = v
	case (*paw.Login):
		v.SetPasswordGenerator(d)
		fo = v
	default:
		fo = v.(paw.FyneObject)
	}

	content, editItem := fo.Edit(ctx, vw.mainView.Window)
	saveBtn := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		metadata := editItem.GetMetadata()

		// TODO: update to use the built-in entry validation
		if metadata.Name == "" {
			d := dialog.NewInformation("", "The title cannot be emtpy", vw.mainView.Window)
			d.Show()
			return
		}

		if metadata.Created.IsZero() && vw.vault.HasItem(editItem) {
			msg := fmt.Sprintf("An item with the name %q already exists", metadata.Name)
			d := dialog.NewInformation("", msg, vw.mainView.Window)
			d.Show()
			return
		}

		var reloadItems bool
		var isNew bool

		metadata.Modified = time.Now()
		if metadata.Created.IsZero() {
			isNew = true
			metadata.Created = time.Now()
		}

		// add item to vault and store into the storage
		vw.vault.AddItem(editItem)
		err := vw.mainView.storage.StoreItem(vw.vault, editItem)
		if err != nil {
			dialog.ShowError(err, vw.mainView)
			return
		}

		if item.GetMetadata().IconResource != editItem.GetMetadata().IconResource {
			reloadItems = true
		}

		if item.ID() != editItem.ID() {
			reloadItems = true
			if !isNew {
				// item ID is changed, delete the old one
				vw.vault.DeleteItem(item)
				err := vw.mainView.storage.DeleteItem(vw.vault, item)
				if err != nil {
					log.Printf("item rename: could not remove old item from storage: %s", err)
				}
			}
		}

		item = editItem

		if reloadItems {
			vw.itemsWidget.Reload(item, vw.filterOptions)
		}

		vw.setContentItem(item, vw.itemView)
		vw.Reload()

	})
	saveBtn.Importance = widget.HighImportance

	top := container.NewBorder(nil, nil, cancelBtn, saveBtn, widget.NewLabel(""))

	// elements should not be displayed on create but only on edit
	var bottomContent fyne.CanvasObject
	var deleteBtn fyne.CanvasObject
	if !item.GetMetadata().Created.IsZero() {
		bottomContent = item.(paw.FyneObject).InfoUI()
		button := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
			msg := widget.NewLabel(fmt.Sprintf("Are you sure you want to delete %q?", item.String()))
			d := dialog.NewCustomConfirm("", "Delete", "Cancel", msg, func(b bool) {
				if b {
					vw.vault.DeleteItem(editItem)
					err := vw.mainView.storage.DeleteItem(vw.vault, editItem)
					if err != nil {
						dialog.ShowError(err, vw.mainView)
						return
					}
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

// auditPasswordView returns a view to audit passwords
func (vw *vaultView) auditPasswordView() fyne.CanvasObject {

	image := imageFromResource(icon.FactCheckOutlinedIconThemed)

	heading := headingText("Password Audit")
	heading.Alignment = fyne.TextAlignCenter

	text := widget.NewLabel("Check Vault passwords against existing data breaches")
	text.Wrapping = fyne.TextWrapWord
	text.Alignment = fyne.TextAlignCenter

	auditBtn := widget.NewButtonWithIcon("Audit", icon.FactCheckOutlinedIconThemed, func() {

		ctx, cancel := context.WithCancel(context.Background())

		itemMetadata := vw.vault.FilterItemMetadata(&paw.VaultFilterOptions{ItemType: paw.PasswordItemType | paw.LoginItemType})

		modalTitle := widget.NewLabel("Auditing items...")
		progressBind := binding.NewFloat()
		progressbar := widget.NewProgressBarWithData(progressBind)
		progressbar.TextFormatter = func() string {
			v, _ := progressBind.Get()
			return fmt.Sprintf("%.0f of %d", v, len(itemMetadata))
		}

		var cancelButton *widget.Button
		cancelButton = widget.NewButton("Cancel", func() {
			modalTitle.SetText("Cancelling auditing, please wait...")
			progressbar.Hide()
			cancelButton.Disable()
			cancel()
		})

		modalContent := container.NewBorder(modalTitle, nil, nil, nil, container.NewCenter(container.NewVBox(progressbar, cancelButton)))
		modal := widget.NewModalPopUp(modalContent, vw.mainView.Canvas())

		var counter uint32
		pwendItems := []haveibeenpwned.Pwned{}

		sem := semaphore.NewWeighted(int64(maxWorkers))
		g := &errgroup.Group{}

		go func() {
			for _, meta := range itemMetadata {
				meta := meta

				err := sem.Acquire(ctx, 1)
				if err != nil {
					cancel()
					break
				}

				g.Go(func() error {
					defer sem.Release(1)

					item, err := vw.mainView.storage.LoadItem(vw.vault, meta)
					if err != nil {
						return err
					}

					isPwend, count, err := haveibeenpwned.Search(ctx, item)
					if err != nil {
						return err
					}
					if isPwend {
						pwendItems = append(pwendItems, haveibeenpwned.Pwned{Item: item, Count: count})
					}

					v := atomic.AddUint32(&counter, 1)
					progressBind.Set(float64(v))
					return nil
				})
			}

			defer modal.Hide()
			err := g.Wait()
			if err != nil || errors.Is(ctx.Err(), context.Canceled) {
				ShowErrorDialog("Error auditing items", err, vw.mainView)
				return
			}

			sort.Slice(pwendItems, func(i, j int) bool { return pwendItems[i].Count > pwendItems[j].Count })

			num := len(pwendItems)
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
					return len(pwendItems)
				},
				func() fyne.CanvasObject {
					return container.NewBorder(nil, nil, widget.NewIcon(icon.PasswordOutlinedIconThemed), widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), nil), widget.NewLabel("item label"))
				},
				func(lii widget.ListItemID, co fyne.CanvasObject) {
					v := pwendItems[lii]
					co.(*fyne.Container).Objects[0].(*widget.Label).SetText(fmt.Sprintf("%s (found %d times)", v.Item.GetMetadata().Name, v.Count))
					co.(*fyne.Container).Objects[1].(*widget.Icon).SetResource(v.Item.(paw.FyneObject).Icon())
					co.(*fyne.Container).Objects[2].(*widget.Button).OnTapped = func() {
						vw.setContentItem(v.Item, vw.editItemView)
					}
				},
			)
			list.OnSelected = func(id widget.ListItemID) {
				v := pwendItems[id]
				vw.setContentItem(v.Item, vw.itemView)
			}

			c := container.NewBorder(container.NewVBox(image, heading, text), nil, nil, nil, list)
			vw.setContent(c)
		}()
		modal.Show()
	})
	auditBtn.Resize(auditBtn.MinSize())

	empty := widget.NewLabel("")
	return container.NewVBox(image, heading, text, container.NewGridWithColumns(3, empty, auditBtn, empty))
}

func (vw *vaultView) importFromFile() {
	d := dialog.NewFileOpen(func(uc fyne.URIReadCloser, e error) {

		ctx, cancel := context.WithCancel(context.Background())

		data := paw.Imported{}
		var counter uint32

		modalTitle := widget.NewLabel("Importing items...")

		progressBind := binding.NewFloat()
		progressbar := widget.NewProgressBarWithData(progressBind)
		progressbar.TextFormatter = func() string {
			v, _ := progressBind.Get()
			return fmt.Sprintf("%.0f of %d", v, len(data.Items))
		}

		var cancelButton *widget.Button
		cancelButton = widget.NewButton("Cancel", func() {
			modalTitle.SetText("Cancelling import, please wait...")
			progressbar.Hide()
			cancelButton.Disable()
			cancel()
		})

		c := container.NewBorder(modalTitle, nil, nil, nil, container.NewCenter(container.NewVBox(progressbar, cancelButton)))
		modal := widget.NewModalPopUp(c, vw.mainView.Canvas())

		rollback := func(vault *paw.Vault, items []paw.Item) {
			for _, item := range items {
				vw.mainView.storage.DeleteItem(vw.vault, item)
				vw.vault.DeleteItem(item)
			}
		}

		go func() {
			if uc == nil {
				// file open dialog has been cancelled
				modal.Hide()
				return
			}
			defer uc.Close()
			// Decode the JSON input file
			err := json.NewDecoder(uc).Decode(&data)
			if err != nil {
				modal.Hide()
				ShowErrorDialog("Error importing items", err, vw.mainView)
				return
			}

			sem := semaphore.NewWeighted(int64(maxWorkers))
			g := &errgroup.Group{}

			processed := []paw.Item{}
			// TODO: handle if an item with same name and type already exists
			for _, item := range data.Items {
				item := item

				err = sem.Acquire(ctx, 1)
				if err != nil {
					cancel()
					break
				}

				g.Go(func() error {
					defer sem.Release(1)
					err := vw.mainView.storage.StoreItem(vw.vault, item)
					if err != nil {
						return err
					}
					processed = append(processed, item)
					v := atomic.AddUint32(&counter, 1)
					progressBind.Set(float64(v))
					return nil
				})
			}

			defer modal.Hide()
			err = g.Wait()
			if err != nil || errors.Is(ctx.Err(), context.Canceled) {
				rollback(vw.vault, processed)
				ShowErrorDialog("Error importing items", err, vw.mainView)
				return
			}

			for _, item := range processed {
				vw.vault.AddItem(item)
			}
			err = vw.mainView.storage.StoreVault(vw.vault)
			if err != nil {
				rollback(vw.vault, processed)
				ShowErrorDialog("Error importing items", err, vw.mainView)
				return
			}
			vw.itemsWidget.Reload(nil, vw.filterOptions)
			vw.setContent(vw.defaultContent())
			vw.Reload()
		}()

		modal.Show()

	}, vw.mainView)
	d.Show()
}

func (vw *vaultView) exportToFile() {
	d := dialog.NewFileSave(func(uc fyne.URIWriteCloser, e error) {

		ctx, cancel := context.WithCancel(context.Background())

		var counter uint32

		modalTitle := widget.NewLabel("Exporting items...")

		progressBind := binding.NewFloat()
		progressbar := widget.NewProgressBarWithData(progressBind)
		progressbar.TextFormatter = func() string {
			v, _ := progressBind.Get()
			return fmt.Sprintf("%.0f of %d", v, vw.vault.Size())
		}

		var cancelButton *widget.Button
		cancelButton = widget.NewButton("Cancel", func() {
			modalTitle.SetText("Cancelling export, please wait...")
			progressbar.Hide()
			cancelButton.Disable()
			cancel()
		})

		c := container.NewBorder(modalTitle, nil, nil, nil, container.NewCenter(container.NewVBox(progressbar, cancelButton)))
		modal := widget.NewModalPopUp(c, vw.mainView.Canvas())

		go func() {
			if uc == nil {
				// file open dialog has been cancelled
				modal.Hide()
				return
			}
			defer uc.Close()

			sem := semaphore.NewWeighted(int64(maxWorkers))
			g := &errgroup.Group{}

			mu := &sync.Mutex{}
			data := map[string][]paw.Item{}
			for _, meta := range vw.vault.ItemMetadata {
				meta := meta

				err := sem.Acquire(ctx, 1)
				if err != nil {
					cancel()
					break
				}

				g.Go(func() error {
					defer sem.Release(1)
					item, err := vw.mainView.storage.LoadItem(vw.vault, meta)
					if err != nil {
						return err
					}

					itemType := item.GetMetadata().Type.String()

					mu.Lock()
					data[itemType] = append(data[itemType], item)
					mu.Unlock()

					v := atomic.AddUint32(&counter, 1)
					progressBind.Set(float64(v))
					return nil
				})
			}

			defer modal.Hide()
			err := g.Wait()
			if err != nil || errors.Is(ctx.Err(), context.Canceled) {
				ShowErrorDialog("Error exporting items", err, vw.mainView)
				return
			}

			err = json.NewEncoder(uc).Encode(data)
			if err != nil {
				ShowErrorDialog("Error exporting items", err, vw.mainView)
			}
		}()
		modal.Show()
	}, vw.mainView)
	d.SetFileName(fmt.Sprintf("%s.paw.json", vw.vault.Name))
	d.Show()
}
