// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import "fmt"

// BuildClonePlan prepares copies from the source into an empty vault.
func BuildClonePlan(local, remote *Manifest) (*Plan, error) {
	if local == nil || len(local.Items) != 0 {
		return nil, fmt.Errorf("clone destination vault must be empty")
	}
	diff, err := BuildDiff(local, remote)
	if err != nil {
		return nil, err
	}
	for _, item := range diff {
		if item.Kind != RemoteOnly {
			return nil, fmt.Errorf("clone source contains an unexpected difference")
		}
	}
	return BuildPlan(diff, local, remote)
}
