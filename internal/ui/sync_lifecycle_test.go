// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"net"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
	pawsync "go.lucor.dev/paw/internal/sync"
	"go.lucor.dev/paw/internal/test"
)

func TestSyncReviewTerminalActionsNotifyResponder(t *testing.T) {
	for _, tc := range []struct {
		name      string
		different bool
		skip      bool
		cancel    bool
	}{
		{name: "identical"},
		{name: "all skipped", different: true, skip: true},
		{name: "cancel review", different: true, cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testApp := test.NewApp()
			t.Cleanup(testApp.Quit)
			window := testApp.NewWindow("Sync")
			t.Cleanup(window.Close)
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			plannerStore, err := storage.NewFileStorage(t.TempDir())
			require.NoError(t, err)
			defer plannerStore.Close()
			plannerVault := test.CreateVault(t, plannerStore, "Planner")
			responderStore, err := storage.NewFileStorage(t.TempDir())
			require.NoError(t, err)
			defer responderStore.Close()
			responderVault := test.CreateVault(t, responderStore, "Responder")
			var itemID string
			if tc.different {
				item, err := model.NewNote("Remote note", "contents")
				require.NoError(t, err)
				itemID = item.ID
				require.NoError(t, responderStore.StoreItem(responderVault, item))
			}
			plannerPublic, _, err := plannerStore.LoadOrCreateSyncIdentity(plannerVault)
			require.NoError(t, err)
			responderPublic, _, err := responderStore.LoadOrCreateSyncIdentity(responderVault)
			require.NoError(t, err)
			require.NoError(t, plannerStore.AddSyncPeer(plannerVault, storage.SyncPeer{
				PublicKey: responderPublic.Bytes(),
				Name:      "Responder",
			}))
			require.NoError(t, responderStore.AddSyncPeer(responderVault, storage.SyncPeer{
				PublicKey: plannerPublic.Bytes(),
				Name:      "Planner",
			}))
			var plannerManager, responderManager pawsync.Manager
			planner, err := plannerManager.StartManual(ctx, plannerStore, plannerVault, "Planner")
			require.NoError(t, err)
			defer plannerManager.Stop()
			responder, err := responderManager.StartManual(ctx, responderStore, responderVault, "Responder")
			require.NoError(t, err)
			defer responderManager.Stop()
			link := pawsync.ConnectionLink{
				Host:        net.ParseIP("127.0.0.1").String(),
				Port:        responder.Port(),
				Fingerprint: pawsync.Fingerprint(responderPublic),
			}
			_, peer, err := planner.ConnectLink(ctx, link)
			require.NoError(t, err)
			peer.Name = "Responder"
			runCtx, runCancel := context.WithCancel(ctx)
			defer runCancel()
			run := &syncConnectRun{
				ctx:            runCtx,
				cancel:         runCancel,
				done:           make(chan struct{}),
				session:        planner,
				vault:          plannerVault,
				finishRequests: make(chan struct{}, 1),
			}
			a := &app{win: window, syncRun: run, storage: plannerStore, vault: plannerVault}
			fyne.DoAndWait(func() {
				run.view = newSyncConnectView(false, func() {}, func() {}, func() {}, nil, nil)
				window.SetContent(run.view.view)
			})
			a.beginSyncReview(run, peer)
			require.NotNil(t, run.reviewView)
			if tc.cancel {
				fyne.DoAndWait(func() {
					for _, object := range test.LaidOutObjects(run.reviewView.view) {
						if button, ok := object.(*widget.Button); ok && button.Text == "Cancel" {
							test.Tap(button)
							break
						}
					}
				})
				require.ErrorIs(t, runCtx.Err(), context.Canceled)
				require.NoError(t, run.notifySyncOutcome(false, true))
				fyne.DoAndWait(func() { window.SetContent(widget.NewLabel("Closed")) })
				close(run.done)
			} else {
				fyne.DoAndWait(func() {
					if tc.skip {
						run.reviewView.actions[itemID].SetSelected("Skip")
						test.Tap(run.reviewView.review)
						test.Tap(run.confirmView.sync)
					} else {
						test.Tap(run.reviewView.review)
					}
				})
				select {
				case <-run.finishRequests:
					require.NoError(t, run.notifySyncOutcome(true, false))
				case <-ctx.Done():
					t.Fatal("Done did not request completion")
				}
			}
			select {
			case successful := <-responder.SyncOutcomes():
				require.Equal(t, !tc.cancel, successful)
				require.Equal(t, tc.cancel, responder.SyncOutcomeCancelled())
			case <-ctx.Done():
				t.Fatal("responder did not receive the outcome")
			}
			require.NoError(t, run.notifySyncOutcome(false, true))
			select {
			case <-responder.SyncOutcomes():
				t.Fatal("repeated termination sent a second outcome")
			default:
			}
			manifest, err := pawsync.BuildManifest(ctx, plannerStore, plannerVault)
			require.NoError(t, err)
			require.Empty(t, manifest.Items)
		})
	}
}
