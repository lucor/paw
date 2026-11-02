// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

// exportToFile opens the active vault's dedicated export view.
func (a *app) exportToFile() {
	if a.vault == nil {
		return
	}
	a.closeItemDetail()
	a.closePasswordAudit()
	a.win.SetContent(NewExportCredentialsView(a.win, a.storage, a.vault, a.showCurrentVaultView))
}
