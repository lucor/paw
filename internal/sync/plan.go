// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"fmt"
	"slices"
	"strings"
)

// Decision is the operation selected for one difference.
type Decision string

const (
	CopyLocalToRemote Decision = "copy_local_to_remote"
	CopyRemoteToLocal Decision = "copy_remote_to_local"
	DeleteLocal       Decision = "delete_local"
	DeleteRemote      Decision = "delete_remote"
	Skip              Decision = "skip"
	RequiresDecision  Decision = "requires_decision"
)

// PlanEntry stores the original difference and state hashes needed for review.
// An empty state hash means that item was absent on that side.
type PlanEntry struct {
	ItemID          string         `json:"item_id"`
	Kind            DifferenceKind `json:"kind"`
	Decision        Decision       `json:"decision"`
	LocalStateHash  string         `json:"local_state_hash,omitempty"`
	RemoteStateHash string         `json:"remote_state_hash,omitempty"`
}

// Plan contains editable operations and is kept in memory by its caller.
type Plan struct {
	Entries  []PlanEntry `json:"entries"`
	original []PlanEntry
}

// PlanSummary counts the decisions currently selected in a plan.
type PlanSummary struct {
	CopyLocalToRemote int
	CopyRemoteToLocal int
	DeleteLocal       int
	DeleteRemote      int
	Skipped           int
	RequiresDecision  int
}

// BuildPlan creates default decisions from the diff and original manifests.
func BuildPlan(diff []Difference, local, remote *Manifest) (*Plan, error) {
	actual, err := BuildDiff(local, remote)
	if err != nil {
		return nil, err
	}
	if !slices.Equal(diff, actual) {
		return nil, fmt.Errorf("diff does not match the supplied manifests")
	}
	localHashes := manifestStateHashes(local)
	remoteHashes := manifestStateHashes(remote)
	plan := &Plan{Entries: make([]PlanEntry, 0, len(diff))}
	for _, difference := range diff {
		entry := PlanEntry{
			ItemID:          difference.ItemID,
			Kind:            difference.Kind,
			LocalStateHash:  localHashes[difference.ItemID],
			RemoteStateHash: remoteHashes[difference.ItemID],
		}
		switch difference.Kind {
		case Identical:
			continue
		case LocalOnly:
			entry.Decision = CopyLocalToRemote
		case RemoteOnly:
			entry.Decision = CopyRemoteToLocal
		case ContentDifferent, StateDifferent:
			entry.Decision = RequiresDecision
		default:
			return nil, fmt.Errorf("diff contains an unsupported classification")
		}
		if err := validateEntry(entry); err != nil {
			return nil, err
		}
		plan.Entries = append(plan.Entries, entry)
	}
	slices.SortFunc(plan.Entries, func(a, b PlanEntry) int {
		return strings.Compare(a.ItemID, b.ItemID)
	})
	plan.original = slices.Clone(plan.Entries)
	return plan, nil
}

// SetDecision changes one entry after validating it against its original diff.
func (plan *Plan) SetDecision(itemID string, decision Decision) error {
	if plan == nil {
		return fmt.Errorf("plan is required")
	}
	for index := range plan.Entries {
		entry := &plan.Entries[index]
		if entry.ItemID != itemID {
			continue
		}
		previous := entry.Decision
		entry.Decision = decision
		if err := validateEntry(*entry); err != nil {
			entry.Decision = previous
			return err
		}
		return nil
	}
	return fmt.Errorf("item is not in the plan")
}

// RequireDecisions clears suggested copy actions before an interactive review.
func (plan *Plan) RequireDecisions() error {
	if err := plan.Validate(); err != nil {
		return err
	}
	for index := range plan.Entries {
		plan.Entries[index].Decision = RequiresDecision
	}
	return nil
}

// ValidateDecision checks whether an operation is allowed for a diff type.
func ValidateDecision(kind DifferenceKind, decision Decision) error {
	return validateAllowedDecision(kind, decision)
}

// Validate checks the plan against its original diff and manifests.
func (plan *Plan) Validate() error {
	if plan == nil {
		return fmt.Errorf("plan is required")
	}
	if len(plan.Entries) != len(plan.original) {
		return fmt.Errorf("plan entries do not match the original diff")
	}
	for index, entry := range plan.Entries {
		if err := validateEntry(entry); err != nil {
			return err
		}
		original := plan.original[index]
		if entry.ItemID != original.ItemID || entry.Kind != original.Kind {
			return fmt.Errorf("plan entries do not match the original diff")
		}
		if entry.LocalStateHash != original.LocalStateHash ||
			entry.RemoteStateHash != original.RemoteStateHash {
			return fmt.Errorf("plan state hashes do not match the original manifests")
		}
	}
	return nil
}

// Ready reports whether the plan is valid and has no unresolved decisions.
func (plan *Plan) Ready() (bool, error) {
	if err := plan.Validate(); err != nil {
		return false, err
	}
	for _, entry := range plan.Entries {
		if entry.Decision == RequiresDecision {
			return false, nil
		}
	}
	return true, nil
}

// Summary derives operation counts from the current plan entries.
func (plan *Plan) Summary() PlanSummary {
	var summary PlanSummary
	if plan == nil {
		return summary
	}
	for _, entry := range plan.Entries {
		switch entry.Decision {
		case CopyLocalToRemote:
			summary.CopyLocalToRemote++
		case CopyRemoteToLocal:
			summary.CopyRemoteToLocal++
		case DeleteLocal:
			summary.DeleteLocal++
		case DeleteRemote:
			summary.DeleteRemote++
		case Skip:
			summary.Skipped++
		case RequiresDecision:
			summary.RequiresDecision++
		}
	}
	return summary
}

func validateEntry(entry PlanEntry) error {
	localExists := entry.LocalStateHash != ""
	remoteExists := entry.RemoteStateHash != ""
	switch entry.Kind {
	case LocalOnly:
		if !localExists || remoteExists {
			return fmt.Errorf("local-only entry has invalid item presence")
		}
	case RemoteOnly:
		if localExists || !remoteExists {
			return fmt.Errorf("remote-only entry has invalid item presence")
		}
	case ContentDifferent, StateDifferent:
		if !localExists || !remoteExists {
			return fmt.Errorf("different entry must exist on both sides")
		}
	default:
		return fmt.Errorf("unsupported diff classification")
	}
	return validateAllowedDecision(entry.Kind, entry.Decision)
}

func validateAllowedDecision(kind DifferenceKind, decision Decision) error {
	allowed := false
	switch kind {
	case LocalOnly:
		allowed = decision == CopyLocalToRemote || decision == DeleteLocal || decision == Skip || decision == RequiresDecision
	case RemoteOnly:
		allowed = decision == CopyRemoteToLocal || decision == DeleteRemote || decision == Skip || decision == RequiresDecision
	case ContentDifferent, StateDifferent:
		allowed = decision == CopyLocalToRemote || decision == CopyRemoteToLocal ||
			decision == Skip || decision == RequiresDecision
	}
	if !allowed {
		return fmt.Errorf("decision is not allowed for diff classification")
	}
	return nil
}

func manifestStateHashes(manifest *Manifest) map[string]string {
	hashes := make(map[string]string, len(manifest.Items))
	for _, item := range manifest.Items {
		hashes[item.ID] = item.StateHash
	}
	return hashes
}
