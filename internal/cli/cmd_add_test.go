// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/huh/v2"
	"github.com/stretchr/testify/require"

	"go.lucor.dev/paw/internal/agent"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/sshkey"
	"go.lucor.dev/paw/internal/storage"
)

func TestAddSubcommandValidation(t *testing.T) {
	for _, args := range [][]string{
		{"login", "--password-stdin", "--totp-stdin", "vault", "name"},
		{"password", "--generate-password", "--password-stdin", "vault", "name"},
		{"login", "--totp-stdin", "vault", "name"},
		{"login", "--url=http://example.com", "vault", "name"},
		{"login", "--password=value", "vault", "name"},
		{"login", "--totp-secret=value", "vault", "name"},
		{"note", "--input", "file", "vault", "name"},
		{"ssh-key", "--input", "-", "vault", "name"},
		{"ssh-key", "--password-stdin", "vault", "name"},
		{"note", "vault", "name", "extra"},
		{"note", "vault", ""},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			out, stderr, err := executeSubcommand(t, append([]string{"add"}, args...), newAddCommand, &command{}, "")
			require.Empty(t, out)
			require.Empty(t, stderr)
			var exit interface{ ExitCode() int }
			require.ErrorAs(t, err, &exit)
			require.Equal(t, ExitCodeInvalidArguments, exit.ExitCode())
		})
	}
}

func TestAddScriptedItems(t *testing.T) {
	base, vault := addTestCommand(t)
	key, err := sshkey.GenerateKey()
	require.NoError(t, err)
	keyFile := filepath.Join(t.TempDir(), "key")
	require.NoError(t, os.WriteFile(keyFile, key.MarshalPrivateKey(), 0o600))
	for _, tc := range []struct {
		name  string
		args  []string
		input string
		kind  model.ItemType
	}{
		{
			name:  "website",
			args:  []string{"login", "--url", "example.com", "--username", "alice", "--generate-password", "--totp-stdin"},
			input: "JBSWY3DPEHPK3PXP\n",
			kind:  model.LoginItemType,
		},
		{
			name:  "raw-totp",
			args:  []string{"login", "--generate-password", "--totp-stdin"},
			input: "jbsw y3dp ehpk 3pxp\r\n",
			kind:  model.LoginItemType,
		},
		{
			name:  "existing",
			args:  []string{"password", "--password-stdin"},
			input: " keep the spaces \r\n",
			kind:  model.PasswordItemType,
		},
		{
			name:  "memo",
			args:  []string{"note", "--input", "-"},
			input: "first line\nsecond line\n",
			kind:  model.NoteItemType,
		},
		{
			name: "imported",
			args: []string{"ssh-key", "--input", keyFile, "--add-to-agent"},
			kind: model.SSHKeyItemType,
		},
		{
			name: "generated",
			args: []string{"ssh-key"},
			kind: model.SSHKeyItemType,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"add", "--session", "active"}, tc.args...)
			args = append(args, "Personal", tc.name, "--tag", "example", "--tag", "second", "--tag", "example")
			out, stderr, err := executeSubcommand(t, args, newAddCommand, base, tc.input)
			require.NoError(t, err)
			loaded, err := base.storage.LoadVault(vault.Accounts[0].ID, vault.Key())
			require.NoError(t, err)
			summary, err := loaded.FindItem(tc.name, tc.kind)
			require.NoError(t, err)
			require.Equal(t, summary.ID+"\n", out)
			item, err := base.storage.LoadItem(loaded, summary.ID)
			require.NoError(t, err)
			require.Equal(t, []string{"example", "second"}, item.Tags)
			switch tc.kind {
			case model.LoginItemType:
				auth, _, err := item.BasicAuth()
				require.NoError(t, err)
				require.Len(t, auth.Password.Value, paw.RandomPasswordDefaultLength)
				require.Equal(t, paw.RandomPasswordDefaultFormat, model.PasswordSettingsFromField(auth.Password).Format)
				credentials, err := model.DecodeCredentials(item.Credentials)
				require.NoError(t, err)
				require.Equal(t, "JBSWY3DPEHPK3PXP", credentials.TOTP.Secret)
				if tc.name == "website" {
					require.Equal(t, "alice", auth.Username.Value)
					require.Equal(t, []string{"https://example.com"}, item.Scope.URLs)
					require.Empty(t, credentials.TOTP.Issuer)
					require.Empty(t, credentials.TOTP.Username)
					require.Equal(t, strings.ToLower(string(paw.TOTPHashDefault)), credentials.TOTP.Algorithm)
					require.EqualValues(t, paw.TOTPDigitsDefault, credentials.TOTP.Digits)
					require.EqualValues(t, paw.TOTPIntervalDefault, credentials.TOTP.Period)
				}
				require.Empty(t, stderr)
			case model.PasswordItemType:
				auth, _, err := item.BasicAuth()
				require.NoError(t, err)
				require.Equal(t, " keep the spaces ", auth.Password.Value)
				require.Empty(t, stderr)
			case model.NoteItemType:
				note, _, err := item.Note()
				require.NoError(t, err)
				require.Equal(t, tc.input, note.Content.Value)
			case model.SSHKeyItemType:
				require.Equal(t, tc.name == "imported", item.Metadata.AddToAgent)
				if tc.name == "imported" {
					require.Equal(t, "The key fingerprint is:\n"+key.Fingerprint()+"\n", stderr)
				}
			}
		})
	}
}

func TestAddInteractiveSelection(t *testing.T) {
	base, vault := addTestCommand(t)
	// Select the first vault and the Note type, then give the item a name.
	base.prompter = testInteractivePrompter(t, "1\ntest vault password\n3\nMemo\nn\n", "  note body  \nsecond line\n")
	out, _, err := executeSubcommand(t, []string{"add", "--session", "active"}, newAddCommand, base, "")
	require.NoError(t, err)
	loaded, err := base.storage.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	summary, err := loaded.FindItem("Memo", model.NoteItemType)
	require.NoError(t, err)
	require.Equal(t, summary.ID+"\n", out)
	item, err := base.storage.LoadItem(loaded, summary.ID)
	require.NoError(t, err)
	note, _, err := item.Note()
	require.NoError(t, err)
	require.Equal(t, "  note body  \nsecond line\n", note.Content.Value)
}

func TestAddWizardCancellationDoesNotStoreItem(t *testing.T) {
	base, vault := addTestCommand(t)
	base.prompter = testInteractivePrompter(t, "")
	base.prompter.runForm = func(*huh.Form) error { return errCanceled }
	_, _, err := executeSubcommand(t, []string{"add", "--session", "active"}, newAddCommand, base, "")
	require.ErrorIs(t, err, errCanceled)
	loaded, err := base.storage.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	require.Empty(t, loaded.Accounts[0].Items)
}

func TestAddStdinNeverFallsBackToPrompts(t *testing.T) {
	base, _ := addTestCommand(t)
	for _, session := range []string{"", "expired", "active"} {
		t.Run(session, func(t *testing.T) {
			reader := strings.NewReader("secret\n")
			base.prompter = testInteractivePrompter(t, "")
			base.prompter.runForm = nil
			base.prompter.readPassword = func() ([]byte, error) {
				t.Fatal("unexpected password prompt")
				return nil, nil
			}
			var stdout bytes.Buffer
			base.out = &stdout
			base.errOut = io.Discard
			cmd := newAddCommand(base)
			cmd.Reader = reader
			cmd.ExitErrHandler = leaveExitToCaller
			err := cmd.Run(t.Context(), []string{"add", "login", "--session=" + session, "--password-stdin", "Personal", "name"})
			if session == "active" {
				require.NoError(t, err)
				require.NotEmpty(t, stdout.String())
				require.Zero(t, reader.Len())
			} else {
				require.ErrorContains(t, err, "session")
				require.Empty(t, stdout.String())
				require.Equal(t, len("secret\n"), reader.Len())
			}
		})
	}
}

// addTestCommand creates a vault with an in-process agent session.
func addTestCommand(t *testing.T) (*command, *model.Vault) {
	t.Helper()
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	vault, err := store.CreateVault("Personal", "test vault password")
	require.NoError(t, err)
	base := &command{
		storage:  store,
		now:      time.Now,
		readFile: os.ReadFile,
		agentClient: func(string) (agent.PawAgent, error) {
			return addTestAgent{key: vault.Key()}, nil
		},
		generateSSHKey: func() (*generatedSSHKey, error) {
			key, err := sshkey.GenerateKey()
			if err != nil {
				return nil, err
			}
			return &generatedSSHKey{PrivateKey: string(key.MarshalPrivateKey())}, nil
		},
	}
	return base, vault
}

type addTestAgent struct {
	agent.PawAgent
	key *paw.Key
}

// Key returns the test vault key for the active session.
func (a addTestAgent) Key(_ string, session string) (*paw.Key, error) {
	if session != "active" {
		return nil, errors.New("session expired")
	}
	return a.key, nil
}
