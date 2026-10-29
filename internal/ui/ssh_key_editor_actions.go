// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"crypto"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/sshkey"
	"golang.org/x/crypto/ssh"
)

// ConnectWindow connects native file dialogs and clipboard actions to the editor.
func (e *SSHKeyEditor) ConnectWindow(window fyne.Window) {
	e.GenerateButton.OnTapped = func() {
		key, err := sshkey.GenerateKey()
		if err != nil {
			dialog.ShowError(err, window)
			return
		}
		e.usePrivateKey(key.PrivateKey(), key.PublicKey().Type(), e.Passphrase.Text, window)
	}
	e.ImportButton.OnTapped = func() { e.importKey(window) }
	e.CopyFingerprintButton.OnTapped = func() { detailCopy("Fingerprint", e.fingerprint) }
	e.CopyPublicButton.OnTapped = func() { detailCopy("Public key", e.publicText) }
	e.ExportPublicButton.OnTapped = func() { detailDownload(window, sshKeyFilename(e.Title.Text)+".pub", e.publicText) }
	e.CopyPrivateButton.OnTapped = func() {
		private, err := e.PrivateKeyText()
		if err != nil {
			dialog.ShowError(err, window)
			return
		}
		detailCopy("Private key", private)
	}
	e.ExportPrivateButton.OnTapped = func() {
		private, err := e.PrivateKeyText()
		if err != nil {
			dialog.ShowError(err, window)
			return
		}
		detailDownload(window, sshKeyFilename(e.Title.Text)+".key", private)
	}
}

func (e *SSHKeyEditor) usePrivateKey(private crypto.PrivateKey, keyType, passphrase string, window fyne.Window) {
	credential, err := encodeSSHKey(private, keyType, e.Title.Text)
	if err == nil {
		err = e.SetCredential(credential, passphrase)
	}
	if err != nil {
		dialog.ShowError(err, window)
	}
}

// GenerateSSHKey creates a key using Paw's existing SSH key generator.
func GenerateSSHKey() (cxf.SSHKey, error) {
	key, err := sshkey.GenerateKey()
	if err != nil {
		return cxf.SSHKey{}, err
	}
	return encodeSSHKey(key.PrivateKey(), key.PublicKey().Type(), "")
}

func encodeSSHKey(private crypto.PrivateKey, keyType, comment string) (cxf.SSHKey, error) {
	if key, ok := private.(*ed25519.PrivateKey); ok {
		private = *key
	}
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return cxf.SSHKey{}, fmt.Errorf("cannot encode SSH private key")
	}
	return cxf.SSHKey{Type: cxf.CredentialTypeSSHKey, KeyType: keyType, KeyComment: comment, PrivateKey: base64.RawURLEncoding.EncodeToString(der)}, nil
}

func (e *SSHKeyEditor) importKey(window fyne.Window) {
	dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, window)
			return
		}
		if reader == nil {
			return
		}
		defer reader.Close()
		data, err := io.ReadAll(reader)
		if err != nil {
			dialog.ShowError(err, window)
			return
		}
		key, err := sshkey.ParseKey(data)
		if err == nil {
			e.usePrivateKey(key.PrivateKey(), key.PublicKey().Type(), "", window)
			return
		}
		if _, ok := err.(*ssh.PassphraseMissingError); !ok {
			dialog.ShowError(err, window)
			return
		}
		entry := widget.NewPasswordEntry()
		dialog.ShowForm("Private key is password protected", "Import", "Cancel", []*widget.FormItem{widget.NewFormItem("Passphrase", entry)}, func(ok bool) {
			passphrase := entry.Text
			entry.SetText("")
			if !ok {
				return
			}
			key, err := sshkey.ParseKeyWithPassphrase(data, []byte(passphrase))
			if err != nil {
				dialog.ShowError(err, window)
				return
			}
			e.usePrivateKey(key.PrivateKey(), key.PublicKey().Type(), passphrase, window)
		}, window)
	}, window).Show()
}
