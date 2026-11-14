// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/test"
)

func TestResolveVaultInfo(t *testing.T) {
	personalID := testVaultID(1, 1)
	secondPersonalID := testVaultID(1, 2)
	workID := testVaultID(2, 1)
	workVaultID := storage.FormatVaultID(workID)
	vaults := []model.VaultInfo{
		{ID: personalID, Name: "Personal"},
		{ID: secondPersonalID, Name: "Personal"},
		{ID: workID, Name: "Work"},
		{ID: testVaultID(3, 1), Name: workVaultID[:8]},
	}
	tests := []struct {
		name string
		ref  string
		id   string
		err  string
	}{
		{name: "unique name", ref: "Work", id: workID},
		{name: "full Vault ID", ref: workVaultID, id: workID},
		{name: "unique Vault ID prefix", ref: workVaultID[:7], id: workID},
		{name: "short prefix", ref: workVaultID[:6], err: "does not exist"},
		{name: "raw internal ID", ref: workID, err: "does not exist"},
		{name: "duplicate name", ref: "Personal", err: "ambiguous vault"},
		{name: "shared prefix", ref: storage.FormatVaultID(personalID)[:7], err: "ambiguous vault"},
		{name: "name and prefix collide", ref: workVaultID[:8], err: "ambiguous vault"},
		{name: "not found", ref: "missing", err: "does not exist"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info, err := resolveVaultInfo(vaults, test.ref)
			if test.err != "" {
				require.ErrorContains(t, err, test.err)
				if test.name == "shared prefix" {
					require.Contains(t, err.Error(), storage.FormatVaultID(personalID)[:7])
					require.NotContains(t, err.Error(), personalID)
				}
				if test.name == "duplicate name" {
					require.Contains(t, err.Error(), "Personal")
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.id, info.ID)
		})
	}
}

func TestListVaultSelectionAndUniqueIDPrefixes(t *testing.T) {
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()
	vault := test.CreateVaultWithPassword(t, store, "Personal", "password")
	otherVault := test.CreateVault(t, store, "Work")
	item, err := model.NewNote("memo", "note")
	require.NoError(t, err)
	require.NoError(t, store.StoreItem(vault, item))
	prompter := newPrompter(strings.NewReader(""), io.Discard)
	prompter.readPassword = func() ([]byte, error) { return []byte("password"), nil }
	var output bytes.Buffer
	cmd := &ListCmd{command: &command{storage: store, prompter: prompter, out: &output}, vaultName: "Personal"}
	require.NoError(t, cmd.Run(context.Background()))
	require.Contains(t, output.String(), "Personal ("+shortVaultID(vault.Accounts[0].ID)+")\n")
	require.NotContains(t, output.String(), "paw/")
	require.Contains(t, output.String(), "note")
	require.Contains(t, output.String(), "memo")
	ids := []string{vault.Accounts[0].ID, otherVault.Accounts[0].ID}
	prefixes := vaultIDPrefixes(ids)
	listCmd := &ListCmd{command: &command{storage: store}}
	node, err := listCmd.vaults()
	require.NoError(t, err)
	require.Len(t, node.Child, 2)
	listed := node.Child[0].Value + "\n" + node.Child[1].Value
	require.Contains(t, listed, "Personal ("+prefixes[vault.Accounts[0].ID]+")")
	require.Contains(t, listed, "Work ("+prefixes[otherVault.Accounts[0].ID]+")")
	require.NotEqual(t, prefixes[vault.Accounts[0].ID], prefixes[otherVault.Accounts[0].ID])
}

func TestSyncRejectsAmbiguousVaultBeforeUnlock(t *testing.T) {
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()
	test.CreateVault(t, store, "Personal")
	test.CreateVault(t, store, "Personal")
	cmd := &SyncCmd{command: &command{storage: store, getenv: os.Getenv, prompter: newPrompter(strings.NewReader(""), nil)}, vaultName: "Personal"}
	err = cmd.Run(context.Background())
	require.ErrorContains(t, err, "ambiguous vault")
}

func TestVaultIDPrefixesAreUniqueAndAtLeastSevenCharacters(t *testing.T) {
	first, second := testVaultID(1, 1), testVaultID(1, 2)
	prefixes := vaultIDPrefixes([]string{first, second})
	require.GreaterOrEqual(t, len(prefixes[first]), 7)
	require.GreaterOrEqual(t, len(prefixes[second]), 7)
	require.NotEqual(t, prefixes[first], prefixes[second])
	require.True(t, strings.HasPrefix(storage.FormatVaultID(first), prefixes[first]))
	require.True(t, strings.HasPrefix(storage.FormatVaultID(second), prefixes[second]))
}

func TestVaultIDPrefixesUseUserFacingIDs(t *testing.T) {
	ids := []string{testVaultID(1, 1), testVaultID(2, 1)}
	prefixes := vaultIDPrefixes(ids)
	for _, id := range ids {
		displayID := storage.FormatVaultID(id)
		prefix := prefixes[id]
		require.GreaterOrEqual(t, len(prefix), 7)
		require.True(t, strings.HasPrefix(displayID, prefix))
		require.NotContains(t, prefix, "-")
		require.NotContains(t, prefix, "_")
		require.NotContains(t, prefix, id)
	}
}

func testVaultID(firstByte, lastByte byte) string {
	data := make([]byte, 16)
	data[0] = firstByte
	data[15] = lastByte
	return base64.RawURLEncoding.EncodeToString(data)
}
