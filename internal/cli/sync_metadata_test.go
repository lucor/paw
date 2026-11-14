// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	pawsync "go.lucor.dev/paw/internal/sync"
)

func TestSyncItemNameUsesRemoteManifestMetadata(t *testing.T) {
	item := &pawsync.ManifestItem{Type: model.LoginItemType, Title: "GitHub"}
	require.Equal(t, "Login / GitHub", syncItemName(item))
	require.Equal(t, "Item", syncItemName(nil))
	require.Equal(t, "Login / Untitled item", syncItemName(&pawsync.ManifestItem{Type: model.LoginItemType}))
	require.Equal(t, "Item / Unknown", syncItemName(&pawsync.ManifestItem{Type: 0, Title: "Unknown"}))
}

func TestFormatSyncVersionTimesUsesEachManifestTimestamp(t *testing.T) {
	localTime := uint64(1794357000)
	remoteTime := uint64(1794443700)
	local := &pawsync.ManifestItem{ModifiedAt: &localTime}
	remote := &pawsync.ManifestItem{ModifiedAt: &remoteTime}
	want := "This device: " + time.Unix(int64(localTime), 0).Local().Format("Jan 2, 15:04") +
		"\nMacBook:     " + time.Unix(int64(remoteTime), 0).Local().Format("Jan 2, 15:04")
	require.Equal(t, want, formatSyncVersionTimes(local, remote, "MacBook"))
	require.Equal(t, "Unknown", formatSyncTimestamp(&pawsync.ManifestItem{}))
}
