// SPDX-FileCopyrightText: 2023-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later


package agent_test

import (
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"lucor.dev/paw/internal/agent"
	"lucor.dev/paw/internal/paw"
)

func Test_Client(t *testing.T) {

	root := filepath.Join(os.TempDir(), t.Name())
	t.Cleanup(func() {
		os.RemoveAll(root)
	})

	s, err := paw.NewOSStorageRooted(root)
	require.NoError(t, err)

	server := agent.NewCLI()
	defer server.Close()
	go agent.Run(server, s.SocketAgentPath())

	key, err := s.CreateVaultKey(t.Name(), "secret")
	require.NoError(t, err)

	_, err = s.CreateVault(t.Name(), key)
	require.NoError(t, err)

	client, err := agent.NewClient(s.SocketAgentPath())
	require.NoError(t, err)

	t.Run("interface", func(t *testing.T) {
		at, err := client.Type()
		require.NoError(t, err)
		require.EqualValues(t, agent.CLI, at)

		sid, err := client.Unlock(t.Name(), key, 0)
		require.NoError(t, err)
		require.Contains(t, sid, agent.SessionIDPrefix)

		sessions, err := client.Sessions()
		require.NoError(t, err)
		assert.Len(t, sessions, 1)

		assert.Equal(t, t.Name(), sessions[0].Vault)
		assert.Equal(t, sid, sessions[0].ID)

		keySession, err := client.Key(t.Name(), sid)
		require.NoError(t, err)
		require.Equal(t, key, keySession)

		err = client.Lock(t.Name())
		require.NoError(t, err)
	})

	t.Run("lifetime session", func(t *testing.T) {
		lifetime := 1 * time.Millisecond
		sid, err := client.Unlock(t.Name(), key, lifetime)
		require.NoError(t, err)
		require.Contains(t, sid, agent.SessionIDPrefix)

		sessions, err := client.Sessions()
		require.NoError(t, err)
		assert.Len(t, sessions, 1)

		assert.Equal(t, t.Name(), sessions[0].Vault)
		assert.Equal(t, sid, sessions[0].ID)

		keySession, err := client.Key(t.Name(), sid)
		require.NoError(t, err)
		require.Equal(t, key, keySession)

		time.Sleep(lifetime)

		keySession, err = client.Key(t.Name(), sid)
		log.Println(err)
		require.Error(t, err)
		require.Nil(t, keySession)
	})
}
