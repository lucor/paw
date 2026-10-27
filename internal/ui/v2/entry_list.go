// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package v2

import (
	"fmt"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/ui/v2/assets/icons"
)

// EntryRow contains the public summary displayed in the vault list.
type EntryRow struct {
	Title, Subtitle, Category string
	Icon                      fyne.Resource
	Open                      func()
}

// EntryCategory describes a Paw-supported presentation category.
type EntryCategory struct {
	ID, Label string
	Icon      fyne.Resource
}

// EntryKind identifies one of the supported entry creation flows.
type EntryKind string

const (
	EntryNote     EntryKind = "note"
	EntryPassword EntryKind = "password"
	EntryLogin    EntryKind = "login"
	EntrySSH      EntryKind = "ssh"
)

// EntryListOptions connects the list to vault navigation and saved filters.
type EntryListOptions struct {
	VaultName        string
	Vaults           []VaultChoice
	VaultActions     []*fyne.MenuItem
	Entries          []EntryRow
	Categories       []EntryCategory
	Search, Category string
	AddVault         func()
	AddEntry         func(EntryKind)
	OnFilterChanged  func(search, category string)
}

// NewEntryList builds the unlocked vault interface with bottom navigation and a native list.
func NewEntryList(options EntryListOptions) fyne.CanvasObject {
	entries := append([]EntryRow(nil), options.Entries...)
	sort.SliceStable(entries, func(i, j int) bool { return strings.ToLower(entries[i].Title) < strings.ToLower(entries[j].Title) })
	var visible []EntryRow
	search, category := options.Search, options.Category
	if category == "" {
		category = "all"
	}
	var list *widget.List
	list = widget.NewList(func() int { return len(visible) }, func() fyne.CanvasObject {
		return container.New(layout.NewCustomPaddedLayout(0, 0, theme.Size(theme.SizeNameInnerPadding), theme.Size(theme.SizeNameInnerPadding)), NewActionRow("Entry", "Summary", icons.Vault, nil))
	}, func(id widget.ListItemID, object fyne.CanvasObject) {
		entry := visible[id]
		row := object.(*fyne.Container).Objects[0].(*ActionRow)
		row.SetContent(entry.Title, entry.Subtitle, entry.Icon)
		row.SetTapped(func() { list.Select(id) })
	})
	list.HideSeparators = true
	list.OnSelected = func(id widget.ListItemID) { entry := visible[id]; list.UnselectAll(); entry.Open() }
	empty := newText("No entries yet. Use + to add an entry.", helpStyle)
	empty.Alignment = fyne.TextAlignCenter
	input := widget.NewEntry()
	input.SetPlaceHolder("Search entries…")
	input.SetText(search)
	input.ActionItem = widget.NewButtonWithIcon("", theme.CancelIcon(), func() { input.SetText("") })
	if len(options.Categories) == 0 {
		options.Categories = []EntryCategory{{ID: "all", Label: "All items", Icon: icons.Vault}}
	}
	categoryFor := func(id string) EntryCategory {
		for _, value := range options.Categories {
			if value.ID == id {
				return value
			}
		}
		return options.Categories[0]
	}
	selectedCategory := categoryFor(category)
	count := newText("0 items", bodyStyle)
	count.Wrapping = fyne.TextWrapOff
	categoryIcon := widget.NewIcon(selectedCategory.Icon)
	categoryLabel := newText(selectedCategory.Label, bodyStyle)
	categoryLabel.Wrapping = fyne.TextWrapOff
	var changed func()
	categoryButton := widget.NewButton("", nil)
	categoryButton.OnTapped = func() {
		var menu *widget.PopUp
		rows := make([]fyne.CanvasObject, 0, len(options.Categories))
		for _, value := range options.Categories {
			entryCategory := value
			rows = append(rows, newSelectionMenuRow(entryCategory.Label, entryCategory.Icon, value.ID == category, func() {
				category = entryCategory.ID
				menu.Hide()
				changed()
			}))
		}
		menu = widget.NewPopUp(container.NewVBox(rows...), fyne.CurrentApp().Driver().CanvasForObject(categoryButton))
		menu.ShowAtRelativePosition(fyne.NewPos(0, categoryButton.Size().Height), categoryButton)
	}
	categoryDisplay := container.NewStack(categoryButton, container.NewPadded(container.NewBorder(nil, nil, categoryIcon, widget.NewIcon(icons.Down), categoryLabel)))
	refresh := func() {
		visible = nil
		query := strings.ToLower(strings.TrimSpace(search))
		for _, entry := range entries {
			if (category == "all" || entry.Category == category) && strings.Contains(strings.ToLower(entry.Title+" "+entry.Subtitle), query) {
				visible = append(visible, entry)
			}
		}
		list.UnselectAll()
		list.Refresh()
		count.SetText(itemCountLabel(len(visible)))
		selected := categoryFor(category)
		categoryIcon.SetResource(selected.Icon)
		categoryLabel.SetText(selected.Label)
		if len(visible) == 0 {
			if len(entries) > 0 {
				empty.SetText("No items match these filters.")
			} else {
				empty.SetText("No entries in this vault yet.")
			}
			empty.Show()
		} else {
			empty.Hide()
		}
	}
	changed = func() {
		refresh()
		if options.OnFilterChanged != nil {
			options.OnFilterChanged(search, category)
		}
	}
	input.OnChanged = func(value string) { search = value; changed() }
	var vaultButton *iconButton
	vaultButton = newIconButton("Select vault", nil, func() {
		var menu *widget.PopUp
		rows := make([]fyne.CanvasObject, 0, len(options.Vaults)+2)
		for _, vault := range options.Vaults {
			status, resource := "Locked", icons.Lock
			if vault.Unlocked {
				status, resource = "Unlocked", icons.LockOpen
			}
			rows = append(rows, newSelectionMenuRow(vault.Name+" — "+status, resource, vault.Active, func() {
				menu.Hide()
				vault.Open()
			}))
		}
		rows = append(rows, widget.NewSeparator(), newSelectionMenuRow("Add vault", icons.Plus, false, func() {
			menu.Hide()
			options.AddVault()
		}))
		menu = widget.NewPopUp(container.NewVBox(rows...), fyne.CurrentApp().Driver().CanvasForObject(vaultButton))
		menu.ShowAtRelativePosition(fyne.NewPos(0, vaultButton.Size().Height), vaultButton)
	})
	vaultLabel := newText(options.VaultName, bodyStyle)
	vaultLabel.Wrapping = fyne.TextWrapOff
	vaultLabel.Truncation = fyne.TextTruncateEllipsis
	vaultControl := container.NewStack(vaultButton, container.NewPadded(container.NewBorder(nil, nil, widget.NewIcon(icons.Vault), widget.NewIcon(icons.Down), vaultLabel)))
	add := newIconButton("Add entry", icons.Plus, nil)
	add.Importance = widget.LowImportance
	add.OnTapped = func() {
		items := make([]*fyne.MenuItem, 0, 4)
		for _, entry := range []struct {
			kind EntryKind
			name string
			icon fyne.Resource
		}{
			{EntryNote, "Note", icons.Note}, {EntryPassword, "Password", icons.Password},
			{EntryLogin, "Login", icons.Login}, {EntrySSH, "SSH key", icons.SSH},
		} {
			kind := entry.kind
			item := fyne.NewMenuItem(entry.name, func() { options.AddEntry(kind) })
			item.Icon = entry.icon
			items = append(items, item)
		}
		widget.ShowPopUpMenuAtRelativePosition(fyne.NewMenu("", items...), fyne.CurrentApp().Driver().CanvasForObject(add), fyne.NewPos(0, add.Size().Height), add)
	}
	more := newIconButton("Vault actions", icons.More, nil)
	more.Importance = widget.LowImportance
	more.OnTapped = func() {
		widget.ShowPopUpMenuAtRelativePosition(fyne.NewMenu("", options.VaultActions...), fyne.CurrentApp().Driver().CanvasForObject(more), fyne.NewPos(0, more.Size().Height), more)
	}
	controls := container.NewHBox(add, more)
	if len(options.VaultActions) == 0 {
		more.Hide()
	}
	header := container.NewVBox(container.NewBorder(nil, nil, nil, controls, vaultControl),
		container.NewBorder(nil, nil, widget.NewIcon(icons.Search), nil, input), container.NewBorder(nil, nil, categoryDisplay, count))
	page := container.New(layout.NewCustomPaddedLayout(24, 24, 24, 24), container.NewBorder(header, nil, nil, nil, container.NewStack(list, container.NewCenter(empty))))
	navigation := NewNavigation(NavigationItem{icons.Vault, icons.VaultActive}, NavigationItem{icons.Codes, icons.CodesActive}, NavigationItem{icons.Settings, icons.SettingsActive})
	back := func() { navigation.Select(0) }
	pages := []fyne.CanvasObject{page, NewPlaceholder("Codes", icons.Codes, back), NewPlaceholder("Settings", icons.Settings, back)}
	content := container.NewStack(page)
	navigation.OnSelected = func(index int) { content.Objects = []fyne.CanvasObject{pages[index]}; content.Refresh() }
	refresh()
	return container.NewBorder(nil, container.NewVBox(widget.NewSeparator(), navigation), nil, nil, content)
}

func itemCountLabel(count int) string {
	if count == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d items", count)
}

func newSelectionMenuRow(text string, icon fyne.Resource, selected bool, tapped func()) fyne.CanvasObject {
	button := widget.NewButton("", tapped)
	button.Importance = widget.LowImportance
	label := newText(text, bodyStyle)
	label.Wrapping = fyne.TextWrapOff
	check := widget.NewIcon(theme.ConfirmIcon())
	if !selected {
		check.Hide()
	}
	iconSize := fyne.NewSquareSize(theme.Size(theme.SizeNameInlineIcon))
	content := container.NewBorder(nil, nil, widget.NewIcon(icon), container.NewCenter(container.NewGridWrap(iconSize, check)), label)
	return container.NewStack(button, container.NewPadded(content))
}
