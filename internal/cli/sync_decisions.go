// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"fmt"
	"strings"

	pawsync "go.lucor.dev/paw/internal/sync"
)

type syncDecisionOption struct {
	key       string
	decision  pawsync.Decision
	label     string
	peerLabel bool
	quit      bool
}

var syncDecisionOptions = map[pawsync.DifferenceKind][]syncDecisionOption{
	pawsync.LocalOnly: {
		{key: "c", decision: pawsync.CopyLocalToRemote, label: "Copy to %s", peerLabel: true},
		{key: "d", decision: pawsync.DeleteLocal, label: "Delete from this device"},
		{key: "s", decision: pawsync.Skip, label: "Skip"},
		{key: "q", label: "Cancel", quit: true},
	},
	pawsync.RemoteOnly: {
		{key: "c", decision: pawsync.CopyRemoteToLocal, label: "Copy to this device"},
		{key: "d", decision: pawsync.DeleteRemote, label: "Delete from %s", peerLabel: true},
		{key: "s", decision: pawsync.Skip, label: "Skip"},
		{key: "q", label: "Cancel", quit: true},
	},
	pawsync.ContentDifferent: {
		{key: "l", decision: pawsync.CopyLocalToRemote, label: "Keep this device's version"},
		{key: "r", decision: pawsync.CopyRemoteToLocal, label: "Keep %s's version", peerLabel: true},
		{key: "s", decision: pawsync.Skip, label: "Skip"},
		{key: "q", label: "Cancel", quit: true},
	},
	pawsync.StateDifferent: {
		{key: "l", decision: pawsync.CopyLocalToRemote, label: "Keep this device's version"},
		{key: "r", decision: pawsync.CopyRemoteToLocal, label: "Keep %s's version", peerLabel: true},
		{key: "s", decision: pawsync.Skip, label: "Skip"},
		{key: "q", label: "Cancel", quit: true},
	},
}

// syncDecisionMenu returns the choices available for a synchronization difference.
func syncDecisionMenu(kind pawsync.DifferenceKind, peer string) []string {
	options := syncDecisionOptions[kind]
	menu := make([]string, 0, len(options))
	for _, option := range options {
		label := option.label
		if option.peerLabel {
			label = fmt.Sprintf(label, peer)
		}
		menu = append(menu, fmt.Sprintf("[%s] %s", option.key, label))
	}
	return menu
}

// parseSyncDecision maps one explicit terminal choice to an allowed plan action.
// parseSyncDecision validates a user's choice for a synchronization difference.
func parseSyncDecision(kind pawsync.DifferenceKind, input string) (pawsync.Decision, bool, error) {
	choice := strings.ToLower(strings.TrimSpace(input))
	if len([]rune(choice)) != 1 {
		return "", false, fmt.Errorf("enter one choice")
	}
	for _, option := range syncDecisionOptions[kind] {
		if choice == option.key {
			return option.decision, option.quit, nil
		}
	}
	return "", false, fmt.Errorf("choice is not valid for this item")
}
