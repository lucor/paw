// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package paw

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFindVaultInfoFindsIDsAndNames checks both accepted vault identifiers.
func TestFindVaultInfoFindsIDsAndNames(t *testing.T) {
	vaults := []VaultInfo{
		{ID: "first-id", Name: "Personal"},
		{ID: "second-id", Name: "Work"},
	}
	byID, err := FindVaultInfo(vaults, "first-id")
	require.NoError(t, err)
	require.Equal(t, vaults[0], byID)
	byName, err := FindVaultInfo(vaults, "Work")
	require.NoError(t, err)
	require.Equal(t, vaults[1], byName)
}

// TestFindVaultInfoRejectsUnknownAndAmbiguousNames checks bad catalog lookups fail.
func TestFindVaultInfoRejectsUnknownAndAmbiguousNames(t *testing.T) {
	_, err := FindVaultInfo([]VaultInfo{{ID: "first-id", Name: "Personal"}}, "missing")
	require.ErrorContains(t, err, "does not exist")
	_, err = FindVaultInfo([]VaultInfo{
		{ID: "first-id", Name: "Personal"},
		{ID: "second-id", Name: "Personal"},
	}, "Personal")
	require.ErrorContains(t, err, "ambiguous")
}
