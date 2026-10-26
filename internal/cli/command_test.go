// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"golang.org/x/crypto/ssh"

	"lucor.dev/paw/internal/agent"
	"lucor.dev/paw/internal/model"
	"lucor.dev/paw/internal/paw"
	"lucor.dev/paw/internal/sshkey"
	"lucor.dev/paw/internal/storage"
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
	storage *storage.StorageMock
	agent   *fakeAgent
	vault   *model.Vault
	state   *paw.AppState
	stored  []*model.Item
	deleted []string
	stamp   time.Time
}

func newCommandFixture(t *testing.T) *commandFixture {
	t.Helper()
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	f := &commandFixture{
		storage: &storage.StorageMock{},
		agent:   &fakeAgent{key: key},
		vault:   func() *model.Vault { v, _ := model.NewVault(key, "personal"); return v }(),
		state:   &paw.AppState{},
		stamp:   time.Date(2026, 10, 22, 12, 0, 0, 0, time.FixedZone("test", 3600)),
	}
	s := f.storage
	s.OnLoadVault = func(name string, key *paw.Key) (*model.Vault, error) {
		require.Equal(t, "personal", name)
		require.Equal(t, f.vault.Key(), key)
		return f.vault, nil
	}
	s.OnLoadAppState = func() (*paw.AppState, error) { return f.state, nil }
	s.OnStoreAppState = func(*paw.AppState) error { return nil }
	s.OnStoreItem = func(v *model.Vault, item *model.Item) error { f.stored = append(f.stored, item); return nil }
	s.OnDeleteItem = func(v *model.Vault, id string) error { f.deleted = append(f.deleted, id); return nil }
	s.OnVaults = func() ([]storage.VaultInfo, error) {
		return []storage.VaultInfo{{ID: "personal", Name: "personal"}}, nil
	}
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

// loadItem adds an item summary and loader callback to the command fixture.
func (f *commandFixture) loadItem(t *testing.T, item *model.Item) {
	t.Helper()
	summary, err := item.Summary()
	require.NoError(t, err)
	f.vault.Accounts[0].Items = append(f.vault.Accounts[0].Items, *summary)
	f.storage.OnLoadItem = func(v *model.Vault, id string) (*model.Item, error) {
		require.Equal(t, item.ID, id)
		return item, nil
	}
}

// fixtureBasicAuth reads a BasicAuth credential from a test item.
func fixtureBasicAuth(t *testing.T, item *model.Item) cxf.BasicAuth {
	t.Helper()
	for _, raw := range item.Credentials {
		if typ, err := cxf.CredentialTypeOf(raw); err == nil && typ == cxf.CredentialTypeBasicAuth {
			var value cxf.BasicAuth
			require.NoError(t, json.Unmarshal(raw, &value))
			return value
		}
	}
	t.Fatal("basic-auth credential missing")
	return cxf.BasicAuth{}
}

// fixtureNote reads a Note credential from a test item.
func fixtureNote(t *testing.T, item *model.Item) cxf.Note {
	t.Helper()
	for _, raw := range item.Credentials {
		if typ, err := cxf.CredentialTypeOf(raw); err == nil && typ == cxf.CredentialTypeNote {
			var value cxf.Note
			require.NoError(t, json.Unmarshal(raw, &value))
			return value
		}
	}
	t.Fatal("note credential missing")
	return cxf.Note{}
}

// marshalTestSSHKey encodes a test key using the CXF PKCS #8 format.
func marshalTestSSHKey(privateKey crypto.PrivateKey) ([]byte, error) {
	if key, ok := privateKey.(*ed25519.PrivateKey); ok {
		privateKey = *key
	}
	return x509.MarshalPKCS8PrivateKey(privateKey)
}

func TestAddLoginSharesPromptInputWithPasswordGenerator(t *testing.T) {
	f := newCommandFixture(t)
	out, prompts, err := executeWithInput(t,
		[]string{"add", "--session=test-session", "personal/login/account"}, newAddCommand(f.base),
		"https://example.com\nalice\n3\n4\nexample note\n")
	require.NoError(t, err)
	require.Len(t, f.stored, 1)
	item := f.stored[0]
	auth := fixtureBasicAuth(t, item)
	require.Equal(t, "alice", auth.Username.Value)
	require.Equal(t, "https://example.com", item.Scope.URLs[0])
	require.Equal(t, "example note", fixtureNote(t, item).Content.Value)
	require.Len(t, auth.Password.Value, 4)
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
	item := f.stored[0]
	require.Equal(t, paw.PinPassword, passwordSettings(fixtureBasicAuth(t, item).Password).Mode)
	require.Len(t, fixtureBasicAuth(t, item).Password.Value, paw.PinPasswordDefaultLength)
	require.Contains(t, out, "added")
}

func TestEditAndRemoveWithCommand(t *testing.T) {
	f := newCommandFixture(t)
	item, err := model.NewNote("example", "original")
	require.NoError(t, err)
	f.loadItem(t, item)
	_, _, err = executeWithInput(t, []string{"edit", "--session=test", "personal/note/example"}, newEditCommand(f.base), "updated\n")
	require.NoError(t, err)
	require.Len(t, f.stored, 1)
	require.Equal(t, "updated", fixtureNote(t, f.stored[0]).Content.Value)
	_, _, err = executeWithInput(t, []string{"rm", "--session=test", "personal/note/example"}, newRemoveCommand(f.base), "yes\ny\n")
	require.NoError(t, err)
	require.Len(t, f.deleted, 1)
	require.Equal(t, item.ID, f.deleted[0])
}

func TestEditLoginPasswordAndSSHKey(t *testing.T) {
	t.Run("login", func(t *testing.T) {
		f := newCommandFixture(t)
		item, err := model.NewLogin("account", "alice", "old-password", model.PasswordSettings{Mode: paw.CustomPassword})
		require.NoError(t, err)
		item.Scope = &cxf.CredentialScope{URLs: []string{"https://example.com"}, AndroidApps: []cxf.AndroidAppID{}}
		f.loadItem(t, item)
		_, _, err = executeWithInput(t,
			[]string{"edit", "--session=test", "personal/login/account"}, newEditCommand(f.base),
			"https://new.example.com\nalice-new\n3\n\nupdated note\n")
		require.NoError(t, err)
		updated := f.stored[0]
		require.Equal(t, "https://new.example.com", updated.Scope.URLs[0])
		auth := fixtureBasicAuth(t, updated)
		require.Equal(t, "alice-new", auth.Username.Value)
		require.Equal(t, paw.PinPassword, passwordSettings(auth.Password).Mode)
		require.Equal(t, "updated note", fixtureNote(t, updated).Content.Value)
		require.Len(t, f.stored, 1)
	})

	t.Run("password", func(t *testing.T) {
		f := newCommandFixture(t)
		item, err := model.NewPassword("account", "old-value", model.PasswordSettings{Mode: paw.CustomPassword})
		require.NoError(t, err)
		f.loadItem(t, item)
		_, _, err = executeWithInput(t,
			[]string{"edit", "--session=test", "personal/password/account"}, newEditCommand(f.base),
			"3\n\n")
		require.NoError(t, err)
		require.Equal(t, paw.PinPassword, passwordSettings(fixtureBasicAuth(t, f.stored[0]).Password).Mode)
		require.Len(t, fixtureBasicAuth(t, f.stored[0]).Password.Value, paw.PinPasswordDefaultLength)
		require.Len(t, f.stored, 1)
	})

	t.Run("SSH key", func(t *testing.T) {
		f := newCommandFixture(t)
		generated, err := sshkey.GenerateKey()
		require.NoError(t, err)
		der, err := marshalTestSSHKey(generated.PrivateKey())
		require.NoError(t, err)
		key := cxf.SSHKey{Type: cxf.CredentialTypeSSHKey, KeyType: generated.PublicKey().Type(), PrivateKey: base64.RawURLEncoding.EncodeToString(der)}
		item, err := model.NewSSHKey("deploy", key, "", model.PasswordSettings{Mode: paw.CustomPassword})
		require.NoError(t, err)
		f.loadItem(t, item)
		_, _, err = executeWithInput(t,
			[]string{"edit", "--session=test", "personal/ssh_key/deploy"}, newEditCommand(f.base),
			"y\nupdated note\n")
		require.NoError(t, err)
		require.True(t, f.stored[0].Metadata.AddToAgent)
		require.Len(t, f.stored, 1)
	})
}

func TestIncompleteInputDoesNotWriteData(t *testing.T) {
	for _, name := range []string{"add", "edit", "rm"} {
		t.Run(name, func(t *testing.T) {
			f := newCommandFixture(t)
			if name != "add" {
				item, err := model.NewNote("example", "")
				require.NoError(t, err)
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
	created := false
	f.storage.OnCreateVault = func(name, password string) (*model.Vault, error) {
		require.Equal(t, "personal", name)
		require.Equal(t, "vault-secret", password)
		created = true
		return f.vault, nil
	}
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
	generatedKey, err := sshkey.GenerateKey()
	require.NoError(t, err)
	f.base.generateSSHKey = func() (*paw.SSHKey, error) {
		generated = true
		item := paw.NewSSHKey()
		item.PrivateKey, item.PublicKey, item.Fingerprint = string(generatedKey.MarshalPrivateKey()), string(generatedKey.MarshalPublicKey()), generatedKey.Fingerprint()
		return item, nil
	}
	_, _, err = executeWithInput(t, []string{"add", "--session=test", "personal/ssh_key/key"}, newAddCommand(f.base), "n\n")
	require.NoError(t, err)
	require.True(t, generated)
	require.Len(t, f.stored, 1)
	credential, err := sshCredential(f.stored[0])
	require.NoError(t, err)
	_, _, fingerprint, err := sshKeyFromCXF(credential)
	require.NoError(t, err)
	require.Equal(t, generatedKey.Fingerprint(), fingerprint)
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
	item, err := model.NewPassword("example", "clipboard-secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
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
	require.Equal(t, f.vault.Accounts[0].ID, f.agent.vault)
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

// TestUnlockPrintsMigrationStatus checks CLI announces legacy vault migration.
func TestUnlockPrintsMigrationStatus(t *testing.T) {
	f := newCommandFixture(t)
	f.storage.OnVaults = func() ([]storage.VaultInfo, error) {
		return []storage.VaultInfo{{ID: "personal", Name: "personal", NeedsMigration: true}}, nil
	}
	out, _, err := executeWithInput(t, []string{"unlock", "personal"}, newUnlockCommand(f.base), "")
	require.NoError(t, err)
	require.Contains(t, out, "Migrating vault...")
}

func TestListUsesCommandWriter(t *testing.T) {
	f := newCommandFixture(t)
	out, _, err := executeWithInput(t, []string{"ls"}, newListCommand(f.base), "")
	require.NoError(t, err)
	require.Equal(t, "paw\n└── personal\n", out)
}

func TestListVaultShowsItemsByType(t *testing.T) {
	f := newCommandFixture(t)
	login, err := model.NewLogin("web", "", "", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	note, err := model.NewNote("memo", "")
	require.NoError(t, err)
	password, err := model.NewPassword("secret", "", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	generated, err := sshkey.GenerateKey()
	require.NoError(t, err)
	der, err := marshalTestSSHKey(generated.PrivateKey())
	require.NoError(t, err)
	sshKey, err := model.NewSSHKey("deploy", cxf.SSHKey{Type: cxf.CredentialTypeSSHKey, KeyType: generated.PublicKey().Type(), PrivateKey: base64.RawURLEncoding.EncodeToString(der)}, "", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	for _, item := range []*model.Item{login, note, password, sshKey} {
		summary, err := item.Summary()
		require.NoError(t, err)
		f.vault.Accounts[0].Items = append(f.vault.Accounts[0].Items, *summary)
	}
	out, _, err := executeWithInput(t,
		[]string{"ls", "--session=test-session", "personal"}, newListCommand(f.base), "")
	require.NoError(t, err)
	for _, want := range []string{"login", "web", "note", "memo", "password", "secret", "ssh_key", "deploy"} {
		require.Contains(t, out, want)
	}
}

// TestListReturnsSafeMigrationReport checks CLI returns a credential-safe migration report.
func TestListReturnsSafeMigrationReport(t *testing.T) {
	f := newCommandFixture(t)
	const secret = "SYNTHETIC_MIGRATION_SECRET"
	report := &storage.MigrationReport{
		TotalItems: 1,
		Failures: []storage.MigrationItemFailure{{
			LegacyID: "legacy-item-id",
			Title:    "Unsupported password",
			ItemType: paw.PasswordItemType,
			Phases:   []storage.MigrationPhase{storage.MigrationPhaseConvert},
			Err:      errors.New("unsupported mode 4"),
		}},
	}
	f.storage.OnLoadVault = func(name string, key *paw.Key) (*model.Vault, error) {
		return nil, report
	}

	_, _, err := executeWithInput(t,
		[]string{"ls", "--session=test-session", "personal"}, newListCommand(f.base), "")
	var actual *storage.MigrationReport
	require.ErrorAs(t, err, &actual)
	require.Same(t, report, actual)
	require.Contains(t, err.Error(), "migration failed: 1/1 items")
	require.Contains(t, err.Error(), "Unsupported password")
	require.Contains(t, err.Error(), "unsupported mode 4")
	require.NotContains(t, err.Error(), secret)
}

func TestUnlockAndLockManageSSHKeysInAgent(t *testing.T) {
	f := newCommandFixture(t)
	generated, err := sshkey.GenerateKey()
	require.NoError(t, err)
	der, err := marshalTestSSHKey(generated.PrivateKey())
	require.NoError(t, err)
	item, err := model.NewSSHKey("deploy", cxf.SSHKey{Type: cxf.CredentialTypeSSHKey, KeyType: generated.PublicKey().Type(), KeyComment: "deploy key", PrivateKey: base64.RawURLEncoding.EncodeToString(der)}, "", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	item.Metadata.AddToAgent = true
	summary, err := item.Summary()
	require.NoError(t, err)
	f.vault.Accounts[0].Items = append(f.vault.Accounts[0].Items, *summary)
	f.storage.OnLoadItem = func(v *model.Vault, id string) (*model.Item, error) {
		require.Equal(t, item.ID, id)
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
	base.vaultID = "personal"
	unlock := &UnlockCmd{command: base, vaultName: "personal"}
	require.NoError(t, unlock.addSSHKeysToAgent(f.agent, f.vault))
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
	item, err := model.NewNote("example", "note content")
	require.NoError(t, err)
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
