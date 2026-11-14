// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	pawsync "go.lucor.dev/paw/internal/sync"
)

func TestSyncDiscoveryHidesStoredOfflinePeers(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	paired := map[string]syncConnectDevice{
		"paired": {Peer: pawsync.Peer{Fingerprint: "paired", Name: "device"}, Name: "device", Trusted: true},
	}
	discovered := map[string]syncConnectDevice{
		"new": {Peer: pawsync.Peer{Fingerprint: "new", Name: "device"}, Name: "device", Online: true},
	}
	var emptyInitially, shown, searchingAgain bool
	fyne.DoAndWait(func() {
		view := newSyncConnectView(false, func() {}, func() {}, func() {}, nil, nil)
		view.SetDevices(syncConnectDeviceSnapshot(nil, paired))
		emptyInitially = len(view.deviceList) == 0 && view.searching.Visible()
		view.SetDevices(syncConnectDeviceSnapshot(discovered, paired))
		shown = len(view.deviceList) == 1 && !view.searching.Visible()
		view.SetDevices(nil)
		searchingAgain = view.searching.Visible()
		view.activity.Stop()
	})
	require.True(t, emptyInitially)
	require.True(t, shown)
	require.True(t, searchingAgain)
	require.Len(t, paired, 1)
	discovered["paired"] = syncConnectDevice{Peer: pawsync.Peer{Fingerprint: "paired", Name: "device"}, Name: "device", Online: true}
	devices := syncConnectDeviceSnapshot(discovered, paired)
	require.Len(t, devices, 2)
	require.True(t, devices[0].Trusted)
	require.Equal(t, "Paired", devices[0].Status)
	require.False(t, devices[1].Trusted)
}

func TestSyncAndCloneViewsHideVaultSubtitle(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	for _, tc := range []struct {
		name  string
		clone bool
		title string
	}{
		{name: "sync", title: "Sync"},
		{name: "clone", clone: true, title: "Clone from device"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			labels := make(map[string]bool)
			fyne.DoAndWait(func() {
				view := newSyncConnectView(tc.clone, func() {}, func() {}, func() {}, nil, nil)
				for _, object := range test.LaidOutObjects(view.view) {
					if label, ok := object.(*widget.Label); ok {
						labels[label.Text] = true
					}
				}
			})
			require.True(t, labels[tc.title])
			require.True(t, labels["Looking for devices. Ready to be found."])
			require.False(t, labels["Connect to"])
			for label := range labels {
				require.False(t, strings.HasPrefix(label, "Vault:"), "unexpected discovery subtitle %q", label)
			}
		})
	}
}
