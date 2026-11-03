package model

import (
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"

	"go.lucor.dev/cxf"
	"golang.org/x/crypto/ssh"
)

// ParseSSHKey decodes an SSH private key for the agent operations.
func ParseSSHKey(credential cxf.SSHKey) (crypto.PrivateKey, ssh.PublicKey, error) {
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

// FormatSSHKey decodes an SSH key and derives its OpenSSH display values.
func FormatSSHKey(credential cxf.SSHKey) (string, string, string, error) {
	key, publicKey, err := ParseSSHKey(credential)
	if err != nil {
		return "", "", "", err
	}
	private, err := ssh.MarshalPrivateKey(key, credential.KeyComment)
	if err != nil {
		return "", "", "", fmt.Errorf("cannot encode SSH private key")
	}
	return string(pem.EncodeToMemory(private)), string(ssh.MarshalAuthorizedKey(publicKey)), ssh.FingerprintSHA256(publicKey), nil
}
