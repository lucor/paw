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

func TestCredentialMissingAndMalformed(t *testing.T) {
	item := &Item{}
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
