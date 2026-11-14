// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	pawsync "go.lucor.dev/paw/internal/sync"
)

func TestSyncReviewLatestOptions(t *testing.T) {
	app := test.NewTempApp(t)
	older, newer, invalid, zero := uint64(100), uint64(200), uint64(math.MaxInt64)+1, uint64(0)
	cases := []struct {
		name          string
		local, remote *uint64
		latest        string
		decision      pawsync.Decision
	}{
		{
			name:     "local",
			local:    &newer,
			remote:   &older,
			latest:   "Keep this device’s version (Latest)",
			decision: pawsync.CopyLocalToRemote,
		},
		{
			name:     "remote",
			local:    &older,
			remote:   &newer,
			latest:   "Keep fedora’s version (Latest)",
			decision: pawsync.CopyRemoteToLocal,
		},
		{
			name:   "equal",
			local:  &older,
			remote: &older,
		},
		{
			name:   "missing",
			remote: &older,
		},
		{
			name:   "zero",
			local:  &zero,
			remote: &older,
		},
		{
			name:   "invalid",
			local:  &invalid,
			remote: &older,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			local := &pawsync.Manifest{Items: []pawsync.ManifestItem{{
				ID:          "one",
				Type:        model.LoginItemType,
				Title:       "Example",
				ModifiedAt:  tc.local,
				ContentHash: reviewHash(1),
				StateHash:   reviewHash(1),
			}}}
			remote := &pawsync.Manifest{Items: []pawsync.ManifestItem{{
				ID:          "one",
				Type:        model.LoginItemType,
				Title:       "Example",
				ModifiedAt:  tc.remote,
				ContentHash: reviewHash(2),
				StateHash:   reviewHash(2),
			}}}
			diff, err := pawsync.BuildDiff(local, remote)
			require.NoError(t, err)
			plan, err := pawsync.BuildPlan(diff, local, remote)
			require.NoError(t, err)
			require.NoError(t, plan.RequireDecisions())
			for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
				app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
				fyne.DoAndWait(func() {
					view := newSyncReviewView("fedora", pawsync.VaultReference{}, pawsync.VaultReference{}, diff, local, remote, plan, func() {}, func() {})
					window := app.NewWindow("Sync")
					window.SetContent(view.view)
					window.Resize(fyne.NewSize(320, 540))
					selectAction := view.actions["one"]
					require.Equal(t, "Choose action", selectAction.Selected)
					require.True(t, view.review.Disabled())
					if tc.latest != "" {
						require.Contains(t, selectAction.Options, tc.latest)
						selectAction.SetSelected(tc.latest)
						require.Equal(t, tc.decision, plan.Entries[0].Decision)
					} else {
						for _, option := range selectAction.Options {
							require.NotContains(t, option, "(Latest)")
						}
					}
					selectAction.SetSelected("Skip")
					require.Equal(t, pawsync.Skip, plan.Entries[0].Decision)
					require.Equal(t, "1 of 1 reviewed", view.progress.Text)
					require.False(t, view.review.Disabled())
					selectAction.SetSelected("Choose action")
					require.Equal(t, "0 of 1 reviewed", view.progress.Text)
					require.True(t, view.review.Disabled())
					window.Close()
				})
			}
		})
	}
}
