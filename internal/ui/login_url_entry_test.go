// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
)

func TestLoginURLEntryFetchesOnFocusLossAndRejectsStaleFavicon(t *testing.T) {
	test.NewTempApp(t)
	settings := model.FaviconDownloaderSettings{Disabled: true}
	entry := NewLoginURLEntry(context.Background(), "https://old.example", settings)
	var received []*model.Favicon
	entry.FaviconListener = func(icon *model.Favicon) { received = append(received, icon) }

	entry.SetText("https://new.example")
	entry.notifyFavicon(model.NewFavicon("old.example", []byte("old"), "png"), "old.example")
	require.Empty(t, received, "an older host response must not update the preview")

	entry.FocusLost()
	require.Len(t, received, 2, "the old icon is cleared before the disabled downloader returns its fallback")
	require.Nil(t, received[0])
	require.Nil(t, received[1])
}
