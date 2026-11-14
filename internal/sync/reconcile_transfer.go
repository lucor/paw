// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"context"
	"fmt"
	"net/http"

	"go.lucor.dev/paw/internal/model"
)

func (s *Session) connectReconcilePeer(ctx context.Context, peer Peer) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if s.peer != nil && Fingerprint(s.peer.public) == peer.Fingerprint {
		trusted, err := s.peerIsTrusted()
		if err != nil {
			return err
		}
		if !trusted {
			return fmt.Errorf("sync peer is not paired")
		}
		s.startSyncHeartbeat(s.peer)
		return nil
	}
	_, err := s.ConnectTrusted(ctx, peer)
	return err
}

// RemoteItem returns one trusted peer item after checking its expected state.
func (s *Session) RemoteItem(ctx context.Context, peer Peer, id, expectedState string) (*model.Item, error) {
	if err := s.connectReconcilePeer(ctx, peer); err != nil {
		return nil, err
	}
	if s.peer == nil {
		return nil, fmt.Errorf("sync peer endpoint is unavailable")
	}
	response, err := s.peer.transferItem(ctx, http.MethodPost, itemRequest{ID: id, Expected: expectedState})
	if err != nil {
		return nil, err
	}
	if err := validateTransferItem(response.Item, id, expectedState); err != nil {
		return nil, err
	}
	return response.Item, nil
}

// CopyItemToPeer uses the normal authenticated Sync transfer path and verifies the result.
// expectedRemoteState is empty when the destination ID must be absent.
func (s *Session) CopyItemToPeer(ctx context.Context, peer Peer, item *model.Item, expectedLocalState, expectedRemoteState string) error {
	if item == nil {
		return fmt.Errorf("item is required")
	}
	if err := s.connectReconcilePeer(ctx, peer); err != nil {
		return err
	}
	if s.peer == nil {
		return fmt.Errorf("sync peer endpoint is unavailable")
	}
	localState, err := item.StateHash()
	if err != nil {
		return err
	}
	if localState != expectedLocalState {
		return fmt.Errorf("local item changed; rebuild the reconciliation plan")
	}
	kind := LocalOnly
	if expectedRemoteState != "" {
		kind = ContentDifferent
	}
	if err := s.executeEntry(ctx, PlanEntry{
		ItemID:          item.ID,
		Kind:            kind,
		Decision:        CopyLocalToRemote,
		LocalStateHash:  localState,
		RemoteStateHash: expectedRemoteState,
	}); err != nil {
		return err
	}
	response, err := s.peer.transferItem(ctx, http.MethodPost, itemRequest{ID: item.ID, Expected: localState})
	if err != nil {
		return fmt.Errorf("verify transferred item: %w", err)
	}
	return validateTransferItem(response.Item, item.ID, localState)
}

// CopyRemoteItemToLocal uses the normal authenticated Sync transfer path and verifies storage.
// expectedLocalState is empty when the destination ID must be absent.
func (s *Session) CopyRemoteItemToLocal(ctx context.Context, peer Peer, id, expectedRemoteState, expectedLocalState string) error {
	if err := s.connectReconcilePeer(ctx, peer); err != nil {
		return err
	}
	if s.peer == nil {
		return fmt.Errorf("sync peer endpoint is unavailable")
	}
	kind := RemoteOnly
	if expectedLocalState != "" {
		kind = ContentDifferent
	}
	if err := s.executeEntry(ctx, PlanEntry{
		ItemID:          id,
		Kind:            kind,
		Decision:        CopyRemoteToLocal,
		LocalStateHash:  expectedLocalState,
		RemoteStateHash: expectedRemoteState,
	}); err != nil {
		return err
	}
	local, err := checkedItem(ctx, s.store, s.vault, id, expectedRemoteState)
	if err != nil {
		return fmt.Errorf("verify received item: %w", err)
	}
	return validateTransferItem(local, id, expectedRemoteState)
}

// DeleteRemoteItem removes an item only after validating its current remote state.
func (s *Session) DeleteRemoteItem(ctx context.Context, peer Peer, id, expectedState string) error {
	if err := s.connectReconcilePeer(ctx, peer); err != nil {
		return err
	}
	if s.peer == nil {
		return fmt.Errorf("sync peer endpoint is unavailable")
	}
	if _, err := s.peer.transferItem(ctx, http.MethodPost, itemRequest{ID: id, Expected: expectedState}); err != nil {
		return err
	}
	response, err := s.peer.transferItem(ctx, http.MethodDelete, itemRequest{ID: id, Expected: expectedState})
	if err != nil {
		return err
	}
	if !response.Committed {
		return fmt.Errorf("peer did not acknowledge item removal")
	}
	verified, err := s.peer.transferItem(ctx, http.MethodPost, itemRequest{ID: id})
	if err != nil {
		return fmt.Errorf("verify removed peer item: %w", err)
	}
	if verified.Item != nil {
		return fmt.Errorf("peer item remains after removal")
	}
	return nil
}

// DeleteLocalItem removes an item only after validating its current local state.
func (s *Session) DeleteLocalItem(ctx context.Context, id, expectedState string) error {
	if _, err := checkedItem(ctx, s.store, s.vault, id, expectedState); err != nil {
		return err
	}
	if err := s.store.DeleteItem(s.vault, id); err != nil {
		return err
	}
	for _, summary := range s.vault.Accounts[0].Items {
		if summary.ID == id {
			return fmt.Errorf("item remains after local removal")
		}
	}
	return nil
}
