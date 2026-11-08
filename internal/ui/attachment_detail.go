// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

type attachmentDetailActions struct {
	files  []cxf.File
	add    func()
	save   func(cxf.File)
	remove func(cxf.File)
}

func newAttachmentDetailSection(window fyne.Window, actions attachmentDetailActions) fyne.CanvasObject {
	heading := newText(fmt.Sprintf("Attachments · %d", len(actions.files)), captionStyle)
	add := widget.NewButtonWithIcon("Add", icons.Plus, actions.add)
	add.Importance = widget.LowImportance
	content := []fyne.CanvasObject{container.NewBorder(nil, nil, heading, add, nil)}
	if len(actions.files) == 0 {
		content = append(content, newText("No attachments", captionStyle))
	}
	for _, file := range actions.files {
		file := file
		content = append(content, newAttachmentRow(window, file, actions.save, actions.remove))
	}
	return container.NewVBox(content...)
}

func newHistoricalAttachmentSection(files []cxf.File) fyne.CanvasObject {
	content := []fyne.CanvasObject{newText(fmt.Sprintf("Attachments · %d", len(files)), captionStyle)}
	for _, file := range files {
		content = append(content, newAttachmentRow(nil, file, nil, nil))
	}
	return container.NewVBox(content...)
}

func newAttachmentRow(window fyne.Window, file cxf.File, saveAttachment, removeAttachment func(cxf.File)) fyne.CanvasObject {
	row := NewActionRow(file.Name, formatAttachmentSize(file.DecryptedSize), icons.File, nil)
	row.SetFlat(true)
	row.SetInteractive(false)
	var actions []fyne.CanvasObject
	if saveAttachment != nil {
		actions = append(actions, newIconButton("Save "+file.Name, icons.Download, func() { saveAttachment(file) }))
	}
	if removeAttachment != nil {
		actions = append(actions, newIconButton("Remove "+file.Name, icons.Trash, func() {
			showDeleteConfirmation("Remove attachment?", "Remove attachment", fmt.Sprintf("Remove %q from this item?", file.Name), func() { removeAttachment(file) }, window)
		}))
	}
	if len(actions) > 0 {
		row.SetTrailingActions(actions...)
	}
	return row
}

func formatAttachmentSize(size uint64) string {
	const unit = uint64(1024)
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	for _, name := range units {
		value /= float64(unit)
		if value < float64(unit) || name == "EiB" {
			return fmt.Sprintf("%.1f %s", value, name)
		}
	}
	return fmt.Sprintf("%.1f EiB", value)
}

func chooseAttachmentFile(window fyne.Window, selected func(fyne.URIReadCloser)) {
	picker := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, window)
			return
		}
		if reader != nil {
			selected(reader)
		}
	}, window)
	configureFilePicker(picker)
	picker.Show()
}

func chooseAttachmentDestination(window fyne.Window, file cxf.File, save func(string)) {
	picker := dialog.NewFolderOpen(func(folder fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, window)
			return
		}
		if folder == nil {
			return
		}
		if folder.Scheme() != "file" || folder.Path() == "" {
			dialog.ShowError(fmt.Errorf("choose a local destination folder"), window)
			return
		}
		showAttachmentFilename(window, folder.Path(), file.Name, save)
	}, window)
	configureFilePicker(picker)
	picker.Show()
}

func showAttachmentFilename(window fyne.Window, directory, proposed string, save func(string)) {
	filename := widget.NewEntry()
	filename.SetText(proposed)
	content := container.NewVBox(newText("Choose a file name for the saved attachment.", bodyStyle), widget.NewForm(widget.NewFormItem("File name", filename)))
	dialog.NewCustomConfirm("Save attachment", "Save", "Cancel", content, func(confirm bool) {
		if !confirm {
			return
		}
		name := filename.Text
		if strings.TrimSpace(name) == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\\`) || filepath.Base(name) != name {
			dialog.ShowError(fmt.Errorf("enter a file name without a path"), window)
			return
		}
		save(filepath.Join(directory, name))
	}, window).Show()
}
