// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/storage"
)

// executeEntry validates and commits one plan entry before returning.
func (s *Session) executeEntry(ctx context.Context, entry PlanEntry) (resultErr error) {
	var uploaded []cxf.File
	remoteCommitted := false
	defer func() {
		if remoteCommitted || len(uploaded) == 0 {
			return
		}
		cleanupErr := s.cleanupRemoteAttachments(ctx, entry.ItemID, uploaded)
		resultErr = errors.Join(resultErr, cleanupErr)
	}()
	local, err := checkedItem(ctx, s.store, s.vault, entry.ItemID, entry.LocalStateHash)
	if err != nil {
		return err
	}
	request := itemRequest{
		ID:       entry.ItemID,
		Expected: entry.RemoteStateHash,
	}
	// Read the remote item even for Skip so stale plans are rejected before completion.
	remote, err := s.peer.transferItem(ctx, http.MethodPost, request)
	if err != nil {
		return err
	}
	if _, err := checkedItem(ctx, s.store, s.vault, entry.ItemID, entry.LocalStateHash); err != nil {
		return err
	}
	switch entry.Decision {
	case Skip:
		return nil
	case CopyLocalToRemote:
		if err := validateTransferItem(local, entry.ItemID, entry.LocalStateHash); err != nil {
			return err
		}
		files, err := storage.AttachmentReferences(local)
		if err != nil {
			return err
		}
		for _, file := range files {
			present, err := s.peer.verifyRemoteAttachment(ctx, file, entry.ItemID)
			if err != nil {
				return err
			}
			if present {
				continue
			}
			uploaded = append(uploaded, file)
			reader, writer := io.Pipe()
			writeDone := make(chan error, 1)
			go func() {
				err := s.store.WriteAttachment(s.vault, entry.ItemID, file, writer)
				_ = writer.CloseWithError(err)
				writeDone <- err
			}()
			created, transferErr := s.peer.transferAttachment(ctx, file, entry.ItemID, reader)
			_ = reader.Close()
			writeErr := <-writeDone
			if transferErr != nil {
				return transferErr
			}
			if !created {
				uploaded = uploaded[:len(uploaded)-1]
			}
			if writeErr != nil {
				return writeErr
			}
		}
		if _, err := checkedItem(ctx, s.store, s.vault, entry.ItemID, entry.LocalStateHash); err != nil {
			return err
		}
		request.Item = local
		request.Source = entry.LocalStateHash
	case CopyRemoteToLocal:
		if err := validateTransferItem(remote.Item, entry.ItemID, entry.RemoteStateHash); err != nil {
			return err
		}
		files, err := storage.AttachmentReferences(remote.Item)
		if err != nil {
			return err
		}
		created := make([]string, 0, len(files))
		committed := false
		defer func() {
			if committed || len(created) == 0 {
				return
			}
			if err := s.store.CleanupAttachments(s.vault, entry.ItemID, created); err != nil {
				resultErr = errors.Join(resultErr, err)
			}
		}()
		for _, file := range files {
			valid, err := s.store.VerifyAttachment(s.vault, entry.ItemID, file)
			if err != nil {
				return err
			}
			if valid {
				continue
			}
			body, err := s.peer.readAttachment(ctx, file, entry.ItemID)
			if err != nil {
				return err
			}
			wasCreated, receiveErr := s.store.ReceiveAttachment(s.vault, entry.ItemID, file, body)
			_ = body.Close()
			if receiveErr != nil {
				return receiveErr
			}
			if wasCreated {
				created = append(created, file.ID)
			}
		}
		if _, err := checkedItem(ctx, s.store, s.vault, entry.ItemID, entry.LocalStateHash); err != nil {
			return err
		}
		response, err := s.peer.transferItem(ctx, http.MethodPost, itemRequest{
			ID:       entry.ItemID,
			Expected: entry.RemoteStateHash,
		})
		if err != nil {
			return err
		}
		if err := validateTransferItem(response.Item, entry.ItemID, entry.RemoteStateHash); err != nil {
			return err
		}
		if err := s.store.StoreItem(s.vault, response.Item); err != nil {
			return err
		}
		committed = true
		return nil
	case DeleteLocal:
		return s.store.DeleteItem(s.vault, entry.ItemID)
	case DeleteRemote:
	default:
		return fmt.Errorf("invalid Sync decision")
	}
	method := http.MethodPut
	if entry.Decision == DeleteRemote {
		method = http.MethodDelete
	}
	response, err := s.peer.transferItem(ctx, method, request)
	if err != nil {
		return err
	}
	if !response.Committed {
		return fmt.Errorf("peer did not acknowledge the storage commit")
	}
	remoteCommitted = true
	return nil
}

// cleanupRemoteAttachments removes only blobs that remain unreferenced after a failed copy.
func (s *Session) cleanupRemoteAttachments(
	ctx context.Context,
	itemID string,
	files []cxf.File,
) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	var cleanupErr error
	for _, file := range files {
		cleanupErr = errors.Join(
			cleanupErr,
			s.peer.removeRemoteAttachment(cleanupCtx, file, itemID),
		)
	}
	return cleanupErr
}

// ExecutionResult records acknowledged operations. A failed remote mutation may have committed.
type ExecutionResult struct {
	Completed   []PlanEntry
	Failed      *PlanEntry
	Unattempted []PlanEntry
}

// ExecutionProgress reports when one plan operation starts and finishes.
type ExecutionProgress func(PlanEntry, bool, error)

// Execute applies a caller-confirmed plan sequentially under Paw's single-writer model.
// The caller must prevent other vault operations throughout execution.
func (s *Session) Execute(ctx context.Context, peer Peer, plan *Plan) (ExecutionResult, error) {
	return s.ExecuteWithProgress(ctx, peer, plan, nil)
}

// ExecuteWithProgress applies a confirmed plan and reports item-level progress.
func (s *Session) ExecuteWithProgress(ctx context.Context, peer Peer, plan *Plan, progress ExecutionProgress) (ExecutionResult, error) {
	result := ExecutionResult{}
	ready, err := plan.Ready()
	if err != nil {
		return result, err
	}
	if !ready {
		return result, fmt.Errorf("resolve all Sync decisions before execution")
	}
	entries := slices.Clone(plan.Entries)
	result.Unattempted = entries
	ctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.ctx, cancel)
	defer stop()
	defer cancel()
	_, err = s.ConnectTrusted(ctx, peer)
	if err != nil {
		return result, err
	}
	if s.peer == nil || s.peer.address == "" {
		return result, fmt.Errorf("could not reach paired device")
	}
	for index, entry := range entries {
		err := ctx.Err()
		if err == nil {
			if progress != nil {
				progress(entry, true, nil)
			}
			err = s.executeEntry(ctx, entry)
		}
		if err != nil {
			if progress != nil {
				progress(entry, false, err)
			}
			result.Failed = &entries[index]
			result.Unattempted = entries[index+1:]
			return result, fmt.Errorf("sync stopped; rebuild the plan: %w", err)
		}
		result.Completed = append(result.Completed, entry)
		if progress != nil {
			progress(entry, false, nil)
		}
	}
	result.Unattempted = nil
	return result, nil
}
