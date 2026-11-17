// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeviceNamePersists(t *testing.T) {
	store := newTestFileStorage(t)
	state, err := store.LoadAppState()
	require.NoError(t, err)
	require.NotEmpty(t, state.Settings.DeviceName)
	initial := state.Settings.DeviceName
	data, err := store.root.ReadFile(appStateFileName)
	require.NoError(t, err)
	require.Contains(t, string(data), `"deviceName"`)
	require.Contains(t, string(data), initial)
	state, err = store.LoadAppState()
	require.NoError(t, err)
	require.Equal(t, initial, state.Settings.DeviceName)
	state.Settings.DeviceName = "My device"
	require.NoError(t, store.StoreAppState(state))
	state, err = store.LoadAppState()
	require.NoError(t, err)
	require.Equal(t, "My device", state.Settings.DeviceName)
}
