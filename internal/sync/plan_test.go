// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildPlanDefaultsAndPreservesStateHashes(t *testing.T) {
	local := &Manifest{Items: []ManifestItem{
		planItem("local", 1, 0),
		planItem("content", 2, 2),
		planItem("state", 3, 3),
		planItem("same", 4, 4),
	}}
	remote := &Manifest{Items: []ManifestItem{
		planItem("remote", 5, 0),
		planItem("content", 6, 6),
		planItem("state", 3, 7),
		planItem("same", 4, 4),
	}}
	diff, err := BuildDiff(local, remote)
	require.NoError(t, err)
	plan, err := BuildPlan(diff, local, remote)
	require.NoError(t, err)
	require.Equal(t, []PlanEntry{
		{ItemID: "content", Kind: ContentDifferent, Decision: RequiresDecision,
			LocalStateHash: planHash(2), RemoteStateHash: planHash(6)},
		{ItemID: "local", Kind: LocalOnly, Decision: CopyLocalToRemote,
			LocalStateHash: planHash(0)},
		{ItemID: "remote", Kind: RemoteOnly, Decision: CopyRemoteToLocal,
			RemoteStateHash: planHash(0)},
		{ItemID: "state", Kind: StateDifferent, Decision: RequiresDecision,
			LocalStateHash: planHash(3), RemoteStateHash: planHash(7)},
	}, plan.Entries)
	ready, err := plan.Ready()
	require.NoError(t, err)
	require.False(t, ready)
	require.Equal(t, PlanSummary{
		CopyLocalToRemote: 1,
		CopyRemoteToLocal: 1,
		RequiresDecision:  2,
	}, plan.Summary())
}

func TestPlanAllowsDecisionsByOriginalClassification(t *testing.T) {
	cases := []struct {
		kind      DifferenceKind
		decisions []Decision
		invalid   Decision
	}{
		{LocalOnly, []Decision{CopyLocalToRemote, DeleteLocal, Skip, RequiresDecision}, CopyRemoteToLocal},
		{RemoteOnly, []Decision{CopyRemoteToLocal, DeleteRemote, Skip, RequiresDecision}, DeleteLocal},
		{ContentDifferent, []Decision{
			CopyLocalToRemote, CopyRemoteToLocal, Skip, RequiresDecision,
		}, DeleteLocal},
		{StateDifferent, []Decision{
			CopyLocalToRemote, CopyRemoteToLocal, Skip, RequiresDecision,
		}, DeleteRemote},
	}
	for _, test := range cases {
		t.Run(string(test.kind), func(t *testing.T) {
			for _, decision := range test.decisions {
				require.NoError(t, ValidateDecision(test.kind, decision))
			}
			require.Error(t, ValidateDecision(test.kind, test.invalid))
		})
	}
}

func TestRequireDecisionsClearsSuggestedCopies(t *testing.T) {
	local := &Manifest{Items: []ManifestItem{planItem("local", 1, 0)}}
	remote := &Manifest{Items: []ManifestItem{planItem("remote", 2, 0)}}
	diff, err := BuildDiff(local, remote)
	require.NoError(t, err)
	plan, err := BuildPlan(diff, local, remote)
	require.NoError(t, err)
	require.NoError(t, plan.RequireDecisions())
	require.Equal(t, RequiresDecision, plan.Entries[0].Decision)
	require.Equal(t, RequiresDecision, plan.Entries[1].Decision)
	ready, err := plan.Ready()
	require.NoError(t, err)
	require.False(t, ready)
}

func TestPlanEditingReadinessAndSummary(t *testing.T) {
	local := &Manifest{Items: []ManifestItem{planItem("local", 1, 0)}}
	remote := &Manifest{Items: []ManifestItem{planItem("remote", 2, 0)}}
	diff, err := BuildDiff(local, remote)
	require.NoError(t, err)
	plan, err := BuildPlan(diff, local, remote)
	require.NoError(t, err)
	require.NoError(t, plan.SetDecision("local", Skip))
	require.NoError(t, plan.SetDecision("remote", DeleteRemote))
	require.NoError(t, plan.Validate())
	ready, err := plan.Ready()
	require.NoError(t, err)
	require.True(t, ready)
	require.Equal(t, PlanSummary{Skipped: 1, DeleteRemote: 1}, plan.Summary())
	require.ErrorContains(t, plan.SetDecision("local", CopyRemoteToLocal), "not allowed")
	require.ErrorContains(t, plan.SetDecision("missing", Skip), "not in the plan")
	require.Equal(t, Skip, plan.Entries[0].Decision)
}

func TestPlanValidationRejectsUnknownDuplicateAndChangedHashes(t *testing.T) {
	local := &Manifest{Items: []ManifestItem{planItem("local", 1, 0)}}
	remote := &Manifest{}
	diff, err := BuildDiff(local, remote)
	require.NoError(t, err)
	plan, err := BuildPlan(diff, local, remote)
	require.NoError(t, err)
	plan.Entries[0].ItemID = "unknown"
	require.ErrorContains(t, plan.Validate(), "do not match")
	plan.Entries[0].ItemID = "local"
	plan.Entries = append(plan.Entries, plan.Entries[0])
	require.ErrorContains(t, plan.Validate(), "do not match")
	plan.Entries = plan.Entries[:1]
	plan.Entries[0].LocalStateHash = planHash(9)
	require.ErrorContains(t, plan.Validate(), "state hashes")
}

func TestEmptyPlanIsValidAndReady(t *testing.T) {
	plan, err := BuildPlan(nil, &Manifest{}, &Manifest{})
	require.NoError(t, err)
	require.Empty(t, plan.Entries)
	ready, err := plan.Ready()
	require.NoError(t, err)
	require.True(t, ready)
	require.Empty(t, plan.Summary())
}

func TestBuildPlanRejectsInvalidDiff(t *testing.T) {
	local := &Manifest{Items: []ManifestItem{planItem("item", 1, 1)}}
	remote := &Manifest{}
	diff, err := BuildDiff(local, remote)
	require.NoError(t, err)
	duplicate := append(append([]Difference(nil), diff...), diff[0])
	_, err = BuildPlan(duplicate, local, remote)
	require.ErrorContains(t, err, "diff does not match")
	_, err = BuildPlan([]Difference{{ItemID: "item", Kind: Identical}}, local, remote)
	require.ErrorContains(t, err, "diff does not match")
}

func planItem(id string, content, state byte) ManifestItem {
	return ManifestItem{
		ID:          id,
		ContentHash: planHash(content),
		StateHash:   planHash(state),
	}
}

func planHash(value byte) string {
	return hashByte(value)
}
