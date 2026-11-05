// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/sshkey"
)

func TestAddLoginSharesPromptInputWithPasswordGenerator(t *testing.T) {
	var prompts strings.Builder
	base := &command{prompter: newPrompter(strings.NewReader("https://example.com\nalice\n3\n4\nexample note\n"), &prompts)}
	item, err := (&AddCmd{command: base, itemPath: itemPath{itemName: "account"}}).newLogin()
	require.NoError(t, err)
	auth, _, err := item.BasicAuth()
	require.NoError(t, err)
	note, _, err := item.Note()
	require.NoError(t, err)
	require.Equal(t, "alice", auth.Username.Value)
	require.Equal(t, "https://example.com", item.Scope.URLs[0])
	require.Equal(t, "example note", note.Content.Value)
	require.Len(t, auth.Password.Value, 4)
	require.Contains(t, prompts.String(), "Username:")
	require.Contains(t, prompts.String(), "Pin length")
}

func TestEditItemFields(t *testing.T) {
	t.Run("login", func(t *testing.T) {
		item, err := model.NewLogin("account", "alice", "old-password", model.PasswordSettings{Mode: paw.CustomPassword})
		require.NoError(t, err)
		item.Scope = &cxf.CredentialScope{URLs: []string{"https://example.com"}, AndroidApps: []cxf.AndroidAppID{}}
		cmd := &EditCmd{command: &command{prompter: newPrompter(strings.NewReader("https://new.example.com\nalice-new\n3\n\nupdated note\n"), io.Discard)}}
		require.NoError(t, cmd.editLogin(item))
		auth, _, err := item.BasicAuth()
		require.NoError(t, err)
		note, _, err := item.Note()
		require.NoError(t, err)
		require.Equal(t, "https://new.example.com", item.Scope.URLs[0])
		require.Equal(t, "alice-new", auth.Username.Value)
		require.Equal(t, paw.PinPassword, model.PasswordSettingsFromField(auth.Password).Mode)
		require.Equal(t, "updated note", note.Content.Value)
	})
	t.Run("password", func(t *testing.T) {
		item, err := model.NewPassword("account", "old-value", model.PasswordSettings{Mode: paw.CustomPassword})
		require.NoError(t, err)
		cmd := &EditCmd{command: &command{prompter: newPrompter(strings.NewReader("3\n\n"), io.Discard)}}
		require.NoError(t, cmd.editPassword(item))
		auth, _, err := item.BasicAuth()
		require.NoError(t, err)
		require.Equal(t, paw.PinPassword, model.PasswordSettingsFromField(auth.Password).Mode)
		require.Len(t, auth.Password.Value, paw.PinPasswordDefaultLength)
	})
	t.Run("SSH key", func(t *testing.T) {
		item := &model.Item{}
		cmd := &EditCmd{command: &command{prompter: newPrompter(strings.NewReader("y\n"), io.Discard)}}
		require.NoError(t, cmd.editSSHKey(item))
		require.True(t, item.Metadata.AddToAgent)
	})
}

func TestPasswordGeneratorUsesCommandStreams(t *testing.T) {
	out, prompts, err := executeWithInput(t, []string{"pwgen"}, newPwGenCommand(&command{}), "2\n4\n")
	require.NoError(t, err)
	require.Len(t, strings.TrimSpace(out), 4)
	require.Contains(t, prompts, "Password type")
	require.NotContains(t, out, "Password type")
}

func TestPasswordGeneratorModes(t *testing.T) {
	modes := []paw.PasswordMode{paw.CustomPassword, paw.RandomPassword, paw.PassphrasePassword, paw.PinPassword}
	for _, tc := range []struct {
		name, input string
		mode        paw.PasswordMode
		length      int
	}{
		{"random", "1\n8\ny\n", paw.RandomPassword, 8},
		{"pin", "3\n4\n", paw.PinPassword, 4},
		{"passphrase", "2\n3\n", paw.PassphrasePassword, 3},
		{"custom", "0\n", paw.CustomPassword, 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newPrompter(strings.NewReader(tc.input), io.Discard)
			p.readPassword = func() ([]byte, error) { return []byte("custom-secret"), nil }
			password, err := (&PwGenCmd{command: &command{prompter: p}}).Pwgen(modes, paw.RandomPassword)
			require.NoError(t, err)
			require.Equal(t, tc.mode, password.Mode)
			switch tc.mode {
			case paw.PassphrasePassword:
				require.Len(t, strings.Split(password.Value, "-"), tc.length)
			case paw.PinPassword:
				require.Len(t, password.Value, tc.length)
				require.Regexp(t, `^\d{4}$`, password.Value)
			case paw.CustomPassword:
				require.Equal(t, "custom-secret", password.Value)
			default:
				require.Len(t, password.Value, tc.length)
				require.NotZero(t, password.Format&paw.SymbolsFormat)
			}
		})
	}
}

func TestSSHKeyOperationsAreInjected(t *testing.T) {
	generated, err := sshkey.GenerateKey()
	require.NoError(t, err)
	base := &command{prompter: newPrompter(strings.NewReader("n\n"), io.Discard), out: io.Discard}
	base.generateSSHKey = func() (*generatedSSHKey, error) {
		return &generatedSSHKey{PrivateKey: string(generated.MarshalPrivateKey()), PublicKey: string(generated.MarshalPublicKey()), Fingerprint: generated.Fingerprint()}, nil
	}
	cmd := &AddCmd{command: base, itemPath: itemPath{itemName: "key"}}
	item, err := cmd.newSSHKey()
	require.NoError(t, err)
	credential, err := item.SSHKey()
	require.NoError(t, err)
	_, _, fingerprint, err := model.FormatSSHKey(credential)
	require.NoError(t, err)
	require.Equal(t, generated.Fingerprint(), fingerprint)
	failure := errors.New("file read failed")
	base.readFile = func(name string) ([]byte, error) {
		require.Equal(t, "virtual-key", name)
		return nil, failure
	}
	cmd.importPath = "virtual-key"
	item, err = cmd.newSSHKey()
	require.ErrorIs(t, err, failure)
	require.Nil(t, item)
}

func TestSSHKeyGenerationFailuresAreReturned(t *testing.T) {
	failure := errors.New("key generation failed")
	base := &command{generateSSHKey: func() (*generatedSSHKey, error) { return nil, failure }}
	item, err := (&AddCmd{command: base}).newSSHKey()
	require.ErrorIs(t, err, failure)
	require.Nil(t, item)
}
