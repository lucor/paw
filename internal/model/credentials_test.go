package model

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/paw"
	"golang.org/x/crypto/ssh"
)

func TestCredentialAccessAndUpdates(t *testing.T) {
	item, err := NewLogin("Example", "alice", "saved", PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	note := cxf.Note{Type: cxf.CredentialTypeNote, Content: cxf.EditableField{FieldType: cxf.FieldTypeString, Value: "original"}}
	require.NoError(t, item.SetNote(-1, note))
	firstNote := append(jsontext.Value(nil), item.Credentials[1]...)
	require.NoError(t, item.SetNote(-1, cxf.Note{Type: cxf.CredentialTypeNote, Content: cxf.EditableField{FieldType: cxf.FieldTypeString, Value: "second"}}))
	decoded, err := DecodeCredentials(item.Credentials)
	require.NoError(t, err)
	require.Equal(t, 0, decoded.BasicAuthIndex)
	require.Equal(t, 1, decoded.NoteIndex)
	require.Equal(t, "original", decoded.Note.Content.Value)
	auth, index, err := item.BasicAuth()
	require.NoError(t, err)
	auth.Password.Value = "updated"
	require.NoError(t, item.SetBasicAuth(index, auth))
	require.Equal(t, firstNote, item.Credentials[1])
	require.Len(t, item.Credentials, 3)
	note, index, err = item.Note()
	require.NoError(t, err)
	require.Equal(t, "original", note.Content.Value)
	require.Equal(t, 1, index)
	before, err := json.Marshal(item.Credentials, json.Deterministic(true))
	require.NoError(t, err)
	require.Error(t, item.SetBasicAuth(5, auth))
	require.Error(t, item.SetNote(-2, note))
	after, err := json.Marshal(item.Credentials, json.Deterministic(true))
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestBasicAuthUpdatePreservesUnknownCredentialAndFieldMembers(t *testing.T) {
	item, err := NewLogin("Example", "alice", "saved", PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	item.Credentials[0] = jsontext.Value(`{"type":"basic-auth","username":{"fieldType":"string","value":"alice","futureField":{"keep":true}},"password":{"fieldType":"concealed-string","value":"saved","futureField":{"keep":true}},"futureCredential":{"retain":true}}`)
	credential, index, err := item.BasicAuth()
	require.NoError(t, err)
	credential.Password.Value = "updated"
	require.NoError(t, item.SetBasicAuth(index, credential))
	var saved map[string]jsontext.Value
	require.NoError(t, json.Unmarshal(item.Credentials[0], &saved))
	require.Contains(t, string(saved["futureCredential"]), `"retain":true`)
	require.Contains(t, string(saved["password"]), `"futureField":{"keep":true}`)
	require.Contains(t, string(saved["username"]), `"futureField":{"keep":true}`)
}

func TestTOTPUpdatePreservesUnknownCredentialMembers(t *testing.T) {
	item := &Item{Credentials: []jsontext.Value{
		jsontext.Value(`{"type":"totp","secret":"JBSWY3DPEHPK3PXP","algorithm":"sha1","period":30,"digits":6,"futureCredential":{"keep":true}}`),
	}}
	decoded, err := DecodeCredentials(item.Credentials)
	require.NoError(t, err)
	decoded.TOTP.Secret = "MZXW6YTBOI"
	require.NoError(t, item.SetTOTP(decoded.TOTPIndex, *decoded.TOTP))
	var saved map[string]jsontext.Value
	require.NoError(t, json.Unmarshal(item.Credentials[0], &saved))
	require.Contains(t, string(saved["futureCredential"]), `"keep":true`)
	require.Contains(t, string(saved["secret"]), "MZXW6YTBOI")
}

func TestSSHKeyUpdatePreservesUnknownCredentialMembers(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	credential, err := EncodeSSHKey(private, "ssh-ed25519", "updated")
	require.NoError(t, err)
	item := &Item{Credentials: []jsontext.Value{
		jsontext.Value(`{"type":"ssh-key","keyType":"ssh-ed25519","privateKey":"` + credential.PrivateKey + `","keyComment":"original","futureCredential":{"keep":true}}`),
	}}
	require.NoError(t, item.SetSSHKey(0, credential))
	var saved map[string]jsontext.Value
	require.NoError(t, json.Unmarshal(item.Credentials[0], &saved))
	require.Contains(t, string(saved["futureCredential"]), `"keep":true`)
	require.Contains(t, string(saved["keyComment"]), "updated")
}

func TestCredentialMissingAndMalformed(t *testing.T) {
	item := &Item{}
	groups, err := item.CustomFields()
	require.NoError(t, err)
	require.Empty(t, groups)
	_, index, err := item.BasicAuth()
	require.NoError(t, err)
	require.Equal(t, -1, index)
	_, index, err = item.Note()
	require.NoError(t, err)
	require.Equal(t, -1, index)
	_, err = item.SSHKey()
	require.ErrorContains(t, err, "missing")
	_, err = DecodeCredentials([]jsontext.Value{jsontext.Value(`{`)})
	require.Error(t, err)
}

func TestCustomFieldsAccessAndMutationPreserveItemData(t *testing.T) {
	item, err := NewLogin("Example", "alice", "saved", PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	item.Subtitle = ptr("login subtitle")
	item.Scope = &cxf.CredentialScope{URLs: []string{"https://example.test"}, AndroidApps: []cxf.AndroidAppID{}}
	first := cxf.CustomFields{
		Type: cxf.CredentialTypeCustomFields,
		ID:   ptr("Z3JvdXAx"), Label: "Repeated",
		Fields: []cxf.EditableField{
			{ID: ptr("ZmllbGQx"), FieldType: cxf.FieldTypeConcealedString, Value: "secret", Label: "Token", Extensions: []jsontext.Value{jsontext.Value(`{"name":"example.test/field","data":1}`)}},
			{FieldType: "future-field", Value: "opaque", Extensions: []jsontext.Value{jsontext.Value(`{"name":"example.test/value"}`)}},
		},
		Extensions: []jsontext.Value{jsontext.Value(`{"name":"example.test/group","enabled":true}`)},
	}
	second := cxf.CustomFields{Type: cxf.CredentialTypeCustomFields, Fields: []cxf.EditableField{}}
	require.NoError(t, item.AppendCustomFields(first))
	item.Credentials[1] = jsontext.Value(`{"type":"custom-fields","id":"Z3JvdXAx","label":"Repeated","fields":[{"id":"ZmllbGQx","fieldType":"concealed-string","value":"secret","label":"Token","extensions":[{"name":"example.test/field","data":1}],"futureField":{"keep":true}},{"fieldType":"future-field","value":"opaque","extensions":[{"name":"example.test/value"}]}],"extensions":[{"name":"example.test/group","enabled":true}],"futureGroup":{"retain":{"x":1}}}`)
	unknown := jsontext.Value(`{"type":"future-credential","opaque":{"keep":true}}`)
	item.Credentials = append(item.Credentials, unknown)
	require.NoError(t, item.AppendCustomFields(second))
	groups, err := item.CustomFields()
	require.NoError(t, err)
	require.Len(t, groups, 2)
	require.Equal(t, 1, groups[0].Index)
	require.Equal(t, 3, groups[1].Index)
	require.Equal(t, "Repeated", groups[0].Credential.Label)
	require.Empty(t, groups[1].Credential.Label)
	require.Equal(t, first, groups[0].Credential)
	require.NoError(t, item.Validate())

	beforeUnrelated := append(jsontext.Value(nil), item.Credentials[0]...)
	replacement := first
	replacement.Label = "Updated"
	replacement.Fields = append([]cxf.EditableField(nil), first.Fields...)
	replacement.Fields[0].Value = "updated-secret"
	require.NoError(t, item.ReplaceCustomFields(groups[0].Index, replacement, []int{0, 1}))
	require.Equal(t, beforeUnrelated, item.Credentials[0])
	require.Equal(t, unknown, item.Credentials[2])
	var raw map[string]jsontext.Value
	require.NoError(t, json.Unmarshal(item.Credentials[1], &raw))
	require.JSONEq(t, `{"retain":{"x":1}}`, string(raw["futureGroup"]))
	var rawFields []map[string]jsontext.Value
	require.NoError(t, json.Unmarshal(raw["fields"], &rawFields))
	require.JSONEq(t, `{"keep":true}`, string(rawFields[0]["futureField"]))
	require.NoError(t, item.Validate())

	unchanged := append(jsontext.Value(nil), item.Credentials[1]...)
	groups, err = item.CustomFields()
	require.NoError(t, err)
	require.NoError(t, item.ReplaceCustomFields(groups[0].Index, groups[0].Credential, []int{0, 1}))
	require.Equal(t, unchanged, item.Credentials[1], "unchanged values retain their original JSON")
	require.NoError(t, item.RemoveCustomFields(groups[0].Index))
	require.Equal(t, unknown, item.Credentials[1])
	groups, err = item.CustomFields()
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, 2, groups[0].Index)
	require.NoError(t, item.Validate())

	encoded, err := json.Marshal(item, json.Deterministic(true))
	require.NoError(t, err)
	var restored Item
	require.NoError(t, json.Unmarshal(encoded, &restored))
	require.Equal(t, item.Metadata.Type, restored.Metadata.Type)
	require.Equal(t, item.Title, restored.Title)
	require.Equal(t, item.Subtitle, restored.Subtitle)
	require.Equal(t, item.Scope, restored.Scope)
	require.Equal(t, item.Credentials, restored.Credentials)
}

func TestCustomFieldsOperationsValidateGroupsAndIndices(t *testing.T) {
	item, err := NewNote("Example", "body")
	require.NoError(t, err)
	valid := cxf.CustomFields{Type: cxf.CredentialTypeCustomFields, Fields: []cxf.EditableField{}}
	require.Error(t, item.AppendCustomFields(cxf.CustomFields{Type: cxf.CredentialTypeCustomFields}))
	require.NoError(t, item.AppendCustomFields(valid))
	require.Error(t, item.ReplaceCustomFields(0, valid, nil), "note credential is not a custom-fields group")
	require.Error(t, item.ReplaceCustomFields(4, valid, nil))
	require.Error(t, item.RemoveCustomFields(0))
	require.Error(t, item.RemoveCustomFields(4))
	groups, err := item.CustomFields()
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, 1, groups[0].Index)
	require.NoError(t, item.Validate())
}

func TestReplaceCustomFieldsPreservesUnknownPropertiesOfRemainingField(t *testing.T) {
	item, err := NewNote("Example", "body")
	require.NoError(t, err)
	item.Credentials = append(item.Credentials, jsontext.Value(`{"type":"custom-fields","fields":[{"id":"ZmllbGQx","fieldType":"string","value":"remove","future":{"owner":"removed"}},{"id":"ZmllbGQy","fieldType":"string","value":"keep","future":{"owner":"kept"}}]}`))

	groups, err := item.CustomFields()
	require.NoError(t, err)
	replacement := groups[0].Credential
	replacement.Fields = []cxf.EditableField{{ID: ptr("ZmllbGQy"), FieldType: cxf.FieldTypeString, Value: "updated"}}
	require.NoError(t, item.ReplaceCustomFields(groups[0].Index, replacement, []int{1}))

	var raw map[string]jsontext.Value
	require.NoError(t, json.Unmarshal(item.Credentials[1], &raw))
	var fields []map[string]jsontext.Value
	require.NoError(t, json.Unmarshal(raw["fields"], &fields))
	require.Len(t, fields, 1)
	require.JSONEq(t, `{"owner":"kept"}`, string(fields[0]["future"]))
}

func ptr(value string) *string { return &value }

func TestPasswordFieldPreservesOtherExtensions(t *testing.T) {
	previous := &cxf.EditableField{FieldType: cxf.FieldTypeConcealedString, Value: "original", Extensions: []jsontext.Value{
		jsontext.Value(`{"name":"example.org/field","value":"keep"}`),
		jsontext.Value(`{"name":"paw.pm/password","mode":0,"future":"keep"}`),
	}}
	settings := PasswordSettings{Mode: paw.RandomPassword, Length: 24, Format: paw.DigitsFormat}
	updated, err := PasswordField("replacement", settings, previous)
	require.NoError(t, err)
	require.Equal(t, settings, PasswordSettingsFromField(updated))
	require.Equal(t, previous.Extensions[0], updated.Extensions[0])
	require.Len(t, updated.Extensions, 2)
	var extension map[string]jsontext.Value
	require.NoError(t, json.Unmarshal(updated.Extensions[1], &extension))
	require.JSONEq(t, `"keep"`, string(extension["future"]))
	require.Equal(t, "original", previous.Value)
	require.Equal(t, PasswordSettings{Mode: paw.CustomPassword}, PasswordSettingsFromField(nil))
	_, err = PasswordField("value", PasswordSettings{Mode: 99}, previous)
	require.Error(t, err)
	_, err = PasswordField("bad\xff", settings, previous)
	require.Error(t, err)
}

func TestSSHCredentialConversion(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	for _, key := range []any{private, &private} {
		credential, err := EncodeSSHKey(key, "ssh-ed25519", "example")
		require.NoError(t, err)
		parsed, public, err := ParseSSHKey(credential)
		require.NoError(t, err)
		require.Equal(t, private, parsed)
		privateText, publicText, fingerprint, err := FormatSSHKey(credential)
		require.NoError(t, err)
		require.Equal(t, string(ssh.MarshalAuthorizedKey(public)), publicText)
		require.Equal(t, ssh.FingerprintSHA256(public), fingerprint)
		reparsed, err := ssh.ParseRawPrivateKey([]byte(privateText))
		require.NoError(t, err)
		require.Equal(t, private, *reparsed.(*ed25519.PrivateKey))
	}
	for _, value := range []string{"invalid!", "bm90LWRlcg"} {
		_, _, err := ParseSSHKey(cxf.SSHKey{PrivateKey: value})
		require.Error(t, err)
		require.NotContains(t, err.Error(), value)
	}
}

func TestMoveCustomFieldPreservesRawCXFProperties(t *testing.T) {
	item, err := NewLogin("Example", "alice", "saved", PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	item.Credentials = append(item.Credentials,
		jsontext.Value(`{"type":"custom-fields","label":"Source","fields":[{"fieldType":"string","value":"keep","label":"Name","futureField":{"retain":true}}],"futureGroup":{"source":true}}`),
		jsontext.Value(`{"type":"custom-fields","label":"Destination","fields":[],"futureGroup":{"destination":true}}`),
	)
	require.NoError(t, item.MoveCustomField(1, 0, "Destination"))
	require.Contains(t, string(item.Credentials[1]), `"futureGroup":{"source":true}`)
	require.Contains(t, string(item.Credentials[2]), `"futureGroup":{"destination":true}`)
	require.Contains(t, string(item.Credentials[2]), `"futureField":{"retain":true}`)
	groups, err := item.CustomFields()
	require.NoError(t, err)
	require.Len(t, groups[0].Credential.Fields, 0)
	require.Equal(t, "Destination", groups[1].Credential.Label)
	require.Equal(t, "keep", groups[1].Credential.Fields[0].Value)
}

func TestFindItemUsesTitleAndCategory(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := NewVault(key, "Personal")
	require.NoError(t, err)
	login, err := NewLogin("Example", "alice", "saved", PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	note, err := NewNote("Example", "text")
	require.NoError(t, err)
	for _, item := range []*Item{login, note} {
		summary, err := item.Summary()
		require.NoError(t, err)
		vault.Accounts[0].Items = append(vault.Accounts[0].Items, *summary)
	}
	found, err := vault.FindItem("Example", LoginItemType)
	require.NoError(t, err)
	require.Equal(t, login.ID, found.ID)
	_, err = vault.FindItem("Missing", LoginItemType)
	require.ErrorContains(t, err, "does not exist")
	vault.Accounts[0].Items = append(vault.Accounts[0].Items, *found)
	_, err = vault.FindItem("Example", LoginItemType)
	require.ErrorContains(t, err, "ambiguous")
	_, err = (&Vault{}).FindItem("Example", LoginItemType)
	require.Error(t, err)
}
