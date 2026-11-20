// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

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
	require.Contains(t, stderr.String(), "copied")
	require.NotContains(t, stderr.String(), "synthetic-secret")
}

func TestShowTOTPSecretIsMaskedAndExplicitlySelectable(t *testing.T) {
	item, err := model.NewLogin("GitHub", "developer", "password", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	item.Credentials = append(item.Credentials, []byte(`{"type":"totp","secret":"JBSWY3DPEHPK3PXP","period":30,"digits":6,"algorithm":"sha1","issuer":"GitHub"}`))
	var output bytes.Buffer
	cmd := &ShowCmd{command: &command{out: &output}, itemPath: itemPath{itemType: model.LoginItemType}}
	require.NoError(t, cmd.showItem(context.Background(), item))
	require.Contains(t, output.String(), "TOTP secret: ********")
	require.NotContains(t, output.String(), "JBSWY3DPEHPK3PXP")
	value, err := itemField(item, model.LoginItemType, "TOTP secret")
	require.NoError(t, err)
	require.Equal(t, "JBSWY3DPEHPK3PXP", value)
	cmd.reveal = true
	output.Reset()
	require.NoError(t, cmd.showItem(context.Background(), item))
	require.Contains(t, output.String(), "TOTP secret: JBSWY3DPEHPK3PXP")
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
	prompts := newPrompter(strings.NewReader("y\n2\nAPI\n3\ndevelopment\n\n4\n"), &bytes.Buffer{})
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
	prompts := newPrompter(strings.NewReader("3\n1\ny\n"), &bytes.Buffer{})
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
