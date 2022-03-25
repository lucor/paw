package paw

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"time"

	"filippo.io/age"
	"filippo.io/age/armor"
	"golang.org/x/crypto/hkdf"

	agepaw "lucor.dev/paw/internal/age"
	"lucor.dev/paw/internal/age/bech32"
)

const (
	ID      = "dev.lucor.paw"
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
	ageIdentity *age.X25519Identity
}

// MakeOneTimeKey generates a one time age secret key.
// The key can be used to generate random passwords
func MakeOneTimeKey() (key *Key, err error) {
	wrapErr := func(err error) error {
		return fmt.Errorf("paw: makekey error: %w", err)
	}

	// Generate the age X25519 Identity
	ageIdentity, ierr := age.GenerateX25519Identity()
	if ierr != nil {
		err = wrapErr(ierr)
		return
	}
	key = &Key{
		ageIdentity: ageIdentity,
	}
	return
}

// MakeKey generates an age secret key. The key is encrypted to w and protect using the provided password
func MakeKey(password string, w io.Writer) (key *Key, err error) {

	wrapErr := func(err error) error {
		return fmt.Errorf("paw: makekey error: %w", err)
	}

	// Generate the age X25519 Identity
	ageIdentity, ierr := age.GenerateX25519Identity()
	if ierr != nil {
		err = wrapErr(ierr)
		return
	}

	ageScryptRecipient, ierr := age.NewScryptRecipient(password)
	if ierr != nil {
		err = wrapErr(ierr)
		return
	}

	a := armor.NewWriter(w)
	defer func() {
		// make sure to handle the error, if any
		if ierr := a.Close(); ierr != nil {
			err = wrapErr(ierr)
			return
		}
	}()
	e, err := age.Encrypt(a, ageScryptRecipient)
	if err != nil {
		err = wrapErr(ierr)
		return
	}

	data := &bytes.Buffer{}
	fmt.Fprintf(data, "# created: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(data, "# public key: %s\n", ageIdentity.Recipient())
	fmt.Fprintf(data, "%s\n", ageIdentity)

	_, err = e.Write(data.Bytes())
	if err != nil {
		err = wrapErr(ierr)
		return
	}
	err = e.Close()
	if err != nil {
		err = wrapErr(ierr)
		return
	}

	key = &Key{
		ageIdentity: ageIdentity,
	}
	return
}

// LoadKey decrypts an age secret key from the reader r using the provided password
func LoadKey(password string, r io.Reader) (key *Key, err error) {

	wrapErr := func(err error) error {
		return fmt.Errorf("paw: loadkey error: %w", err)
	}

	ageScryptIdentity, ierr := age.NewScryptIdentity(password)
	if ierr != nil {
		err = wrapErr(ierr)
		return
	}

	a := armor.NewReader(r)
	d, ierr := age.Decrypt(a, ageScryptIdentity)
	if ierr != nil {
		err = wrapErr(ierr)
		return
	}

	// Generate the age X25519 Identity
	ageIdentities, ierr := age.ParseIdentities(d)
	if ierr != nil {
		err = wrapErr(ierr)
		return
	}

	if len(ageIdentities) > 1 {
		err = wrapErr(fmt.Errorf("only one identity per file is supported, found %d", len(ageIdentities)))
		return
	}

	ageIdentity, ok := ageIdentities[0].(*age.X25519Identity)
	if !ok {
		err = wrapErr(fmt.Errorf("only *age.X25519Identity are supported, got %T", ageIdentities[0]))
		return
	}

	key = &Key{
		ageIdentity: ageIdentity,
	}
	return
}

func (k *Key) Passphrase(numWords int) (string, error) {
	var words []string
	for i := 0; i < numWords; i++ {
		words = append(words, agepaw.RandomWord())
	}
	return strings.Join(words, "-"), nil
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

	// decode the age identity to be used as secret for HKDF function
	_, data, err := bech32.Decode(k.ageIdentity.String())
	if err != nil {
		panic(fmt.Sprintf("could not decode the age identity %s", err))
	}

	// reader to derive a key
	reader := hkdf.New(sha256.New, data, salt, seeder.Info())
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
	return age.Encrypt(dst, k.ageIdentity.Recipient())
}
