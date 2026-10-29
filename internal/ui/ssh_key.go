// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"

	"go.lucor.dev/cxf"
	"golang.org/x/crypto/ssh"
)

// FormatSSHKey derives OpenSSH text and a fingerprint from a CXF key.
func FormatSSHKey(credential cxf.SSHKey) (string, string, string, error) {
	der, err := base64.RawURLEncoding.DecodeString(credential.PrivateKey)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid SSH private key: %w", err)
	}
	private, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid SSH private key: %w", err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		return "", "", "", fmt.Errorf("unsupported SSH private key: %w", err)
	}
	block, err := ssh.MarshalPrivateKey(private, credential.KeyComment)
	if err != nil {
		return "", "", "", fmt.Errorf("cannot format SSH private key: %w", err)
	}
	return string(pem.EncodeToMemory(block)), string(ssh.MarshalAuthorizedKey(signer.PublicKey())), ssh.FingerprintSHA256(signer.PublicKey()), nil
}
