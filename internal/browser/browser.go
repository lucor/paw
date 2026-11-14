// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package browser implements the bridge to Paw's browser extension.
package browser

import (
	"log/slog"
	"os"
	"strings"

	"go.lucor.dev/paw/internal/browser/messaging"
	"go.lucor.dev/paw/internal/storage"
)

func MessageFromExtension(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if strings.HasPrefix(args[0], "chrome-extension") {
		slog.Debug("native messaging request detected", "component", "browser", "browser", "chrome")
		return true
	}
	if len(args) > 1 && strings.Contains(args[0], nativeMessagingManifestFileName) && strings.Contains(firefoxExtensionIDs, args[1]) {
		slog.Debug("native messaging request detected", "component", "browser", "browser", "firefox")
		return true
	}
	return false
}

func HandleNativeMessage(s *storage.FileStorage) {
	m := messaging.NewPawMux(
		&messaging.CreateVaultHandler{Storage: s},
		&messaging.ListVaultHandler{Storage: s},
		&messaging.UnlockVaultHandler{Storage: s},
		&messaging.ListItemsVaultHandler{Storage: s},
		&messaging.GetTLDPlusOneHandler{},
		&messaging.GetLoginItemHandler{Storage: s},
		&messaging.GetAppStateHandler{Storage: s},
	)
	slog.Debug("starting native messaging listener", "component", "browser")
	err := m.Handle(os.Stdout, os.Stdin)
	if err != nil {
		slog.Error("could not handle native messaging request", "component", "browser", "error", err)
	}
}
