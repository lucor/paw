// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

// exportToFile opens the active vault's export dialog.
func (a *app) exportToFile() { ShowExportCredentials(a.win, a.storage, a.vault) }
