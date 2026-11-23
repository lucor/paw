// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/sshkey"
)

func TestShowMasksSecretsAndAllowsExplicitFieldOutput(t *testing.T) {
	item, err := model.NewLogin("GitHub", "developer", "login-secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	item.Scope = &cxf.CredentialScope{URLs: []string{"https://github.com"}}
	require.NoError(t, item.AppendCustomFields(cxf.CustomFields{Type: cxf.CredentialTypeCustomFields, Label: "API", Fields: []cxf.EditableField{
		{FieldType: cxf.FieldTypeConcealedString, Label: "Token", Value: "api-secret"},
		{FieldType: cxf.FieldTypeString, Label: "Description", Value: "first line\nsecond line"},
	}}))
	var output, stderr bytes.Buffer
	cmd := &ShowCmd{command: &command{out: &output, errOut: &stderr}, itemPath: itemPath{itemType: model.LoginItemType}}
	require.NoError(t, cmd.showItem(context.Background(), item))
	require.Contains(t, output.String(), "ID: "+item.ID+" ("+item.ID[:7]+")\nName: GitHub\nType: login\n")
	require.Contains(t, output.String(), "Password: ********")
	require.Contains(t, output.String(), "Token: ********")
	require.NotContains(t, output.String(), "login-secret")
	require.NotContains(t, output.String(), "api-secret")
	require.Contains(t, output.String(), "Description: first line\nsecond line")

	output.Reset()
	cmd.reveal = true
	require.NoError(t, cmd.showItem(context.Background(), item))
	require.Contains(t, output.String(), "Password: login-secret")
	require.Contains(t, output.String(), "Token: api-secret")

	output.Reset()
	cmd.reveal = false
	cmd.field = "API/Token"
	require.NoError(t, cmd.showItem(context.Background(), item))
	require.Equal(t, "api-secret", output.String())
	require.NotContains(t, output.String(), "ID:")
	require.Empty(t, stderr.String())
}

func TestShowFullItemIncludesIDForEachItemType(t *testing.T) {
	password, err := model.NewPassword("Wi-Fi", "synthetic-secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	note, err := model.NewNote("Secure note", "note content")
	require.NoError(t, err)
	key, err := sshkey.GenerateKey()
	require.NoError(t, err)
	credential, err := model.EncodeSSHKey(key.PrivateKey(), key.PublicKey().Type(), "key")
	require.NoError(t, err)
	sshItem, err := model.NewSSHKey("Deployment", credential, "", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)

	for _, testCase := range []struct {
		name     string
		item     *model.Item
		itemType model.ItemType
	}{
		{name: "password", item: password, itemType: model.PasswordItemType},
		{name: "note", item: note, itemType: model.NoteItemType},
		{name: "SSH key", item: sshItem, itemType: model.SSHKeyItemType},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var output bytes.Buffer
			cmd := &ShowCmd{
				command:  &command{out: &output},
				itemPath: itemPath{itemType: testCase.itemType},
			}
			require.NoError(t, cmd.showItem(context.Background(), testCase.item))
			shortID := testCase.item.ID[:7]
			wantHeader := "ID: " + testCase.item.ID + " (" + shortID + ")\nName: " + testCase.item.Title + "\nType: " + testCase.itemType.String() + "\n"
			require.True(t, strings.HasPrefix(output.String(), wantHeader))
			require.NotContains(t, output.String(), "Custom fields:")
			require.NotContains(t, output.String(), "Tags:")
			require.NotContains(t, output.String(), "Attachments:")
			require.NotContains(t, output.String(), "History:")
			switch testCase.itemType {
			case model.PasswordItemType:
				require.Contains(t, output.String(), "Password: ********\n")
			case model.NoteItemType:
				require.Contains(t, output.String(), "Note:\nnote content\n")
			case model.SSHKeyItemType:
				require.Contains(t, output.String(), "Private key: ********\n")
				require.Contains(t, output.String(), "Public key: ")
				require.Contains(t, output.String(), "\nFingerprint: ")
			}
		})
	}
}

func TestShowPrintsMultilineNoteAfterLabelOnNewLine(t *testing.T) {
	item, err := model.NewNote("Note", "kdsakl\ndsakldslakdsal\ndksaldksa;dksaldksa\ndsakldslakdsal")
	require.NoError(t, err)
	var output bytes.Buffer
	cmd := &ShowCmd{command: &command{out: &output}, itemPath: itemPath{itemType: model.NoteItemType}}

	require.NoError(t, cmd.showItem(context.Background(), item))
	require.Contains(t, output.String(), "Note:\nkdsakl\ndsakldslakdsal\ndksaldksa;dksaldksa\ndsakldslakdsal\n")
}

func TestShowUsesFixedSevenCharacterIDPrefixOnCollision(t *testing.T) {
	first, err := model.NewNote("First", "content")
	require.NoError(t, err)
	second, err := model.NewNote("Second", "content")
	require.NoError(t, err)
	first.ID = "abcdefgfirst"
	second.ID = "abcdefgsecond"

	for _, item := range []*model.Item{first, second} {
		var output bytes.Buffer
		cmd := &ShowCmd{command: &command{out: &output}, itemPath: itemPath{itemType: model.NoteItemType}}
		require.NoError(t, cmd.showItem(context.Background(), item))
		require.Contains(t, output.String(), "ID: "+item.ID+" (abcdefg)\n")
	}
}

func TestShowPrintsCreatedBeforeModified(t *testing.T) {
	item, err := model.NewNote("Note", "content")
	require.NoError(t, err)
	created := uint64(1795455520)
	modified := uint64(1795480900)
	item.CreationAt = &created
	item.ModifiedAt = &modified
	var output bytes.Buffer
	cmd := &ShowCmd{command: &command{out: &output}, itemPath: itemPath{itemType: model.NoteItemType}}

	require.NoError(t, cmd.showItem(context.Background(), item))
	result := output.String()
	require.Less(t, strings.Index(result, "Created:"), strings.Index(result, "Modified:"))
	require.Contains(t, result, "Created: "+time.Unix(int64(created), 0).Local().Format(humanTimeLayout)+"\n")
	require.Contains(t, result, "Modified: "+time.Unix(int64(modified), 0).Local().Format(humanTimeLayout)+"\n")
}

func TestShowClipboardWritesOnlyDiagnosticToStderr(t *testing.T) {
	item, err := model.NewPassword("Wi-Fi", "synthetic-secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	var output, stderr bytes.Buffer
	var copied string
	base := &command{out: &output, errOut: &stderr, clipboard: func(_ context.Context, value []byte) error { copied = string(value); return nil }}
	cmd := &ShowCmd{command: base, itemPath: itemPath{itemType: model.PasswordItemType}, clipboard: true}
	require.NoError(t, cmd.showItem(context.Background(), item))
	require.Equal(t, "synthetic-secret", copied)
	require.Empty(t, output.String())
	require.Equal(t, "Password copied to clipboard\n", stderr.String())
	require.NotContains(t, stderr.String(), "synthetic-secret")
	require.NotContains(t, stderr.String(), "[✓]")
}

func TestShowPropagatesOutputWriteErrors(t *testing.T) {
	item, err := model.NewPassword("Wi-Fi", "synthetic-secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	writeErr := errors.New("stdout unavailable")
	writer := &failingShowWriter{err: writeErr}
	cmd := &ShowCmd{
		command: &command{
			out: writer,
		},
		itemPath: itemPath{itemType: model.PasswordItemType},
	}
	require.ErrorIs(t, cmd.showItem(context.Background(), item), writeErr)
	require.Equal(t, 1, writer.writes)
}

type failingShowWriter struct {
	err    error
	writes int
}

func (writer *failingShowWriter) Write([]byte) (int, error) {
	writer.writes++
	return 0, writer.err
}

func TestShowTOTPSecretIsMaskedAndExplicitlySelectable(t *testing.T) {
	item, err := model.NewLogin("GitHub", "developer", "password", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	item.Credentials = append(item.Credentials, []byte(`{"type":"totp","secret":"JBSWY3DPEHPK3PXP","period":30,"digits":6,"algorithm":"sha1","issuer":"GitHub"}`))
	var output bytes.Buffer
	cmd := &ShowCmd{command: &command{out: &output}, itemPath: itemPath{itemType: model.LoginItemType}}
	require.NoError(t, cmd.showItem(context.Background(), item))
	require.Contains(t, output.String(), "\nTOTP: configured\n")
	require.NotContains(t, output.String(), "TOTP secret:")
	require.NotContains(t, output.String(), "JBSWY3DPEHPK3PXP")
	value, err := itemField(item, model.LoginItemType, "TOTP secret")
	require.NoError(t, err)
	require.Equal(t, "JBSWY3DPEHPK3PXP", value)
	cmd.reveal = true
	output.Reset()
	require.NoError(t, cmd.showItem(context.Background(), item))
	require.Contains(t, output.String(), "\nTOTP:\n  Secret: JBSWY3DPEHPK3PXP\n  Algorithm: SHA1\n  Digits: 6\n  Period: 30 seconds\n")
}

func TestShowJSONIncludesSecretsAndSelectedTOTP(t *testing.T) {
	item, err := model.NewLogin("Example", "alice", "password", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	item.Credentials = append(item.Credentials, []byte(`{"type":"totp","secret":"OBQXO","period":30,"digits":6,"algorithm":"sha1"}`))
	var output bytes.Buffer
	cmd := &ShowCmd{
		command:  &command{out: &output, now: func() time.Time { return time.Date(2024, 2, 9, 23, 3, 59, 0, time.UTC) }},
		itemPath: itemPath{itemType: model.LoginItemType},
		json:     true,
	}
	require.NoError(t, cmd.showItem(context.Background(), item))
	require.Contains(t, output.String(), `"password":"password"`)
	require.Contains(t, output.String(), `"secret":"OBQXO"`)
	require.NotContains(t, output.String(), "003475")
	output.Reset()
	cmd.field = "totp"
	require.NoError(t, cmd.showItem(context.Background(), item))
	require.Equal(t, "\"003475\"\n", output.String())
}

func TestShowJSONUsesCXFValuesAndOmitsUnknownExtensions(t *testing.T) {
	item, err := model.NewLogin("Example", "alice", "secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	item.Tags = []string{"personal"}
	require.NoError(t, item.AppendCustomFields(cxf.CustomFields{
		Type:  cxf.CredentialTypeCustomFields,
		ID:    stringPointer("Z3JvdXA"),
		Label: "API",
		Fields: []cxf.EditableField{{
			ID:        stringPointer("ZmllbGQ"),
			FieldType: cxf.FieldTypeConcealedString,
			Label:     "Token",
			Value:     "custom-secret",
		}},
	}))
	item.Credentials = append(item.Credentials,
		jsontext.Value(`{"type":"unknown","secret":"unknown-secret"}`),
		jsontext.Value(`{"type":"custom-fields","fields":[],"extensions":[{"name":"example.com/unknown","secret":"extension-secret"}]}`),
	)
	var output bytes.Buffer
	cmd := &ShowCmd{
		command:  &command{out: &output},
		itemPath: itemPath{itemType: model.LoginItemType},
		json:     true,
	}
	require.NoError(t, cmd.showItem(context.Background(), item))
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
	require.Equal(t, item.ID, decoded["id"])
	require.NotContains(t, decoded, "shortId")
	require.Equal(t, "secret", decoded["password"])
	require.Equal(t, []any{"personal"}, decoded["tags"])
	require.NotContains(t, output.String(), "unknown-secret")
	require.NotContains(t, output.String(), "extension-secret")
	require.Contains(t, output.String(), "custom-secret")
}

func TestShowRejectsIncompatibleSelectors(t *testing.T) {
	for _, testCase := range []struct {
		args []string
		want string
	}{
		{[]string{"show", "--username", "--password", "Personal/login/example"}, "select only one field"},
		{[]string{"show", "--totp", "--field", "username", "Personal/login/example"}, "select only one field"},
		{[]string{"show", "--reveal", "--password", "Personal/login/example"}, "--reveal cannot be combined with a field selector"},
		{[]string{"show", "--json", "--clip", "Personal/login/example"}, "--json cannot be combined with --clip"},
	} {
		_, _, err := executeSubcommand(t, testCase.args, newShowCommand, &command{}, "")
		require.ErrorContains(t, err, testCase.want)
	}
}

func TestShowSSHPrivateKeyAndPassphraseAreMasked(t *testing.T) {
	generated, err := sshkey.GenerateKey()
	require.NoError(t, err)
	credential, err := model.EncodeSSHKey(generated.PrivateKey(), generated.PublicKey().Type(), "synthetic-key")
	require.NoError(t, err)
	item, err := model.NewSSHKey("Deployment", credential, "synthetic-passphrase", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	var output bytes.Buffer
	cmd := &ShowCmd{command: &command{out: &output}, itemPath: itemPath{itemType: model.SSHKeyItemType}}
	require.NoError(t, cmd.showItem(context.Background(), item))
	require.Contains(t, output.String(), "Private key: ********")
	require.Contains(t, output.String(), "Passphrase: ********")
	require.NotContains(t, output.String(), "synthetic-passphrase")
	value, err := itemField(item, model.SSHKeyItemType, "private key")
	require.NoError(t, err)
	require.Contains(t, value, "BEGIN OPENSSH PRIVATE KEY")
	require.NotContains(t, value, "********")
}

func TestItemFieldRejectsDuplicateNames(t *testing.T) {
	item, err := model.NewLogin("GitHub", "developer", "secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	require.NoError(t, item.AppendCustomFields(cxf.CustomFields{Type: cxf.CredentialTypeCustomFields, Fields: []cxf.EditableField{{FieldType: cxf.FieldTypeString, Label: "Username", Value: "other"}}}))
	_, err = itemField(item, model.LoginItemType, "username")
	require.ErrorContains(t, err, "ambiguous")
}

func TestItemFieldTreatsMultipleURLsAsAmbiguous(t *testing.T) {
	item, err := model.NewLogin("GitHub", "developer", "secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	item.Scope = &cxf.CredentialScope{URLs: []string{"https://github.com", "https://git.example"}}
	_, err = itemField(item, model.LoginItemType, "url")
	require.ErrorContains(t, err, "ambiguous")
}

func TestItemFieldReportsMissingOptionalStandardField(t *testing.T) {
	item, err := model.NewLogin("GitHub", "", "secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	auth, index, err := item.BasicAuth()
	require.NoError(t, err)
	auth.Username = nil
	require.NoError(t, item.SetBasicAuth(index, auth))
	_, err = itemField(item, model.LoginItemType, "username")
	require.ErrorContains(t, err, "not found")
}

func TestSafeTerminalTextEscapesControlsAndKeepsMultilineContent(t *testing.T) {
	require.Equal(t, "line one\nline two\\u001b[31m", safeTerminalText("line one\nline two\x1b[31m"))
}

func TestEditCustomFieldPreservesWhitespaceTypeAndSectionOnRemoval(t *testing.T) {
	item, err := model.NewLogin("GitHub", "developer", "secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	require.NoError(t, item.AppendCustomFields(cxf.CustomFields{Type: cxf.CredentialTypeCustomFields, Label: "API", Fields: []cxf.EditableField{{FieldType: cxf.FieldTypeConcealedString, Label: "Token", Value: "old"}, {FieldType: cxf.FieldTypeString, Label: "Note", Value: "keep"}}}))
	cmd := &EditCmd{command: &command{}, field: "API/Token", input: strings.NewReader("  new token\n")}
	require.NoError(t, cmd.editCustomAndTags(item))
	groups, err := item.CustomFields()
	require.NoError(t, err)
	require.Equal(t, cxf.FieldTypeConcealedString, groups[0].Credential.Fields[0].FieldType)
	require.Equal(t, "  new token\n", groups[0].Credential.Fields[0].Value)

	cmd = &EditCmd{command: &command{}, field: "API/Token", remove: true}
	require.NoError(t, cmd.editCustomAndTags(item))
	groups, err = item.CustomFields()
	require.NoError(t, err)
	require.Equal(t, "API", groups[0].Credential.Label)
	require.Equal(t, []cxf.EditableField{{FieldType: cxf.FieldTypeString, Label: "Note", Value: "keep"}}, groups[0].Credential.Fields)
}

func TestChooseCustomFieldSelectsDuplicateLabelByPosition(t *testing.T) {
	item, err := model.NewNote("Note", "content")
	require.NoError(t, err)
	require.NoError(t, item.AppendCustomFields(cxf.CustomFields{Type: cxf.CredentialTypeCustomFields, Fields: []cxf.EditableField{
		{FieldType: cxf.FieldTypeString, Label: "Duplicate", Value: "first"},
		{FieldType: cxf.FieldTypeString, Label: "Duplicate", Value: "second"},
	}}))
	groups, err := item.CustomFields()
	require.NoError(t, err)
	cmd := &EditCmd{command: &command{prompter: testInteractivePrompter(t, "2\n")}}

	groupIndex, fieldIndex, ok, err := cmd.chooseCustomField(groups)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 0, groupIndex)
	require.Equal(t, 1, fieldIndex)
}

func TestManageSectionsSelectsDuplicateLabelByPosition(t *testing.T) {
	item, err := model.NewNote("Note", "content")
	require.NoError(t, err)
	for range 2 {
		require.NoError(t, item.AppendCustomFields(cxf.CustomFields{Type: cxf.CredentialTypeCustomFields, Label: "Duplicate", Fields: []cxf.EditableField{}}))
	}
	prompts := testInteractivePrompter(t, "2\n2\nRenamed\n")
	cmd := &EditCmd{command: &command{prompter: prompts}}

	require.NoError(t, cmd.manageSections(item))
	groups, err := item.CustomFields()
	require.NoError(t, err)
	require.Equal(t, "Duplicate", groups[0].Credential.Label)
	require.Equal(t, "Renamed", groups[1].Credential.Label)
}

func TestEditCustomFieldTypeAndTagAssociation(t *testing.T) {
	item, err := model.NewLogin("GitHub", "developer", "secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	cmd := &EditCmd{command: &command{}, field: "API/Description", typeName: "hidden-multiline", input: strings.NewReader("secret\nvalue")}
	require.NoError(t, cmd.editCustomAndTags(item))
	groups, err := item.CustomFields()
	require.NoError(t, err)
	require.Equal(t, cxf.FieldTypeConcealedString, groups[0].Credential.Fields[0].FieldType)
	cmd = &EditCmd{command: &command{}, field: "API/Refresh", input: strings.NewReader("")}
	require.NoError(t, cmd.editCustomAndTags(item))
	groups, err = item.CustomFields()
	require.NoError(t, err)
	require.Len(t, groups, 1, "new fields in an existing section keep its group")
	require.Len(t, groups[0].Credential.Fields, 2)
	require.NoError(t, cmd.editCustomAndTags(item))

	cmd = &EditCmd{command: &command{}, tag: "development"}
	require.NoError(t, cmd.editCustomAndTags(item))
	require.NoError(t, cmd.editCustomAndTags(item))
	require.Equal(t, []string{"development"}, item.Tags)
	cmd.remove = true
	require.NoError(t, cmd.editCustomAndTags(item))
	require.Empty(t, item.Tags)
}

func TestEditCustomFieldKeepsDuplicateSectionsAmbiguous(t *testing.T) {
	item, err := model.NewNote("Note", "content")
	require.NoError(t, err)
	for range 2 {
		require.NoError(t, item.AppendCustomFields(cxf.CustomFields{Type: cxf.CredentialTypeCustomFields, Label: "API", Fields: []cxf.EditableField{}}))
	}
	cmd := &EditCmd{
		command: &command{},
		field:   "API/Token",
		tag:     "development",
		input:   strings.NewReader("secret"),
	}

	require.ErrorContains(t, cmd.editCustomAndTags(item), "section \"API\" is ambiguous")
	require.Empty(t, item.Tags)
	groups, err := item.CustomFields()
	require.NoError(t, err)
	require.Len(t, groups, 2)
	for _, group := range groups {
		require.Empty(t, group.Credential.Fields)
	}
}

type failedFieldReader struct{}

func (failedFieldReader) Read([]byte) (int, error) { return 0, errors.New("stdin failed") }

func TestFailedFieldInputDoesNotApplyTagOrChangeField(t *testing.T) {
	item, err := model.NewNote("Secure note", "content")
	require.NoError(t, err)
	require.NoError(t, item.AppendCustomFields(cxf.CustomFields{Type: cxf.CredentialTypeCustomFields, Fields: []cxf.EditableField{{FieldType: cxf.FieldTypeString, Label: "Description", Value: "original"}}}))
	cmd := &EditCmd{command: &command{}, field: "Description", tag: "development", input: failedFieldReader{}}
	require.ErrorContains(t, cmd.editCustomAndTags(item), "stdin failed")
	require.Empty(t, item.Tags)
	groups, err := item.CustomFields()
	require.NoError(t, err)
	require.Equal(t, "original", groups[0].Credential.Fields[0].Value)
}

func TestAddOptionalDataCanCreateSectionAndAssociateTag(t *testing.T) {
	item, err := model.NewNote("Secure note", "content")
	require.NoError(t, err)
	prompts := testInteractivePrompter(t, "y\n2\nAPI\n3\ndevelopment\n\n4\n")
	prompts.output = &bytes.Buffer{}
	cmd := &AddCmd{command: &command{prompter: prompts}}
	require.NoError(t, cmd.addOptionalData(item, &model.Vault{}))
	groups, err := item.CustomFields()
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, "API", groups[0].Credential.Label)
	require.Empty(t, groups[0].Credential.Fields)
	require.Equal(t, []string{"development"}, item.Tags)
}

func TestSectionRemovalConfirmsAndMovesFieldsWithoutDeletingThem(t *testing.T) {
	item, err := model.NewNote("Secure note", "content")
	require.NoError(t, err)
	require.NoError(t, item.AppendCustomFields(cxf.CustomFields{Type: cxf.CredentialTypeCustomFields, Label: "API", Fields: []cxf.EditableField{{ID: stringPointer("ZmllbGQx"), FieldType: cxf.FieldTypeString, Label: "Token", Value: "kept"}}}))
	prompts := testInteractivePrompter(t, "3\n1\ny\n")
	prompts.output = &bytes.Buffer{}
	cmd := &EditCmd{command: &command{prompter: prompts, errOut: &bytes.Buffer{}}}
	require.NoError(t, cmd.manageSections(item))
	groups, err := item.CustomFields()
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Empty(t, groups[0].Credential.Label)
	require.Equal(t, "kept", groups[0].Credential.Fields[0].Value)
	require.Equal(t, "ZmllbGQx", *groups[0].Credential.Fields[0].ID)
}

func stringPointer(value string) *string { return &value }
