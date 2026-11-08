// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// NewEntryDetail displays the CXF credentials of a supported entry.
func NewEntryDetail(ctx context.Context, item *model.Item, window fyne.Window, back, edit, deleteEntry func()) (fyne.CanvasObject, error) {
	view, clear, err := newEntryDetailWithClear(ctx, item, window, back, edit, deleteEntry)
	if err == nil {
		if done := ctx.Done(); done != nil {
			go func() {
				<-done
				fyne.DoAndWait(clear)
			}()
		}
	}
	return view, err
}

func newEntryDetailWithClear(ctx context.Context, item *model.Item, window fyne.Window, back, edit, deleteEntry func()) (fyne.CanvasObject, func(), error) {
	return newEntryDetailWithActions(ctx, item, window, back, edit, deleteEntry, false, 0, nil, nil, nil)
}

func newEntryDetailWithHistory(ctx context.Context, item *model.Item, window fyne.Window, back, edit, deleteEntry, openHistory func()) (fyne.CanvasObject, func(), error) {
	return newEntryDetailWithActions(ctx, item, window, back, edit, deleteEntry, false, 0, openHistory, nil, nil)
}

func newEntryDetailWithHistoryAndAttachments(ctx context.Context, item *model.Item, window fyne.Window, back, edit, deleteEntry, openHistory func(), attachments *attachmentDetailActions) (fyne.CanvasObject, func(), error) {
	return newEntryDetailWithActions(ctx, item, window, back, edit, deleteEntry, false, 0, openHistory, nil, attachments)
}

func newHistoricalEntryDetailWithClear(ctx context.Context, item *model.Item, window fyne.Window, capturedAt uint64, back, restore func()) (fyne.CanvasObject, func(), error) {
	return newEntryDetailWithActions(ctx, item, window, back, nil, nil, true, capturedAt, nil, restore, nil)
}

func newEntryDetailWithActions(
	ctx context.Context,
	item *model.Item,
	window fyne.Window,
	back, edit, deleteEntry func(),
	historical bool,
	capturedAt uint64,
	openHistory, restore func(),
	attachments *attachmentDetailActions,
) (fyne.CanvasObject, func(), error) {
	if err := item.Validate(); err != nil {
		return nil, nil, err
	}
	var resource fyne.Resource
	switch item.Metadata.Type {
	case model.LoginItemType:
		resource = icons.Globe
		if item.Metadata.Favicon != nil && len(item.Metadata.Favicon.Data) > 0 {
			resource = item.Metadata.Favicon
		}
	case model.PasswordItemType:
		resource = icons.Password
	case model.NoteItemType:
		resource = icons.Note
	case model.SSHKeyItemType:
		resource = icons.SSH
	default:
		return nil, nil, fmt.Errorf("unsupported entry category")
	}
	decoded, err := model.DecodeCredentials(item.Credentials)
	if err != nil {
		return nil, nil, err
	}
	var bodyObjects []fyne.CanvasObject
	var clearSensitive []*detailSecret
	var clearDisplays []*TOTPDisplay
	if item.Metadata.Type == model.SSHKeyItemType {
		if decoded.SSHKey == nil {
			return nil, nil, fmt.Errorf("SSH key credential is missing")
		}
		sshDetails, secrets, err := detailSSHKey(window, item.Title, *decoded.SSHKey, decoded.BasicAuth)
		if err != nil {
			return nil, nil, err
		}
		bodyObjects = append(bodyObjects, sshDetails...)
		clearSensitive = append(clearSensitive, secrets...)
	} else {
		if auth := decoded.BasicAuth; auth != nil {
			if auth.Username != nil && auth.Username.Value != "" {
				appendDetailSection(&bodyObjects, newDetailField("Username", newText(auth.Username.Value, bodyStyle), newIconButton("Copy Username", icons.Copy, func() { detailCopy("Username", auth.Username.Value) })))
			}
			if auth.Password != nil {
				field, secret := detailSecretField("Password", auth.Password.Value)
				if field != nil {
					appendDetailSection(&bodyObjects, field)
					clearSensitive = append(clearSensitive, secret)
				}
			}
		}
		if item.Scope != nil {
			for _, address := range item.Scope.URLs {
				if website := detailWebsite(address); website != nil {
					appendDetailSection(&bodyObjects, website)
				}
			}
		}
		if decoded.TOTP != nil {
			display, view := detailTOTP(ctx, *decoded.TOTP)
			if display != nil {
				clearDisplays = append(clearDisplays, display)
			}
			appendDetailSection(&bodyObjects, view)
		}
		if note := decoded.Note; note != nil && note.Content.Value != "" {
			copyNotes := newIconButton("Copy Notes", icons.Copy, func() { detailCopy("Notes", note.Content.Value) })
			noteValue := newText(preserveLineEdgeSpaces(note.Content.Value), bodyStyle)
			noteValue.Wrapping = fyne.TextWrapBreak
			noteValue.Selectable = true
			appendDetailSection(&bodyObjects, newDetailMultilineField("Notes", noteValue, copyNotes))
		}
	}
	if attachments != nil && !historical {
		appendDetailSection(&bodyObjects, newAttachmentDetailSection(window, *attachments))
	} else if historical && (item.Metadata.Type == model.NoteItemType || item.Metadata.Type == model.LoginItemType) {
		files, err := item.Files()
		if err != nil {
			return nil, nil, fmt.Errorf("cannot read historical attachment metadata: %w", err)
		}
		if len(files) > 0 {
			appendDetailSection(&bodyObjects, newHistoricalAttachmentSection(files))
		}
	}
	var clearOnce sync.Once
	clear := func() {
		clearOnce.Do(func() {
			clearDetailSecrets(clearSensitive)
			for _, display := range clearDisplays {
				display.Clear()
			}
		})
	}
	backButton := newIconButton("Back", icons.Back, func() { clear(); back() })
	backButton.Importance = widget.LowImportance
	title := newText(item.Title, titleStyle)
	title.Truncation = fyne.TextTruncateEllipsis
	headerTitle := container.NewBorder(nil, nil, container.NewPadded(newEntryIcon(resource)), nil, title)
	var trailing fyne.CanvasObject
	if historical {
		restoreButton := widget.NewButton("Restore", restore)
		restoreButton.Importance = widget.HighImportance
		trailing = restoreButton
	} else {
		more := newIconButton("Entry actions", icons.More, nil)
		more.Importance = widget.LowImportance
		editItem := fyne.NewMenuItem("Edit entry", edit)
		editItem.Icon = icons.Edit
		deleteItem := fyne.NewMenuItem("Delete entry", deleteEntry)
		deleteItem.Icon = icons.Delete
		more.OnTapped = func() {
			widget.ShowPopUpMenuAtRelativePosition(fyne.NewMenu("", editItem, fyne.NewMenuItemSeparator(), deleteItem), window.Canvas(), fyne.NewPos(0, more.Size().Height), more)
		}
		editItem.Action = func() { clear(); edit() }
		deleteItem.Action = func() { clear(); deleteEntry() }
		trailing = more
	}
	header := newPageHeader(backButton, headerTitle, trailing)
	if historical {
		historyContext := newText("History · "+formatDetailTimestamp(capturedAt), captionStyle)
		historyContext.Alignment = fyne.TextAlignTrailing
		header = container.NewVBox(
			header,
			historyContext,
		)
	}
	dates := []string{}
	for _, date := range []struct {
		label string
		value *uint64
	}{{"Created", item.CreationAt}, {"Modified", item.ModifiedAt}} {
		if date.value == nil {
			continue
		}
		value := formatDetailTimestamp(*date.value)
		dates = append(dates, date.label+" · "+value)
	}
	var metadata fyne.CanvasObject
	if len(dates) > 0 {
		metadata = newDetailMetadata(dates)
	}
	var footerAction fyne.CanvasObject
	if !historical {
		if history, err := item.History(); err == nil && len(history.Entries) > 0 && openHistory != nil {
			historyButton := widget.NewButtonWithIcon("History", theme.HistoryIcon(), openHistory)
			historyButton.Importance = widget.LowImportance
			footerAction = historyButton
		}
	}
	if footerAction != nil {
		metadata = container.NewBorder(nil, nil, nil, footerAction, metadata)
	}
	if len(bodyObjects) > 0 {
		bodyObjects = append([]fyne.CanvasObject{widget.NewSeparator()}, bodyObjects...)
	}
	body := newDetailScrollableContent(bodyObjects, metadata)
	pagePadding := theme.Size(theme.SizeNamePadding) * 2
	page := container.New(layout.NewCustomPaddedLayout(pagePadding, pagePadding, pagePadding, pagePadding), container.NewBorder(header, nil, nil, nil, container.NewVScroll(body)))
	return page, clear, nil
}

func detailSSHKey(window fyne.Window, title string, credential cxf.SSHKey, passphrase *cxf.BasicAuth) ([]fyne.CanvasObject, []*detailSecret, error) {
	privateText, publicText, fingerprint, err := model.FormatSSHKey(credential)
	if err != nil {
		return nil, nil, err
	}
	var details []fyne.CanvasObject
	var secrets []*detailSecret
	if passphrase != nil && passphrase.Password != nil {
		field, secret := detailSecretField("Passphrase", passphrase.Password.Value)
		if field != nil {
			appendDetailSection(&details, field)
			secrets = append(secrets, secret)
		}
	}
	algorithm := newText(strings.ToUpper(strings.TrimPrefix(credential.KeyType, "ssh-")), bodyStyle)
	algorithm.Wrapping = fyne.TextWrapOff
	appendDetailSection(&details, container.NewBorder(nil, nil, newText("SSH key", captionStyle), algorithm, nil))
	fingerprintValue := newText(fingerprint, bodyStyle)
	fingerprintValue.TextStyle.Monospace = true
	fingerprintValue.Wrapping = fyne.TextWrapBreak
	copyFingerprint := newIconButton("Copy fingerprint", icons.Copy, func() { detailCopy("Fingerprint", fingerprint) })
	appendDetailSection(&details, newDetailField("Fingerprint", fingerprintValue, copyFingerprint))

	publicKey := newText(publicText, bodyStyle)
	publicKey.TextStyle.Monospace = true
	publicKey.Wrapping = fyne.TextWrapBreak
	publicKey.Selectable = true
	filename := sshKeyFilename(title)
	copyPublic := newIconButton("Copy public key", icons.Copy, func() { detailCopy("Public key", publicText) })
	downloadPublic := newIconButton("Download public key", icons.Download, func() { detailDownload(window, filename+".pub", publicText) })
	appendDetailSection(&details, newDetailMultilineField("Public key", publicKey, copyPublic, downloadPublic))
	privateBody := newText("", bodyStyle)
	privateBody.TextStyle.Monospace = true
	privateBody.Wrapping = fyne.TextWrapBreak
	privateBody.Selectable = true
	toggle := newIconButton("Show private key", icons.Show, nil)
	privateSurface := widget.NewCard("", "", nil)
	privateSurface.Hide()
	privateValue := &detailSecret{value: privateText, text: privateBody, button: toggle, label: "private key", surface: privateSurface}
	toggle.OnTapped = privateValue.Toggle
	copyPrivate := newIconButton("Copy private key", icons.Copy, func() { detailCopy("Private key", privateValue.value) })
	downloadPrivate := newIconButton("Download private key", icons.Download, func() { detailDownload(window, filename+".key", privateValue.value) })
	privateHeader := newDetailFieldHeader("Private key", toggle, copyPrivate, downloadPrivate)
	appendDetailSection(&details, container.NewVBox(privateHeader, privateSurface))
	secrets = append(secrets, privateValue)
	return details, secrets, nil
}

func sshKeyFilename(title string) string {
	filename := strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(strings.TrimSpace(title))
	if filename == "" || filename == "." || filename == ".." {
		return "ssh-key"
	}
	return filename
}

func detailDownload(window fyne.Window, filename, value string) {
	save := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, window)
			return
		}
		if writer == nil {
			return
		}
		data := []byte(value)
		written, err := writer.Write(data)
		if err == nil && written != len(data) {
			err = io.ErrShortWrite
		}
		if err != nil {
			_ = writer.Close()
			dialog.ShowError(err, window)
			return
		}
		if err := writer.Close(); err != nil {
			dialog.ShowError(err, window)
			return
		}
		fyne.CurrentApp().SendNotification(&fyne.Notification{Title: "Paw", Content: filename + " saved"})
	}, window)
	save.SetFileName(filename)
	configureFilePicker(save)
	save.Show()
}

func detailCopy(label, value string) {
	fyne.CurrentApp().Clipboard().SetContent(value)
	fyne.CurrentApp().SendNotification(&fyne.Notification{Title: "Paw", Content: label + " copied"})
}

func detailWebsite(address string) fyne.CanvasObject {
	if strings.TrimSpace(address) == "" {
		return nil
	}
	copyWebsite := newIconButton("Copy Website", icons.Copy, func() { detailCopy("Website", address) })
	parsed, err := url.Parse(address)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return newDetailField("Website", newText(address, bodyStyle), copyWebsite)
	}
	link := widget.NewHyperlink(address, parsed)
	link.SizeName = bodyStyle.size
	link.Wrapping = fyne.TextWrapBreak
	open := newIconButton("Open website", icons.External, func() {
		if err := fyne.CurrentApp().OpenURL(parsed); err != nil {
			fyne.LogError("could not open website", err)
		}
	})
	return newDetailField("Website", link, open, copyWebsite)
}

// preserveLineEdgeSpaces keeps leading and trailing spaces visible in Fyne labels.
func preserveLineEdgeSpaces(value string) string {
	lines := strings.Split(value, "\n")
	for index, line := range lines {
		leading := len(line) - len(strings.TrimLeft(line, " "))
		trailing := len(line) - len(strings.TrimRight(line, " "))
		if leading+trailing > len(line) {
			trailing = len(line) - leading
		}
		lines[index] = strings.Repeat("\u00a0", leading) + line[leading:len(line)-trailing] + strings.Repeat("\u00a0", trailing)
	}
	return strings.Join(lines, "\n")
}

func detailTOTP(ctx context.Context, credential cxf.TOTP) (*TOTPDisplay, fyne.CanvasObject) {
	display, ok := newTOTPDisplay(credential)
	if !ok {
		return nil, nil
	}
	go display.Run(ctx)
	return display, container.NewVBox(newText("Verification code", captionStyle), display.View())
}
