package model

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"slices"

	"go.lucor.dev/cxf"
)

// CustomFieldsGroup pairs a CXF custom-fields credential with its item position.
type CustomFieldsGroup struct {
	Index      int
	Credential cxf.CustomFields
}

// Credentials holds the first supported credential of each type and its position.
type Credentials struct {
	BasicAuth      *cxf.BasicAuth
	BasicAuthIndex int
	Note           *cxf.Note
	NoteIndex      int
	TOTP           *cxf.TOTP
	TOTPIndex      int
	SSHKey         *cxf.SSHKey
	SSHKeyIndex    int
	CustomFields   []CustomFieldsGroup
}

// DecodeCredentials reads supported credentials once, keeping their original positions.
func DecodeCredentials(raws []jsontext.Value) (Credentials, error) {
	decoded := Credentials{BasicAuthIndex: -1, NoteIndex: -1, TOTPIndex: -1, SSHKeyIndex: -1}
	for index, raw := range raws {
		typeName, err := cxf.CredentialTypeOf(raw)
		if err != nil {
			return decoded, err
		}
		switch typeName {
		case cxf.CredentialTypeBasicAuth:
			if decoded.BasicAuth != nil {
				continue
			}
			var credential cxf.BasicAuth
			if err := json.Unmarshal(raw, &credential); err != nil {
				return decoded, err
			}
			decoded.BasicAuth = &credential
			decoded.BasicAuthIndex = index
		case cxf.CredentialTypeNote:
			if decoded.Note != nil {
				continue
			}
			var credential cxf.Note
			if err := json.Unmarshal(raw, &credential); err != nil {
				return decoded, err
			}
			decoded.Note = &credential
			decoded.NoteIndex = index
		case cxf.CredentialTypeTOTP:
			if decoded.TOTP != nil {
				continue
			}
			var credential cxf.TOTP
			if err := json.Unmarshal(raw, &credential); err != nil {
				return decoded, err
			}
			decoded.TOTP = &credential
			decoded.TOTPIndex = index
		case cxf.CredentialTypeSSHKey:
			if decoded.SSHKey != nil {
				continue
			}
			var credential cxf.SSHKey
			if err := json.Unmarshal(raw, &credential); err != nil {
				return decoded, err
			}
			decoded.SSHKey = &credential
			decoded.SSHKeyIndex = index
		case cxf.CredentialTypeCustomFields:
			var credential cxf.CustomFields
			if err := json.Unmarshal(raw, &credential); err != nil {
				return decoded, err
			}
			decoded.CustomFields = append(decoded.CustomFields, CustomFieldsGroup{Index: index, Credential: credential})
		}
	}
	return decoded, nil
}

// CustomFields returns every custom-fields credential in item order.
func (item *Item) CustomFields() ([]CustomFieldsGroup, error) {
	decoded, err := DecodeCredentials(item.Credentials)
	return decoded.CustomFields, err
}

// AppendCustomFields appends a validated custom-fields credential.
func (item *Item) AppendCustomFields(credential cxf.CustomFields) error {
	if err := credential.Validate(); err != nil {
		return fmt.Errorf("invalid custom-fields credential: %w", err)
	}
	encoded, err := json.Marshal(credential, json.Deterministic(true))
	if err != nil {
		return errors.New("cannot encode custom-fields credential")
	}
	item.Credentials = append(item.Credentials, encoded)
	return nil
}

// ReplaceCustomFields replaces a group at its item index. originalFields gives
// the old position of each replacement field, or -1 for a new field.
func (item *Item) ReplaceCustomFields(index int, credential cxf.CustomFields, originalFields []int) error {
	if index < 0 || index >= len(item.Credentials) {
		return errors.New("credential index out of range")
	}
	if err := credential.Validate(); err != nil {
		return fmt.Errorf("invalid custom-fields credential: %w", err)
	}
	typeName, err := cxf.CredentialTypeOf(item.Credentials[index])
	if err != nil {
		return err
	}
	if typeName != cxf.CredentialTypeCustomFields {
		return errors.New("credential at index is not custom-fields")
	}
	encoded, err := json.Marshal(credential, json.Deterministic(true))
	if err != nil {
		return errors.New("cannot encode custom-fields credential")
	}
	var previousCredential cxf.CustomFields
	if err := json.Unmarshal(item.Credentials[index], &previousCredential); err != nil {
		return err
	}
	previousTyped, err := json.Marshal(previousCredential, json.Deterministic(true))
	if err != nil {
		return errors.New("cannot encode existing custom-fields credential")
	}
	if len(originalFields) != len(credential.Fields) {
		return errors.New("missing original field positions")
	}
	identity := true
	for index, original := range originalFields {
		identity = identity && index == original
	}
	if identity && string(previousTyped) == string(encoded) {
		return nil
	}
	encoded, err = preserveCustomFieldsUnknown(item.Credentials[index], encoded, originalFields)
	if err != nil {
		return fmt.Errorf("cannot preserve custom-fields properties: %w", err)
	}
	if string(item.Credentials[index]) != string(encoded) {
		item.Credentials[index] = encoded
	}
	return nil
}

// RemoveCustomFields removes the custom-fields credential at its item index.
func (item *Item) RemoveCustomFields(index int) error {
	if index < 0 || index >= len(item.Credentials) {
		return errors.New("credential index out of range")
	}
	typeName, err := cxf.CredentialTypeOf(item.Credentials[index])
	if err != nil {
		return err
	}
	if typeName != cxf.CredentialTypeCustomFields {
		return errors.New("credential at index is not custom-fields")
	}
	item.Credentials = append(item.Credentials[:index], item.Credentials[index+1:]...)
	return nil
}

// preserveCustomFieldsUnknown retains unmodeled properties for one replaced group.
func preserveCustomFieldsUnknown(previous, replacement []byte, originalFields []int) ([]byte, error) {
	var oldGroup, newGroup map[string]jsontext.Value
	if err := json.Unmarshal(previous, &oldGroup); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(replacement, &newGroup); err != nil {
		return nil, err
	}
	mergeUnknown(oldGroup, newGroup, []string{"type", "id", "label", "fields", "extensions"})
	var oldFields, newFields []map[string]jsontext.Value
	if err := json.Unmarshal(oldGroup["fields"], &oldFields); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(newGroup["fields"], &newFields); err != nil {
		return nil, err
	}
	used := make([]bool, len(oldFields))
	for newIndex, oldIndex := range originalFields {
		if oldIndex == -1 {
			continue
		}
		if oldIndex < 0 || oldIndex >= len(oldFields) || used[oldIndex] {
			return nil, errors.New("invalid original field position")
		}
		used[oldIndex] = true
		mergeUnknown(oldFields[oldIndex], newFields[newIndex], []string{"id", "fieldType", "value", "label", "extensions"})
	}
	fields, err := json.Marshal(newFields, json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	newGroup["fields"] = fields
	return json.Marshal(newGroup, json.Deterministic(true))
}

func mergeUnknown(previous, replacement map[string]jsontext.Value, known []string) {
	for key, value := range previous {
		if !slices.Contains(known, key) {
			if _, exists := replacement[key]; !exists {
				replacement[key] = value
			}
		}
	}
}

// BasicAuth reads the first basic-auth credential and its position.
func (item *Item) BasicAuth() (cxf.BasicAuth, int, error) {
	decoded, err := DecodeCredentials(item.Credentials)
	if decoded.BasicAuth == nil {
		return cxf.BasicAuth{}, -1, err
	}
	return *decoded.BasicAuth, decoded.BasicAuthIndex, err
}

// Note reads the first note credential and its position.
func (item *Item) Note() (cxf.Note, int, error) {
	decoded, err := DecodeCredentials(item.Credentials)
	if decoded.Note == nil {
		return cxf.Note{}, -1, err
	}
	return *decoded.Note, decoded.NoteIndex, err
}

// SSHKey reads the first SSH-key credential.
func (item *Item) SSHKey() (cxf.SSHKey, error) {
	decoded, err := DecodeCredentials(item.Credentials)
	if err != nil {
		return cxf.SSHKey{}, err
	}
	if decoded.SSHKey == nil {
		return cxf.SSHKey{}, fmt.Errorf("SSH key credential is missing")
	}
	return *decoded.SSHKey, nil
}

// SetSSHKey replaces or appends an SSH key credential.
func (item *Item) SetSSHKey(index int, credential cxf.SSHKey) error {
	if index < -1 || index >= len(item.Credentials) {
		return errors.New("credential index out of range")
	}
	if err := credential.Validate(); err != nil {
		return fmt.Errorf("invalid SSH key credential: %w", err)
	}
	encoded, err := json.Marshal(credential, json.Deterministic(true))
	if err != nil {
		return errors.New("cannot encode SSH key credential")
	}
	if index < 0 {
		item.Credentials = append(item.Credentials, encoded)
		return nil
	}
	typeName, err := cxf.CredentialTypeOf(item.Credentials[index])
	if err != nil {
		return err
	}
	if typeName != cxf.CredentialTypeSSHKey {
		return errors.New("credential at index is not an SSH key")
	}
	encoded, err = preserveCredentialProperties(item.Credentials[index], encoded, []string{
		"type", "keyType", "privateKey", "keyComment", "creationDate", "expiryDate", "keyGenerationSource",
	})
	if err != nil {
		return fmt.Errorf("cannot preserve SSH key properties: %w", err)
	}
	item.Credentials[index] = encoded
	return nil
}

// SetBasicAuth replaces or appends a basic-auth credential.
func (item *Item) SetBasicAuth(index int, credential cxf.BasicAuth) error {
	if index < -1 || index >= len(item.Credentials) {
		return errors.New("credential index out of range")
	}
	encoded, err := json.Marshal(credential, json.Deterministic(true))
	if err != nil {
		return errors.New("cannot encode basic-auth credential")
	}
	if index < 0 {
		item.Credentials = append(item.Credentials, encoded)
	} else {
		encoded, err = preserveCredentialUnknown(item.Credentials[index], encoded, []string{"type", "username", "password"})
		if err != nil {
			return fmt.Errorf("cannot preserve basic-auth properties: %w", err)
		}
		item.Credentials[index] = encoded
	}
	return nil
}

// SetNote replaces or appends a note credential.
func (item *Item) SetNote(index int, note cxf.Note) error {
	if index < -1 || index >= len(item.Credentials) {
		return errors.New("credential index out of range")
	}
	encoded, err := json.Marshal(note, json.Deterministic(true))
	if err != nil {
		return errors.New("cannot encode note credential")
	}
	if index < 0 {
		item.Credentials = append(item.Credentials, encoded)
	} else {
		encoded, err = preserveCredentialUnknown(item.Credentials[index], encoded, []string{"type", "content"})
		if err != nil {
			return fmt.Errorf("cannot preserve note properties: %w", err)
		}
		item.Credentials[index] = encoded
	}
	return nil
}

// SetTOTP replaces or appends a TOTP credential.
func (item *Item) SetTOTP(index int, credential cxf.TOTP) error {
	if index < -1 || index >= len(item.Credentials) {
		return errors.New("credential index out of range")
	}
	if err := credential.Validate(); err != nil {
		return fmt.Errorf("invalid TOTP credential: %w", err)
	}
	encoded, err := json.Marshal(credential, json.Deterministic(true))
	if err != nil {
		return errors.New("cannot encode TOTP credential")
	}
	if index < 0 {
		item.Credentials = append(item.Credentials, encoded)
		return nil
	}
	typeName, err := cxf.CredentialTypeOf(item.Credentials[index])
	if err != nil {
		return err
	}
	if typeName != cxf.CredentialTypeTOTP {
		return errors.New("credential at index is not TOTP")
	}
	encoded, err = preserveCredentialProperties(item.Credentials[index], encoded, []string{
		"type", "secret", "algorithm", "period", "digits", "issuer", "username",
	})
	if err != nil {
		return fmt.Errorf("cannot preserve TOTP properties: %w", err)
	}
	item.Credentials[index] = encoded
	return nil
}

func preserveCredentialUnknown(previous, replacement []byte, fields []string) ([]byte, error) {
	var oldObject, newObject map[string]jsontext.Value
	if err := json.Unmarshal(previous, &oldObject); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(replacement, &newObject); err != nil {
		return nil, err
	}
	mergeUnknown(oldObject, newObject, fields)
	for _, name := range fields[1:] {
		oldRaw, oldOK := oldObject[name]
		newRaw, newOK := newObject[name]
		if !oldOK || !newOK || string(oldRaw) == "null" || string(newRaw) == "null" {
			continue
		}
		var oldField, newField map[string]jsontext.Value
		if err := json.Unmarshal(oldRaw, &oldField); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(newRaw, &newField); err != nil {
			return nil, err
		}
		mergeUnknown(oldField, newField, []string{"id", "fieldType", "value", "label", "extensions"})
		encoded, err := json.Marshal(newField, json.Deterministic(true))
		if err != nil {
			return nil, err
		}
		newObject[name] = encoded
	}
	return json.Marshal(newObject, json.Deterministic(true))
}

func preserveCredentialProperties(previous, replacement []byte, known []string) ([]byte, error) {
	var oldObject, newObject map[string]jsontext.Value
	if err := json.Unmarshal(previous, &oldObject); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(replacement, &newObject); err != nil {
		return nil, err
	}
	mergeUnknown(oldObject, newObject, known)
	return json.Marshal(newObject, json.Deterministic(true))
}
