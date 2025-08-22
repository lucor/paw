// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package browser

import (
	"log"
	"os"
	"strings"

	"lucor.dev/paw/internal/browser/messaging"
	"lucor.dev/paw/internal/paw"
)

func MessageFromExtension(args []string) bool {
	if strings.HasPrefix(args[0], "chrome-extension") {
		log.Println("message from chrome extension:", args[0])
		return true
	}
	if strings.Contains(args[0], nativeMessagingManifestFileName) && strings.Contains(firefoxExtensionIDs, args[1]) {
		log.Println("message from chrome extension:", args[0])
		return true
	}
	return false
}

func HandleNativeMessage(s paw.Storage) {
	m := messaging.NewPawMux(
		&messaging.CreateVaultHandler{Storage: s},
		&messaging.ListVaultHandler{Storage: s},
		&messaging.UnlockVaultHandler{Storage: s},
		&messaging.ListItemsVaultHandler{Storage: s},
		&messaging.GetTLDPlusOneHandler{},
		&messaging.GetLoginItemHandler{Storage: s},
		&messaging.GetAppStateHandler{Storage: s},
	)
	log.Println("[browser] starting native messaging listener")
	err := m.Handle(os.Stdout, os.Stdin)
	if err != nil {
		log.Println("[browser] could not handle message:", err)
	}
}
