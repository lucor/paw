// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// entryDetailCredentials holds the supported credentials decoded for display.
type entryDetailCredentials struct {
	basicAuth      *cxf.BasicAuth
	basicAuthIndex int
	note           *cxf.Note
	noteIndex      int
	totp           *cxf.TOTP
	totpIndex      int
	sshKey         *cxf.SSHKey
	sshKeyIndex    int
}

func decodeEntryDetailCredentials(raws []json.RawMessage) (entryDetailCredentials, error) {
	decoded := entryDetailCredentials{basicAuthIndex: -1, noteIndex: -1, totpIndex: -1, sshKeyIndex: -1}
	for index, raw := range raws {
		typeName, err := cxf.CredentialTypeOf(raw)
		if err != nil {
			return decoded, err
		}
		switch typeName {
		case cxf.CredentialTypeBasicAuth:
			if decoded.basicAuth != nil {
				continue
			}
			var credential cxf.BasicAuth
			if err := json.Unmarshal(raw, &credential); err != nil {
				return decoded, err
			}
			decoded.basicAuth = &credential
			decoded.basicAuthIndex = index
		case cxf.CredentialTypeNote:
			if decoded.note != nil {
				continue
			}
			var credential cxf.Note
			if err := json.Unmarshal(raw, &credential); err != nil {
				return decoded, err
			}
			decoded.note = &credential
			decoded.noteIndex = index
		case cxf.CredentialTypeTOTP:
			if decoded.totp != nil {
				continue
			}
			var credential cxf.TOTP
			if err := json.Unmarshal(raw, &credential); err != nil {
				return decoded, err
			}
			decoded.totp = &credential
			decoded.totpIndex = index
		case cxf.CredentialTypeSSHKey:
			if decoded.sshKey != nil {
				continue
			}
			var credential cxf.SSHKey
			if err := json.Unmarshal(raw, &credential); err != nil {
				return decoded, err
			}
			decoded.sshKey = &credential
			decoded.sshKeyIndex = index
		}
	}
	return decoded, nil
}

// NewEntryDetail displays the CXF credentials of a supported entry.
func NewEntryDetail(ctx context.Context, item *model.Item, window fyne.Window, back, edit, deleteEntry func()) (fyne.CanvasObject, error) {
	view, _, err := newEntryDetailWithClear(ctx, item, window, back, edit, deleteEntry)
	return view, err
}

func newEntryDetailWithClear(ctx context.Context, item *model.Item, window fyne.Window, back, edit, deleteEntry func()) (fyne.CanvasObject, func(), error) {
	if err := item.Validate(); err != nil {
		return nil, nil, err
	}
	var resource fyne.Resource
	switch item.Metadata.Type {
	case paw.LoginItemType:
		resource = icons.Globe
		if item.Metadata.Favicon != nil && len(item.Metadata.Favicon.Data) > 0 {
			resource = item.Metadata.Favicon
		}
	case paw.PasswordItemType:
		resource = icons.Password
	case paw.NoteItemType:
		resource = icons.Note
	case paw.SSHKeyItemType:
		resource = icons.SSH
	default:
		return nil, nil, fmt.Errorf("unsupported entry category")
	}
	decoded, err := decodeEntryDetailCredentials(item.Credentials)
	if err != nil {
		return nil, nil, err
	}
	var bodyObjects []fyne.CanvasObject
	var clearSensitive []*detailSecret
	if item.Metadata.Type == paw.SSHKeyItemType {
		if decoded.sshKey == nil {
			return nil, nil, fmt.Errorf("SSH key credential is missing")
		}
		sshDetails, secrets, err := detailSSHKey(window, item.Title, *decoded.sshKey, decoded.basicAuth)
		if err != nil {
			return nil, nil, err
		}
		bodyObjects = append(bodyObjects, sshDetails...)
		clearSensitive = append(clearSensitive, secrets...)
	} else {
		var credentialFields []fyne.CanvasObject
		if auth := decoded.basicAuth; auth != nil {
			if auth.Username != nil && auth.Username.Value != "" {
				credentialFields = append(credentialFields, newDetailField("Username", newText(auth.Username.Value, bodyStyle), newIconButton("Copy Username", icons.Copy, func() { detailCopy("Username", auth.Username.Value) })))
			}
			if auth.Password != nil {
				field, secret := detailSecretField("Password", auth.Password.Value)
				if field != nil {
					credentialFields = append(credentialFields, field)
					clearSensitive = append(clearSensitive, secret)
				}
			}
		}
		if item.Scope != nil {
			for _, address := range item.Scope.URLs {
				if website := detailWebsite(address); website != nil {
					credentialFields = append(credentialFields, website)
				}
			}
		}
		if len(credentialFields) > 0 {
			appendDetailSection(&bodyObjects, container.New(layout.NewCustomPaddedVBoxLayout(theme.Size(theme.SizeNamePadding)), credentialFields...))
		}
		if decoded.totp != nil {
			appendDetailSection(&bodyObjects, detailTOTP(ctx, *decoded.totp))
		}
		if note := decoded.note; note != nil && note.Content.Value != "" {
			copyNotes := newIconButton("Copy Notes", icons.Copy, func() { detailCopy("Notes", note.Content.Value) })
			noteValue := newText(preserveLineEdgeSpaces(note.Content.Value), bodyStyle)
			noteValue.Wrapping = fyne.TextWrapBreak
			noteValue.Selectable = true
			appendDetailSection(&bodyObjects, newDetailMultilineField("Notes", noteValue, copyNotes))
		}
	}
	var clearOnce sync.Once
	clear := func() {
		clearOnce.Do(func() { clearDetailSecrets(clearSensitive) })
	}
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
	backButton := newIconButton("Back", icons.Back, func() { clear(); back() })
	backButton.Importance = widget.LowImportance
	title := newText(item.Title, titleStyle)
	title.Truncation = fyne.TextTruncateEllipsis
	headerTitle := container.NewBorder(nil, nil, container.NewPadded(newEntryIcon(resource)), nil, title)
	header := newPageHeader(backButton, headerTitle, more)
	dates := []string{}
	for _, date := range []struct {
		label string
		value *uint64
	}{{"Created", item.CreationAt}, {"Modified", item.ModifiedAt}} {
		if date.value == nil {
			continue
		}
		value := time.Unix(int64(*date.value), 0).Local().Format("Jan 2, 2006")
		dates = append(dates, date.label+" · "+value)
	}
	var metadata fyne.CanvasObject
	if len(dates) > 0 {
		metadata = newDetailMetadata(dates)
	}
	body := newDetailScrollableContent(bodyObjects, metadata)
	pagePadding := theme.Size(theme.SizeNamePadding) * 2
	page := container.New(layout.NewCustomPaddedLayout(pagePadding, pagePadding, pagePadding, pagePadding), container.NewBorder(header, nil, nil, nil, container.NewVScroll(body)))
	if done := ctx.Done(); done != nil {
		go func() {
			<-done
			fyne.DoAndWait(clear)
		}()
	}
	return page, clear, nil
}

func detailSSHKey(window fyne.Window, title string, credential cxf.SSHKey, passphrase *cxf.BasicAuth) ([]fyne.CanvasObject, []*detailSecret, error) {
	privateText, publicText, fingerprint, err := FormatSSHKey(credential)
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
	parsed, err := url.Parse(address)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return newDetailField("Website", newText(address, bodyStyle))
	}
	link := widget.NewHyperlink(address, parsed)
	link.SizeName = bodyStyle.size
	link.Wrapping = fyne.TextWrapBreak
	open := newIconButton("Open website", icons.External, func() {
		if err := fyne.CurrentApp().OpenURL(parsed); err != nil {
			fyne.LogError("could not open website", err)
		}
	})
	return newDetailField("Website", link, open)
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

func detailTOTP(ctx context.Context, credential cxf.TOTP) fyne.CanvasObject {
	display, ok := newTOTPDisplay(credential)
	if !ok {
		return nil
	}
	go display.Run(ctx)
	return container.NewVBox(newText("Verification code", captionStyle), display.View())
}
