// SPDX-FileCopyrightText: 2023-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package agent_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/agent"
	"go.lucor.dev/paw/internal/storage"
)

func Test_Client(t *testing.T) {

	s, err := storage.NewOSStorageRooted(t.TempDir())
	require.NoError(t, err)

	server := agent.NewCLI()
	defer server.Close()
	go agent.Run(server, s.SocketAgentPath())

	vault, err := s.CreateVault("client-test", "secret")
	require.NoError(t, err)
	vaultID := vault.Accounts[0].ID
	key := vault.Key()

	client, err := agent.NewClient(s.SocketAgentPath())
	require.NoError(t, err)

	t.Run("interface", func(t *testing.T) {
		at, err := client.Type()
		require.NoError(t, err)
		require.EqualValues(t, agent.CLI, at)

		sid, err := client.Unlock(vaultID, key, 0)
		require.NoError(t, err)
		require.Contains(t, sid, agent.SessionIDPrefix)

		sessions, err := client.Sessions()
		require.NoError(t, err)
		assert.Len(t, sessions, 1)

		assert.Equal(t, vaultID, sessions[0].Vault)
		assert.Equal(t, sid, sessions[0].ID)

		keySession, err := client.Key(vaultID, sid)
		require.NoError(t, err)
		require.Equal(t, key, keySession)

		err = client.Lock(vaultID)
		require.NoError(t, err)
	})

	t.Run("lifetime session", func(t *testing.T) {
		lifetime := 1 * time.Second
		sid, err := client.Unlock(vaultID, key, lifetime)
		require.NoError(t, err)
		require.Contains(t, sid, agent.SessionIDPrefix)

		sessions, err := client.Sessions()
		require.NoError(t, err)
		assert.Len(t, sessions, 1)

		assert.Equal(t, vaultID, sessions[0].Vault)
		assert.Equal(t, sid, sessions[0].ID)

		keySession, err := client.Key(vaultID, sid)
		require.NoError(t, err)
		require.Equal(t, key, keySession)

		time.Sleep(lifetime + 10*time.Millisecond)

		keySession, err = client.Key(vaultID, sid)
		require.Error(t, err)
		require.Nil(t, keySession)
	})
}
