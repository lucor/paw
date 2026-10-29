// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import "go.lucor.dev/paw/internal/ui/v2"

// exportToFile opens the active vault's export dialog.
func (a *app) exportToFile() { v2.ShowExportCredentials(a.win, a.storage, a.vault) }
