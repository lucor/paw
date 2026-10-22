// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package cli

import (
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	sshagent "golang.org/x/crypto/ssh/agent"

	"lucor.dev/paw/internal/agent"
	"lucor.dev/paw/internal/paw"
	"lucor.dev/paw/internal/sshkey"
)

type sessionStorage struct {
	paw.StorageMock
	socketPath string
}

func (s *sessionStorage) SocketAgentPath() string { return s.socketPath }

func TestCommandsWithAgentSession(t *testing.T) {
	// Keep the socket path short enough for Unix sockets on macOS.
	root, err := os.MkdirTemp("", "paw-cli-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(root) })
	s := &sessionStorage{socketPath: filepath.Join(root, "agent.sock")}
	listener, err := net.Listen("unix", s.socketPath)
	require.NoError(t, err)
	server := agent.NewCLI()
	var connections []net.Conn
	var workers sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			connections = append(connections, conn)
			workers.Go(func() { _ = sshagent.ServeAgent(server, conn) })
		}
	}()
	t.Cleanup(func() {
		listener.Close()
		<-done
		for _, conn := range connections {
			conn.Close()
		}
		workers.Wait()
		server.Close()
	})

	client, err := agent.NewClient(s.socketPath)
	require.NoError(t, err)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	session, err := client.Unlock("personal", key, 0)
	require.NoError(t, err)
	vault := paw.NewVault(key, "personal")
	item := paw.NewNote()
	item.Name = "example"
	item.Value = "example note"
	require.NoError(t, vault.AddItem(item))
	loaded := 0
	s.OnLoadVault = func(name string, actualKey *paw.Key) (*paw.Vault, error) {
		require.Equal(t, "personal", name)
		require.Equal(t, key, actualKey)
		loaded++
		return vault, nil
	}
	s.OnLoadItem = func(actualVault *paw.Vault, metadata *paw.Metadata) (paw.Item, error) {
		require.Same(t, vault, actualVault)
		require.Equal(t, item.Name, metadata.Name)
		require.Equal(t, paw.NoteItemType, metadata.Type)
		return item, nil
	}
	s.OnLoadAppState = func() (*paw.AppState, error) { return &paw.AppState{}, nil }
	s.OnVaults = func() ([]string, error) { return []string{"personal"}, nil }

	t.Setenv(paw.ENV_SESSION, session)
	require.NoError(t, Run([]string{"paw", "cli", "show", "personal/note/example"}, s))
	require.Equal(t, 1, loaded)

	t.Setenv(paw.ENV_SESSION, "invalid-session")
	require.NoError(t, Run([]string{"paw", "cli", "show", "--session", session, "personal/note/example"}, s))
	require.Equal(t, 2, loaded)
	require.Equal(t, "invalid-session", os.Getenv(paw.ENV_SESSION))

	require.NoError(t, Run([]string{"paw", "cli", "ls", "personal/*", "--session", session}, s))
	require.Equal(t, 3, loaded)

	t.Setenv(paw.ENV_SESSION, session)
	missing := filepath.Join(root, "missing-key")
	err = Run([]string{"paw", "cli", "add", "-i", missing, "personal/ssh_key/import"}, s)
	require.True(t, os.IsNotExist(err), "expected an SSH key read error, got %v", err)

	t.Run("cancel SSH key import", func(t *testing.T) {
		imported, err := sshkey.GenerateKey()
		require.NoError(t, err)
		keyPath := filepath.Join(root, "import-key")
		require.NoError(t, os.WriteFile(keyPath, imported.MarshalPrivateKey(), 0600))
		// Missing storage callbacks also make an unintended write fail the test.
		_, _, err = executeWithInput(t, []string{"add", "-i", keyPath, "personal/ssh_key/import"}, newCommand(s), "n\n")
		require.ErrorIs(t, err, errCanceled)
	})
	t.Run("cancel removal", func(t *testing.T) {
		_, _, err := executeWithInput(t, []string{"rm", "personal/note/example"}, newCommand(s), "n\n")
		require.NoError(t, err)
		require.True(t, vault.HasItem(item))
	})
}
