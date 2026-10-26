// Package model defines Paw's current data model using CXF types.
package model

import (
	"crypto/rand"
	"encoding/base64"
)

// newID returns 16 random bytes encoded as unpadded Base64url.
func newID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(id[:]), nil
}
