package model

import (
	"crypto"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"fmt"

	"go.lucor.dev/cxf"
)

// EncodeSSHKey stores a private key in CXF's PKCS #8 representation.
func EncodeSSHKey(private crypto.PrivateKey, keyType, comment string) (cxf.SSHKey, error) {
	if key, ok := private.(*ed25519.PrivateKey); ok {
		private = *key
	}
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return cxf.SSHKey{}, fmt.Errorf("cannot encode SSH private key")
	}
	return cxf.SSHKey{Type: cxf.CredentialTypeSSHKey, KeyType: keyType, KeyComment: comment, PrivateKey: base64.RawURLEncoding.EncodeToString(der)}, nil
}
