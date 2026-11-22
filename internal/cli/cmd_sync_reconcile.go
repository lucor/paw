// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	pawsync "go.lucor.dev/paw/internal/sync"
)

// contentHashReconcile hashes item content without using either item's
// randomly generated item ID.
func contentHashReconcile(item *model.Item) (string, error) {
	if item == nil {
		return "", fmt.Errorf("item is required")
	}
	snapshot, err := item.Snapshot()
	if err != nil {
		return "", err
	}
	// CXF validates identifiers during decoding, so use a fixed valid ID that
	// cannot carry identity from either original item into the content hash.
	snapshot.ID = "AAAAAAAAAAAAAAAAAAAAAA"
	data, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("copy item for reconciliation hash: %w", err)
	}
	var copy model.Item
	if err := json.Unmarshal(data, &copy); err != nil {
		return "", fmt.Errorf("copy item for reconciliation hash: %w", err)
	}
	return copy.ContentHash()
}

type reconcileItemMatch struct {
	local  model.Item
	remote model.Item
}

// uniqueReconcileMatches returns one-to-one content matches across item IDs.
func uniqueReconcileMatches(local, remote []*model.Item) ([]reconcileItemMatch, error) {
	localByHash := make(map[string][]*model.Item, len(local))
	remoteByHash := make(map[string][]*model.Item, len(remote))
	for _, item := range local {
		hash, err := contentHashReconcile(item)
		if err != nil {
			return nil, fmt.Errorf("hash local item: %w", err)
		}
		localByHash[hash] = append(localByHash[hash], item)
	}
	for _, item := range remote {
		hash, err := contentHashReconcile(item)
		if err != nil {
			return nil, fmt.Errorf("hash remote item: %w", err)
		}
		remoteByHash[hash] = append(remoteByHash[hash], item)
	}
	matches := make([]reconcileItemMatch, 0)
	for hash, localItems := range localByHash {
		remoteItems := remoteByHash[hash]
		if len(localItems) != 1 || len(remoteItems) != 1 || localItems[0].ID == remoteItems[0].ID {
			continue
		}
		matches = append(matches, reconcileItemMatch{local: *localItems[0], remote: *remoteItems[0]})
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].local.ID < matches[j].local.ID })
	return matches, nil
}

type reconcileItem struct {
	item     *model.Item
	manifest pawsync.ManifestItem
}

type reconcileOperation struct {
	label       string
	localSource bool
	sourceID    string
	sourceState string
	targetState string
	removeID    string
	removeLocal bool
	removeState string
}

type reconcileSession interface {
	CopyItemToPeer(context.Context, pawsync.Peer, *model.Item, string, string) error
	CopyRemoteItemToLocal(context.Context, pawsync.Peer, string, string, string) error
	DeleteLocalItem(context.Context, string, string) error
	DeleteRemoteItem(context.Context, pawsync.Peer, string, string) error
}

type reconcileItemKey struct {
	typeID model.ItemType
	title  string
}

// reviewReconciliation builds and applies an explicit plan without changing normal Sync.
func (cmd *SyncCmd) reviewReconciliation(
	ctx context.Context,
	session *pawsync.Session,
	peer pawsync.Peer,
	vault *model.Vault,
	localManifest, remoteManifest *pawsync.Manifest,
	input <-chan syncInput,
) (bool, error) {
	localItems := make(map[string]reconcileItem, len(localManifest.Items))
	for _, entry := range localManifest.Items {
		item, err := cmd.storage.LoadItem(vault, entry.ID)
		if err != nil {
			return false, fmt.Errorf("load local item for reconciliation: %w", err)
		}
		stateHash, err := item.StateHash()
		if err != nil {
			return false, err
		}
		if stateHash != entry.StateHash {
			return false, fmt.Errorf("local item changed; rebuild the reconciliation plan")
		}
		localItems[entry.ID] = reconcileItem{item: item, manifest: entry}
	}
	remoteItems := make(map[string]reconcileItem, len(remoteManifest.Items))
	for _, entry := range remoteManifest.Items {
		item, err := session.RemoteItem(ctx, peer, entry.ID, entry.StateHash)
		if err != nil {
			return false, fmt.Errorf("read remote item for reconciliation: %w", err)
		}
		remoteItems[entry.ID] = reconcileItem{item: item, manifest: entry}
	}

	operations := make([]reconcileOperation, 0)
	pairedLocal := make(map[string]bool)
	pairedRemote := make(map[string]bool)
	sameIDs := make([]string, 0)
	for id := range localItems {
		if remoteItems[id].item != nil {
			sameIDs = append(sameIDs, id)
		}
	}
	sort.Strings(sameIDs)
	for _, id := range sameIDs {
		local := localItems[id]
		remote, exists := remoteItems[id]
		if !exists {
			return false, fmt.Errorf("item disappeared during reconciliation planning")
		}
		pairedLocal[id] = true
		pairedRemote[id] = true
		if local.manifest.StateHash == remote.manifest.StateHash {
			continue
		}
		kind := pawsync.StateDifferent
		if local.manifest.ContentHash != remote.manifest.ContentHash {
			kind = pawsync.ContentDifferent
		}
		if _, err := fmt.Fprintf(cmd.out, "\n%s\nDifferent versions with the same Item ID.\n%s\n", syncItemName(&local.manifest), formatSyncVersionTimes(&local.manifest, &remote.manifest, peer.Name)); err != nil {
			return false, err
		}
		decision, err := cmd.askReconcileSyncDecision(ctx, input, kind, peer.Name)
		if err != nil {
			return false, err
		}
		switch decision {
		case pawsync.CopyLocalToRemote:
			operations = append(operations, reconcileOperation{
				label:       "Keep this device's version of " + syncItemName(&local.manifest),
				localSource: true,
				sourceID:    id,
				sourceState: local.manifest.StateHash,
				targetState: remote.manifest.StateHash,
			})
		case pawsync.CopyRemoteToLocal:
			operations = append(operations, reconcileOperation{
				label:       "Keep " + peer.Name + "'s version of " + syncItemName(&remote.manifest),
				sourceID:    id,
				sourceState: remote.manifest.StateHash,
				targetState: local.manifest.StateHash,
			})
		case pawsync.Skip:
		}
	}

	localOnly, remoteOnly := unmatchedReconcileItems(localItems, remoteItems, pairedLocal, pairedRemote)
	localCandidates := make([]*model.Item, 0, len(localOnly))
	remoteCandidates := make([]*model.Item, 0, len(remoteOnly))
	for _, entry := range localOnly {
		localCandidates = append(localCandidates, entry.item)
	}
	for _, entry := range remoteOnly {
		remoteCandidates = append(remoteCandidates, entry.item)
	}
	uniqueMatches, err := uniqueReconcileMatches(localCandidates, remoteCandidates)
	if err != nil {
		return false, err
	}
	for _, match := range uniqueMatches {
		local := localItems[match.local.ID]
		remote := remoteItems[match.remote.ID]
		pairedLocal[match.local.ID] = true
		pairedRemote[match.remote.ID] = true
		operation := reconcileOperation{
			label:       fmt.Sprintf("Keep planner Item ID %s for %s", match.local.ID, syncItemName(&local.manifest)),
			localSource: true,
			sourceID:    match.local.ID,
			sourceState: local.manifest.StateHash,
		}
		equivalentHistory, err := reconcileHistoriesEquivalent(local.item, remote.item)
		if err != nil {
			return false, fmt.Errorf("compare item histories: %w", err)
		}
		if equivalentHistory {
			operation.removeID = match.remote.ID
			operation.removeState = remote.manifest.StateHash
		} else {
			if _, err := fmt.Fprintf(cmd.out, "\nKeeping both IDs for %s because their History differs.\n", syncItemName(&local.manifest)); err != nil {
				return false, err
			}
		}
		operations = append(operations, operation)
	}

	remainingLocal, remainingRemote := unmatchedReconcileItems(localItems, remoteItems, pairedLocal, pairedRemote)
	if err := cmd.reviewReconcileTitleCandidates(ctx, peer.Name, input, remainingLocal, remainingRemote, &operations, pairedLocal, pairedRemote); err != nil {
		return false, err
	}
	operations = append(operations, oneSidedReconcileOperations(peer.Name, remainingLocal, remainingRemote, pairedLocal, pairedRemote)...)

	if len(operations) == 0 {
		if _, err := fmt.Fprintln(cmd.out, "No changes to apply."); err != nil {
			return true, err
		}
		return true, nil
	}
	sort.SliceStable(operations, func(i, j int) bool {
		if operations[i].sourceID != operations[j].sourceID {
			return operations[i].sourceID < operations[j].sourceID
		}
		return operations[i].label < operations[j].label
	})
	if _, err := fmt.Fprintf(cmd.out, "\nComplete reconciliation plan (%d item decisions):\n", len(operations)); err != nil {
		return false, err
	}
	if _, err := fmt.Fprintln(cmd.out, "Back up both vaults, including History and attachments, before continuing. Backup is your responsibility."); err != nil {
		return false, err
	}
	for index, operation := range operations {
		if _, err := fmt.Fprintf(cmd.out, "  %d. %s\n", index+1, operation.label); err != nil {
			return false, err
		}
		if operation.removeID != "" {
			device := peer.Name
			if operation.removeLocal {
				device = "this device"
			}
			if _, err := fmt.Fprintf(cmd.out, "     Then remove old ID %s from %s.\n", operation.removeID, device); err != nil {
				return false, err
			}
		}
	}
	confirmed, err := cmd.askSyncYesNo(ctx, nil, input, nil, nil, "Proceed with the complete plan? [y/N]")
	if err != nil {
		return false, err
	}
	if !confirmed {
		return false, nil
	}
	return cmd.executeReconciliation(ctx, session, peer, localItems, operations)
}

// unmatchedReconcileItems returns items that remain unpaired on either device.
func unmatchedReconcileItems(
	local, remote map[string]reconcileItem,
	pairedLocal, pairedRemote map[string]bool,
) (map[string]reconcileItem, map[string]reconcileItem) {
	localOnly := make(map[string]reconcileItem)
	remoteOnly := make(map[string]reconcileItem)
	for id, item := range local {
		if !pairedLocal[id] && remote[id].item == nil {
			localOnly[id] = item
		}
	}
	for id, item := range remote {
		if !pairedRemote[id] && local[id].item == nil {
			remoteOnly[id] = item
		}
	}
	return localOnly, remoteOnly
}

// oneSidedReconcileOperations plans copies for items present on only one device.
func oneSidedReconcileOperations(
	peerName string,
	local, remote map[string]reconcileItem,
	pairedLocal, pairedRemote map[string]bool,
) []reconcileOperation {
	operations := make([]reconcileOperation, 0, len(local)+len(remote))
	localIDs := make([]string, 0, len(local))
	for id := range local {
		if !pairedLocal[id] {
			localIDs = append(localIDs, id)
		}
	}
	sort.Strings(localIDs)
	for _, id := range localIDs {
		item := local[id]
		operations = append(operations, reconcileOperation{
			label:       "Copy " + syncItemName(&item.manifest) + " to " + peerName,
			localSource: true,
			sourceID:    id,
			sourceState: item.manifest.StateHash,
		})
	}
	remoteIDs := make([]string, 0, len(remote))
	for id := range remote {
		if !pairedRemote[id] {
			remoteIDs = append(remoteIDs, id)
		}
	}
	sort.Strings(remoteIDs)
	for _, id := range remoteIDs {
		item := remote[id]
		operations = append(operations, reconcileOperation{
			label:       "Copy " + syncItemName(&item.manifest) + " to this device",
			sourceID:    id,
			sourceState: item.manifest.StateHash,
		})
	}
	return operations
}

// reviewReconcileTitleCandidates asks whether same-title items should be paired.
func (cmd *SyncCmd) reviewReconcileTitleCandidates(
	ctx context.Context,
	peerName string,
	input <-chan syncInput,
	local, remote map[string]reconcileItem,
	operations *[]reconcileOperation,
	pairedLocal, pairedRemote map[string]bool,
) error {
	localByTitle := make(map[reconcileItemKey][]string)
	remoteByTitle := make(map[reconcileItemKey][]string)
	for id, item := range local {
		key := reconcileItemKey{typeID: item.manifest.Type, title: item.manifest.Title}
		localByTitle[key] = append(localByTitle[key], id)
	}
	for id, item := range remote {
		key := reconcileItemKey{typeID: item.manifest.Type, title: item.manifest.Title}
		remoteByTitle[key] = append(remoteByTitle[key], id)
	}
	keys := make([]reconcileItemKey, 0)
	for key, localIDs := range localByTitle {
		if len(localIDs) > 0 && len(remoteByTitle[key]) > 0 {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].typeID != keys[j].typeID {
			return keys[i].typeID < keys[j].typeID
		}
		return keys[i].title < keys[j].title
	})
	for _, key := range keys {
		localIDs, remoteIDs := localByTitle[key], remoteByTitle[key]
		sort.Strings(localIDs)
		sort.Strings(remoteIDs)
		if len(localIDs) != 1 || len(remoteIDs) != 1 {
			if _, err := fmt.Fprintf(cmd.out, "\nAmbiguous possible match group: %s / %s (%d items here, %d on %s).\n", key.typeID.Label(), key.title, len(localIDs), len(remoteIDs), peerName); err != nil {
				return err
			}
			for _, id := range localIDs {
				if err := cmd.showReconcileSummary("This device", local[id]); err != nil {
					return err
				}
			}
			for _, id := range remoteIDs {
				if err := cmd.showReconcileSummary(peerName, remote[id]); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(cmd.out, "No pair is selected automatically; these items will remain separate and sync by their existing IDs."); err != nil {
				return err
			}
			if err := cmd.askAmbiguousReconcileChoice(ctx, input); err != nil {
				return err
			}
			continue
		}
		localID, remoteID := localIDs[0], remoteIDs[0]
		localItem, remoteItem := local[localID], remote[remoteID]
		if _, err := fmt.Fprintf(cmd.out, "\nPossible match: %s\n", key.typeID.Label()+" / "+key.title); err != nil {
			return err
		}
		if err := cmd.showReconcileComparison(localItem, remoteItem, peerName); err != nil {
			return err
		}
		suggested := "l"
		suggestion := "this device"
		if reconcileModifiedAt(remoteItem.manifest.ModifiedAt) > reconcileModifiedAt(localItem.manifest.ModifiedAt) {
			suggested = "r"
			suggestion = peerName
		}
		if _, err := fmt.Fprintf(cmd.out, "Suggested version: %s (most recently modified).\n", suggestion); err != nil {
			return err
		}
		decision, err := cmd.askReconcilePairDecision(ctx, input, suggested, peerName)
		if err != nil {
			return err
		}
		switch decision {
		case "l":
			pairedLocal[localID], pairedRemote[remoteID] = true, true
			*operations = append(*operations, reconcileOperation{
				label:       "Keep this device's version and ID for " + key.title,
				localSource: true,
				sourceID:    localID,
				sourceState: localItem.manifest.StateHash,
				removeID:    remoteID,
				removeState: remoteItem.manifest.StateHash,
			})
		case "r":
			pairedLocal[localID], pairedRemote[remoteID] = true, true
			*operations = append(*operations, reconcileOperation{
				label:       "Keep " + peerName + "'s version and ID for " + key.title,
				sourceID:    remoteID,
				sourceState: remoteItem.manifest.StateHash,
				removeID:    localID,
				removeLocal: true,
				removeState: localItem.manifest.StateHash,
			})
		case "s":
		}
	}
	return nil
}

// askAmbiguousReconcileChoice waits for the user to skip or cancel matching.
func (cmd *SyncCmd) askAmbiguousReconcileChoice(ctx context.Context, input <-chan syncInput) error {
	for {
		if _, err := fmt.Fprint(cmd.out, "[s] Skip matching and keep the items separate, [q] cancel reconciliation: "); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case answer := <-input:
			if answer.err != nil {
				return answer.err
			}
			switch strings.ToLower(strings.TrimSpace(answer.line)) {
			case "s":
				return nil
			case "q":
				return errCanceled
			default:
				if _, err := fmt.Fprintln(cmd.out, "Enter s or q."); err != nil {
					return err
				}
			}
		}
	}
}

type reconcileDiff struct {
	field  string
	local  string
	remote string
}

// showReconcileComparison displays the differing fields of two candidate items.
func (cmd *SyncCmd) showReconcileComparison(local, remote reconcileItem, peerName string) error {
	rows, err := reconcileDifferences(local, remote)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	table := tabwriter.NewWriter(cmd.out, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintf(table, "\nField\tThis device\t%s\n", peerName); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(table, "────────────────────────────────────────────────────────"); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(table, "%s\t%s\t%s\n", row.field, row.local, row.remote); err != nil {
			return err
		}
	}
	return table.Flush()
}

// reconcileDifferences returns the user-visible differences between two items.
func reconcileDifferences(local, remote reconcileItem) ([]reconcileDiff, error) {
	rows := make([]reconcileDiff, 0)
	add := func(field, left, right string) {
		if left != right {
			rows = append(rows, reconcileDiff{field: field, local: left, remote: right})
		}
	}
	add("Subtitle", reconcileOptionalString(local.item.Subtitle), reconcileOptionalString(remote.item.Subtitle))
	add("Favorite", reconcileFavorite(local.item.Favorite), reconcileFavorite(remote.item.Favorite))
	localScope, err := json.Marshal(local.item.Scope)
	if err != nil {
		return nil, fmt.Errorf("compare local item scope: %w", err)
	}
	remoteScope, err := json.Marshal(remote.item.Scope)
	if err != nil {
		return nil, fmt.Errorf("compare remote item scope: %w", err)
	}
	add("Scope", string(localScope), string(remoteScope))
	add("Tags", strings.Join(local.item.Tags, ", "), strings.Join(remote.item.Tags, ", "))
	if err := compareReconcileCredentials(&rows, local.item, remote.item); err != nil {
		return nil, err
	}
	localHistory, err := local.item.History()
	if err != nil {
		return nil, fmt.Errorf("read local item history: %w", err)
	}
	remoteHistory, err := remote.item.History()
	if err != nil {
		return nil, fmt.Errorf("read remote item history: %w", err)
	}
	add("Modified", reconcileTime(local.manifest.ModifiedAt), reconcileTime(remote.manifest.ModifiedAt))
	add("History", fmt.Sprintf("%d versions", len(localHistory.Entries)), fmt.Sprintf("%d versions", len(remoteHistory.Entries)))
	return rows, nil
}

// compareReconcileCredentials appends differences between item credentials.
func compareReconcileCredentials(rows *[]reconcileDiff, local, remote *model.Item) error {
	localAuth, localAuthIndex, localAuthErr := local.BasicAuth()
	remoteAuth, remoteAuthIndex, remoteAuthErr := remote.BasicAuth()
	if localAuthErr != nil || remoteAuthErr != nil {
		return fmt.Errorf("decode basic-auth credential for comparison")
	}
	if (localAuthIndex < 0) != (remoteAuthIndex < 0) {
		left, right := "Present", "Present"
		if localAuthIndex < 0 {
			left = "Absent"
		}
		if remoteAuthIndex < 0 {
			right = "Absent"
		}
		*rows = append(*rows, reconcileDiff{field: "Login credential", local: left, remote: right})
	}
	if !reconcileEditableEqual(localAuth.Username, remoteAuth.Username) {
		left, right := reconcileEditableDisplay(localAuth.Username), reconcileEditableDisplay(remoteAuth.Username)
		*rows = append(*rows, reconcileDiff{field: "Username", local: left, remote: right})
	}
	if !reconcileEditableEqual(localAuth.Password, remoteAuth.Password) {
		*rows = append(*rows, reconcileDiff{field: "Password", local: "[hidden]", remote: "[hidden]"})
	}
	localNote, localNoteIndex, localNoteErr := local.Note()
	remoteNote, remoteNoteIndex, remoteNoteErr := remote.Note()
	if localNoteErr != nil || remoteNoteErr != nil {
		return fmt.Errorf("decode note credential for comparison")
	}
	if (localNoteIndex < 0) != (remoteNoteIndex < 0) || !reconcileEditableEqual(&localNote.Content, &remoteNote.Content) {
		*rows = append(*rows, reconcileDiff{field: "Note", local: localNote.Content.Value, remote: remoteNote.Content.Value})
	}
	localDecoded, err := model.DecodeCredentials(local.Credentials)
	if err != nil {
		return fmt.Errorf("decode local credentials for comparison: %w", err)
	}
	remoteDecoded, err := model.DecodeCredentials(remote.Credentials)
	if err != nil {
		return fmt.Errorf("decode remote credentials for comparison: %w", err)
	}
	if reconcileTOTPSecret(localDecoded.TOTP) != reconcileTOTPSecret(remoteDecoded.TOTP) {
		*rows = append(*rows, reconcileDiff{field: "TOTP secret", local: reconcileTOTPSecret(localDecoded.TOTP), remote: reconcileTOTPSecret(remoteDecoded.TOTP)})
	}
	if reconcileTOTPUsername(localDecoded.TOTP) != reconcileTOTPUsername(remoteDecoded.TOTP) {
		*rows = append(*rows, reconcileDiff{field: "TOTP username", local: reconcileTOTPUsername(localDecoded.TOTP), remote: reconcileTOTPUsername(remoteDecoded.TOTP)})
	}
	if reconcileTOTPIssuer(localDecoded.TOTP) != reconcileTOTPIssuer(remoteDecoded.TOTP) {
		*rows = append(*rows, reconcileDiff{field: "TOTP issuer", local: reconcileTOTPIssuer(localDecoded.TOTP), remote: reconcileTOTPIssuer(remoteDecoded.TOTP)})
	}
	if reconcileTOTPSettings(localDecoded.TOTP) != reconcileTOTPSettings(remoteDecoded.TOTP) {
		*rows = append(*rows, reconcileDiff{field: "TOTP settings", local: reconcileTOTPSettings(localDecoded.TOTP), remote: reconcileTOTPSettings(remoteDecoded.TOTP)})
	}
	if reconcileSSHPrivateKey(localDecoded.SSHKey) != reconcileSSHPrivateKey(remoteDecoded.SSHKey) {
		*rows = append(*rows, reconcileDiff{field: "SSH private key", local: reconcileSSHPrivateKey(localDecoded.SSHKey), remote: reconcileSSHPrivateKey(remoteDecoded.SSHKey)})
	}
	if reconcileSSHKeyType(localDecoded.SSHKey) != reconcileSSHKeyType(remoteDecoded.SSHKey) {
		*rows = append(*rows, reconcileDiff{field: "SSH key type", local: reconcileSSHKeyType(localDecoded.SSHKey), remote: reconcileSSHKeyType(remoteDecoded.SSHKey)})
	}
	if reconcileSSHKeyComment(localDecoded.SSHKey) != reconcileSSHKeyComment(remoteDecoded.SSHKey) {
		*rows = append(*rows, reconcileDiff{field: "SSH key comment", local: reconcileSSHKeyComment(localDecoded.SSHKey), remote: reconcileSSHKeyComment(remoteDecoded.SSHKey)})
	}
	if !reconcileEditableEqual(reconcileSSHCreationDate(localDecoded.SSHKey), reconcileSSHCreationDate(remoteDecoded.SSHKey)) {
		*rows = append(*rows, reconcileDiff{field: "SSH creation date", local: reconcileEditableDisplay(reconcileSSHCreationDate(localDecoded.SSHKey)), remote: reconcileEditableDisplay(reconcileSSHCreationDate(remoteDecoded.SSHKey))})
	}
	if !reconcileEditableEqual(reconcileSSHExpiryDate(localDecoded.SSHKey), reconcileSSHExpiryDate(remoteDecoded.SSHKey)) {
		*rows = append(*rows, reconcileDiff{field: "SSH expiry date", local: reconcileEditableDisplay(reconcileSSHExpiryDate(localDecoded.SSHKey)), remote: reconcileEditableDisplay(reconcileSSHExpiryDate(remoteDecoded.SSHKey))})
	}
	if !reconcileEditableEqual(reconcileSSHKeyGenerationSource(localDecoded.SSHKey), reconcileSSHKeyGenerationSource(remoteDecoded.SSHKey)) {
		*rows = append(*rows, reconcileDiff{field: "SSH key generation source", local: reconcileEditableDisplay(reconcileSSHKeyGenerationSource(localDecoded.SSHKey)), remote: reconcileEditableDisplay(reconcileSSHKeyGenerationSource(remoteDecoded.SSHKey))})
	}
	localOther, err := reconcileOtherCredentials(local.Credentials)
	if err != nil {
		return fmt.Errorf("compare local credentials: %w", err)
	}
	remoteOther, err := reconcileOtherCredentials(remote.Credentials)
	if err != nil {
		return fmt.Errorf("compare remote credentials: %w", err)
	}
	if string(localOther) != string(remoteOther) {
		*rows = append(*rows, reconcileDiff{field: "Other credentials", local: string(localOther), remote: string(remoteOther)})
	}
	localFiles, err := local.Files()
	if err != nil {
		return fmt.Errorf("read local attachments for comparison: %w", err)
	}
	remoteFiles, err := remote.Files()
	if err != nil {
		return fmt.Errorf("read remote attachments for comparison: %w", err)
	}
	localFilesJSON, err := json.Marshal(localFiles)
	if err != nil {
		return fmt.Errorf("compare local attachments: %w", err)
	}
	remoteFilesJSON, err := json.Marshal(remoteFiles)
	if err != nil {
		return fmt.Errorf("compare remote attachments: %w", err)
	}
	if string(localFilesJSON) != string(remoteFilesJSON) {
		*rows = append(*rows, reconcileDiff{field: "Attachments", local: string(localFilesJSON), remote: string(remoteFilesJSON)})
	}
	return nil
}

// reconcileOptionalString formats an optional string for comparison output.
func reconcileOptionalString(value *string) string {
	if value == nil {
		return "[absent]"
	}
	return *value
}

// reconcileFavorite formats an optional favorite value for comparison output.
func reconcileFavorite(value *bool) string {
	if value == nil || !*value {
		return "No"
	}
	return "Yes"
}

// reconcileEditableEqual compares editable fields by type and value.
func reconcileEditableEqual(left, right *cxf.EditableField) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.FieldType == right.FieldType && left.Value == right.Value
}

// reconcileEditableDisplay returns an editable field's value for comparison output.
func reconcileEditableDisplay(field *cxf.EditableField) string {
	if field == nil {
		return ""
	}
	return field.Value
}

// reconcileTOTPSecret returns an authenticator secret or an empty string.
func reconcileTOTPSecret(credential *cxf.TOTP) string {
	if credential == nil {
		return ""
	}
	return credential.Secret
}

// reconcileTOTPUsername returns an authenticator username or an empty string.
func reconcileTOTPUsername(credential *cxf.TOTP) string {
	if credential == nil {
		return ""
	}
	return credential.Username
}

// reconcileTOTPIssuer returns an authenticator issuer or an empty string.
func reconcileTOTPIssuer(credential *cxf.TOTP) string {
	if credential == nil {
		return ""
	}
	return credential.Issuer
}

// reconcileTOTPSettings formats authenticator timing and algorithm settings.
func reconcileTOTPSettings(credential *cxf.TOTP) string {
	if credential == nil {
		return ""
	}
	return fmt.Sprintf("%d sec, %d digits, %s", credential.Period, credential.Digits, credential.Algorithm)
}

// reconcileSSHPrivateKey returns an SSH private key or an empty string.
func reconcileSSHPrivateKey(credential *cxf.SSHKey) string {
	if credential == nil {
		return ""
	}
	return credential.PrivateKey
}

// reconcileSSHKeyType returns an SSH key type or an empty string.
func reconcileSSHKeyType(credential *cxf.SSHKey) string {
	if credential == nil {
		return ""
	}
	return credential.KeyType
}

// reconcileSSHKeyComment returns an SSH key comment or an empty string.
func reconcileSSHKeyComment(credential *cxf.SSHKey) string {
	if credential == nil {
		return ""
	}
	return credential.KeyComment
}

// reconcileSSHCreationDate returns the SSH key creation date when present.
func reconcileSSHCreationDate(credential *cxf.SSHKey) *cxf.EditableField {
	if credential == nil {
		return nil
	}
	return credential.CreationDate
}

// reconcileSSHExpiryDate returns the SSH key expiry date when present.
func reconcileSSHExpiryDate(credential *cxf.SSHKey) *cxf.EditableField {
	if credential == nil {
		return nil
	}
	return credential.ExpiryDate
}

// reconcileSSHKeyGenerationSource returns the SSH key source when present.
func reconcileSSHKeyGenerationSource(credential *cxf.SSHKey) *cxf.EditableField {
	if credential == nil {
		return nil
	}
	return credential.KeyGenerationSource
}

// reconcileOtherCredentials encodes credentials outside the standard types.
func reconcileOtherCredentials(credentials []jsontext.Value) ([]byte, error) {
	other := make([]jsontext.Value, 0)
	for _, raw := range credentials {
		typeName, err := cxf.CredentialTypeOf(raw)
		if err != nil {
			return nil, err
		}
		switch typeName {
		case cxf.CredentialTypeBasicAuth, cxf.CredentialTypeNote, cxf.CredentialTypeTOTP, cxf.CredentialTypeSSHKey, cxf.CredentialTypeFile:
			continue
		default:
			other = append(other, raw)
		}
	}
	return json.Marshal(other)
}

// reconcileTime formats a timestamp for reconciliation output.
func reconcileTime(timestamp *uint64) string {
	if timestamp == nil {
		return "Unknown"
	}
	return time.Unix(int64(*timestamp), 0).Local().Format("02 Jan 2006, 15:04")
}

// showReconcileSummary prints an item's side and modification time.
func (cmd *SyncCmd) showReconcileSummary(side string, item reconcileItem) error {
	_, err := fmt.Fprintf(cmd.out, "  %s — Modified %s\n", side, reconcileTime(item.manifest.ModifiedAt))
	return err
}

// reconcileModifiedAt returns zero when the modification time is absent.
func reconcileModifiedAt(timestamp *uint64) uint64 {
	if timestamp == nil {
		return 0
	}
	return *timestamp
}

// askReconcileSyncDecision reads a valid sync decision from stdin.
func (cmd *SyncCmd) askReconcileSyncDecision(ctx context.Context, input <-chan syncInput, kind pawsync.DifferenceKind, peer string) (pawsync.Decision, error) {
	for {
		for _, option := range syncDecisionMenu(kind, peer) {
			if _, err := fmt.Fprintf(cmd.out, "  %s\n", option); err != nil {
				return "", err
			}
		}
		if _, err := fmt.Fprint(cmd.out, "Choice: "); err != nil {
			return "", err
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case answer := <-input:
			if answer.err != nil {
				return "", answer.err
			}
			decision, quit, err := parseSyncDecision(kind, answer.line)
			if err != nil {
				if _, writeErr := fmt.Fprintln(cmd.out, err); writeErr != nil {
					return "", writeErr
				}
				continue
			}
			if quit {
				return "", errCanceled
			}
			return decision, nil
		}
	}
}

// askReconcilePairDecision reads how to resolve a candidate item pair.
func (cmd *SyncCmd) askReconcilePairDecision(ctx context.Context, input <-chan syncInput, suggested, peer string) (string, error) {
	for {
		if _, err := fmt.Fprintf(cmd.out, "  [l] Keep this device's version and ID\n  [r] Keep %s's version and ID\n  [s] Reject this match and keep both IDs\n  [q] Cancel reconciliation\nChoice (suggested %s): ", peer, suggested); err != nil {
			return "", err
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case answer := <-input:
			if answer.err != nil {
				return "", answer.err
			}
			choice := strings.ToLower(strings.TrimSpace(answer.line))
			if choice == "l" || choice == "r" || choice == "s" {
				return choice, nil
			}
			if choice == "q" {
				return "", errCanceled
			}
			if _, err := fmt.Fprintln(cmd.out, "Enter l, r, s, or q."); err != nil {
				return "", err
			}
		}
	}
}

// reconcileHistoriesEquivalent compares item history entries without item IDs.
func reconcileHistoriesEquivalent(local, remote *model.Item) (bool, error) {
	localHistory, err := local.History()
	if err != nil {
		return false, err
	}
	remoteHistory, err := remote.History()
	if err != nil {
		return false, err
	}
	if len(localHistory.Entries) != len(remoteHistory.Entries) {
		return false, nil
	}
	for index, localEntry := range localHistory.Entries {
		remoteEntry := remoteHistory.Entries[index]
		if localEntry.CapturedAt != remoteEntry.CapturedAt {
			return false, nil
		}
		localHash, err := reconcileSnapshotStateHash(&localEntry.Item)
		if err != nil {
			return false, err
		}
		remoteHash, err := reconcileSnapshotStateHash(&remoteEntry.Item)
		if err != nil {
			return false, err
		}
		if localHash != remoteHash {
			return false, nil
		}
	}
	return true, nil
}

// reconcileSnapshotStateHash hashes a history snapshot independently of its ID.
func reconcileSnapshotStateHash(snapshot *cxf.Item) (string, error) {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return "", err
	}
	var item model.Item
	if err := json.Unmarshal(data, &item); err != nil {
		return "", err
	}
	item.ID = "AAAAAAAAAAAAAAAAAAAAAA"
	return item.StateHash()
}

// executeReconciliation applies reviewed copies and removes replaced IDs.
func (cmd *SyncCmd) executeReconciliation(
	ctx context.Context,
	session reconcileSession,
	peer pawsync.Peer,
	localItems map[string]reconcileItem,
	operations []reconcileOperation,
) (bool, error) {
	completed := 0
	total := 0
	for _, operation := range operations {
		total++
		if operation.removeID != "" {
			total++
		}
	}
	for _, operation := range operations {
		if _, err := fmt.Fprintf(cmd.out, "[%d/%d] %s... ", completed+1, total, operation.label); err != nil {
			return false, err
		}
		var err error
		if operation.localSource {
			local := localItems[operation.sourceID]
			err = session.CopyItemToPeer(ctx, peer, local.item, operation.sourceState, operation.targetState)
		} else {
			err = session.CopyRemoteItemToLocal(ctx, peer, operation.sourceID, operation.sourceState, operation.targetState)
		}
		if err != nil {
			if _, writeErr := fmt.Fprintf(cmd.errOut, "Failed: %v\n", err); writeErr != nil {
				return false, writeErr
			}
			if _, writeErr := fmt.Fprintf(cmd.errOut, "%d operations completed; %d operations are incomplete or were not attempted. The current transfer outcome is uncertain.\n", completed, total-completed); writeErr != nil {
				return false, writeErr
			}
			return false, err
		}
		completed++
		if _, err := fmt.Fprintln(cmd.out, "Copied and verified"); err != nil {
			return false, err
		}
		if operation.removeID == "" {
			continue
		}
		device := peer.Name
		if operation.removeLocal {
			device = "this device"
		}
		if _, err := fmt.Fprintf(cmd.out, "[%d/%d] Remove old ID %s from %s... ", completed+1, total, operation.removeID, device); err != nil {
			return false, err
		}
		if operation.removeLocal {
			err = session.DeleteLocalItem(ctx, operation.removeID, operation.removeState)
		} else {
			err = session.DeleteRemoteItem(ctx, peer, operation.removeID, operation.removeState)
		}
		if err != nil {
			if _, writeErr := fmt.Fprintf(cmd.errOut, "Failed: %v\n", err); writeErr != nil {
				return false, writeErr
			}
			if _, writeErr := fmt.Fprintf(cmd.errOut, "The replacement is copied and verified; removal was not confirmed. %d operations completed; %d operations are incomplete or were not attempted.\n", completed, total-completed); writeErr != nil {
				return false, writeErr
			}
			return false, err
		}
		completed++
		if _, err := fmt.Fprintln(cmd.out, "Removed"); err != nil {
			return false, err
		}
	}
	_, err := fmt.Fprintf(cmd.out, "\nReconciliation completed. %d operations completed.\n", completed)
	return true, err
}
