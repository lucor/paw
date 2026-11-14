// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"fmt"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	pawsync "go.lucor.dev/paw/internal/sync"
)

func TestSyncConnectViewShowsDevicesAndStartsConnections(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Sync")
	t.Cleanup(window.Close)
	var selected pawsync.Peer
	var selectedTrusted bool
	var manualLink, shared string
	var qrShown bool
	view := newSyncConnectView(false, func() {}, func() { shared = "link" }, func() { qrShown = true }, func(peer pawsync.Peer, trusted bool) {
		selected = peer
		selectedTrusted = trusted
	}, func(link string) { manualLink = link })
	view.SetDevices([]syncConnectDevice{
		{Peer: pawsync.Peer{ID: "nearby", Fingerprint: "phoenix-fingerprint", Name: "phoenix"}, Name: "phoenix", Status: "New", Online: true},
		{Peer: pawsync.Peer{ID: "known", Fingerprint: "fedora-fingerprint", Name: "fedora"}, Name: "fedora", Status: "Paired", Trusted: true, Online: true},
		{Peer: pawsync.Peer{Fingerprint: "laptop-fingerprint", Name: "laptop"}, Name: "laptop", Status: "Offline", Trusted: true},
	})
	view.SetReady(true)
	require.Len(t, view.deviceList, 2)
	window.SetContent(view.view)
	window.Resize(fyne.NewSize(320, 540))
	window.Show()

	labels := make(map[string]*widget.Label)
	badgeTexts := make(map[string]bool)
	for _, object := range test.LaidOutObjects(view.view) {
		if label, ok := object.(*widget.Label); ok {
			labels[label.Text] = label
		}
		if text, ok := object.(*canvas.Text); ok {
			badgeTexts[text.Text] = true
		}
	}
	for _, label := range []string{"Sync", "Connection link", "Share connection details", "phoenix", "fedora"} {
		require.Contains(t, labels, label)
	}
	for _, badge := range []string{"Paired", "New"} {
		require.True(t, badgeTexts[badge], "expected status badge %q", badge)
	}
	require.True(t, view.connect.Disabled())
	view.link.SetText(" ")
	require.True(t, view.connect.Disabled())
	view.link.SetText("")
	phoenix := labels["phoenix"]
	test.TapCanvas(window.Canvas(), app.Driver().AbsolutePositionForObject(phoenix).Add(fyne.NewPos(phoenix.Size().Width/2, phoenix.Size().Height/2)))
	require.Equal(t, "nearby", selected.ID)
	require.False(t, selectedTrusted)

	fedora := labels["fedora"]
	test.TapCanvas(window.Canvas(), app.Driver().AbsolutePositionForObject(fedora).Add(fyne.NewPos(fedora.Size().Width/2, fedora.Size().Height/2)))
	require.Equal(t, "known", selected.ID)
	require.True(t, selectedTrusted)

	validLink := "paw://127.0.0.1:61654/sync?fp=7a2a9d534f4b092dcc8e5a53f644508ed498fdd0203f387f717c74f8b63c118d"
	view.link.SetText("paw://device-link")
	require.True(t, view.connect.Disabled())
	view.link.SetText(validLink)
	require.False(t, view.connect.Disabled())
	test.Tap(view.connect)
	require.Equal(t, validLink, manualLink)

	var copyButton, qrButton *iconButton
	for _, object := range test.LaidOutObjects(view.view) {
		if button, ok := object.(*iconButton); ok {
			switch button.label {
			case "Copy connection link":
				copyButton = button
			case "Show connection QR code":
				qrButton = button
			}
		}
	}
	require.NotNil(t, copyButton)
	require.NotNil(t, qrButton)
	for _, object := range test.LaidOutObjects(view.view) {
		if scroll, ok := object.(*container.Scroll); ok {
			scroll.ScrollToBottom()
		}
	}
	test.TapCanvas(window.Canvas(), app.Driver().AbsolutePositionForObject(copyButton).Add(fyne.NewPos(copyButton.Size().Width/2, copyButton.Size().Height/2)))
	require.Equal(t, "link", shared)
	test.TapCanvas(window.Canvas(), app.Driver().AbsolutePositionForObject(qrButton).Add(fyne.NewPos(qrButton.Size().Width/2, qrButton.Size().Height/2)))
	require.True(t, qrShown)
	view.SetStatus("Could not connect.", true)
	view.SetDevices([]syncConnectDevice{{Peer: pawsync.Peer{ID: "other", Fingerprint: "other-fingerprint", Name: "other"}, Name: "other", Status: "New", Online: true}})
	require.Equal(t, "Could not connect.", view.status.Text)
}

func TestSyncConnectDevicesMergeByFingerprintAndSortByStatus(t *testing.T) {
	paired := map[string]syncConnectDevice{
		"fp-paired-z": {Peer: pawsync.Peer{Fingerprint: "fp-paired-z", Name: "zeta"}, Name: "zeta", Status: "Offline", Trusted: true},
		"fp-shared":   {Peer: pawsync.Peer{Fingerprint: "fp-shared", Name: "mu"}, Name: "mu", Status: "Offline", Trusted: true},
		"fp-offline":  {Peer: pawsync.Peer{Fingerprint: "fp-offline", Name: "offline"}, Name: "offline", Status: "Offline", Trusted: true},
	}
	discovered := map[string]syncConnectDevice{
		"fp-paired-z": {Peer: pawsync.Peer{Fingerprint: "fp-paired-z", Name: "zeta"}, Name: "zeta", Status: "New", Online: true},
		"fp-shared":   {Peer: pawsync.Peer{Fingerprint: "fp-shared", Name: "mu"}, Name: "mu", Status: "New", Online: true},
		"fp-new-z":    {Peer: pawsync.Peer{Fingerprint: "fp-new-z", Name: "alpha"}, Name: "alpha", Status: "New", Online: true},
		"fp-new-a":    {Peer: pawsync.Peer{Fingerprint: "fp-new-a", Name: "alpha"}, Name: "alpha", Status: "New", Online: true},
		"fp-new-b":    {Peer: pawsync.Peer{Fingerprint: "fp-new-b", Name: "beta"}, Name: "beta", Status: "New", Online: true},
	}
	devices := syncConnectDeviceSnapshot(discovered, paired)
	require.Len(t, devices, 6)
	require.Equal(t, []string{"Paired", "Paired", "New", "New", "New", "Offline"}, []string{devices[0].Status, devices[1].Status, devices[2].Status, devices[3].Status, devices[4].Status, devices[5].Status})
	require.Equal(t, []string{"mu", "zeta", "alpha", "alpha", "beta", "offline"}, []string{devices[0].Name, devices[1].Name, devices[2].Name, devices[3].Name, devices[4].Name, devices[5].Name})
	require.True(t, devices[0].Online)
	require.True(t, devices[0].Trusted)
	require.Equal(t, "fp-shared", devices[0].Peer.Fingerprint)

	test.NewTempApp(t)
	rows := syncConnectRows(devices, nil, true)
	content := container.NewVBox(rows...)
	var fingerprintCaptions []string
	for _, object := range test.LaidOutObjects(content) {
		if label, ok := object.(*widget.Label); ok && (label.Text == "fp-new-a" || label.Text == "fp-new-z") {
			fingerprintCaptions = append(fingerprintCaptions, label.Text)
		}
	}
	require.Len(t, fingerprintCaptions, 2)
	require.NotEqual(t, fingerprintCaptions[0], fingerprintCaptions[1])
}

func TestSyncQRCodeViewDisplaysConnectionURI(t *testing.T) {
	test.NewTempApp(t)
	link := "paw://127.0.0.1:61654/sync?fp=7a2a9d534f4b092dcc8e5a53f644508ed498fdd0203f387f717c74f8b63c118d"
	view := newSyncQRCodeView(link, widget.NewLabel("qr image"), func() {})
	found := false
	for _, object := range test.LaidOutObjects(view) {
		if label, ok := object.(*widget.Label); ok && label.Text == link {
			found = true
		}
	}
	require.True(t, found)
}

func TestCloneConnectViewUsesCloneHeading(t *testing.T) {
	test.NewTempApp(t)
	view := newSyncConnectView(true, func() {}, func() {}, func() {}, nil, nil)
	var cloneHeading bool
	for _, object := range test.LaidOutObjects(view.view) {
		if label, ok := object.(*widget.Label); ok && label.Text == "Clone from device" {
			cloneHeading = true
		}
	}
	require.True(t, cloneHeading)
}

func TestSyncPairingInitiatorShowsVerificationCodeAndWaitState(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	labels := make(map[string]bool)
	buttons := make(map[string]bool)
	fyne.DoAndWait(func() {
		view := newSyncPairingView("fedora", "760763", false, func() {}, nil)
		for _, object := range test.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok {
				labels[label.Text] = true
			}
			if button, ok := object.(*widget.Button); ok {
				buttons[button.Text] = true
			}
		}
	})
	require.True(t, labels["Pairing"])
	require.True(t, labels["Pairing with fedora"])
	require.True(t, labels["Confirm this code on fedora."])
	require.True(t, labels["760 763"])
	require.True(t, labels["Waiting for confirmation…"])
	require.True(t, buttons["Cancel"])
}

func TestSyncPairingReceiverRequiresExplicitDecision(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Pairing")
	t.Cleanup(window.Close)
	var declined, confirmed bool
	labels := make(map[string]bool)
	buttons := make(map[string]*widget.Button)
	fyne.DoAndWait(func() {
		view := newSyncPairingView("phoenix", "123456", true, func() { declined = true }, func() { confirmed = true })
		window.SetContent(view.view)
		window.Show()
		for _, object := range test.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok {
				labels[label.Text] = true
			}
			if button, ok := object.(*widget.Button); ok {
				buttons[button.Text] = button
			}
		}
		test.Tap(buttons["Confirm pairing"])
	})
	require.True(t, labels["Pair with phoenix"])
	require.True(t, labels["Does this code match the one on phoenix?"])
	require.True(t, labels["123 456"])
	require.NotNil(t, buttons["Decline"])
	require.NotNil(t, buttons["Confirm pairing"])
	require.True(t, confirmed)
	require.False(t, declined)
	fyne.DoAndWait(func() {
		declineView := newSyncPairingView("phoenix", "123456", true, func() { declined = true }, func() {})
		for _, object := range test.LaidOutObjects(declineView.view) {
			button, ok := object.(*widget.Button)
			if ok && button.Text == "Decline" {
				test.Tap(button)
				break
			}
		}
	})
	require.True(t, declined)
}

func TestSyncPairingReceiverCanGoBackWhileWaiting(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Pairing")
	t.Cleanup(window.Close)
	var backed, confirmed bool
	fyne.DoAndWait(func() {
		view := newSyncPairingView("phoenix", "123456", true, func() { backed = true }, func() { confirmed = true })
		window.SetContent(view.view)
		window.Show()
		for _, object := range test.LaidOutObjects(view.view) {
			button, ok := object.(*widget.Button)
			if ok && button.Text == "Confirm pairing" {
				test.Tap(button)
			}
		}
		for _, object := range test.LaidOutObjects(view.view) {
			button, ok := object.(*widget.Button)
			if ok && button.Text == "" {
				test.Tap(button)
				break
			}
		}
	})
	require.True(t, confirmed)
	require.True(t, backed)
}

func TestSyncReviewRequiresAValidDecisionForEveryDifference(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Sync")
	t.Cleanup(window.Close)
	modifiedLocal := uint64(100)
	modifiedRemote := uint64(200)
	local := &pawsync.Manifest{Items: []pawsync.ManifestItem{
		{
			ID:          "local",
			Type:        model.LoginItemType,
			Title:       "GitHub",
			ModifiedAt:  &modifiedLocal,
			ContentHash: reviewHash(1),
			StateHash:   reviewHash(1),
		},
		{
			ID:          "different",
			Type:        model.PasswordItemType,
			Title:       "VPN",
			ModifiedAt:  &modifiedLocal,
			ContentHash: reviewHash(2),
			StateHash:   reviewHash(2),
		},
		{
			ID:          "same",
			Type:        model.NoteItemType,
			Title:       "Shared",
			ContentHash: reviewHash(4),
			StateHash:   reviewHash(4),
		},
	}}
	remote := &pawsync.Manifest{Items: []pawsync.ManifestItem{
		{
			ID:          "remote",
			Type:        model.NoteItemType,
			Title:       "Meeting notes",
			ModifiedAt:  &modifiedRemote,
			ContentHash: reviewHash(5),
			StateHash:   reviewHash(5),
		},
		{
			ID:          "different",
			Type:        model.PasswordItemType,
			Title:       "VPN",
			ModifiedAt:  &modifiedRemote,
			ContentHash: reviewHash(3),
			StateHash:   reviewHash(3),
		},
		{
			ID:          "same",
			Type:        model.NoteItemType,
			Title:       "Shared",
			ContentHash: reviewHash(4),
			StateHash:   reviewHash(4),
		},
	}}
	diff, err := pawsync.BuildDiff(local, remote)
	require.NoError(t, err)
	plan, err := pawsync.BuildPlan(diff, local, remote)
	require.NoError(t, err)
	require.NoError(t, plan.RequireDecisions())
	var reviewed bool
	var view *syncReviewView
	var initialDisabled, readyDisabled bool
	var readyBeforeReset bool
	var initialProgress, finalProgress string
	var headingFits, timestampsFit bool
	var itemSpacing []float32
	labels := make(map[string]bool)
	fyne.DoAndWait(func() {
		view = newSyncReviewView(
			"fedora",
			pawsync.VaultReference{Name: "Personal"},
			pawsync.VaultReference{Name: "Personal"},
			diff,
			local,
			remote,
			plan,
			func() {},
			func() { reviewed = true },
		)
		initialDisabled = view.review.Disabled()
		initialProgress = view.progress.Text
		window.SetContent(view.view)
		window.Resize(fyne.NewSize(320, 540))
		var heading, device, itemTitle, itemSubtitle, localTime, remoteTime, remoteDevice *widget.Label
		action := view.actions["different"]
		for _, object := range test.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok {
				labels[label.Text] = true
				switch label.Text {
				case "Sync with":
					heading = label
				case "fedora":
					if label.TextStyle.Bold {
						device = label
					} else {
						remoteDevice = label
					}
				case "VPN":
					itemTitle = label
				case "Password · Different versions":
					itemSubtitle = label
				case syncReviewTimestamp(&local.Items[1]):
					localTime = label
				case syncReviewTimestamp(&remote.Items[1]):
					remoteTime = label
				}
			}
		}
		if heading != nil && device != nil {
			headingFits = app.Driver().AbsolutePositionForObject(heading).X+heading.Size().Width <= app.Driver().AbsolutePositionForObject(device).X
		}
		if localTime != nil && remoteTime != nil {
			timestampsFit = localTime.Size().Width >= localTime.MinSize().Width && remoteTime.Size().Width >= remoteTime.MinSize().Width
		}
		if itemTitle != nil && itemSubtitle != nil && localTime != nil && remoteDevice != nil && remoteTime != nil && action != nil {
			objects := []fyne.CanvasObject{itemTitle, itemSubtitle, localTime, remoteTime, action}
			for index := 1; index < len(objects); index++ {
				previous := objects[index-1]
				current := objects[index]
				previousBottom := app.Driver().AbsolutePositionForObject(previous).Y + previous.Size().Height
				currentTop := app.Driver().AbsolutePositionForObject(current).Y
				itemSpacing = append(itemSpacing, currentTop-previousBottom)
			}
		}
	})
	require.True(t, initialDisabled)
	require.True(t, labels["Sync with"])
	require.True(t, labels["fedora"])
	require.True(t, labels["1 identical item · 3 items to review"])
	require.Equal(t, "0 of 3 reviewed", initialProgress)
	require.True(t, headingFits)
	require.True(t, timestampsFit)
	require.Len(t, itemSpacing, 4)
	for _, spacing := range itemSpacing {
		require.InDelta(t, 0, spacing, 1, "sync review item rows should not add vertical layout spacing")
	}
	require.True(t, labels["GitHub"])
	require.True(t, labels["Meeting notes"])
	require.True(t, labels["Password · Different versions"])
	fyne.DoAndWait(func() {
		view.actions["local"].SetSelected("Copy to fedora")
		view.actions["remote"].SetSelected("Copy to this device")
		view.actions["different"].SetSelected("Keep fedora’s version (Latest)")
		finalProgress = view.progress.Text
		readyDisabled = view.review.Disabled()
		readyBeforeReset, _ = plan.Ready()
		test.Tap(view.review)
		view.actions["different"].SetSelected("Choose action")
	})
	require.True(t, readyBeforeReset)
	require.Equal(t, "3 of 3 reviewed", finalProgress)
	require.False(t, readyDisabled)
	require.True(t, reviewed)
	ready, err := plan.Ready()
	require.NoError(t, err)
	require.False(t, ready)
}

func TestSyncConfirmViewSummarizesAndRequiresExplicitSyncAction(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	local := &pawsync.Manifest{Items: []pawsync.ManifestItem{
		{
			ID:          "local",
			Type:        model.LoginItemType,
			Title:       "GitHub",
			ContentHash: reviewHash(1),
			StateHash:   reviewHash(1),
		},
		{
			ID:          "same",
			Type:        model.NoteItemType,
			Title:       "Shared",
			ContentHash: reviewHash(4),
			StateHash:   reviewHash(4),
		},
	}}
	remote := &pawsync.Manifest{Items: []pawsync.ManifestItem{
		{
			ID:          "remote",
			Type:        model.NoteItemType,
			Title:       "Meeting notes",
			ContentHash: reviewHash(5),
			StateHash:   reviewHash(5),
		},
		{
			ID:          "delete",
			Type:        model.PasswordItemType,
			Title:       "Old VPN",
			ContentHash: reviewHash(6),
			StateHash:   reviewHash(6),
		},
		{
			ID:          "skip",
			Type:        model.SSHKeyItemType,
			Title:       "Build key",
			ContentHash: reviewHash(7),
			StateHash:   reviewHash(7),
		},
		{
			ID:          "same",
			Type:        model.NoteItemType,
			Title:       "Shared",
			ContentHash: reviewHash(4),
			StateHash:   reviewHash(4),
		},
	}}
	diff, err := pawsync.BuildDiff(local, remote)
	require.NoError(t, err)
	plan, err := pawsync.BuildPlan(diff, local, remote)
	require.NoError(t, err)
	require.NoError(t, plan.SetDecision("local", pawsync.CopyLocalToRemote))
	require.NoError(t, plan.SetDecision("remote", pawsync.CopyRemoteToLocal))
	require.NoError(t, plan.SetDecision("delete", pawsync.DeleteRemote))
	require.NoError(t, plan.SetDecision("skip", pawsync.Skip))
	localVault := pawsync.VaultReference{Name: "Personal"}
	remoteVault := pawsync.VaultReference{Name: "Personal"}
	var back, synced bool
	var view *syncConfirmView
	var disabled bool
	labels := make(map[string]bool)
	buttons := make(map[string]*widget.Button)
	fyne.DoAndWait(func() {
		disabledView := newSyncConfirmView("fedora", localVault, remoteVault, local, remote, plan, func() {}, nil, nil)
		disabled = disabledView.sync.Disabled()
		view = newSyncConfirmView("fedora", localVault, remoteVault, local, remote, plan, func() { back = true }, func() { synced = true }, func() {})
		for _, object := range test.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok {
				labels[label.Text] = true
			}
			if button, ok := object.(*widget.Button); ok {
				buttons[button.Text] = button
			}
		}
		test.Tap(buttons["Back"])
		test.Tap(buttons["Sync"])
	})
	require.True(t, disabled)
	for _, label := range []string{
		"Ready to sync with fedora",
		"Items (3)",
		"GitHub",
		"Meeting notes",
		"Old VPN",
		"Delete from fedora",
	} {
		require.True(t, labels[label], "expected label %q", label)
	}
	require.False(t, labels["Personal ↔ Personal"])
	require.True(t, labels["Copy to fedora:"])
	require.True(t, labels["1"])
	require.True(t, labels["Delete from fedora"])
	require.True(t, labels["Password"])
	require.True(t, labels["1 item skipped."])
	require.False(t, labels["Skip"])
	require.True(t, back)
	require.True(t, synced)
}

func TestSyncConfirmViewUsesDoneWhenNoOperationsRemain(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Sync")
	t.Cleanup(window.Close)
	called := false
	buttonText := ""
	labels := make(map[string]bool)
	local := &pawsync.Manifest{Items: []pawsync.ManifestItem{{ID: "same", Type: model.NoteItemType, Title: "Shared", ContentHash: reviewHash(1), StateHash: reviewHash(1)}}}
	remote := &pawsync.Manifest{Items: []pawsync.ManifestItem{{ID: "same", Type: model.NoteItemType, Title: "Shared", ContentHash: reviewHash(1), StateHash: reviewHash(1)}}}
	diff, err := pawsync.BuildDiff(local, remote)
	require.NoError(t, err)
	plan, err := pawsync.BuildPlan(diff, local, remote)
	require.NoError(t, err)
	fyne.DoAndWait(func() {
		view := newSyncConfirmView("fedora", pawsync.VaultReference{}, pawsync.VaultReference{}, local, remote, plan, func() {}, func() { t.Error("must not start an empty sync") }, func() { called = true })
		window.SetContent(view.view)
		for _, object := range test.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok {
				labels[label.Text] = true
			}
		}
		test.Tap(view.sync)
		buttonText = view.sync.Text
	})
	require.True(t, labels["Already in sync"])
	require.True(t, labels["Both vaults are up to date."])
	require.False(t, labels["Ready to sync with fedora"])
	require.False(t, labels["Items (0)"])
	require.True(t, called)
	require.Equal(t, "Done", buttonText)
}

func TestSyncReviewIdenticalVaultsDoneCompletes(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	var completed, canceled bool
	var buttonText string
	var labels = make(map[string]bool)
	fyne.DoAndWait(func() {
		view := newSyncReviewView("fedora", pawsync.VaultReference{}, pawsync.VaultReference{}, nil, &pawsync.Manifest{}, &pawsync.Manifest{}, &pawsync.Plan{}, func() { canceled = true }, func() { completed = true })
		for _, object := range test.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok {
				labels[label.Text] = true
			}
		}
		buttonText = view.review.Text
		test.Tap(view.review)
	})
	require.Equal(t, "Done", buttonText)
	require.True(t, labels["Already in sync"])
	require.True(t, completed)
	require.False(t, canceled)
}

func TestSyncConfirmViewAllSkippedPlanDoesNotStartTransfer(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Sync")
	t.Cleanup(window.Close)
	local := &pawsync.Manifest{Items: []pawsync.ManifestItem{{ID: "one", Type: model.NoteItemType, Title: "Note", ContentHash: reviewHash(1), StateHash: reviewHash(1)}}}
	remote := &pawsync.Manifest{Items: []pawsync.ManifestItem{{ID: "one", Type: model.NoteItemType, Title: "Note", ContentHash: reviewHash(2), StateHash: reviewHash(2)}}}
	diff, err := pawsync.BuildDiff(local, remote)
	require.NoError(t, err)
	plan, err := pawsync.BuildPlan(diff, local, remote)
	require.NoError(t, err)
	require.NoError(t, plan.SetDecision("one", pawsync.Skip))
	var labels = make(map[string]bool)
	var done bool
	var buttonText string
	var messageCentered bool
	fyne.DoAndWait(func() {
		view := newSyncConfirmView("fedora", pawsync.VaultReference{}, pawsync.VaultReference{}, local, remote, plan,
			func() {},
			func() { t.Error("all-skipped plan must not start Sync") },
			func() { done = true },
		)
		window.SetContent(view.view)
		window.Resize(fyne.NewSize(320, 540))
		for _, object := range test.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok {
				labels[label.Text] = true
				if label.Text == "1 item skipped." {
					messageCentered = label.Alignment == fyne.TextAlignCenter && label.Size().Width > 100
				}
			}
		}
		test.Tap(view.sync)
		buttonText = view.sync.Text
	})
	require.True(t, labels["No changes selected"])
	require.True(t, labels["1 item skipped."])
	require.True(t, messageCentered)
	require.False(t, labels["Note"])
	require.False(t, labels["Items (1)"])
	require.True(t, done)
	require.Equal(t, "Done", buttonText)
}

func reviewHash(value int) string {
	return fmt.Sprintf("%064x", value)
}

func TestSyncNavigationWaitsForSessionShutdown(t *testing.T) {
	test.NewTempApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	run := &syncConnectRun{
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
		view:   newSyncConnectView(false, func() {}, func() {}, func() {}, nil, nil),
	}
	a := &app{syncRun: run}
	next := make(chan struct{})
	var stopping bool
	fyne.DoAndWait(func() {
		stopping = a.stopSyncConnectBefore(func() { close(next) })
	})
	require.True(t, stopping)
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	select {
	case <-next:
		t.Fatal("navigation continued before the Sync session stopped")
	default:
	}
	close(run.done)
	select {
	case <-next:
	case <-time.After(time.Second):
		t.Fatal("navigation did not continue after the Sync session stopped")
	}
}

func TestSyncVaultActionOpensConnectForUnlockedVault(t *testing.T) {
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	window := driver.NewWindow("Sync")
	t.Cleanup(window.Close)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	a := &app{
		win:   window,
		vault: vault,
		main:  container.NewVScroll(widget.NewLabel("Vault")),
	}

	var run *syncConnectRun
	fyne.DoAndWait(func() {
		a.makeVaultActions()[3].Action()
		run = a.syncRun
	})
	require.NotNil(t, run)
	fyne.DoAndWait(a.closeSyncConnect)
	waitSyncConnectRun(t, run)
	var current fyne.CanvasObject
	fyne.DoAndWait(func() { current = window.Content() })
	require.Contains(t, connectViewText(t, current), "Sync")
}

func TestCloneOpensConnectWithoutLocalVault(t *testing.T) {
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	window := driver.NewWindow("Clone")
	t.Cleanup(window.Close)
	a := &app{win: window}

	var run *syncConnectRun
	fyne.DoAndWait(func() {
		a.showCloneView()
		run = a.syncRun
	})
	require.NotNil(t, run)
	fyne.DoAndWait(a.closeSyncConnect)
	waitSyncConnectRun(t, run)
	var current fyne.CanvasObject
	fyne.DoAndWait(func() { current = window.Content() })
	require.Contains(t, connectViewText(t, current), "Clone from device")
}

func connectViewText(t *testing.T, view fyne.CanvasObject) string {
	t.Helper()
	var text string
	fyne.DoAndWait(func() {
		for _, object := range test.LaidOutObjects(view) {
			if label, ok := object.(*widget.Label); ok {
				text += label.Text + "\n"
			}
		}
	})
	return text
}

func waitSyncConnectRun(t *testing.T, run *syncConnectRun) {
	t.Helper()
	select {
	case <-run.done:
	case <-time.After(time.Second):
		t.Fatal("Sync connect run did not stop")
	}
}
