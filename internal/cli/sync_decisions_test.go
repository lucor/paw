// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"go.lucor.dev/paw/internal/storage"
	pawsync "go.lucor.dev/paw/internal/sync"
)

func TestParseSyncDecisionRequiresExplicitAllowedChoice(t *testing.T) {
	tests := []struct {
		kind pawsync.DifferenceKind
		in   string
		want pawsync.Decision
		quit bool
		bad  bool
	}{
		{kind: pawsync.LocalOnly, in: "C", want: pawsync.CopyLocalToRemote},
		{kind: pawsync.LocalOnly, in: "d", want: pawsync.DeleteLocal},
		{kind: pawsync.LocalOnly, in: "s", want: pawsync.Skip},
		{kind: pawsync.RemoteOnly, in: "c", want: pawsync.CopyRemoteToLocal},
		{kind: pawsync.RemoteOnly, in: "d", want: pawsync.DeleteRemote},
		{kind: pawsync.ContentDifferent, in: "l", want: pawsync.CopyLocalToRemote},
		{kind: pawsync.StateDifferent, in: "R", want: pawsync.CopyRemoteToLocal},
		{kind: pawsync.StateDifferent, in: "s", want: pawsync.Skip},
		{kind: pawsync.LocalOnly, in: "q", quit: true},
		{kind: pawsync.LocalOnly, in: "", bad: true},
		{kind: pawsync.RemoteOnly, in: " ", bad: true},
		{kind: pawsync.ContentDifferent, in: "c", bad: true},
		{kind: pawsync.StateDifferent, in: "d", bad: true},
		{kind: pawsync.LocalOnly, in: "cc", bad: true},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind)+"/"+tt.in, func(t *testing.T) {
			got, quit, err := parseSyncDecision(tt.kind, tt.in)
			if tt.bad {
				require.Error(t, err)
				require.Empty(t, got)
				require.False(t, quit)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.Equal(t, tt.quit, quit)
		})
	}
}

func TestSyncDecisionMenusMatchParser(t *testing.T) {
	for kind, options := range syncDecisionOptions {
		menu := syncDecisionMenu(kind, "Peer")
		require.Len(t, menu, len(options)-1)
		for _, option := range options {
			decision, quit, err := parseSyncDecision(kind, option.key)
			require.NoError(t, err)
			require.Equal(t, option.decision, decision)
			require.Equal(t, option.quit, quit)
			if !option.quit {
				require.Contains(t, strings.Join(menu, "\n"), fmt.Sprintf("[%s]", option.key))
			}
		}
	}
}

func TestSyncCommandRequiresExistingVault(t *testing.T) {
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	_, _, err = executeWithInput(t, []string{"paw cli", "sync", "missing"}, store, "")
	require.Error(t, err)
	require.NotContains(t, strings.ToLower(fmt.Sprint(err)), "create vault")
	require.NotErrorIs(t, err, context.Canceled)
}
