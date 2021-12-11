package paw

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"filippo.io/age"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/scrypt"
)

const (
	keySize    = 64
	workFactor = 16
)

const (
	Version = "lucor/paw/v1"
)

type Ruler interface {
	Template() (string, error)
	Len() int
}

type Seeder interface {
	Ruler
	// Salt returns the salt used to generate the secret
	Salt() []byte
	// Info holds the info used to generate the secret
	Info() []byte
}

type SecretMaker interface {
	Secret(seeder Seeder) (string, error)
}

type Key struct {
	seedKey      []byte
	pubKey       []byte
	ageRecipient *age.ScryptRecipient
	ageIdentity  *age.ScryptIdentity
	unlocked     time.Time
}

func New(name string, password string) (*Key, error) {

	secret := bytes.Buffer{}
	secret.WriteString(name)
	secret.WriteString(password)

	ageIdentity, err := age.NewScryptIdentity(secret.String())
	if err != nil {
		return nil, fmt.Errorf("failed to generate age identity: %v", err)
	}

	ageRecipient, err := age.NewScryptRecipient(secret.String())
	if err != nil {
		return nil, fmt.Errorf("failed to generate age recipient: %v", err)
	}
	ageRecipient.SetWorkFactor(workFactor)

	salt := []byte(Version)
	key, err := scrypt.Key(secret.Bytes(), salt, 1<<workFactor, 8, 1, keySize)
	if err != nil {
		return nil, fmt.Errorf("failed to generate seedKey: %v", err)
	}

	kc := &Key{
		ageIdentity:  ageIdentity,
		ageRecipient: ageRecipient,
		seedKey:      make([]byte, 32),
		pubKey:       make([]byte, 32),
		unlocked:     time.Now(),
	}
	copy(kc.seedKey, key[:32])
	copy(kc.pubKey, key[32:])
	return kc, nil
}

// Secret derives a secret from the seeder
func (k *Key) Secret(seeder Seeder) (string, error) {

	// Underlying hash function for HMAC.
	hash := sha256.New
	salt := seeder.Salt()
	if salt == nil {
		salt = make([]byte, hash().Size())
		if _, err := rand.Read(salt); err != nil {
			panic(err)
		}
	}

	// reader to derive a key
	reader := hkdf.New(sha256.New, k.seedKey, salt, seeder.Info())
	template, err := seeder.Template()
	if err != nil {
		return "", err
	}

	chars := []byte(template)

	var secret bytes.Buffer
	for {
		buf := make([]byte, 1) // TODO define max len attempts
		_, err := io.ReadFull(reader, buf)
		if err != nil {
			return "", err
		}

		if !bytes.Contains(chars, buf) {
			continue
		}

		secret.WriteByte(buf[0])
		if secret.Len() == seeder.Len() {
			break
		}
	}

	return secret.String(), nil
}

// Decrypt decrypts the message
func (k *Key) Decrypt(src io.Reader) (io.Reader, error) {
	return age.Decrypt(src, k.ageIdentity)
}

// Encrypt a message
func (k *Key) Encrypt(dst io.Writer) (io.WriteCloser, error) {
	return age.Encrypt(dst, k.ageRecipient)
}

// String returns a string representation of the key
func (k *Key) String() string {
	return hex.EncodeToString(k.pubKey)
}
