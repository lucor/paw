package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/paw"
)

// TestItemTypeCompatibility protects stored category values during package moves.
func TestItemTypeCompatibility(t *testing.T) {
	for _, test := range []struct {
		kind  ItemType
		value int
		name  string
	}{
		{NoteItemType, 2, "note"},
		{PasswordItemType, 4, "password"},
		{LoginItemType, 8, "login"},
		{SSHKeyItemType, 16, "ssh_key"},
	} {
		require.Equal(t, test.value, int(test.kind))
		kind, err := ItemTypeFromString(test.name)
		require.NoError(t, err)
		require.Equal(t, test.kind, kind)
		require.Equal(t, test.name, kind.String())
	}
	_, err := ItemTypeFromString("metadata")
	require.Error(t, err)
}

func TestSettingsCompatibility(t *testing.T) {
	var state AppState
	require.NoError(t, json.Unmarshal([]byte(`{"settings":{"theme":"dark","auto_lock":"1m","password":{"random":{"default_length":27,"default_format":15}}}}`), &state))
	require.Equal(t, ThemeDark, state.Settings.Theme)
	require.Equal(t, AutoLockOneMinute, state.Settings.AutoLock)
	require.Equal(t, 27, state.Settings.Password.Random.DefaultLength)
	data, err := json.Marshal(&state)
	require.NoError(t, err)
	var restored AppState
	require.NoError(t, json.Unmarshal(data, &restored))
	require.Equal(t, state, restored)
	defaults := NewDefaultSettings()
	require.Equal(t, AutoLockNever, defaults.AutoLock)
	require.Equal(t, paw.RandomPasswordDefaultLength, defaults.Password.Random.DefaultLength)
}

func TestVaultCatalogLookup(t *testing.T) {
	vaults := []VaultInfo{{ID: "first", Name: "Personal"}, {ID: "second", Name: "Work"}}
	info, err := FindVaultInfo(vaults, "personal")
	require.NoError(t, err)
	require.Equal(t, vaults[0], info)
	info, err = FindVaultInfo(vaults, "second")
	require.NoError(t, err)
	require.Equal(t, vaults[1], info)
	_, err = FindVaultInfo(vaults, "missing")
	require.Error(t, err)
	vaults = append(vaults, VaultInfo{ID: "third", Name: "PERSONAL"})
	_, err = FindVaultInfo(vaults, "Personal")
	require.ErrorContains(t, err, "ambiguous")
	require.Equal(t, DefaultVaultAppearance(), NormalizeVaultAppearance(VaultAppearance{Icon: "invalid", Color: "invalid"}))
}
