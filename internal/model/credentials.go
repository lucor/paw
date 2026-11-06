package model

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"

	"go.lucor.dev/cxf"
)

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
		}
	}
	return decoded, nil
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
		item.Credentials[index] = encoded
	}
	return nil
}
