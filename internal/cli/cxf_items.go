// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"

	"go.lucor.dev/cxf"
	"golang.org/x/crypto/ssh"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// findVaultItem finds one summary by its title and Paw item category.
func findVaultItem(vault *model.Vault, title string, itemType paw.ItemType) (*cxf.Item, error) {
	var found *cxf.Item
	for index := range vault.Accounts[0].Items {
		candidate := &vault.Accounts[0].Items[index]
		if candidate.Title != title {
			continue
		}
		candidateType, err := model.ItemTypeOf(candidate)
		if err != nil {
			return nil, err
		}
		if candidateType != itemType {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("item name %q is ambiguous", title)
		}
		found = candidate
	}
	if found == nil {
		return nil, fmt.Errorf("item %q does not exist", title)
	}
	return found, nil
}

// sshKeyFromCXF decodes an SSH key and derives its OpenSSH display values.
func sshKeyFromCXF(credential cxf.SSHKey) (string, string, string, error) {
	der, err := base64.RawURLEncoding.DecodeString(credential.PrivateKey)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid SSH private key")
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid SSH private key")
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return "", "", "", fmt.Errorf("unsupported SSH private key")
	}
	private, err := ssh.MarshalPrivateKey(key, credential.KeyComment)
	if err != nil {
		return "", "", "", fmt.Errorf("cannot encode SSH private key")
	}
	return string(pem.EncodeToMemory(private)), string(ssh.MarshalAuthorizedKey(signer.PublicKey())), ssh.FingerprintSHA256(signer.PublicKey()), nil
}

// sshCredential reads the SSH-key credential from an item.
func sshCredential(item *model.Item) (cxf.SSHKey, error) {
	for _, raw := range item.Credentials {
		typeName, err := cxf.CredentialTypeOf(raw)
		if err != nil {
			return cxf.SSHKey{}, err
		}
		if typeName != cxf.CredentialTypeSSHKey {
			continue
		}
		var credential cxf.SSHKey
		if err := json.Unmarshal(raw, &credential); err != nil {
			return cxf.SSHKey{}, err
		}
		return credential, nil
	}
	return cxf.SSHKey{}, fmt.Errorf("SSH key credential is missing")
}

// sshAgentKey decodes an SSH private key for the agent operations.
func sshAgentKey(credential cxf.SSHKey) (crypto.PrivateKey, ssh.PublicKey, error) {
	der, err := base64.RawURLEncoding.DecodeString(credential.PrivateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid SSH private key")
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid SSH private key")
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("unsupported SSH private key")
	}
	return key, signer.PublicKey(), nil
}

// basicAuth reads the first basic-auth credential in an item.
func basicAuth(item *model.Item) (cxf.BasicAuth, int, error) {
	for index, raw := range item.Credentials {
		typeName, err := cxf.CredentialTypeOf(raw)
		if err != nil {
			return cxf.BasicAuth{}, -1, err
		}
		if typeName != cxf.CredentialTypeBasicAuth {
			continue
		}
		var credential cxf.BasicAuth
		if err := json.Unmarshal(raw, &credential); err != nil {
			return cxf.BasicAuth{}, -1, err
		}
		return credential, index, nil
	}
	return cxf.BasicAuth{}, -1, nil
}

// setBasicAuth replaces one basic-auth credential in an item.
func setBasicAuth(item *model.Item, index int, credential cxf.BasicAuth) error {
	data, err := json.Marshal(credential)
	if err != nil {
		return fmt.Errorf("could not encode basic-auth credential")
	}
	if index < 0 {
		item.Credentials = append(item.Credentials, data)
	} else {
		item.Credentials[index] = data
	}
	return nil
}

// passwordSettings reads Paw's generation settings from a CXF password field.
func passwordSettings(field *cxf.EditableField) model.PasswordSettings {
	settings := model.PasswordSettings{Mode: paw.CustomPassword}
	if field == nil {
		return settings
	}
	for _, raw := range field.Extensions {
		var extension struct {
			Name   string           `json:"name"`
			Mode   paw.PasswordMode `json:"mode"`
			Length int              `json:"length"`
			Format paw.Format       `json:"format"`
		}
		if json.Unmarshal(raw, &extension) == nil && extension.Name == "paw.pm/password" {
			return model.PasswordSettings{Mode: extension.Mode, Length: extension.Length, Format: extension.Format}
		}
	}
	return settings
}

// makePasswordField stores the generation settings with a concealed password.
func makePasswordField(value string, settings model.PasswordSettings) (*cxf.EditableField, error) {
	extension, err := json.Marshal(struct {
		Name string `json:"name"`
		model.PasswordSettings
	}{Name: "paw.pm/password", PasswordSettings: settings})
	if err != nil {
		return nil, fmt.Errorf("cannot encode password settings")
	}
	return &cxf.EditableField{FieldType: cxf.FieldTypeConcealedString, Value: value, Extensions: []json.RawMessage{extension}}, nil
}

// noteCredential reads a note credential from an item.
func noteCredential(item *model.Item) (cxf.Note, int, error) {
	for index, raw := range item.Credentials {
		typeName, err := cxf.CredentialTypeOf(raw)
		if err != nil {
			return cxf.Note{}, -1, err
		}
		if typeName != cxf.CredentialTypeNote {
			continue
		}
		var credential cxf.Note
		if err := json.Unmarshal(raw, &credential); err != nil {
			return cxf.Note{}, -1, err
		}
		return credential, index, nil
	}
	return cxf.Note{}, -1, nil
}

// setNoteCredential replaces or appends a note credential.
func setNoteCredential(item *model.Item, index int, value string) error {
	data, err := json.Marshal(cxf.Note{Type: cxf.CredentialTypeNote, Content: cxf.EditableField{FieldType: cxf.FieldTypeString, Value: value}})
	if err != nil {
		return fmt.Errorf("could not encode note credential")
	}
	if index < 0 {
		item.Credentials = append(item.Credentials, data)
	} else {
		item.Credentials[index] = data
	}
	return nil
}

// passwordValue returns the password value when the credential has one.
func passwordValue(credential cxf.BasicAuth) string {
	if credential.Password == nil {
		return ""
	}
	return credential.Password.Value
}

// usernameValue returns the username value when the credential has one.
func usernameValue(credential cxf.BasicAuth) string {
	if credential.Username == nil {
		return ""
	}
	return credential.Username.Value
}
