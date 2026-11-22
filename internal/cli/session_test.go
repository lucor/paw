// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package cli

import (
	"context"
	"crypto"
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

type sshFailureAgent struct {
	agent.PawAgent
	err error
}

func (a sshFailureAgent) AddSSHKey(crypto.PrivateKey, string) error { return a.err }

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
	created, diagnostics, err := executeSubcommand(t, []string{"init", "personal"}, newInitCommand, base, "")
	require.NoError(t, err)
	require.Empty(t, created)
	require.Empty(t, diagnostics)
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
	vaultPath := storage.FormatVaultID(vaultID)

	t.Setenv(paw.ENV_SESSION, session)
	require.NoError(t, Run([]string{"paw", "cli", "show", vaultPath + "/note/example"}, s))
	require.NoError(t, Run([]string{"paw", "cli", "attachment", "list", vaultPath + "/note/example"}, s))

	t.Setenv(paw.ENV_SESSION, "invalid-session")
	require.NoError(t, Run([]string{"paw", "cli", "show", "--session", session, vaultPath + "/note/example"}, s))
	require.Equal(t, "invalid-session", os.Getenv(paw.ENV_SESSION))

	require.NoError(t, Run([]string{"paw", "cli", "ls", vaultPath + "/*", "--session", session}, s))
	addArgs := []string{"add", "note", "--session", session, vaultPath, "created"}
	added, _, err := executeWithInput(t, addArgs, s, "created note\n")
	require.Empty(t, added)
	require.ErrorContains(t, err, "requires stdin to be a terminal")
	addBase := *base
	addPrompter := testInteractivePrompter(t, "n\n")
	addPrompter.editor = &testTextEditor{value: "created note\n"}
	addBase.prompter = addPrompter
	added, diagnostics, err = executeSubcommand(t, addArgs, newAddCommand, &addBase, "")
	require.NoError(t, err)
	require.Empty(t, diagnostics)
	updatedVault, err := store.LoadVault(vaultID, key)
	require.NoError(t, err)
	addedItem, err := updatedVault.FindItem("created", model.NoteItemType)
	require.NoError(t, err)
	require.Equal(t, addedItem.ID+"\n", added)
	failureBase := addBase
	failurePrompter := testInteractivePrompter(t, "n\n")
	failurePrompter.editor = &testTextEditor{value: "another note\n"}
	failureBase.prompter = failurePrompter
	writeFailure := errors.New("stdout unavailable")
	writer := &failingAddWriter{err: writeFailure}
	failureBase.out = writer
	failureCommand := newAddCommand(&failureBase)
	failureCommand.ExitErrHandler = leaveExitToCaller
	failureCommand.Reader = strings.NewReader("")
	failureCommand.Writer = io.Discard
	failureCommand.ErrWriter = io.Discard
	err = failureCommand.Run(context.Background(), []string{"add", "note", "--session", session, vaultPath, "another"})
	require.ErrorIs(t, err, writeFailure)
	require.Equal(t, 1, writer.writes)
	_, _, err = executeWithInput(t, []string{"edit", "--session", session, vaultPath + "/note/example"}, s, "updated note\n")
	require.ErrorContains(t, err, "requires stdin to be a terminal")
	loaded, err := store.LoadVault(vaultID, vault.Key())
	require.NoError(t, err)
	require.Len(t, loaded.Accounts[0].Items, 3)
	updated, err := store.LoadItem(loaded, item.ID)
	require.NoError(t, err)
	content, _, err := updated.Note()
	require.NoError(t, err)
	require.Equal(t, "example note", content.Content.Value)
	_, _, err = executeWithInput(t, []string{"rm", "--session", session, vaultPath + "/note/example"}, s, "y\n")
	require.ErrorContains(t, err, "requires stdin to be a terminal")
	loaded, err = store.LoadVault(vaultID, vault.Key())
	require.NoError(t, err)
	require.Len(t, loaded.Accounts[0].Items, 3)

	t.Setenv(paw.ENV_SESSION, session)
	missing := filepath.Join(root, "missing-key")
	err = Run([]string{"paw", "cli", "add", "ssh-key", "--input", missing, vaultPath, "import"}, s)
	require.True(t, os.IsNotExist(err), "expected an SSH key read error, got %v", err)

	t.Run("removal confirmation requires a terminal", func(t *testing.T) {
		_, _, err := executeWithInput(t, []string{"rm", vaultPath + "/note/example"}, s, "n\n")
		require.ErrorContains(t, err, "requires stdin to be a terminal")
		loaded, err := store.LoadVault(vaultID, vault.Key())
		require.NoError(t, err)
		require.Len(t, loaded.Accounts[0].Items, 3)
	})
	for _, name := range []string{"add", "edit", "rm"} {
		args := []string{name, "--session", session, vaultPath + "/note/example"}
		if name == "add" {
			args = []string{"add", "note", "--session", session, vaultPath, "incomplete"}
		}
		_, _, err := executeWithInput(t, args, s, "")
		require.ErrorContains(t, err, "requires stdin to be a terminal")
		unchanged, err := store.LoadVault(vaultID, key)
		require.NoError(t, err)
		require.Len(t, unchanged.Accounts[0].Items, 3)
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
	_, _, err = executeWithInput(t, []string{"add", "password", "--session", session, vaultPath, "secret"}, s, "3\n\n")
	require.ErrorContains(t, err, "requires stdin to be a terminal")
	loaded, err = store.LoadVault(vaultID, key)
	require.NoError(t, err)
	_, err = loaded.FindItem("secret", model.PasswordItemType)
	require.Error(t, err)
	passwordItem, err := model.NewPassword("secret", "1234", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	require.NoError(t, store.StoreItem(loaded, passwordItem))
	auth, _, err := passwordItem.BasicAuth()
	require.NoError(t, err)
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
	output, _, err := executeWithInput(t, []string{"ls", "--session", session, vaultPath + "/*"}, s, "")
	require.NoError(t, err)
	for _, want := range []string{"login", "web", "note", "memo", "password", "secret", "ssh_key", "deploy"} {
		require.Contains(t, output, want)
	}
	loginPath := vaultPath + "/login/web"
	_, _, err = executeWithInput(t, []string{"edit", "--session", session, "--field", "API/Token", "--type", "hidden", loginPath}, s, "  synthetic-token\n")
	require.NoError(t, err)
	unchanged, _, err := executeWithInput(t, []string{"edit", "--session", session, "--field", "API/Token", loginPath}, s, "  synthetic-token\n")
	require.NoError(t, err)
	require.Contains(t, unchanged, "unchanged")
	shown, _, err := executeWithInput(t, []string{"show", "--session", session, loginPath}, s, "")
	require.NoError(t, err)
	require.Contains(t, shown, "Token: ********")
	require.NotContains(t, shown, "synthetic-token")
	fieldValue, _, err := executeWithInput(t, []string{"show", "--session", session, "--field", "API/Token", loginPath}, s, "")
	require.NoError(t, err)
	require.Equal(t, "  synthetic-token\n", fieldValue)
	revealed, _, err := executeWithInput(t, []string{"show", "--session", session, "--reveal", loginPath}, s, "")
	require.NoError(t, err)
	require.Contains(t, revealed, "Token:   synthetic-token\n")
	clipboardBase := &command{
		storage:     s,
		agentClient: func(string) (agent.PawAgent, error) { return client, nil },
	}
	var copied string
	clipboardBase.clipboard = func(_ context.Context, value []byte) error { copied = string(value); return nil }
	clipOut, clipErr, err := executeSubcommand(t, []string{"show", "--session", session, "--field", "API/Token", "--clip", loginPath}, newShowCommand, clipboardBase, "")
	require.NoError(t, err)
	require.Empty(t, clipOut)
	require.Contains(t, clipErr, "copied")
	require.NotContains(t, clipErr, "synthetic-token")
	require.Equal(t, "  synthetic-token\n", copied)
	_, _, err = executeWithInput(t, []string{"edit", "--session", session, "--tag", "development", loginPath}, s, "")
	require.NoError(t, err)
	_, _, err = executeWithInput(t, []string{"edit", "--session", session, "--tag", "development", vaultPath + "/note/memo"}, s, "")
	require.NoError(t, err)
	_, _, err = executeWithInput(t, []string{"edit", "--session", session, "--tag", "development", "--remove", loginPath}, s, "")
	require.NoError(t, err)
	loaded, err = store.LoadVault(vaultID, key)
	require.NoError(t, err)
	loginSummary, err := loaded.FindItem("web", model.LoginItemType)
	require.NoError(t, err)
	updatedLogin, err := store.LoadItem(loaded, loginSummary.ID)
	require.NoError(t, err)
	require.Empty(t, updatedLogin.Tags)
	custom, err := updatedLogin.CustomFields()
	require.NoError(t, err)
	require.Equal(t, "  synthetic-token\n", custom[0].Credential.Fields[0].Value)
	noteSummary, err := loaded.FindItem("memo", model.NoteItemType)
	require.NoError(t, err)
	updatedMemo, err := store.LoadItem(loaded, noteSummary.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"development"}, updatedMemo.Tags)
	filtered, _, err := executeWithInput(t, []string{"list", "--session", session, vaultPath + "/*", "--tag", "development"}, s, "")
	require.NoError(t, err)
	require.Contains(t, filtered, "memo")
	require.NotContains(t, filtered, "web")
	renamed, _, err := executeWithInput(t, []string{"edit", "--session", session, "--name", "renamed memo", vaultPath + "/note/memo"}, s, "")
	require.NoError(t, err)
	require.Contains(t, renamed, "renamed memo")
	_, _, err = executeWithInput(t, []string{"edit", "--session", session, "--name", "example", vaultPath + "/note/renamed memo"}, s, "")
	require.ErrorContains(t, err, "same name already exists")
	loaded, err = store.LoadVault(vaultID, key)
	require.NoError(t, err)
	_, err = loaded.FindItem("renamed memo", model.NoteItemType)
	require.NoError(t, err)
	out, diagnostics, err := executeWithInput(t, []string{"add", "note", "--session", session, "--tag", "automation", "--tag", "release", "--input", "-", vaultPath, "scripted note"}, s, "note from stdin\n")
	require.NoError(t, err)
	require.Empty(t, diagnostics)
	loaded, err = store.LoadVault(vaultID, key)
	require.NoError(t, err)
	scripted, err := loaded.FindItem("scripted note", model.NoteItemType)
	require.NoError(t, err)
	require.Equal(t, scripted.ID+"\n", out)

	t.Run("clipboard", func(t *testing.T) {
		calls := 0
		base.clipboard = func(ctx context.Context, value []byte) error {
			calls++
			require.Equal(t, auth.Password.Value, string(value))
			_, ok := ctx.Deadline()
			require.True(t, ok)
			return nil
		}
		args := []string{"show", "-c", "--session", session, vaultPath + "/password/secret"}
		out, errOut, err := executeSubcommand(t, args, newShowCommand, base, "")
		require.NoError(t, err)
		require.Equal(t, 1, calls)
		require.Empty(t, out)
		require.Contains(t, errOut, "copied")
		require.NotContains(t, out, auth.Password.Value)
		failure := errors.New("clipboard unavailable")
		base.clipboard = func(context.Context, []byte) error { return failure }
		_, _, err = executeSubcommand(t, args, newShowCommand, base, "")
		require.ErrorIs(t, err, failure)
		base.clipboard = func(ctx context.Context, _ []byte) error { return ctx.Err() }
		cmd := newShowCommand(base)
		base.out = io.Discard
		base.errOut = io.Discard
		cmd.Writer = io.Discard
		cmd.ErrWriter = io.Discard
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.ErrorIs(t, cmd.Run(ctx, args), context.Canceled)
	})

	t.Run("agent sessions and SSH keys", func(t *testing.T) {
		out, errOut, err := executeSubcommand(t, []string{"unlock", "-t", "45m", "personal"}, newUnlockCommand, base, "")
		require.NoError(t, err)
		unlockSession := strings.TrimSpace(out)
		require.NotEmpty(t, unlockSession)
		require.Equal(t, unlockSession+"\n", out)
		require.NotContains(t, out, "Session ID:")
		require.NotContains(t, out, "vault unlocked")
		require.NotContains(t, out, "added SSH keys")
		require.NotContains(t, errOut, "added:")
		require.NotContains(t, errOut, "ssh-")
		t.Setenv(paw.ENV_SESSION, session)
		base.agentClient = func(string) (agent.PawAgent, error) {
			return sshFailureAgent{PawAgent: client, err: errors.New("agent refused key")}, nil
		}
		warningOut, warningErr, err := executeSubcommand(t, []string{"unlock", "personal"}, newUnlockCommand, base, "")
		require.NoError(t, err)
		warningSession := strings.TrimSpace(warningOut)
		require.NotEmpty(t, warningSession)
		require.Equal(t, warningSession+"\n", warningOut)
		require.NotContains(t, warningOut, "agent refused key")
		require.Contains(t, warningErr, "could not add SSH key \"deploy\" to the agent: agent refused key")
		require.NotContains(t, warningErr, "ssh-")
		base.agentClient = func(string) (agent.PawAgent, error) { return client, nil }
		sessions, err := client.Sessions()
		require.NoError(t, err)
		require.Len(t, sessions, 3)
		require.Contains(t, []string{sessions[0].ID, sessions[1].ID, sessions[2].ID}, unlockSession)
		require.Contains(t, []string{sessions[0].ID, sessions[1].ID, sessions[2].ID}, warningSession)
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
		t.Setenv(paw.ENV_SESSION, "invalid-session")
		pipedInput := strings.NewReader("piped-password\n")
		originalPrompter := base.prompter
		base.prompter = newPrompter(pipedInput, io.Discard)
		_, _, err = executeSubcommand(t, []string{"unlock", "personal"}, newUnlockCommand, base, "")
		require.ErrorContains(t, err, "requires stdin to be a terminal")
		require.Equal(t, len("piped-password\n"), pipedInput.Len(), "unlock must not read a password from piped stdin")
		base.prompter = originalPrompter
		t.Setenv(paw.ENV_SESSION, session)
		out, _, err = executeSubcommand(t, []string{"agent", "sessions"}, newAgentCommand, base, "")
		require.NoError(t, err)
		require.Contains(t, out, session)
		startOut, startErr, err := executeSubcommand(t, []string{"agent", "start"}, newAgentCommand, base, "")
		require.ErrorContains(t, err, "already running")
		require.Empty(t, startOut)
		require.Empty(t, startErr)
		base.agentClient = func(string) (agent.PawAgent, error) { return nil, errors.New("not running") }
		started := false
		base.startAgent = func(path string) error { started = true; require.Equal(t, s.SocketAgentPath(), path); return nil }
		startOut, startErr, err = executeSubcommand(t, []string{"agent", "start"}, newAgentCommand, base, "")
		require.NoError(t, err)
		require.Empty(t, startOut)
		require.Empty(t, startErr)
		require.True(t, started)
		_, err = os.Lstat(s.SocketAgentPath())
		require.ErrorIs(t, err, os.ErrNotExist)
		base.agentClient = func(string) (agent.PawAgent, error) { return client, nil }
		out, _, err = executeSubcommand(t, []string{"lock", "personal"}, newLockCommand, base, "")
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
		out, diagnostics, err := executeSubcommand(t, []string{"show", "--session", session, "personal/note/example"}, newShowCommand, base, "")
		require.NoError(t, err)
		require.Contains(t, out, "example note")
		require.Contains(t, diagnostics, "Session is invalid or expired")
		require.NotContains(t, out+diagnostics, "synthetic-password")
	})
	session, err = client.Unlock(vaultID, key, 0)
	require.NoError(t, err)
	t.Setenv(paw.ENV_SESSION, session)
	t.Run("export and restore require a terminal for confirmations", func(t *testing.T) {
		plainPath := filepath.Join(root, "credentials.cxf")
		base.prompter = nil
		_, warning, err := executeSubcommand(t, []string{"export", "--plaintext", "--session", session, "personal", plainPath}, newExportCommand, base, "y\n")
		require.ErrorContains(t, err, "requires stdin to be a terminal")
		require.Empty(t, warning)
		_, err = os.Stat(plainPath)
		require.ErrorIs(t, err, os.ErrNotExist)
		base.prompter = newPrompter(strings.NewReader(""), io.Discard)
		base.prompter.readPassword = func() ([]byte, error) { return []byte("backup-secret"), nil }
		backupPath := filepath.Join(root, "backup.without-extension")
		_, _, err = executeSubcommand(t, []string{"export", "--session", session, "personal", backupPath}, newExportCommand, base, "")
		require.NoError(t, err)
		_, _, err = executeSubcommand(t, []string{"restore", "--name", "Restored", backupPath}, newRestoreCommand, base, "")
		require.ErrorContains(t, err, "requires stdin to be a terminal")
		infos, err := store.Vaults()
		require.NoError(t, err)
		require.Len(t, infos, 1)
		out, err := execute(t, []string{"ls"}, store)
		require.NoError(t, err)
		require.Contains(t, out, "personal")
	})
}

// failingAddWriter records writes and returns a configured error.
type failingAddWriter struct {
	err    error
	writes int
}

// Write records a write attempt and returns the configured error.
func (writer *failingAddWriter) Write([]byte) (int, error) {
	writer.writes++
	return 0, writer.err
}
