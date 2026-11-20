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
	p := testInteractivePrompter(t, "https://example.com\nalice\n4\n4\nn\n")
	p.output = &prompts
	base := &command{prompter: p}
	item, err := (&AddCmd{command: base, itemPath: itemPath{itemName: "account"}}).newLogin()
	require.NoError(t, err)
	auth, _, err := item.BasicAuth()
	require.NoError(t, err)
	note, _, err := item.Note()
	require.NoError(t, err)
	require.Equal(t, "alice", auth.Username.Value)
	require.Equal(t, "https://example.com", item.Scope.URLs[0])
	require.Empty(t, note.Content.Value)
	require.Len(t, auth.Password.Value, 4)
	require.Contains(t, prompts.String(), "Username")
	require.Contains(t, prompts.String(), "Pin length")
}

func TestNormalizeLoginURLMatchesGUIValidation(t *testing.T) {
	for _, testCase := range []struct {
		value string
		want  string
		err   string
	}{
		{value: "", want: ""},
		{value: "example.com", want: "https://example.com"},
		{value: "https://example.com/login", want: "https://example.com/login"},
		{value: "http://example.com", err: "website must use HTTPS"},
		{value: "%", err: "enter a valid website address"},
	} {
		got, err := normalizeLoginURL(testCase.value)
		if testCase.err != "" {
			require.EqualError(t, err, testCase.err)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, testCase.want, got)
	}
}

func TestEditItemPromptsRequireTTY(t *testing.T) {
	t.Run("login", func(t *testing.T) {
		item, err := model.NewLogin("account", "alice", "old-password", model.PasswordSettings{Mode: paw.CustomPassword})
		require.NoError(t, err)
		item.Scope = &cxf.CredentialScope{URLs: []string{"https://example.com"}, AndroidApps: []cxf.AndroidAppID{}}
		cmd := &EditCmd{command: &command{prompter: newPrompter(strings.NewReader("https://new.example.com\nalice-new\n"), io.Discard)}}
		require.ErrorContains(t, cmd.editLogin(item), "requires stdin to be a terminal")
		require.Equal(t, []string{"https://example.com"}, item.Scope.URLs)
	})
	t.Run("preserves additional URLs", func(t *testing.T) {
		item, err := model.NewLogin("account", "alice", "password", model.PasswordSettings{Mode: paw.CustomPassword})
		require.NoError(t, err)
		item.Scope = &cxf.CredentialScope{URLs: []string{"https://primary.example", "https://secondary.example"}, AndroidApps: []cxf.AndroidAppID{}}
		cmd := &EditCmd{command: &command{prompter: newPrompter(strings.NewReader("https://updated.example\nalice\n3\n\n\nn\n"), io.Discard)}}
		require.NoError(t, cmd.editLogin(item))
		require.Equal(t, []string{"https://updated.example", "https://secondary.example"}, item.Scope.URLs)
	})
	t.Run("password", func(t *testing.T) {
		item, err := model.NewPassword("account", "old-value", model.PasswordSettings{Mode: paw.CustomPassword})
		require.NoError(t, err)
		cmd := &EditCmd{command: &command{prompter: newPrompter(strings.NewReader("3\n\n"), io.Discard)}}
		require.ErrorContains(t, cmd.editPassword(item), "requires stdin to be a terminal")
		auth, _, err := item.BasicAuth()
		require.NoError(t, err)
		require.Equal(t, "old-value", auth.Password.Value)
	})
	t.Run("SSH key", func(t *testing.T) {
		item := &model.Item{}
		cmd := &EditCmd{command: &command{prompter: newPrompter(strings.NewReader("y\nn\n"), io.Discard)}}
		require.ErrorContains(t, cmd.editSSHKey(item), "requires stdin to be a terminal")
		require.False(t, item.Metadata.AddToAgent)
	})
}

func TestInteractiveLoginEditCanKeepPassword(t *testing.T) {
	item, err := model.NewLogin("account", "alice", "saved password", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	item.Scope = &cxf.CredentialScope{URLs: []string{"https://example.com"}}
	p := testInteractivePrompter(t, "\n\nn\nn\nn\n")
	cmd := &EditCmd{command: &command{prompter: p}}
	require.NoError(t, cmd.editLogin(item))
	credential, _, err := item.BasicAuth()
	require.NoError(t, err)
	require.Equal(t, "saved password", credential.Password.Value)
}

func TestInteractivePasswordEditCanKeepCurrentSecret(t *testing.T) {
	item, err := model.NewPassword("account", "saved password", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	p := testInteractivePrompter(t, "n\n")
	cmd := &EditCmd{command: &command{prompter: p}}
	require.NoError(t, cmd.editPassword(item))
	credential, _, err := item.BasicAuth()
	require.NoError(t, err)
	require.Equal(t, "saved password", credential.Password.Value)
}

func TestEditTOTPSecretPreservesSettingsAndCanRemoveCredential(t *testing.T) {
	item, err := model.NewLogin("account", "alice", "password", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	original := cxf.TOTP{
		Type:      cxf.CredentialTypeTOTP,
		Secret:    "JBSWY3DPEHPK3PXP",
		Algorithm: "sha256",
		Period:    45,
		Digits:    8,
		Issuer:    "Example",
	}
	require.NoError(t, item.SetTOTP(-1, original))
	p := testInteractivePrompter(t, "y\nn\n")
	p.readPassword = func() ([]byte, error) { return []byte("jbsw-y3dp-ehpk3pxp"), nil }
	cmd := &EditCmd{command: &command{prompter: p}}
	require.NoError(t, cmd.editTOTP(item))
	decoded, err := model.DecodeCredentials(item.Credentials)
	require.NoError(t, err)
	require.Equal(t, "JBSWY3DPEHPK3PXP", decoded.TOTP.Secret)
	require.Equal(t, "sha256", decoded.TOTP.Algorithm)
	require.Equal(t, uint16(45), decoded.TOTP.Period)
	require.Equal(t, uint16(8), decoded.TOTP.Digits)
	require.Equal(t, "Example", decoded.TOTP.Issuer)

	p = testInteractivePrompter(t, "y\n")
	p.readPassword = func() ([]byte, error) { return []byte(":clear"), nil }
	cmd.prompter = p
	require.NoError(t, cmd.editTOTP(item))
	decoded, err = model.DecodeCredentials(item.Credentials)
	require.NoError(t, err)
	require.Nil(t, decoded.TOTP)

	p = testInteractivePrompter(t, "y\nn\n")
	p.readPassword = func() ([]byte, error) { return []byte("jbswy3dpehpk3pxp"), nil }
	cmd.prompter = p
	require.NoError(t, cmd.editTOTP(item))
	decoded, err = model.DecodeCredentials(item.Credentials)
	require.NoError(t, err)
	require.Equal(t, "JBSWY3DPEHPK3PXP", decoded.TOTP.Secret)
	require.Equal(t, "sha1", decoded.TOTP.Algorithm)
	require.Equal(t, uint16(paw.TOTPIntervalDefault), decoded.TOTP.Period)
	require.Equal(t, uint16(paw.TOTPDigitsDefault), decoded.TOTP.Digits)
}

func TestAddTOTPUsesDefaultsAndRejectsDuplicateCredentials(t *testing.T) {
	item, err := model.NewLogin("account", "alice", "password", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	p := testInteractivePrompter(t, "y\n4\nn\n5\n")
	p.readPassword = func() ([]byte, error) { return []byte("jbsw-y3dp-ehpk3pxp"), nil }
	cmd := &AddCmd{
		command:  &command{prompter: p},
		itemPath: itemPath{itemType: model.LoginItemType},
	}
	require.NoError(t, cmd.addOptionalData(item, &model.Vault{}))
	credentials, err := model.DecodeCredentials(item.Credentials)
	require.NoError(t, err)
	require.Equal(t, "JBSWY3DPEHPK3PXP", credentials.TOTP.Secret)
	require.Equal(t, "sha1", credentials.TOTP.Algorithm)
	require.Equal(t, uint16(paw.TOTPIntervalDefault), credentials.TOTP.Period)
	require.Equal(t, uint16(paw.TOTPDigitsDefault), credentials.TOTP.Digits)
	require.ErrorContains(t, cmd.addTOTP(item), "already has an authenticator")
}

func TestEditTOTPChangesSettingsWithoutReplacingSecret(t *testing.T) {
	item, err := model.NewLogin("account", "alice", "password", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	original := cxf.TOTP{
		Type:      cxf.CredentialTypeTOTP,
		Secret:    "JBSWY3DPEHPK3PXP",
		Algorithm: "sha1",
		Period:    30,
		Digits:    6,
	}
	require.NoError(t, item.SetTOTP(-1, original))
	p := testInteractivePrompter(t, "n\ny\nSHA512\n9\n47\n")
	cmd := &EditCmd{command: &command{prompter: p}}
	require.NoError(t, cmd.editTOTP(item))
	credentials, err := model.DecodeCredentials(item.Credentials)
	require.NoError(t, err)
	require.Equal(t, original.Secret, credentials.TOTP.Secret)
	require.Equal(t, "sha512", credentials.TOTP.Algorithm)
	require.Equal(t, uint16(9), credentials.TOTP.Digits)
	require.Equal(t, uint16(47), credentials.TOTP.Period)
}

func TestEditCanImportReplacementSSHKey(t *testing.T) {
	oldKey, err := sshkey.GenerateKey()
	require.NoError(t, err)
	oldCredential, err := model.EncodeSSHKey(oldKey.PrivateKey(), oldKey.PublicKey().Type(), "old")
	require.NoError(t, err)
	item, err := model.NewSSHKey("Deployment", oldCredential, "old passphrase", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	newKey, err := sshkey.GenerateKey()
	require.NoError(t, err)
	base := &command{
		prompter: newPrompter(strings.NewReader(""), io.Discard),
		readFile: func(string) ([]byte, error) { return newKey.MarshalPrivateKey(), nil },
		errOut:   io.Discard,
	}
	cmd := &EditCmd{command: base, importPath: "replacement.key"}
	require.NoError(t, cmd.replaceSSHKey(item))
	credential, err := item.SSHKey()
	require.NoError(t, err)
	_, _, fingerprint, err := model.FormatSSHKey(credential)
	require.NoError(t, err)
	require.Equal(t, newKey.Fingerprint(), fingerprint)
	auth, _, err := item.BasicAuth()
	require.NoError(t, err)
	require.Empty(t, auth.Password.Value)
	generatedKey, err := sshkey.GenerateKey()
	require.NoError(t, err)
	base.generateSSHKey = func() (*generatedSSHKey, error) {
		return &generatedSSHKey{
			PrivateKey:  string(generatedKey.MarshalPrivateKey()),
			PublicKey:   string(generatedKey.MarshalPublicKey()),
			Fingerprint: generatedKey.Fingerprint(),
		}, nil
	}
	cmd.importPath = ""
	cmd.generateKey = true
	require.NoError(t, cmd.replaceSSHKey(item))
	credential, err = item.SSHKey()
	require.NoError(t, err)
	_, _, fingerprint, err = model.FormatSSHKey(credential)
	require.NoError(t, err)
	require.Equal(t, generatedKey.Fingerprint(), fingerprint)
}

func TestEditSSHKeyCanChangePassphraseWithoutReplacingKey(t *testing.T) {
	key, err := sshkey.GenerateKey()
	require.NoError(t, err)
	credential, err := model.EncodeSSHKey(key.PrivateKey(), key.PublicKey().Type(), "key")
	require.NoError(t, err)
	item, err := model.NewSSHKey("Deployment", credential, "old passphrase", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	p := testInteractivePrompter(t, "n\ny\n")
	p.readPassword = func() ([]byte, error) { return []byte("new passphrase"), nil }
	cmd := &EditCmd{command: &command{prompter: p}}
	require.NoError(t, cmd.editSSHKey(item))
	auth, _, err := item.BasicAuth()
	require.NoError(t, err)
	require.Equal(t, "new passphrase", auth.Password.Value)
	updated, err := item.SSHKey()
	require.NoError(t, err)
	_, _, fingerprint, err := model.FormatSSHKey(updated)
	require.NoError(t, err)
	require.Equal(t, key.Fingerprint(), fingerprint)
}

func TestInteractiveEditCanFinishWithoutOpeningStandardFields(t *testing.T) {
	item, err := model.NewLogin("GitHub", "developer", "secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	var output strings.Builder
	p := testInteractivePrompter(t, "2\n8\n4\n")
	p.output = &output
	cmd := &EditCmd{command: &command{prompter: p, errOut: &output}}
	require.NoError(t, cmd.editInteractive(item, &model.Vault{}))
	require.Contains(t, output.String(), "Add field")
	require.NotContains(t, output.String(), "Password:")
}

func TestInteractiveEditRenamesNote(t *testing.T) {
	item, err := model.NewNote("Old title", "old note")
	require.NoError(t, err)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	summary, err := item.Summary()
	require.NoError(t, err)
	vault.Accounts[0].Items = append(vault.Accounts[0].Items, *summary)
	input := "1\nNew title\ny\n4\n"
	var output strings.Builder
	p := testInteractivePrompter(t, input)
	p.output = &output
	p.editor = &testTextEditor{value: "updated note\n"}
	cmd := &EditCmd{
		command:  &command{prompter: p, errOut: &output},
		itemPath: itemPath{itemType: model.NoteItemType},
	}
	require.NoError(t, cmd.editInteractive(item, vault))
	require.Equal(t, "New title", item.Title)
	note, _, err := item.Note()
	require.NoError(t, err)
	require.Equal(t, "updated note", note.Content.Value)
}

func TestInteractiveEditReturnsStandardFieldErrors(t *testing.T) {
	item, err := model.NewNote("Title", "note")
	require.NoError(t, err)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	summary, err := item.Summary()
	require.NoError(t, err)
	vault.Accounts[0].Items = append(vault.Accounts[0].Items, *summary)
	p := newPrompter(strings.NewReader("1\nRenamed\ny\n"), io.Discard)
	cmd := &EditCmd{
		command:  &command{prompter: p, errOut: io.Discard},
		itemPath: itemPath{itemType: model.NoteItemType},
	}
	require.ErrorContains(t, cmd.editInteractive(item, vault), "requires stdin to be a terminal")
}

func TestInteractiveNoteEditSupportsMultilineAndClear(t *testing.T) {
	item, err := model.NewNote("Title", "original note")
	require.NoError(t, err)
	p := testInteractivePrompter(t, "y\n")
	p.editor = &testTextEditor{value: "first line\nsecond line\n"}
	cmd := &EditCmd{command: &command{prompter: p}}
	require.NoError(t, cmd.editNote(item))
	note, _, err := item.Note()
	require.NoError(t, err)
	require.Equal(t, "first line\nsecond line", note.Content.Value)

	p = testInteractivePrompter(t, "y\n")
	p.editor = &testTextEditor{value: ""}
	cmd.prompter = p
	require.NoError(t, cmd.editNote(item))
	note, _, err = item.Note()
	require.NoError(t, err)
	require.Empty(t, note.Content.Value)
}

func TestPasswordGeneratorRequiresTTYForPrompts(t *testing.T) {
	out, prompts, err := executeSubcommand(t, []string{"pwgen"}, newPwGenCommand, &command{}, "2\n4\n")
	require.ErrorContains(t, err, "requires stdin to be a terminal")
	require.Empty(t, out)
	require.Empty(t, prompts)
}

func TestPasswordGeneratorModes(t *testing.T) {
	modes := []paw.PasswordMode{paw.CustomPassword, paw.RandomPassword, paw.PassphrasePassword, paw.PinPassword}
	for _, tc := range []struct {
		name, input string
		mode        paw.PasswordMode
		length      int
	}{
		{"random", "2\n8\ny\n", paw.RandomPassword, 8},
		{"pin", "4\n4\n", paw.PinPassword, 4},
		{"passphrase", "3\n3\n", paw.PassphrasePassword, 3},
		{"custom", "1\n", paw.CustomPassword, 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testInteractivePrompter(t, tc.input)
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
	base := &command{prompter: testInteractivePrompter(t, "n\n"), out: io.Discard}
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
