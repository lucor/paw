// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package cli

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	sshagent "golang.org/x/crypto/ssh/agent"

	"go.lucor.dev/paw/internal/agent"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/sshkey"
	"go.lucor.dev/paw/internal/storage"
)

func TestCommandsWithAgentSession(t *testing.T) {
	// Keep the socket path short enough for Unix sockets on macOS.
	root, err := os.MkdirTemp("", "paw-cli-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(root) })
	store, err := storage.NewFileStorage(filepath.Join(root, "data"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	s := store
	listener, err := net.Listen("unix", s.SocketAgentPath())
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

	client, err := agent.NewClient(s.SocketAgentPath())
	require.NoError(t, err)
	p := newPrompter(strings.NewReader(""), io.Discard)
	p.readPassword = func() ([]byte, error) { return []byte("synthetic-password"), nil }
	base := &command{storage: store, prompter: p, now: time.Now, getenv: os.Getenv, readFile: os.ReadFile,
		agentClient: func(string) (agent.PawAgent, error) { return client, nil }}
	created, _, err := executeWithInput(t, []string{"init", "personal"}, newInitCommand(base), "")
	require.NoError(t, err)
	require.Contains(t, created, "created")
	require.NotContains(t, created, "synthetic-password")
	infos, err := store.Vaults()
	require.NoError(t, err)
	require.Len(t, infos, 1)
	key, err := store.LoadVaultKey(infos[0].ID, "synthetic-password")
	require.NoError(t, err)
	vault, err := store.LoadVault(infos[0].ID, key)
	require.NoError(t, err)
	session, err := client.Unlock(vault.Accounts[0].ID, vault.Key(), 0)
	require.NoError(t, err)
	item, err := model.NewNote("example", "example note")
	require.NoError(t, err)
	require.NoError(t, store.StoreItem(vault, item))
	vaultID := vault.Accounts[0].ID

	t.Setenv(paw.ENV_SESSION, session)
	require.NoError(t, Run([]string{"paw", "cli", "show", vaultID + "/note/example"}, s))

	t.Setenv(paw.ENV_SESSION, "invalid-session")
	require.NoError(t, Run([]string{"paw", "cli", "show", "--session", session, vaultID + "/note/example"}, s))
	require.Equal(t, "invalid-session", os.Getenv(paw.ENV_SESSION))

	require.NoError(t, Run([]string{"paw", "cli", "ls", vaultID + "/*", "--session", session}, s))
	createdPath := vaultID + "/note/created"
	beforeAdd := time.Now().UTC()
	added, _, err := executeWithInput(t, []string{"add", "--session", session, createdPath}, newCommand(s), "created note\n")
	require.NoError(t, err)
	require.Contains(t, added, "added")
	state, err := store.LoadAppState()
	require.NoError(t, err)
	require.False(t, state.Modified.Before(beforeAdd))
	require.False(t, state.Modified.After(time.Now().UTC()))
	require.NoError(t, Run([]string{"paw", "cli", "show", "--session", session, createdPath}, s))
	_, _, err = executeWithInput(t, []string{"edit", "--session", session, createdPath}, newCommand(s), "updated note\n")
	require.NoError(t, err)
	loaded, err := store.LoadVault(vaultID, vault.Key())
	require.NoError(t, err)
	updated, err := store.LoadItem(loaded, loaded.Accounts[0].Items[1].ID)
	require.NoError(t, err)
	content, _, err := updated.Note()
	require.NoError(t, err)
	require.Equal(t, "updated note", content.Content.Value)
	_, _, err = executeWithInput(t, []string{"rm", "--session", session, createdPath}, newCommand(s), "y\n")
	require.NoError(t, err)
	loaded, err = store.LoadVault(vaultID, vault.Key())
	require.NoError(t, err)
	require.Len(t, loaded.Accounts[0].Items, 1)

	t.Setenv(paw.ENV_SESSION, session)
	missing := filepath.Join(root, "missing-key")
	err = Run([]string{"paw", "cli", "add", "-i", missing, vaultID + "/ssh_key/import"}, s)
	require.True(t, os.IsNotExist(err), "expected an SSH key read error, got %v", err)

	t.Run("cancel SSH key import", func(t *testing.T) {
		imported, err := sshkey.GenerateKey()
		require.NoError(t, err)
		keyPath := filepath.Join(root, "import-key")
		require.NoError(t, os.WriteFile(keyPath, imported.MarshalPrivateKey(), 0600))
		_, _, err = executeWithInput(t, []string{"add", "-i", keyPath, vaultID + "/ssh_key/import"}, newCommand(s), "n\n")
		require.ErrorIs(t, err, errCanceled)
		loaded, err := store.LoadVault(vaultID, vault.Key())
		require.NoError(t, err)
		require.Len(t, loaded.Accounts[0].Items, 1)
	})
	t.Run("cancel removal", func(t *testing.T) {
		_, _, err := executeWithInput(t, []string{"rm", vaultID + "/note/example"}, newCommand(s), "n\n")
		require.NoError(t, err)
		loaded, err := store.LoadVault(vaultID, vault.Key())
		require.NoError(t, err)
		require.Len(t, loaded.Accounts[0].Items, 1)
	})
	for _, name := range []string{"add", "edit", "rm"} {
		path := vaultID + "/note/example"
		if name == "add" {
			path = vaultID + "/note/incomplete"
		}
		_, _, err := executeWithInput(t, []string{name, "--session", session, path}, newCommand(s), "")
		require.ErrorIs(t, err, io.EOF)
		unchanged, err := store.LoadVault(vaultID, key)
		require.NoError(t, err)
		require.Len(t, unchanged.Accounts[0].Items, 1)
		unchangedItem, err := store.LoadItem(unchanged, item.ID)
		require.NoError(t, err)
		note, _, err := unchangedItem.Note()
		require.NoError(t, err)
		require.Equal(t, "example note", note.Content.Value)
	}

	login, err := model.NewLogin("web", "", "", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	note, err := model.NewNote("memo", "")
	require.NoError(t, err)
	_, _, err = executeWithInput(t, []string{"add", "--session", session, vaultID + "/password/secret"}, newCommand(s), "3\n\n")
	require.NoError(t, err)
	loaded, err = store.LoadVault(vaultID, key)
	require.NoError(t, err)
	summary, err := loaded.FindItem("secret", model.PasswordItemType)
	require.NoError(t, err)
	password, err := store.LoadItem(loaded, summary.ID)
	require.NoError(t, err)
	auth, _, err := password.BasicAuth()
	require.NoError(t, err)
	require.Equal(t, paw.PinPassword, model.PasswordSettingsFromField(auth.Password).Mode)
	require.Len(t, auth.Password.Value, paw.PinPasswordDefaultLength)
	generated, err := sshkey.GenerateKey()
	require.NoError(t, err)
	credential, err := model.EncodeSSHKey(generated.PrivateKey(), generated.PublicKey().Type(), "deploy key")
	require.NoError(t, err)
	sshKey, err := model.NewSSHKey("deploy", credential, "", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	sshKey.Metadata.AddToAgent = true
	for _, item := range []*model.Item{login, note, sshKey} {
		require.NoError(t, store.StoreItem(loaded, item))
	}
	output, _, err := executeWithInput(t, []string{"ls", "--session", session, vaultID + "/*"}, newCommand(s), "")
	require.NoError(t, err)
	for _, want := range []string{"login", "web", "note", "memo", "password", "secret", "ssh_key", "deploy"} {
		require.Contains(t, output, want)
	}

	t.Run("clipboard", func(t *testing.T) {
		calls := 0
		base.clipboard = func(ctx context.Context, value []byte) error {
			calls++
			require.Equal(t, auth.Password.Value, string(value))
			_, ok := ctx.Deadline()
			require.True(t, ok)
			return nil
		}
		args := []string{"show", "-c", "--session", session, vaultID + "/password/secret"}
		out, _, err := executeWithInput(t, args, newShowCommand(base), "")
		require.NoError(t, err)
		require.Equal(t, 1, calls)
		require.Contains(t, out, "copied")
		require.NotContains(t, out, auth.Password.Value)
		failure := errors.New("clipboard unavailable")
		base.clipboard = func(context.Context, []byte) error { return failure }
		_, _, err = executeWithInput(t, args, newShowCommand(base), "")
		require.ErrorIs(t, err, failure)
		base.clipboard = func(ctx context.Context, _ []byte) error { return ctx.Err() }
		cmd := newShowCommand(base)
		cmd.Writer, cmd.ErrWriter = io.Discard, io.Discard
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.ErrorIs(t, cmd.Run(ctx, args), context.Canceled)
	})

	t.Run("agent sessions and SSH keys", func(t *testing.T) {
		out, _, err := executeWithInput(t, []string{"unlock", "-t", "45m", "personal"}, newUnlockCommand(base), "")
		require.NoError(t, err)
		require.Contains(t, out, "Session ID:")
		require.Contains(t, out, "added:")
		sessions, err := client.Sessions()
		require.NoError(t, err)
		require.Len(t, sessions, 2)
		expiring := 0
		for _, active := range sessions {
			require.Equal(t, vaultID, active.Vault)
			if active.Lifetime > 0 {
				expiring++
				require.Greater(t, active.Lifetime, 44*time.Minute)
				require.LessOrEqual(t, active.Lifetime, 45*time.Minute)
			}
		}
		require.Equal(t, 1, expiring)
		keys, err := server.List()
		require.NoError(t, err)
		require.Len(t, keys, 1)
		require.Equal(t, "deploy key", keys[0].Comment)
		out, _, err = executeWithInput(t, []string{"agent", "sessions"}, newAgentCommand(base), "")
		require.NoError(t, err)
		require.Contains(t, out, session)
		_, _, err = executeWithInput(t, []string{"agent", "start"}, newAgentCommand(base), "")
		require.ErrorContains(t, err, "already running")
		base.agentClient = func(string) (agent.PawAgent, error) { return nil, errors.New("not running") }
		started := false
		base.startAgent = func(path string) error { started = true; require.Equal(t, s.SocketAgentPath(), path); return nil }
		_, _, err = executeWithInput(t, []string{"agent", "start"}, newAgentCommand(base), "")
		require.NoError(t, err)
		require.True(t, started)
		base.agentClient = func(string) (agent.PawAgent, error) { return client, nil }
		out, _, err = executeWithInput(t, []string{"lock", "personal"}, newLockCommand(base), "")
		require.NoError(t, err)
		require.Contains(t, out, "Removed key:")
		keys, err = server.List()
		require.NoError(t, err)
		require.Empty(t, keys)
		sessions, err = client.Sessions()
		require.NoError(t, err)
		require.Empty(t, sessions)
	})
	t.Run("expired session falls back to password", func(t *testing.T) {
		out, diagnostics, err := executeWithInput(t, []string{"show", "--session", session, "personal/note/example"}, newShowCommand(base), "")
		require.NoError(t, err)
		require.Contains(t, out, "example note")
		require.Contains(t, diagnostics, "Session is invalid or expired")
		require.NotContains(t, out+diagnostics, "synthetic-password")
	})
	session, err = client.Unlock(vaultID, key, 0)
	require.NoError(t, err)
	t.Setenv(paw.ENV_SESSION, session)
	t.Run("export and restore", func(t *testing.T) {
		plainPath := filepath.Join(root, "credentials.cxf")
		base.prompter = nil
		_, warning, err := executeWithInput(t, []string{"export", "--plaintext", "--session", session, "personal", plainPath}, newExportCommand(base), "y\n")
		require.NoError(t, err)
		require.Contains(t, warning, "credentials in plaintext")
		_, err = os.Stat(plainPath)
		require.NoError(t, err)
		base.prompter = newPrompter(strings.NewReader(""), io.Discard)
		base.prompter.readPassword = func() ([]byte, error) { return []byte("backup-secret"), nil }
		backupPath := filepath.Join(root, "backup.without-extension")
		_, _, err = executeWithInput(t, []string{"export", "--session", session, "personal", backupPath}, newExportCommand(base), "")
		require.NoError(t, err)
		passwords := []string{"backup-secret", "new-master", "new-master"}
		base.prompter = newPrompter(strings.NewReader("y\n"), io.Discard)
		base.prompter.readPassword = func() ([]byte, error) { value := passwords[0]; passwords = passwords[1:]; return []byte(value), nil }
		out, _, err := executeWithInput(t, []string{"restore", "--name", "Restored", backupPath}, newRestoreCommand(base), "")
		require.NoError(t, err)
		require.Contains(t, out, "Vault restored successfully")
		require.Contains(t, out, "independent copy")
		require.NotContains(t, out, "new-master")
		require.NotContains(t, out, "backup-secret")
		infos, err := store.Vaults()
		require.NoError(t, err)
		require.Len(t, infos, 2)
		info, err := model.FindVaultInfo(infos, "Restored")
		require.NoError(t, err)
		require.NotEqual(t, vaultID, info.ID)
		restoredKey, err := store.LoadVaultKey(info.ID, "new-master")
		require.NoError(t, err)
		restored, err := store.LoadVault(info.ID, restoredKey)
		require.NoError(t, err)
		require.Len(t, restored.Accounts[0].Items, len(loaded.Accounts[0].Items))
		summary, err := restored.FindItem("secret", model.PasswordItemType)
		require.NoError(t, err)
		item, err := store.LoadItem(restored, summary.ID)
		require.NoError(t, err)
		restoredAuth, _, err := item.BasicAuth()
		require.NoError(t, err)
		require.Equal(t, auth.Password.Value, restoredAuth.Password.Value)
		_, err = store.LoadVaultKey(info.ID, "backup-secret")
		require.Error(t, err)
		out, err = execute(t, []string{"ls"}, store)
		require.NoError(t, err)
		require.Contains(t, out, "personal")
		require.Contains(t, out, "Restored")
	})
}
