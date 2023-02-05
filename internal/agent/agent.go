package agent

import (
	"crypto"
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	sshagent "golang.org/x/crypto/ssh/agent"
	"lucor.dev/paw/internal/paw"
)

var ErrOperationUnsupported = errors.New("operation unsupported")

// Type represents the agent type
type Type string

const (
	// CLI represents the agent started in CLI mode
	CLI Type = "CLI"
	// GUI represents the agent started in GUI mode
	GUI = "GUI"
)

func NewCLI() *Agent {
	return &Agent{
		sshagent: sshagent.NewKeyring(),
		sessions: make(map[string]session),
		t:        CLI,
	}
}

func NewGUI() *Agent {
	return &Agent{
		sshagent: sshagent.NewKeyring(),
		sessions: make(map[string]session),
		t:        GUI,
	}
}

type Agent struct {
	t Type

	sshagent sshagent.Agent

	mu       sync.Mutex
	sessions map[string]session
}

type session struct {
	expire    *time.Time
	id        string
	key       *paw.Key
	vaultName string
}

func (a *Agent) AddSSHKey(key crypto.PrivateKey, comment string) error {
	return a.Add(sshagent.AddedKey{
		PrivateKey: key,
		Comment:    comment,
	})
}

func (a *Agent) Close() error {
	return nil
}

/* sshagent.ExtendedAgent implementation */
var _ sshagent.ExtendedAgent = &Agent{}

// Add implements agent.ExtendedAgent
func (a *Agent) Add(key sshagent.AddedKey) error {
	return a.sshagent.Add(key)
}

// List implements agent.ExtendedAgent
func (a *Agent) List() ([]*sshagent.Key, error) {
	return a.sshagent.List()
}

// Lock implements agent.ExtendedAgent
func (a *Agent) Lock(passphrase []byte) error {
	return a.sshagent.Lock(passphrase)
}

// Remove implements agent.ExtendedAgent
func (a *Agent) Remove(key ssh.PublicKey) error {
	return a.sshagent.Remove(key)
}

// RemoveAll implements agent.ExtendedAgent
func (a *Agent) RemoveAll() error {
	return a.sshagent.RemoveAll()
}

// Sign implements agent.ExtendedAgent
func (a *Agent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return a.sshagent.Sign(key, data)
}

// Signers implements agent.ExtendedAgent
func (a *Agent) Signers() ([]ssh.Signer, error) {
	return a.sshagent.Signers()
}

// Unlock implements agent.ExtendedAgent
func (a *Agent) Unlock(passphrase []byte) error {
	return a.sshagent.Unlock(passphrase)
}

func (a *Agent) SignWithFlags(key ssh.PublicKey, data []byte, flags sshagent.SignatureFlags) (*ssh.Signature, error) {
	return nil, ErrOperationUnsupported
}

func (a *Agent) Extension(extensionType string, contents []byte) ([]byte, error) {
	if extensionType == SessionExtension {
		return a.processSessionRequest(contents)
	}
	if extensionType == TypeExtension {
		return a.processTypeRequest(contents)
	}
	return nil, sshagent.ErrExtensionUnsupported
}

func (a *Agent) serveConn(c net.Conn) {
	if err := sshagent.ServeAgent(a, c); err != io.EOF {
		log.Println("Agent client connection ended with error:", err)
	}
}
