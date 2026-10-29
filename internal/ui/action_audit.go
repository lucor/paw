// SPDX-FileCopyrightText: 2022-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"go.lucor.dev/paw/internal/haveibeenpwned"
)

// makeAuditPasswordView connects the transient audit to the active vault.
func (a *app) makeAuditPasswordView() fyne.CanvasObject {
	vault, store := a.vault, a.storage
	name, _ := vault.Name()
	auditor := NewPasswordAuditor(vault, store, haveibeenpwned.Client{})
	a.audit = NewPasswordAudit(name, auditor, a.win.SetContent, func(id string) {
		item, err := store.LoadItem(vault, id)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		a.showItemView(item)
	}, a.showCurrentVaultView)
	return a.audit.View
}
