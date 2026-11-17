// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"crypto/mldsa"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/model"
	pawsync "go.lucor.dev/paw/internal/sync"
	"go.lucor.dev/paw/internal/ui/assets/icons"
	"go.lucor.dev/paw/internal/ui/widgets"
	"rsc.io/qr"
)

type syncConnectDevice struct {
	Peer    pawsync.Peer
	Name    string
	Status  string
	Trusted bool
	Online  bool
}

type syncConnectView struct {
	view       fyne.CanvasObject
	devices    *fyne.Container
	searching  fyne.CanvasObject
	activity   *widget.Activity
	status     *widget.Label
	link       *widget.Entry
	connect    *widget.Button
	connection *ActionRow
	selectPeer func(pawsync.Peer, bool)
	deviceList []syncConnectDevice
	ready      bool
}

type syncConnectRequest struct {
	peer    pawsync.Peer
	trusted bool
	link    *pawsync.ConnectionLink
}

type cloneCreateInput struct {
	name     string
	password string
}

type syncConnectRun struct {
	deviceName              string
	ctx                     context.Context
	cancel                  context.CancelFunc
	done                    chan struct{}
	manager                 *pawsync.Manager
	view                    *syncConnectView
	pairingView             *syncPairingView
	reviewView              *syncReviewView
	confirmView             *syncConfirmView
	progressView            *syncProgressView
	doneView                *syncDoneView
	cloneCreateView         *cloneCreateView
	receiverProgressView    *syncReceiverProgressView
	incomingSyncPeerName    string
	cloneWaiting            bool
	cloneVault              pawsync.VaultReference
	cloneConfirmed          bool
	cloneCreateRequests     chan cloneCreateInput
	previousVault           *model.Vault
	previousVaultID         string
	previousMainContent     fyne.CanvasObject
	cloneCreatedVault       *model.Vault
	cloneSucceeded          bool
	cloneCleanupFailed      bool
	clonePairingRemoved     bool
	cloneCompletionSent     bool
	session                 *pawsync.Session
	vault                   *model.Vault
	clone                   bool
	connecting              bool
	connected               bool
	peer                    pawsync.Peer
	trusted                 bool
	trustedByFingerprint    map[string]syncConnectDevice
	discoveredByFingerprint map[string]syncConnectDevice
	connectionLink          string
	connections             chan syncConnectRequest
	transferRequests        chan struct{}
	finishRequests          chan struct{}
	transferCtx             context.Context
	transferCancel          context.CancelFunc
	syncOutcomeOnce         sync.Once
	syncOutcomeErr          error
}

func newSyncConnectView(
	clone bool,
	back, shareLink func(),
	showQRCode func(),
	selectPeer func(pawsync.Peer, bool),
	connectLink func(string),
) *syncConnectView {
	pageTitleText := "Sync"
	if clone {
		pageTitleText = "Clone from device"
	}
	backButton := widget.NewButtonWithIcon("", icons.Back, back)
	backButton.Importance = widget.LowImportance
	pageHeader := newPageHeader(backButton, newText(pageTitleText, titleStyle), nil)

	status := newText("", captionStyle)
	status.Hide()
	activity := widget.NewActivity()
	activity.Start()
	searching := container.NewHBox(activity, newText("Looking for devices. Ready to be found.", captionStyle))
	devices := container.NewVBox(searching)
	link := widget.NewEntry()
	link.SetPlaceHolder("paw://...")
	connect := widget.NewButton("Connect", func() {
		if connectLink != nil && link.Text != "" {
			connectLink(strings.TrimSpace(link.Text))
		}
	})
	connect.Importance = widget.HighImportance
	connect.Disable()
	var connectView *syncConnectView
	link.OnChanged = func(value string) {
		updateConnectButton(connect, value, connectView != nil && connectView.ready)
	}
	linkRow := container.NewBorder(nil, nil, nil, connect, link)

	connectionHeading := newText("Connection link", sectionTitleStyle)
	connection := NewActionRow("Connection link", "", icons.Link, nil)
	connection.SetFlat(true)
	copyButton := newIconButton("Copy connection link", icons.Copy, shareLink)
	copyButton.Importance = widget.LowImportance
	qrButton := newIconButton("Show connection QR code", icons.QRCode, showQRCode)
	qrButton.Importance = widget.LowImportance
	connection.SetTrailingActions(copyButton, qrButton)
	shareHeading := newText("Share connection details", sectionTitleStyle)
	contentObjects := make([]fyne.CanvasObject, 0, 9)
	contentObjects = append(contentObjects,
		devices,
		widget.NewSeparator(),
		connectionHeading,
		linkRow,
		widget.NewSeparator(),
		shareHeading,
		connection,
		status,
	)
	content := container.New(layout.NewCustomPaddedVBoxLayout(pawSize(pawSizeNameFormSpacing)), contentObjects...)

	view := newSecondaryPage(pageHeader, nil, container.NewVScroll(content))
	connectView = &syncConnectView{
		view:       view,
		devices:    devices,
		searching:  searching,
		activity:   activity,
		status:     status,
		link:       link,
		connect:    connect,
		connection: connection,
		selectPeer: selectPeer,
	}
	return connectView
}

func (v *syncConnectView) SetDevices(devices []syncConnectDevice) {
	v.deviceList = nil
	for _, device := range devices {
		if device.Online {
			v.deviceList = append(v.deviceList, device)
		}
	}
	v.deviceList = sortedSyncConnectDeviceList(v.deviceList)
	v.refreshDevices()
}

func (v *syncConnectView) refreshDevices() {
	v.devices.Objects = syncConnectRows(v.deviceList, v.selectPeer, v.ready)
	if len(v.deviceList) == 0 {
		v.searching.Show()
		v.activity.Start()
		v.devices.Objects = []fyne.CanvasObject{v.searching}
	} else {
		v.activity.Stop()
		v.searching.Hide()
	}
	v.devices.Refresh()
	v.view.Refresh()
}

func (v *syncConnectView) SetReady(ready bool) {
	v.ready = ready
	updateConnectButton(v.connect, v.link.Text, ready)
	v.refreshDevices()
}

func updateConnectButton(button *widget.Button, value string, ready bool) {
	if !ready {
		button.Disable()
		return
	}
	if _, err := pawsync.ParseConnectionLink(strings.TrimSpace(value)); err != nil {
		button.Disable()
		return
	}
	button.Enable()
}

func syncConnectRows(devices []syncConnectDevice, selectPeer func(pawsync.Peer, bool), ready bool) []fyne.CanvasObject {
	rows := make([]fyne.CanvasObject, 0, len(devices)*2)
	nameCounts := make(map[string]int, len(devices))
	for _, device := range devices {
		nameCounts[syncConnectDeviceName(device)]++
	}
	for index, device := range devices {
		if index > 0 {
			rows = append(rows, widget.NewSeparator())
		}
		status := device.Status
		if status == "" {
			status = "New"
		}
		badge := syncConnectStatusBadge(status)
		var detail fyne.CanvasObject = badge
		if nameCounts[syncConnectDeviceName(device)] > 1 {
			fingerprint := device.Peer.Fingerprint
			if len(fingerprint) < 8 {
				fingerprint = device.Peer.ID
			}
			if len(fingerprint) > 8 {
				fingerprint = fingerprint[:8]
			}
			if fingerprint != "" {
				detail = container.NewHBox(badge, newText(fingerprint, captionStyle))
			}
		}
		var tapped func()
		if ready && device.Online && selectPeer != nil {
			peer := device.Peer
			trusted := device.Trusted
			tapped = func() { selectPeer(peer, trusted) }
		}
		name := syncConnectDeviceName(device)
		row := NewActionRow(name, "", icons.Laptop, tapped)
		row.SetFlat(true)
		row.SetDetailObject(container.NewHBox(detail))
		if !device.Online {
			row.SetInteractive(false)
		}
		rows = append(rows, row)
	}
	return rows
}

func syncConnectStatusBadge(status string) *widgets.Badge {
	background := pawColorNameNeutralBadgeBackground
	foreground := pawColorNameNeutralBadgeForeground
	switch status {
	case "Paired":
		background = pawColorNameSuccessBadgeBackground
		foreground = pawColorNameSuccessBadgeForeground
	case "New":
		background = pawColorNameInfoBadgeBackground
		foreground = pawColorNameInfoBadgeForeground
	}
	badge := widgets.NewBadge(status, pawColor(background), pawColor(foreground))
	badge.TextSize = theme.Size(theme.SizeNameCaptionText)
	badge.PaddingX = theme.Size(theme.SizeNamePadding)
	badge.PaddingY = theme.Size(theme.SizeNameInnerPadding) / 2
	badge.Radius = theme.Size(theme.SizeNameButtonRadius)
	return badge
}

func (v *syncConnectView) SetConnectionLink(value string) {
	v.connection.SetContent("Connection link", value, icons.Link)
}

func syncConnectDeviceName(device syncConnectDevice) string {
	if device.Online && device.Peer.Name != "" {
		return device.Peer.Name
	}
	if device.Name != "" {
		return device.Name
	}
	if device.Peer.Name != "" {
		return device.Peer.Name
	}
	return "Unknown Paw device"
}

func sortedSyncConnectDeviceList(devices []syncConnectDevice) []syncConnectDevice {
	result := append([]syncConnectDevice(nil), devices...)
	slices.SortFunc(result, func(left, right syncConnectDevice) int {
		leftGroup := syncConnectDeviceGroup(left)
		rightGroup := syncConnectDeviceGroup(right)
		if leftGroup != rightGroup {
			return leftGroup - rightGroup
		}
		if order := strings.Compare(strings.ToLower(syncConnectDeviceName(left)), strings.ToLower(syncConnectDeviceName(right))); order != 0 {
			return order
		}
		return strings.Compare(left.Peer.Fingerprint, right.Peer.Fingerprint)
	})
	return result
}

func syncConnectDeviceGroup(device syncConnectDevice) int {
	if !device.Online {
		return 2
	}
	if device.Trusted {
		return 0
	}
	return 1
}

func (v *syncConnectView) SetStatus(message string, failed bool) {
	v.status.SetText(message)
	if message == "" {
		v.status.Hide()
	} else {
		v.status.Show()
	}
	if failed {
		v.status.Importance = widget.DangerImportance
	} else {
		v.status.Importance = widget.MediumImportance
	}
	v.status.Refresh()
}

func (a *app) showSyncView() {
	if a.vault == nil || a.vault.Key() == nil {
		return
	}
	a.openSyncConnect(false)
}

func (a *app) showCloneView() {
	a.openSyncConnect(true)
}

func (a *app) openSyncConnect(clone bool) {
	previous := a.syncRun
	if previous != nil {
		previous.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	var previousMainContent fyne.CanvasObject
	if a.main != nil {
		previousMainContent = a.main.Content
	}
	deviceName := a.state.Settings.DeviceName
	run := &syncConnectRun{
		deviceName:              deviceName,
		ctx:                     ctx,
		cancel:                  cancel,
		done:                    make(chan struct{}),
		manager:                 &pawsync.Manager{},
		clone:                   clone,
		vault:                   a.vault,
		previousVault:           a.vault,
		previousVaultID:         a.vaultID,
		previousMainContent:     previousMainContent,
		trustedByFingerprint:    make(map[string]syncConnectDevice),
		discoveredByFingerprint: make(map[string]syncConnectDevice),
		connections:             make(chan syncConnectRequest, 1),
		transferRequests:        make(chan struct{}, 1),
		finishRequests:          make(chan struct{}, 1),
		cloneCreateRequests:     make(chan cloneCreateInput, 1),
	}
	view := newSyncConnectView(clone, func() {
		if a.syncRun != run {
			return
		}
		if clone {
			a.showAddVaultView()
			return
		}
		a.showCurrentVaultView()
	}, func() {
		a.copySyncConnectionLink(run)
	}, func() {
		a.showSyncQRCode(run)
	}, func(peer pawsync.Peer, trusted bool) {
		a.connectSyncPeer(run, peer, trusted)
	}, func(link string) {
		a.connectSyncLink(run, link)
	})
	run.view = view
	a.syncRun = run
	a.win.SetContent(view.view)

	var previousDone <-chan struct{}
	if previous != nil {
		previousDone = previous.done
	}
	go a.runSyncConnect(run, previousDone)
}

func (a *app) closeSyncConnect() {
	if a.syncRun == nil {
		return
	}
	run := a.syncRun
	a.syncRun = nil
	if run.pairingView != nil {
		run.pairingView.Stop()
	}
	run.cancel()
}

func (a *app) stopSyncConnectBefore(next func()) bool {
	run := a.syncRun
	if run == nil {
		return false
	}
	var current fyne.CanvasObject
	if a.win != nil {
		current = a.win.Content()
	}
	a.syncRun = nil
	if run.pairingView != nil {
		run.pairingView.Stop()
	}
	run.view.SetReady(false)
	run.view.SetStatus("Stopping Sync…", false)
	run.cancel()
	go func() {
		<-run.done
		fyne.Do(func() {
			if a.syncRun != nil || (a.win != nil && a.win.Content() != current) {
				return
			}
			next()
		})
	}()
	return true
}

func (a *app) runSyncConnect(run *syncConnectRun, previousDone <-chan struct{}) {
	defer close(run.done)
	if previousDone != nil {
		select {
		case <-previousDone:
		case <-run.ctx.Done():
			return
		}
	}
	if run.ctx.Err() != nil {
		return
	}
	deviceName := run.deviceName
	var err error
	var session *pawsync.Session
	if run.clone {
		session, err = run.manager.StartClone(run.ctx, a.storage, deviceName, true)
	} else {
		session, err = run.manager.Start(run.ctx, a.storage, run.vault, deviceName)
	}
	if err != nil {
		slog.Warn("could not start Sync discovery", "component", "ui.sync", "error", err)
		fyne.Do(func() {
			if a.syncRun == run {
				run.view.SetStatus(err.Error(), true)
			}
		})
		return
	}
	defer func() {
		if run.clone && !run.cloneSucceeded {
			a.cleanupFailedClone(run, session)
		}
		if !run.clone && run.connected && run.peer.Fingerprint != "" {
			if err := run.notifySyncOutcome(false, true); err != nil {
				slog.Warn("could not notify peer that Sync was cancelled", "component", "ui.sync", "error", err)
			}
		}
		if err := run.manager.Stop(); err != nil {
			slog.Warn("could not stop Sync session", "component", "ui.sync", "error", err)
		}
	}()

	known, err := a.syncKnownDevices(run, session)
	if err != nil {
		slog.Warn("could not load trusted Sync devices", "component", "ui.sync", "error", err)
		fyne.Do(func() {
			if a.syncRun == run {
				run.view.SetStatus(err.Error(), true)
			}
		})
		return
	}
	knownSnapshot := slices.Clone(known)
	fyne.Do(func() {
		if a.syncRun != run {
			return
		}
		run.session = session
		run.view.SetDevices(knownSnapshot)
		run.view.SetReady(true)
		a.loadSyncConnectionLink(run, func(string) {})
	})
	discoveredByFingerprint := run.discoveredByFingerprint
	knownByFingerprint := run.trustedByFingerprint
	peers := session.Peers()
	pairingRequests := session.PairingRequests()
	incomingSyncs := session.IncomingSyncs()
	syncProgress := session.SyncProgress()
	syncOutcomes := session.SyncOutcomes()
	cloneLost := session.CloneConnectionLost()
	syncLost := session.SyncConnectionLost()
	terminal := false
	for {
		select {
		case <-run.ctx.Done():
			return
		case peer, ok := <-peers:
			if !ok {
				peers = nil
				continue
			}
			if peer.Fingerprint == "" {
				continue
			}
			if device, ok := knownByFingerprint[peer.Fingerprint]; ok {
				device.Peer = peer
				device.Online = true
				device.Status = "Paired"
				knownByFingerprint[peer.Fingerprint] = device
				devicesSnapshot := syncConnectDeviceSnapshot(discoveredByFingerprint, knownByFingerprint)
				fyne.Do(func() {
					if a.syncRun == run {
						run.view.SetDevices(devicesSnapshot)
					}
				})
				continue
			}
			discoveredByFingerprint[peer.Fingerprint] = syncConnectDevice{
				Peer:   peer,
				Name:   peer.Name,
				Status: "New",
				Online: true,
			}
			devicesSnapshot := syncConnectDeviceSnapshot(discoveredByFingerprint, knownByFingerprint)
			fyne.Do(func() {
				if a.syncRun == run {
					run.view.SetDevices(devicesSnapshot)
				}
			})
		case request, ok := <-pairingRequests:
			if !ok {
				pairingRequests = nil
				continue
			}
			a.handleIncomingSyncPairing(run, request)
		case deviceName, ok := <-incomingSyncs:
			if !ok {
				incomingSyncs = nil
				continue
			}
			if terminal {
				continue
			}
			run.incomingSyncPeerName = deviceName
			if run.cloneWaiting {
				fyne.Do(func() {
					if a.syncRun == run {
						a.showCloneWaiting(run, deviceName)
					}
				})
			} else {
				fyne.Do(func() {
					if a.syncRun == run {
						a.showSyncWaiting(run, deviceName)
					}
				})
			}
		case event, ok := <-syncProgress:
			if !ok {
				syncProgress = nil
				continue
			}
			if terminal {
				continue
			}
			a.showSyncReceiverProgress(run, event)
		case successful, ok := <-syncOutcomes:
			if !ok {
				syncOutcomes = nil
				continue
			}
			if terminal {
				continue
			}
			terminal = true
			if !successful && !run.cloneWaiting && session.SyncOutcomeCancelled() {
				fyne.Do(func() {
					if a.syncRun == run {
						view := newSyncCancelledView(run.incomingSyncPeerName, func() {
							a.stopSyncConnectBefore(a.showCurrentVaultView)
						})
						a.win.SetContent(view.view)
					}
				})
			} else {
				a.finishSyncReceiverProgress(run, successful, syncProgress)
			}
		case <-cloneLost:
			if terminal {
				continue
			}
			terminal = true
			if run.cloneWaiting {
				fyne.Do(func() {
					if a.syncRun == run {
						view := newCloneSourceUnknownView(run.incomingSyncPeerName, func() {
							a.stopSyncConnectBefore(a.showCurrentVaultView)
						})
						a.win.SetContent(view.view)
					}
				})
			}
		case <-syncLost:
			if terminal {
				continue
			}
			terminal = true
			fyne.Do(func() {
				if a.syncRun == run {
					view := newSyncConnectionLostView(run.incomingSyncPeerName, func() {
						a.stopSyncConnectBefore(a.showCurrentVaultView)
					})
					a.win.SetContent(view.view)
				}
			})
		case request := <-run.connections:
			a.handleSyncConnectRequest(run, request)
		case <-run.transferRequests:
			a.executeSyncTransfer(run)
		case <-run.finishRequests:
			err := run.notifySyncOutcome(true, false)
			if err != nil {
				slog.Warn("could not notify peer that Sync completed", "component", "ui.sync", "error", err)
			}
			fyne.Do(func() {
				if a.syncRun == run {
					a.stopSyncConnectBefore(a.showCurrentVaultView)
				}
			})
		case input := <-run.cloneCreateRequests:
			a.createCloneVault(run, input.name, input.password)
		}
	}
}

func (a *app) syncKnownDevices(run *syncConnectRun, session *pawsync.Session) ([]syncConnectDevice, error) {
	if run.clone {
		return nil, nil
	}
	peers, err := session.PairedPeers()
	if err != nil {
		return nil, err
	}
	known := make([]syncConnectDevice, 0, len(peers))
	for _, peer := range peers {
		public, err := mldsa.NewPublicKey(mldsa.MLDSA44(), peer.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("read trusted Sync device: %w", err)
		}
		fingerprint := pawsync.Fingerprint(public)
		device := syncConnectDevice{
			Peer: pawsync.Peer{
				Name:        peer.Name,
				Fingerprint: fingerprint,
			},
			Name:    peer.Name,
			Status:  "Offline",
			Trusted: true,
		}
		run.trustedByFingerprint[fingerprint] = device
		known = append(known, device)
	}
	return sortedSyncConnectDeviceList(known), nil
}

func syncConnectDeviceSnapshot(discovered, paired map[string]syncConnectDevice) []syncConnectDevice {
	result := make([]syncConnectDevice, 0, len(discovered)+len(paired))
	for fingerprint, device := range paired {
		if discoveredDevice, ok := discovered[fingerprint]; ok {
			device.Peer = discoveredDevice.Peer
			device.Online = true
			device.Status = "Paired"
		}
		result = append(result, device)
	}
	for fingerprint, device := range discovered {
		if _, pairedAlready := paired[fingerprint]; pairedAlready {
			continue
		}
		result = append(result, device)
	}
	return sortedSyncConnectDeviceList(result)
}

func (a *app) connectSyncPeer(run *syncConnectRun, peer pawsync.Peer, trusted bool) {
	if a.syncRun != run || run.session == nil || run.connecting || run.connected {
		return
	}
	if !run.clone && (a.vault == nil || a.vault.Key() == nil) {
		a.closeSyncConnect()
		return
	}
	run.connecting = true
	run.view.SetReady(false)
	run.view.SetStatus("Connecting to "+peer.Name+"…", false)
	run.connections <- syncConnectRequest{
		peer:    peer,
		trusted: trusted,
	}
}

func (a *app) connectSyncLink(run *syncConnectRun, value string) {
	if a.syncRun != run || run.session == nil || run.connecting || run.connected {
		return
	}
	if !run.clone && (a.vault == nil || a.vault.Key() == nil) {
		a.closeSyncConnect()
		return
	}
	link, err := pawsync.ParseConnectionLink(value)
	if err != nil {
		run.view.SetStatus(err.Error(), true)
		return
	}
	run.connecting = true
	run.view.SetReady(false)
	run.view.SetStatus("Connecting to "+link.Host+"…", false)
	run.connections <- syncConnectRequest{link: &link}
}

func (a *app) handleSyncConnectRequest(run *syncConnectRun, request syncConnectRequest) {
	var peer pawsync.Peer
	var trusted bool
	var err error
	if request.link != nil {
		_, peer, err = run.session.ConnectLink(run.ctx, *request.link)
		trusted = run.trustedByFingerprint[peer.Fingerprint].Trusted
	} else {
		peer = request.peer
		trusted = request.trusted
		if trusted {
			_, err = run.session.ConnectTrusted(run.ctx, peer)
		} else {
			_, err = run.session.Connect(run.ctx, peer)
		}
	}
	if err != nil {
		a.finishSyncConnection(run, peer, trusted, err)
		return
	}
	if !run.clone && (a.vault == nil || a.vault.Key() == nil) {
		a.closeSyncConnect()
		return
	}
	if !trusted {
		a.beginSyncPairing(run, peer)
		return
	}
	if run.clone {
		run.peer = peer
		a.beginCloneVaultInfo(run, peer)
		return
	}
	a.beginSyncReview(run, peer)
}

func (a *app) finishSyncConnection(run *syncConnectRun, peer pawsync.Peer, trusted bool, connectErr error) {
	fyne.Do(func() {
		if a.syncRun != run {
			return
		}
		run.connecting = false
		if connectErr != nil {
			run.view.SetReady(true)
			run.view.SetStatus(connectErr.Error(), true)
			return
		}
		run.peer = peer
		run.trusted = trusted
		run.connected = trusted
		run.view.SetReady(false)
		run.view.SetStatus("Connected to paired device "+peer.Name, false)
	})
}

func (a *app) beginSyncPairing(run *syncConnectRun, peer pawsync.Peer) {
	pairing, err := run.session.RequestPairing(run.ctx, peer)
	if err != nil {
		a.finishSyncPairing(run, "Could not start pairing: "+err.Error(), true)
		return
	}
	code := pairing.Code()
	if code == "" {
		a.finishSyncPairing(run, "Could not read the pairing code.", true)
		return
	}
	pairCtx, cancel := context.WithCancel(run.ctx)
	cancelled := make(chan struct{}, 1)
	var pairingView *syncPairingView
	fyne.DoAndWait(func() {
		if a.syncRun == run {
			pairingView = newSyncPairingView(peer.Name, code, false, func() {
				select {
				case cancelled <- struct{}{}:
				default:
				}
				cancel()
			}, nil)
			run.pairingView = pairingView
			a.win.SetContent(pairingView.view)
			pairingView.Start()
		}
	})
	if pairingView == nil {
		cancel()
		return
	}
	err = pairing.Confirm(pairCtx, code)
	cancel()
	if err != nil {
		cancelCtx, cancelRequest := context.WithTimeout(context.Background(), 3*time.Second)
		_ = pairing.Cancel(cancelCtx)
		cancelRequest()
		message := "Pairing failed: " + err.Error()
		select {
		case <-cancelled:
			message = "Pairing cancelled."
		default:
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			message = "Pairing cancelled."
		}
		a.finishSyncPairing(run, message, true)
		return
	}
	if run.clone {
		run.peer = peer
		a.beginCloneVaultInfo(run, peer)
		return
	}
	a.markSyncPeerTrusted(run, peer)
	a.beginSyncReview(run, peer)
}

func (a *app) beginCloneVaultInfo(run *syncConnectRun, peer pawsync.Peer) {
	if !run.clone || run.ctx.Err() != nil {
		return
	}
	info, err := run.session.RemoteVaultInfo(run.ctx)
	if err != nil {
		a.finishCloneCreation(run, err)
		return
	}
	fyne.Do(func() {
		if a.syncRun != run {
			return
		}
		run.cloneConfirmed = true
		run.cloneVault = info
		run.peer = peer
		run.connected = true
		a.showCloneCreate(run)
	})
}

func (a *app) showCloneCreate(run *syncConnectRun) {
	if a.syncRun != run || !run.clone || !run.cloneConfirmed {
		return
	}
	view := newCloneCreateView(run.peer.Name, run.cloneVault, func() {
		if a.syncRun == run {
			a.win.SetContent(run.view.view)
		}
	}, func(_ string, password string) error {
		if a.syncRun != run || !run.cloneConfirmed {
			return fmt.Errorf("clone confirmation is no longer active")
		}
		select {
		case run.cloneCreateRequests <- cloneCreateInput{name: run.cloneCreateView.name.Text, password: password}:
			return nil
		default:
			return fmt.Errorf("vault creation is already in progress")
		}
	})
	run.cloneCreateView = view
	a.win.SetContent(view.view)
}

func (a *app) createCloneVault(run *syncConnectRun, name, password string) {
	if a.syncRun != run || !run.clone || !run.cloneConfirmed || run.cloneCreateView == nil || run.session == nil {
		return
	}
	remote, err := run.session.RemoteManifest(run.ctx, run.peer)
	if err != nil {
		a.finishCloneCreation(run, err)
		return
	}
	plan, err := pawsync.BuildClonePlan(&pawsync.Manifest{}, remote)
	if err != nil {
		a.finishCloneCreation(run, err)
		return
	}
	vault, err := a.storage.CreateCloneVault(name, password)
	if err != nil {
		a.finishCloneCreation(run, err)
		return
	}
	run.cloneCreatedVault = vault
	if err := run.session.AttachCloneVault(vault); err != nil {
		if deleteErr := a.storage.DeleteVault(vault.Accounts[0].ID); deleteErr != nil {
			err = errors.Join(err, deleteErr)
		} else {
			run.cloneCreatedVault = nil
		}
		a.finishCloneCreation(run, err)
		return
	}
	local := &pawsync.Manifest{}
	run.vault = vault
	run.reviewView = &syncReviewView{
		plan:        plan,
		local:       local,
		remote:      remote,
		deviceName:  run.peer.Name,
		remoteVault: run.cloneVault,
	}
	fyne.Do(func() {
		if a.syncRun != run {
			return
		}
		vaultID := vault.Accounts[0].ID
		a.setVaultView(vault, vaultID)
		a.startSyncTransfer(run, &syncConfirmView{plan: plan})
	})
}

func (a *app) finishCloneCreation(run *syncConnectRun, err error) {
	if cleanupErr := a.cleanupFailedClone(run, run.session); cleanupErr != nil {
		err = errors.Join(err, cleanupErr)
	}
	slog.Warn("could not prepare Clone", "component", "ui.sync", "error", err)
	run.reviewView = &syncReviewView{plan: &pawsync.Plan{}}
	fyne.Do(func() {
		if a.syncRun != run {
			return
		}
		a.showSyncFailed(run, pawsync.ExecutionResult{})
	})
}

func (a *app) handleIncomingSyncPairing(run *syncConnectRun, request pawsync.PairingRequest) {
	waitCtx, cancelWait := context.WithCancel(run.ctx)
	defer cancelWait()
	code, err := request.VerificationCode()
	if err != nil {
		_ = request.Decline()
		a.finishSyncPairing(run, "Could not read the pairing code.", true)
		return
	}
	decision := make(chan struct{}, 1)
	var pairingView *syncPairingView
	fyne.DoAndWait(func() {
		if a.syncRun == run {
			pairingView = newSyncPairingView(request.Name, code, true, cancelWait, func() {
				select {
				case decision <- struct{}{}:
				default:
				}
			})
			run.pairingView = pairingView
			a.win.SetContent(pairingView.view)
		}
	})
	if pairingView == nil {
		_ = request.Decline()
		return
	}
	select {
	case <-waitCtx.Done():
		_ = request.Decline()
		if run.ctx.Err() == nil {
			a.finishSyncPairing(run, "Pairing declined.", false)
		}
	case <-decision:
		if err := request.Accept(); err != nil {
			a.finishSyncPairing(run, "Could not accept pairing: "+err.Error(), true)
			return
		}
		fyne.Do(func() { pairingView.SetStatus("Waiting for the other device…", false) })
		if err := request.Wait(waitCtx); err != nil {
			if waitCtx.Err() != nil && run.ctx.Err() == nil {
				_ = request.Cancel()
				a.finishSyncPairing(run, "Pairing cancelled.", false)
				return
			}
			a.finishSyncPairing(run, "Pairing failed: "+err.Error(), true)
			return
		}
		if request.Vault.ID == "" {
			run.cloneWaiting = true
			fyne.Do(func() {
				if a.syncRun == run {
					a.showCloneWaiting(run, request.Name)
				}
			})
			return
		}
		a.finishSyncPairing(run, "Paired with "+request.Name+".", false)
	}
}

func (a *app) showCloneWaiting(run *syncConnectRun, deviceName string) {
	if a.syncRun != run || run.clone {
		return
	}
	back := func() { a.stopSyncConnectBefore(a.showCurrentVaultView) }
	a.win.SetContent(newSyncWaitingView("Clone", "Waiting for clone", "", deviceName+" is setting up the new vault.", back))
}

func (a *app) showSyncWaiting(run *syncConnectRun, deviceName string) {
	if a.syncRun != run || run.clone {
		return
	}
	back := func() { a.stopSyncConnectBefore(a.showCurrentVaultView) }
	a.win.SetContent(newSyncWaitingView("Sync", "Waiting for sync", "Connected to "+deviceName+".", "The other device is reviewing changes.", back))
}

func newSyncWaitingView(pageTitle, title, connection, message string, cancel func()) fyne.CanvasObject {
	backButton := widget.NewButtonWithIcon("", icons.Back, cancel)
	backButton.Importance = widget.LowImportance
	header := newPageHeader(backButton, newText(pageTitle, titleStyle), nil)
	activity := widget.NewActivity()
	activity.Start()
	content := []fyne.CanvasObject{layout.NewSpacer()}
	content = append(content, container.NewCenter(newText(title, headingStyle)))
	if connection != "" {
		content = append(content, wrappedCenteredText(connection, bodyStyle))
	}
	if message != "" {
		content = append(content, wrappedCenteredText(message, bodyStyle))
	}
	content = append(content, container.NewCenter(activity))
	content = append(content, layout.NewSpacer())
	cancelButton := widget.NewButton("Cancel", cancel)
	footer := container.NewBorder(widget.NewSeparator(), nil, cancelButton, nil, nil)
	return newSecondaryPage(header, footer, container.NewVBox(content...))
}

func (a *app) finishSyncPairing(run *syncConnectRun, message string, failed bool) {
	fyne.Do(func() {
		if a.syncRun != run {
			return
		}
		run.connecting = false
		if run.pairingView != nil {
			run.pairingView.Stop()
			run.pairingView = nil
		}
		a.win.SetContent(run.view.view)
		retry := failed || message == "Pairing declined." || message == "Pairing cancelled."
		if retry {
			run.connected = false
		} else if strings.HasPrefix(message, "Paired with ") {
			run.connected = true
		}
		run.view.SetReady(retry)
		run.view.SetStatus(message, failed)
	})
}

func (a *app) markSyncPeerTrusted(run *syncConnectRun, peer pawsync.Peer) {
	device := syncConnectDevice{
		Peer:    peer,
		Name:    peer.Name,
		Status:  "Paired",
		Trusted: true,
		Online:  true,
	}
	if device.Name == "" {
		device.Name = "Unknown Paw device"
	}
	run.trustedByFingerprint[peer.Fingerprint] = device
	delete(run.discoveredByFingerprint, peer.Fingerprint)
	fyne.Do(func() {
		if a.syncRun != run {
			return
		}
		run.view.SetDevices(syncConnectDeviceSnapshot(run.discoveredByFingerprint, run.trustedByFingerprint))
	})
}

func (a *app) beginSyncReview(run *syncConnectRun, peer pawsync.Peer) {
	if run.ctx.Err() != nil {
		return
	}
	if run.vault == nil || run.vault.Key() == nil {
		a.closeSyncConnect()
		return
	}
	fyne.DoAndWait(func() {
		if a.syncRun == run {
			run.peer = peer
			run.connected = true
			run.view.SetReady(false)
			run.view.SetStatus("Comparing vaults with "+peer.Name+"…", false)
		}
	})
	local, err := pawsync.BuildManifest(run.ctx, a.storage, run.vault)
	if err != nil {
		a.failSyncReview(run, "Could not read this vault: "+err.Error())
		return
	}
	remote, err := run.session.RemoteManifest(run.ctx, peer)
	if err != nil {
		a.failSyncReview(run, "Could not read the other vault: "+err.Error())
		return
	}
	remoteVault, err := run.session.RemoteVaultInfo(run.ctx)
	if err != nil {
		a.failSyncReview(run, "Could not read the other vault information: "+err.Error())
		return
	}
	diff, err := pawsync.BuildDiff(local, remote)
	if err != nil {
		a.failSyncReview(run, "Could not compare vaults: "+err.Error())
		return
	}
	plan, err := pawsync.BuildPlan(diff, local, remote)
	if err != nil {
		a.failSyncReview(run, "Could not prepare the Sync plan: "+err.Error())
		return
	}
	if err := plan.RequireDecisions(); err != nil {
		a.failSyncReview(run, "Could not prepare the Sync plan: "+err.Error())
		return
	}
	localVault, _ := run.session.VaultReferences()
	var reviewView *syncReviewView
	fyne.DoAndWait(func() {
		if a.syncRun != run {
			return
		}
		reviewView = newSyncReviewView(
			peer.Name,
			localVault,
			remoteVault,
			diff,
			local,
			remote,
			plan,
			func() { a.stopSyncConnectBefore(a.showCurrentVaultView) },
			func() {
				if len(plan.Entries) == 0 {
					reviewView.review.Disable()
					go a.completeNoopSync(run)
					return
				}
				a.openSyncConfirmPlan(run, reviewView)
			},
		)
		if run.pairingView != nil {
			run.pairingView.Stop()
			run.pairingView = nil
		}
		run.peer = peer
		run.trusted = true
		run.connected = true
		run.connecting = false
		run.reviewView = reviewView
		a.win.SetContent(reviewView.view)
	})
}

func (a *app) openSyncConfirmPlan(run *syncConnectRun, reviewView *syncReviewView) {
	if a.syncRun != run || reviewView == nil {
		return
	}
	ready, err := reviewView.plan.Ready()
	if err != nil || !ready {
		return
	}
	var confirmView *syncConfirmView
	confirmView = newSyncConfirmView(
		reviewView.deviceName,
		reviewView.localVault,
		reviewView.remoteVault,
		reviewView.local,
		reviewView.remote,
		reviewView.plan,
		func() { a.win.SetContent(reviewView.view) },
		func() { a.startSyncTransfer(run, confirmView) },
		func() {
			confirmView.sync.Disable()
			go a.completeNoopSync(run)
		},
	)
	run.confirmView = confirmView
	a.win.SetContent(confirmView.view)
}

func (a *app) completeNoopSync(run *syncConnectRun) {
	select {
	case run.finishRequests <- struct{}{}:
	case <-run.ctx.Done():
	}
}

func (run *syncConnectRun) notifySyncOutcome(successful, cancelled bool) error {
	run.syncOutcomeOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if cancelled {
			run.syncOutcomeErr = run.session.NotifySyncCancelled(ctx, run.peer)
		} else {
			run.syncOutcomeErr = run.session.NotifySyncComplete(ctx, run.peer, successful)
		}
	})
	return run.syncOutcomeErr
}

func (a *app) startSyncTransfer(run *syncConnectRun, confirmView *syncConfirmView) {
	if a.syncRun != run || confirmView == nil || run.reviewView == nil || run.transferCancel != nil {
		return
	}
	ready, err := run.reviewView.plan.Ready()
	if err != nil || !ready {
		return
	}
	transferCtx, cancel := context.WithCancel(run.ctx)
	progressView := newSyncProgressView(
		run.peer.Name,
		run.reviewView.local,
		run.reviewView.remote,
		run.reviewView.plan,
		func() { a.cancelSyncTransfer(run) },
		run.clone,
	)
	run.transferCtx = transferCtx
	run.transferCancel = cancel
	run.progressView = progressView
	a.win.SetContent(progressView.view)
	run.transferRequests <- struct{}{}
}

func (a *app) cancelSyncTransfer(run *syncConnectRun) {
	if a.syncRun != run || run.transferCancel == nil || run.progressView == nil {
		return
	}
	run.progressView.cancel.Disable()
	run.progressView.setStatus("Cancelling Sync…", false)
	run.transferCancel()
}

func (a *app) showSyncReceiverProgress(run *syncConnectRun, event pawsync.SyncProgressEvent) {
	fyne.Do(func() {
		if a.syncRun != run {
			return
		}
		if run.receiverProgressView == nil {
			deviceName := event.DeviceName
			if deviceName == "" {
				deviceName = run.incomingSyncPeerName
			}
			run.receiverProgressView = newSyncReceiverProgressView(deviceName, func() {
				a.stopSyncConnectBefore(a.showCurrentVaultView)
			}, run.cloneWaiting)
			a.win.SetContent(run.receiverProgressView.view)
		}
		run.receiverProgressView.Apply(event)
	})
}

func (a *app) finishSyncReceiverProgress(
	run *syncConnectRun,
	successful bool,
	progressEvents <-chan pawsync.SyncProgressEvent,
) {
	var pending []pawsync.SyncProgressEvent
	for progressEvents != nil {
		select {
		case event, ok := <-progressEvents:
			if !ok {
				progressEvents = nil
				continue
			}
			pending = append(pending, event)
		default:
			progressEvents = nil
		}
	}
	if run.incomingSyncPeerName == "" {
		select {
		case run.incomingSyncPeerName = <-run.session.IncomingSyncs():
		default:
		}
	}
	fyne.Do(func() {
		if a.syncRun != run {
			return
		}
		if run.cloneWaiting && successful {
			done := newCloneSourceDoneView(run.incomingSyncPeerName, func() {
				a.stopSyncConnectBefore(a.showCurrentVaultView)
			})
			a.win.SetContent(done.view)
			return
		}
		if run.cloneWaiting && !successful {
			failed := newCloneSourceFailedView(run.incomingSyncPeerName, func() {
				a.stopSyncConnectBefore(a.showCurrentVaultView)
			})
			a.win.SetContent(failed.view)
			return
		}
		if run.receiverProgressView == nil {
			deviceName := run.incomingSyncPeerName
			if deviceName == "" {
				deviceName = "another device"
			}
			run.receiverProgressView = newSyncReceiverProgressView(deviceName, func() {
				a.stopSyncConnectBefore(a.showCurrentVaultView)
			}, run.cloneWaiting)
		}
		for _, event := range pending {
			run.receiverProgressView.Apply(event)
		}
		run.receiverProgressView.Finish(successful)
		a.win.SetContent(run.receiverProgressView.view)
	})
}

func (a *app) executeSyncTransfer(run *syncConnectRun) {
	if run.transferCtx == nil || run.reviewView == nil || run.progressView == nil {
		return
	}
	result, err := run.session.ExecuteWithProgress(
		run.transferCtx,
		run.peer,
		run.reviewView.plan,
		func(entry pawsync.PlanEntry, started bool, transferErr error) {
			fyne.Do(func() {
				if a.syncRun == run {
					run.progressView.Apply(entry, started, transferErr)
				}
			})
		},
	)
	var notifyErr error
	if run.clone {
		if err != nil {
			cleanupErr := a.cleanupFailedClone(run, run.session)
			if cleanupErr != nil {
				err = errors.Join(err, cleanupErr)
			}
		} else {
			if vault := run.cloneCreatedVault; vault != nil {
				if completeErr := a.storage.CompleteClone(vault.Accounts[0].ID); completeErr != nil {
					err = fmt.Errorf("could not complete cloned vault: %w", completeErr)
					cleanupErr := a.cleanupFailedClone(run, run.session)
					if cleanupErr != nil {
						err = errors.Join(err, cleanupErr)
					}
				}
			}
		}
		if err == nil {
			run.cloneSucceeded = true
			notifyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			notifyErr = run.session.NotifySyncComplete(notifyCtx, run.peer, true)
			cancel()
			if notifyErr == nil {
				run.cloneCompletionSent = true
				run.clonePairingRemoved = true
			}
		}
	} else {
		notifyErr = run.notifySyncOutcome(err == nil, errors.Is(err, context.Canceled))
	}
	if notifyErr != nil {
		slog.Warn("could not notify Sync peer of completion", "component", "ui.sync", "device", run.peer.Name, "error", notifyErr)
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Warn("Sync transfer did not complete", "component", "ui.sync", "device", run.peer.Name, "error", err)
	}
	fyne.Do(func() {
		if a.syncRun != run {
			return
		}
		run.transferCancel = nil
		run.transferCtx = nil
		if err != nil {
			a.showSyncFailed(run, result)
			return
		}
		note := ""
		if notifyErr != nil {
			activity := "Sync"
			if run.clone {
				activity = "Clone"
			}
			note = fmt.Sprintf("Could not notify %s that %s completed: %v", run.peer.Name, activity, notifyErr)
		}
		a.showSyncDone(run, note)
	})
}

func (a *app) cleanupFailedClone(run *syncConnectRun, session *pawsync.Session) error {
	var cleanupErr error
	if vault := run.cloneCreatedVault; vault != nil {
		vaultID := vault.Accounts[0].ID
		if err := a.storage.DeleteVault(vaultID); err != nil {
			run.cloneCleanupFailed = true
			cleanupErr = errors.Join(cleanupErr, err)
			slog.Error("could not delete incomplete Clone vault", "component", "ui.sync", "error", err)
		} else {
			run.cloneCleanupFailed = false
			run.cloneCreatedVault = nil
			fyne.DoAndWait(func() {
				delete(a.unlockedVault, vaultID)
				if a.vaultID != vaultID {
					return
				}
				a.vault = run.previousVault
				a.vaultID = run.previousVaultID
				a.main.Content = run.previousMainContent
				a.main.Refresh()
				a.setWindowTitle()
				if a.state != nil && a.state.Settings != nil {
					a.state.Settings.LastVaultID = run.previousVaultID
					if err := a.storage.StoreAppState(a.state); err != nil {
						slog.Warn("could not restore last used vault after Clone", "component", "ui.sync", "error", err)
					}
				}
			})
		}
	}
	if session != nil && run.peer.Fingerprint != "" && !run.cloneCompletionSent {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := session.NotifySyncComplete(ctx, run.peer, false); err != nil {
			slog.Warn("could not notify Sync peer of failed Clone", "component", "ui.sync", "device", run.peer.Name, "error", err)
			cleanupErr = errors.Join(cleanupErr, err)
		} else {
			run.cloneCompletionSent = true
			run.clonePairingRemoved = true
		}
		cancel()
	}
	return cleanupErr
}

func (a *app) showSyncFailed(run *syncConnectRun, result pawsync.ExecutionResult) {
	if a.syncRun != run || run.reviewView == nil {
		return
	}
	newFailed := newSyncFailedView
	if run.clone {
		newFailed = func(deviceName string, plan *pawsync.Plan, result pawsync.ExecutionResult, done func()) *syncFailedView {
			return newCloneFailedView(deviceName, plan, result, run.cloneCleanupFailed, run.clonePairingRemoved, done)
		}
	}
	failedView := newFailed(
		run.peer.Name,
		run.reviewView.plan,
		result,
		func() { a.stopSyncConnectBefore(a.showCurrentVaultView) },
	)
	a.win.SetContent(failedView.view)
}

func (a *app) showSyncDone(run *syncConnectRun, note string) {
	if a.syncRun != run || run.reviewView == nil {
		return
	}
	if run.clone {
		done := func() {
			a.stopSyncConnectBefore(func() {
				if run.vault != nil {
					a.setVaultView(run.vault, run.vault.Accounts[0].ID)
				}
				a.showCurrentVaultView()
			})
		}
		doneView := newCloneDoneView(run.cloneVault, done)
		run.doneView = doneView
		a.win.SetContent(doneView.view)
		return
	}
	doneView := newSyncDoneView(
		run.peer.Name,
		run.reviewView.plan.Summary(),
		note,
		func() {
			a.stopSyncConnectBefore(func() {
				a.refreshCurrentView()
				a.showCurrentVaultView()
			})
		},
	)
	run.doneView = doneView
	a.win.SetContent(doneView.view)
}

func (a *app) failSyncReview(run *syncConnectRun, message string) {
	fyne.Do(func() {
		if a.syncRun != run {
			return
		}
		if run.pairingView != nil {
			run.pairingView.Stop()
			run.pairingView = nil
		}
		run.connecting = false
		run.connected = false
		a.win.SetContent(run.view.view)
		run.view.SetReady(true)
		run.view.SetStatus(message, true)
	})
}

func (a *app) copySyncConnectionLink(run *syncConnectRun) {
	a.loadSyncConnectionLink(run, func(link string) {
		fyne.CurrentApp().Clipboard().SetContent(link)
		run.view.SetStatus("Connection link copied.", false)
	})
}

func (a *app) showSyncQRCode(run *syncConnectRun) {
	a.loadSyncConnectionLink(run, func(link string) {
		code, err := qr.Encode(link, qr.M)
		if err != nil {
			run.view.SetStatus("Could not create the connection QR code: "+err.Error(), true)
			return
		}
		pngData := code.PNG()
		resource := fyne.NewStaticResource("sync-connection-qr.png", pngData)
		qrImage := canvas.NewImageFromResource(resource)
		qrImage.FillMode = canvas.ImageFillContain
		qrImageArea := container.NewGridWrap(fyne.NewSquareSize(pawSize(pawSizeNameDetailIconArea)*4), qrImage)
		content := container.NewCenter(qrImageArea)
		dialog.NewCustom("Connection QR code", "Done", content, a.win).Show()
	})
}

func (a *app) loadSyncConnectionLink(run *syncConnectRun, loaded func(string)) {
	if a.syncRun != run {
		return
	}
	if run.connectionLink != "" {
		run.view.SetConnectionLink(run.connectionLink)
		loaded(run.connectionLink)
		return
	}
	if run.session == nil {
		run.view.SetStatus("Starting Sync…", false)
		return
	}
	session := run.session
	go func() {
		link, err := session.ConnectionLink()
		fyne.Do(func() {
			if a.syncRun != run {
				return
			}
			if err != nil {
				run.view.SetStatus(err.Error(), true)
				return
			}
			run.connectionLink = link
			run.view.SetConnectionLink(link)
			loaded(link)
		})
	}()
}

func newSyncQRCodeView(link string, qrImage fyne.CanvasObject, back func()) fyne.CanvasObject {
	backButton := widget.NewButtonWithIcon("", icons.Back, back)
	backButton.Importance = widget.LowImportance
	header := newPageHeader(backButton, newText("Connection QR code", titleStyle), nil)
	linkText := newText(link, captionStyle)
	linkText.Wrapping = fyne.TextWrapBreak
	content := container.NewVBox(container.NewCenter(qrImage), linkText)
	return newSecondaryPage(header, nil, container.NewVScroll(content))
}
