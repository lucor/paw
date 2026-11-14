package model

import (
	json "encoding/json/v2"
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

func TestItemTypeIsZero(t *testing.T) {
	for _, test := range []struct {
		itemType ItemType
		want     bool
	}{
		{itemType: NoteItemType},
		{itemType: PasswordItemType},
		{itemType: LoginItemType},
		{itemType: SSHKeyItemType},
		{itemType: 0, want: true},
		{itemType: 1, want: true},
	} {
		require.Equal(t, test.want, test.itemType.IsZero())
	}
}

func TestSettingsCompatibility(t *testing.T) {
	var state AppState
	require.NoError(t, json.Unmarshal([]byte(`{"settings":{"theme":"dark","auto_lock":"1m","password":{"random":{"default_length":27,"default_format":15}}}}`), &state))
	require.Equal(t, ThemeDark, state.Settings.Theme)
	require.Equal(t, AutoLockOneMinute, state.Settings.AutoLock)
	require.Equal(t, 27, state.Settings.Password.Random.DefaultLength)
	data, err := json.Marshal(&state, json.Deterministic(true))
	require.NoError(t, err)
	var restored AppState
	require.NoError(t, json.Unmarshal(data, &restored))
	require.Equal(t, state, restored)
	defaults := NewDefaultSettings()
	require.Equal(t, AutoLockNever, defaults.AutoLock)
	require.Equal(t, paw.RandomPasswordDefaultLength, defaults.Password.Random.DefaultLength)
}

func TestVaultCatalogLookup(t *testing.T) {
	vaults := []VaultInfo{
		{ID: "first", Name: "Personal"},
		{ID: "second", Name: "Work"},
		{ID: "third", Name: "PERSONAL"},
	}
	cases := []struct {
		name  string
		ref   string
		want  VaultInfo
		found bool
	}{
		{name: "exact name", ref: "Personal", want: vaults[0], found: true},
		{name: "exact name with different case", ref: "PERSONAL", want: vaults[2], found: true},
		{name: "case sensitive name", ref: "personal"},
		{name: "ID", ref: "second", want: vaults[1], found: true},
		{name: "missing", ref: "missing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := FindVaultInfo(vaults, tc.ref)
			if !tc.found {
				require.ErrorContains(t, err, "does not exist")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, info)
		})
	}
}

func TestNormalizeVaultAppearance(t *testing.T) {
	require.Equal(t, DefaultVaultAppearance(), NormalizeVaultAppearance(VaultAppearance{Icon: "invalid", Color: "invalid"}))
}
