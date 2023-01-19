package sshkey

import (
	"crypto"
	"crypto/ed25519"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/mikesmitty/edkey"
	"golang.org/x/crypto/ssh"
)

// GenerateKey generates an ed25519 sshkey
func GenerateKey() (sshkey, error) {
	pubKey, privKey, err := ed25519.GenerateKey(cryptorand.Reader)
	if err != nil {
		return sshkey{}, fmt.Errorf("could not generate ed25519 key: %w", err)
	}
	return sshkey{privateKey: &privKey, publicKey: pubKey}, nil
}

// ParseKey parses a raw RSA or Ed22519 ssh key
func ParseKey(b []byte) (sshkey, error) {
	k, err := ssh.ParseRawPrivateKey(b)
	if err != nil {
		return sshkey{}, err
	}
	return newSSHKeyFromPrivateKey(k)
}

// ParseKey parses a raw RSA or Ed22519 ssh key encrypted with a passphrase
func ParseKeyWithPassphrase(b, passphrase []byte) (sshkey, error) {
	k, err := ssh.ParseRawPrivateKeyWithPassphrase(b, passphrase)
	if err != nil {
		return sshkey{}, err
	}
	return newSSHKeyFromPrivateKey(k)
}

func newSSHKeyFromPrivateKey(key interface{}) (sshkey, error) {
	switch v := key.(type) {
	case *ed25519.PrivateKey:
		return sshkey{privateKey: v, publicKey: v.Public()}, nil
	case *rsa.PrivateKey:
		return sshkey{privateKey: v, publicKey: v.Public()}, nil
	default:
		return sshkey{}, fmt.Errorf("unsupported type %T", v)
	}
}

type sshkey struct {
	privateKey crypto.PrivateKey
	publicKey  crypto.PublicKey
}

func (sk sshkey) PrivateKey() crypto.PrivateKey {
	return sk.privateKey
}

func (sk sshkey) MarshalPrivateKey() []byte {
	var pemBlock *pem.Block
	switch v := sk.privateKey.(type) {
	case *ed25519.PrivateKey:
		// TODO move to x/crypto/ssh once https://go-review.googlesource.com/c/crypto/+/218620/ is merged
		// see golang/go#37132
		pemBlock = &pem.Block{
			Type:  "OPENSSH PRIVATE KEY",
			Bytes: edkey.MarshalED25519PrivateKey(*v),
		}
	case *rsa.PrivateKey:
		pemBlock = &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(v),
		}
	}
	return pem.EncodeToMemory(pemBlock)
}

func (sk sshkey) PublicKey() ssh.PublicKey {
	sshPublicKey, err := ssh.NewPublicKey(sk.publicKey)
	if err != nil {
		panic("could not generate ssh public key from the crypto public key")
	}
	return sshPublicKey
}

func (sk sshkey) MarshalPublicKey() []byte {
	return ssh.MarshalAuthorizedKey(sk.PublicKey())
}

func (sk sshkey) Fingerprint() string {
	return ssh.FingerprintSHA256(sk.PublicKey())
}
