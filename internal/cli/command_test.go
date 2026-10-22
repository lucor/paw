// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"crypto"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"lucor.dev/paw/internal/agent"
	"lucor.dev/paw/internal/paw"
	"lucor.dev/paw/internal/sshkey"
)

type fakeAgent struct {
	agent.PawAgent
	key                    *paw.Key
	keyErr                 error
	vault, session, locked string
	lifetime               time.Duration
	sessions               []agent.Session
	addedKeys              []crypto.PrivateKey
	addedComments          []string
	removedKeys            []ssh.PublicKey
}

func (a *fakeAgent) Key(vault, session string) (*paw.Key, error) {
	a.vault, a.session = vault, session
	return a.key, a.keyErr
}
func (a *fakeAgent) Type() (agent.Type, error)          { return agent.CLI, nil }
func (a *fakeAgent) Lock(vault string) error            { a.locked = vault; return nil }
func (a *fakeAgent) Sessions() ([]agent.Session, error) { return a.sessions, nil }
func (a *fakeAgent) Unlock(vault string, key *paw.Key, lifetime time.Duration) (string, error) {
	a.vault, a.key, a.lifetime = vault, key, lifetime
	return "new-session", nil
}
func (a *fakeAgent) AddSSHKey(key crypto.PrivateKey, comment string) error {
	a.addedKeys = append(a.addedKeys, key)
	a.addedComments = append(a.addedComments, comment)
	return nil
}
func (a *fakeAgent) RemoveSSHKey(key ssh.PublicKey) error {
	a.removedKeys = append(a.removedKeys, key)
	return nil
}

type commandFixture struct {
	base    *command
	storage *paw.StorageMock
	agent   *fakeAgent
	vault   *paw.Vault
	state   *paw.AppState
	stored  []paw.Item
	deleted []paw.Item
	stamp   time.Time
}

func newCommandFixture(t *testing.T) *commandFixture {
	t.Helper()
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	f := &commandFixture{
		storage: &paw.StorageMock{},
		agent:   &fakeAgent{key: key},
		vault:   paw.NewVault(key, "personal"),
		state:   &paw.AppState{},
		stamp:   time.Date(2026, 10, 22, 12, 0, 0, 0, time.FixedZone("test", 3600)),
	}
	s := f.storage
	s.OnLoadVault = func(name string, key *paw.Key) (*paw.Vault, error) {
		require.Equal(t, "personal", name)
		require.Equal(t, f.vault.Key(), key)
		return f.vault, nil
	}
	s.OnLoadAppState = func() (*paw.AppState, error) { return f.state, nil }
	s.OnStoreAppState = func(*paw.AppState) error { return nil }
	s.OnStoreVault = func(*paw.Vault) error { return nil }
	s.OnStoreItem = func(v *paw.Vault, item paw.Item) error { f.stored = append(f.stored, item); return nil }
	s.OnDeleteItem = func(v *paw.Vault, item paw.Item) error { f.deleted = append(f.deleted, item); return nil }
	s.OnVaults = func() ([]string, error) { return []string{"personal"}, nil }
	f.base = &command{storage: s}
	f.base.agentClient = func(path string) (agent.PawAgent, error) {
		require.Equal(t, s.SocketAgentPath(), path)
		return f.agent, nil
	}
	f.base.getenv = func(name string) string {
		require.Equal(t, paw.ENV_SESSION, name)
		return "environment-session"
	}
	f.base.now = func() time.Time { return f.stamp }
	f.base.oneTimeKey = func() (*paw.Key, error) { return key, nil }
	return f
}

func (f *commandFixture) loadItem(t *testing.T, item paw.Item) {
	t.Helper()
	require.NoError(t, f.vault.AddItem(item))
	f.storage.OnLoadItem = func(v *paw.Vault, metadata *paw.Metadata) (paw.Item, error) {
		require.Equal(t, item.GetMetadata().Name, metadata.Name)
		require.Equal(t, item.GetMetadata().Type, metadata.Type)
		return item, nil
	}
}

func TestAddLoginSharesPromptInputWithPasswordGenerator(t *testing.T) {
	f := newCommandFixture(t)
	out, prompts, err := executeWithInput(t,
		[]string{"add", "--session=test-session", "personal/login/account"}, newAddCommand(f.base),
		"https://example.com\nalice\n3\n4\nexample note\n")
	require.NoError(t, err)
	require.Len(t, f.stored, 1)
	item := f.stored[0].(*paw.Login)
	require.Equal(t, "alice", item.Username)
	require.Equal(t, "https://example.com", item.URL.String())
	require.Equal(t, "example note", item.Note.Value)
	require.Equal(t, paw.PinPassword, item.Password.Mode)
	require.Len(t, item.Password.Value, 4)
	require.Equal(t, f.stamp.UTC(), f.vault.Modified)
	require.Equal(t, f.stamp.UTC(), f.state.Modified)
	require.Equal(t, "test-session", f.agent.session)
	require.Contains(t, out, "added")
	require.NotContains(t, out, "Username:")
	require.Contains(t, prompts, "Username:")
	require.Contains(t, prompts, "Pin length")
}

func TestAddPasswordUsesGeneratorAndStoresNote(t *testing.T) {
	f := newCommandFixture(t)
	out, _, err := executeWithInput(t,
		[]string{"add", "--session=test-session", "personal/password/account"}, newAddCommand(f.base),
		"3\n\nnote for password\n")
	require.NoError(t, err)
	require.Len(t, f.stored, 1)
	item := f.stored[0].(*paw.Password)
	require.Equal(t, paw.PinPassword, item.Mode)
	require.Len(t, item.Value, paw.PinPasswordDefaultLength)
	require.Equal(t, "note for password", item.Note.Value)
	require.Contains(t, out, "added")
}

func TestEditAndRemoveWithCommand(t *testing.T) {
	f := newCommandFixture(t)
	item := paw.NewNote()
	item.Name, item.Value = "example", "original"
	f.loadItem(t, item)
	_, _, err := executeWithInput(t, []string{"edit", "--session=test", "personal/note/example"}, newEditCommand(f.base), "updated\n")
	require.NoError(t, err)
	require.Len(t, f.stored, 1)
	require.Equal(t, "updated", item.Value)
	require.Equal(t, f.stamp.UTC(), item.Modified)
	_, _, err = executeWithInput(t, []string{"rm", "--session=test", "personal/note/example"}, newRemoveCommand(f.base), "yes\ny\n")
	require.NoError(t, err)
	require.Len(t, f.deleted, 1)
	require.False(t, f.vault.HasItem(item))
}

func TestEditLoginPasswordAndSSHKey(t *testing.T) {
	t.Run("login", func(t *testing.T) {
		f := newCommandFixture(t)
		item := paw.NewLogin()
		item.Name = "account"
		item.Username = "alice"
		require.NoError(t, item.URL.Set("https://example.com"))
		f.loadItem(t, item)
		_, _, err := executeWithInput(t,
			[]string{"edit", "--session=test", "personal/login/account"}, newEditCommand(f.base),
			"https://new.example.com\nalice-new\n3\n\nupdated note\n")
		require.NoError(t, err)
		require.Equal(t, "https://new.example.com", item.URL.String())
		require.Equal(t, "alice-new", item.Username)
		require.Equal(t, paw.PinPassword, item.Password.Mode)
		require.Equal(t, "updated note", item.Note.Value)
		require.Len(t, f.stored, 1)
	})

	t.Run("password", func(t *testing.T) {
		f := newCommandFixture(t)
		item := paw.NewPassword()
		item.Name, item.Value = "account", "old-value"
		f.loadItem(t, item)
		_, _, err := executeWithInput(t,
			[]string{"edit", "--session=test", "personal/password/account"}, newEditCommand(f.base),
			"3\n\nupdated note\n")
		require.NoError(t, err)
		require.Equal(t, paw.PinPassword, item.Mode)
		require.Len(t, item.Value, paw.PinPasswordDefaultLength)
		require.Equal(t, "updated note", item.Note.Value)
		require.Len(t, f.stored, 1)
	})

	t.Run("SSH key", func(t *testing.T) {
		f := newCommandFixture(t)
		item := paw.NewSSHKey()
		item.Name, item.AddToAgent, item.Note.Value = "deploy", false, "old note"
		f.loadItem(t, item)
		_, _, err := executeWithInput(t,
			[]string{"edit", "--session=test", "personal/ssh_key/deploy"}, newEditCommand(f.base),
			"y\nupdated note\n")
		require.NoError(t, err)
		require.True(t, item.AddToAgent)
		require.Equal(t, "updated note", item.Note.Value)
		require.Len(t, f.stored, 1)
	})
}

func TestIncompleteInputDoesNotWriteData(t *testing.T) {
	for _, name := range []string{"add", "edit", "rm"} {
		t.Run(name, func(t *testing.T) {
			f := newCommandFixture(t)
			if name != "add" {
				item := paw.NewNote()
				item.Name = "example"
				f.loadItem(t, item)
			}
			cmd := newAddCommand(f.base)
			switch name {
			case "edit":
				cmd = newEditCommand(f.base)
			case "rm":
				cmd = newRemoveCommand(f.base)
			}
			_, _, err := executeWithInput(t, []string{name, "--session=test", "personal/note/example"}, cmd, "")
			require.ErrorIs(t, err, io.EOF)
			require.Empty(t, f.stored)
			require.Empty(t, f.deleted)
		})
	}
}

func TestInitWithInjectedPassword(t *testing.T) {
	f := newCommandFixture(t)
	p := newPrompter(strings.NewReader(""), io.Discard)
	p.readPassword = func() ([]byte, error) { return []byte("vault-secret"), nil }
	f.base.prompter = p
	f.storage.OnCreateVaultKey = func(name, password string) (*paw.Key, error) {
		require.Equal(t, "personal", name)
		require.Equal(t, "vault-secret", password)
		return f.vault.Key(), nil
	}
	created := false
	f.storage.OnCreateVault = func(name string, key *paw.Key) (*paw.Vault, error) { created = true; return f.vault, nil }
	out, _, err := executeWithInput(t, []string{"init", "personal"}, newInitCommand(f.base), "")
	require.NoError(t, err)
	require.True(t, created)
	require.Contains(t, out, "created")
	require.NotContains(t, out, "vault-secret")
}

func TestPasswordGeneratorUsesCommandStreams(t *testing.T) {
	f := newCommandFixture(t)
	out, prompts, err := executeWithInput(t, []string{"pwgen"}, newPwGenCommand(f.base), "2\n4\n")
	require.NoError(t, err)
	require.Len(t, strings.TrimSpace(out), 4)
	require.Contains(t, prompts, "Password type")
	require.NotContains(t, out, "Password type")
}

func TestPasswordGeneratorSupportsRandomPassphraseAndCustomModes(t *testing.T) {
	f := newCommandFixture(t)
	key := f.vault.Key()
	modes := []paw.PasswordMode{paw.CustomPassword, paw.RandomPassword, paw.PassphrasePassword, paw.PinPassword}

	t.Run("random", func(t *testing.T) {
		f.base.prompter = newPrompter(strings.NewReader("1\n8\ny\n"), io.Discard)
		password, err := (&PwGenCmd{command: f.base}).Pwgen(key, modes, paw.RandomPassword)
		require.NoError(t, err)
		require.Equal(t, paw.RandomPassword, password.Mode)
		require.Len(t, password.Value, 8)
		require.NotZero(t, password.Format&paw.SymbolsFormat)
	})

	t.Run("passphrase", func(t *testing.T) {
		f.base.prompter = newPrompter(strings.NewReader("2\n3\n"), io.Discard)
		password, err := (&PwGenCmd{command: f.base}).Pwgen(key, modes, paw.RandomPassword)
		require.NoError(t, err)
		require.Equal(t, paw.PassphrasePassword, password.Mode)
		require.Len(t, strings.Split(password.Value, "-"), 3)
	})

	t.Run("custom", func(t *testing.T) {
		p := newPrompter(strings.NewReader("0\n"), io.Discard)
		secrets := [][]byte{[]byte("custom-secret"), []byte("custom-secret")}
		p.readPassword = func() ([]byte, error) {
			secret := secrets[0]
			secrets = secrets[1:]
			return secret, nil
		}
		f.base.prompter = p
		password, err := (&PwGenCmd{command: f.base}).Pwgen(key, modes, paw.RandomPassword)
		require.NoError(t, err)
		require.Equal(t, paw.CustomPassword, password.Mode)
		require.Equal(t, "custom-secret", password.Value)
	})
}

func TestSSHKeyOperationsAreInjected(t *testing.T) {
	f := newCommandFixture(t)
	generated := false
	f.base.generateSSHKey = func() (*paw.SSHKey, error) {
		generated = true
		item := paw.NewSSHKey()
		item.PrivateKey, item.PublicKey, item.Fingerprint = "test-private-key", "test-public-key", "test-fingerprint"
		return item, nil
	}
	_, _, err := executeWithInput(t, []string{"add", "--session=test", "personal/ssh_key/key"}, newAddCommand(f.base), "n\nnote\n")
	require.NoError(t, err)
	require.True(t, generated)
	require.Len(t, f.stored, 1)
	require.Equal(t, "test-fingerprint", f.stored[0].(*paw.SSHKey).Fingerprint)
	failure := errors.New("file read failed")
	f.base.readFile = func(name string) ([]byte, error) {
		require.Equal(t, "virtual-key", name)
		return nil, failure
	}
	_, _, err = executeWithInput(t, []string{"add", "-i", "virtual-key", "--session=test", "personal/ssh_key/import"}, newAddCommand(f.base), "")
	require.ErrorIs(t, err, failure)
	require.Len(t, f.stored, 1)
}

func TestClipboardCommandAndContext(t *testing.T) {
	f := newCommandFixture(t)
	item := paw.NewPassword()
	item.Name, item.Value = "example", "clipboard-secret"
	f.loadItem(t, item)
	calls := 0
	f.base.clipboard = func(ctx context.Context, value []byte) error {
		calls++
		require.Equal(t, "clipboard-secret", string(value))
		_, ok := ctx.Deadline()
		require.True(t, ok)
		return nil
	}
	out, _, err := executeWithInput(t, []string{"show", "-c", "--session=test", "personal/password/example"}, newShowCommand(f.base), "")
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.NotContains(t, out, "clipboard-secret")
	require.Contains(t, out, "copied")
	failure := errors.New("clipboard unavailable")
	f.base.clipboard = func(context.Context, []byte) error { return failure }
	_, _, err = executeWithInput(t, []string{"show", "-c", "--session=test", "personal/password/example"}, newShowCommand(f.base), "")
	require.ErrorIs(t, err, failure)
	f.base.clipboard = func(ctx context.Context, _ []byte) error { return ctx.Err() }
	cmd := newShowCommand(f.base)
	cmd.Writer, cmd.ErrWriter = io.Discard, io.Discard
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, cmd.Run(ctx, []string{"show", "-c", "--session=test", "personal/password/example"}), context.Canceled)
}

func TestAgentCommandsWithInjectedClient(t *testing.T) {
	f := newCommandFixture(t)
	f.agent.sessions = []agent.Session{{ID: "session-id", Vault: "personal", Lifetime: time.Minute}}
	out, _, err := executeWithInput(t, []string{"agent", "sessions"}, newAgentCommand(f.base), "")
	require.NoError(t, err)
	require.Contains(t, out, "session-id")
	_, _, err = executeWithInput(t, []string{"agent", "start"}, newAgentCommand(f.base), "")
	require.ErrorContains(t, err, "already running")
	f.base.agentClient = func(string) (agent.PawAgent, error) { return nil, errors.New("not running") }
	started := false
	f.base.startAgent = func(path string) error {
		started = true
		require.Equal(t, f.storage.SocketAgentPath(), path)
		return nil
	}
	_, _, err = executeWithInput(t, []string{"agent", "start"}, newAgentCommand(f.base), "")
	require.NoError(t, err)
	require.True(t, started)
}

func TestUnlockAndLockWithInjectedClient(t *testing.T) {
	f := newCommandFixture(t)
	out, _, err := executeWithInput(t, []string{"unlock", "-t", "45m", "personal"}, newUnlockCommand(f.base), "")
	require.NoError(t, err)
	require.Equal(t, "environment-session", f.agent.session)
	require.Equal(t, 45*time.Minute, f.agent.lifetime)
	require.Contains(t, out, "new-session")
	p := newPrompter(strings.NewReader(""), io.Discard)
	p.readPassword = func() ([]byte, error) { return []byte("vault-secret"), nil }
	f.base.prompter = p
	f.storage.OnLoadVaultKey = func(name, password string) (*paw.Key, error) {
		require.Equal(t, "vault-secret", password)
		return f.vault.Key(), nil
	}
	_, _, err = executeWithInput(t, []string{"lock", "personal"}, newLockCommand(f.base), "")
	require.NoError(t, err)
	require.Equal(t, "personal", f.agent.locked)
}

func TestListUsesCommandWriter(t *testing.T) {
	f := newCommandFixture(t)
	out, _, err := executeWithInput(t, []string{"ls"}, newListCommand(f.base), "")
	require.NoError(t, err)
	require.Equal(t, "paw\n└── personal\n", out)
}

func TestListVaultShowsItemsByType(t *testing.T) {
	f := newCommandFixture(t)
	login := paw.NewLogin()
	login.Name = "web"
	note := paw.NewNote()
	note.Name = "memo"
	password := paw.NewPassword()
	password.Name = "secret"
	sshKey := paw.NewSSHKey()
	sshKey.Name = "deploy"
	for _, item := range []paw.Item{login, note, password, sshKey} {
		require.NoError(t, f.vault.AddItem(item))
	}
	out, _, err := executeWithInput(t,
		[]string{"ls", "--session=test-session", "personal"}, newListCommand(f.base), "")
	require.NoError(t, err)
	for _, want := range []string{"login", "web", "note", "memo", "password", "secret", "ssh_key", "deploy"} {
		require.Contains(t, out, want)
	}
}

func TestUnlockAndLockManageSSHKeysInAgent(t *testing.T) {
	f := newCommandFixture(t)
	generated, err := sshkey.GenerateKey()
	require.NoError(t, err)
	item := paw.NewSSHKey()
	item.Name = "deploy"
	item.PrivateKey = string(generated.MarshalPrivateKey())
	item.PublicKey = string(generated.MarshalPublicKey())
	item.Comment = "deploy key"
	item.AddToAgent = true
	require.NoError(t, f.vault.AddItem(item))
	f.storage.OnLoadItem = func(v *paw.Vault, metadata *paw.Metadata) (paw.Item, error) {
		require.Equal(t, item.ID(), metadata.ID())
		return item, nil
	}

	var output strings.Builder
	passwordPrompt := newPrompter(strings.NewReader(""), io.Discard)
	passwordPrompt.readPassword = func() ([]byte, error) { return []byte("vault-secret"), nil }
	f.storage.OnLoadVaultKey = func(name, password string) (*paw.Key, error) {
		require.Equal(t, "personal", name)
		require.Equal(t, "vault-secret", password)
		return f.vault.Key(), nil
	}
	base := &command{storage: f.storage, prompter: passwordPrompt, out: &output}
	unlock := &UnlockCmd{command: base, vaultName: "personal"}
	require.NoError(t, unlock.addSSHKeysToAgent(f.agent, f.vault.Key()))
	require.Len(t, f.agent.addedKeys, 1)
	require.Equal(t, "deploy key", f.agent.addedComments[0])
	require.Contains(t, output.String(), "added:")

	lock := &LockCmd{command: base, vaultName: "personal"}
	require.NoError(t, lock.removeSSHKeysFromAgent(f.agent))
	require.Len(t, f.agent.removedKeys, 1)
	require.Contains(t, output.String(), "Removed key:")
}

func TestSessionFailureFallsBackToInjectedPassword(t *testing.T) {
	f := newCommandFixture(t)
	item := paw.NewNote()
	item.Name, item.Value = "example", "note content"
	f.loadItem(t, item)
	f.agent.keyErr = errors.New("session expired")
	p := newPrompter(strings.NewReader(""), io.Discard)
	p.readPassword = func() ([]byte, error) { return []byte("fallback-secret"), nil }
	f.base.prompter = p
	f.storage.OnLoadVaultKey = func(name, password string) (*paw.Key, error) {
		require.Equal(t, "personal", name)
		require.Equal(t, "fallback-secret", password)
		return f.vault.Key(), nil
	}
	out, diagnostics, err := executeWithInput(t, []string{"show", "--session=expired", "personal/note/example"}, newShowCommand(f.base), "")
	require.NoError(t, err)
	require.Contains(t, out, "note content")
	require.Contains(t, diagnostics, "Session is invalid or expired")
	require.NotContains(t, out+diagnostics, "fallback-secret")
}

func TestKeyGenerationFailuresAreReturned(t *testing.T) {
	f := newCommandFixture(t)
	failure := errors.New("key generation failed")
	f.base.oneTimeKey = func() (*paw.Key, error) { return nil, failure }
	_, _, err := executeWithInput(t, []string{"pwgen"}, newPwGenCommand(f.base), "")
	require.ErrorIs(t, err, failure)
	f.base.generateSSHKey = func() (*paw.SSHKey, error) { return nil, failure }
	_, _, err = executeWithInput(t, []string{"add", "--session=test", "personal/ssh_key/new"}, newAddCommand(f.base), "")
	require.ErrorIs(t, err, failure)
	require.Empty(t, f.stored)
}
