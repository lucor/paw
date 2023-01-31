package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"time"

	sshagent "golang.org/x/crypto/ssh/agent"
	"lucor.dev/paw/internal/paw"
)

// PawSessionExtendedAgent wraps the method for the Paw agent client to handle sessions
type PawSessionExtendedAgent interface {
	Key(vaultName string, sessionID string) (*paw.Key, error)
	Lock(vaultName string) error
	Sessions() ([]Session, error)
	Unlock(vaultName string, key *paw.Key, lifetime time.Duration) (string, error)
}

var _ PawSessionExtendedAgent = &client{}

type client struct {
	sshclient sshagent.ExtendedAgent
}

// NewClient returns a Paw agent client to manage sessions.
// The communication with agent is done using the SSH agent protocol.
func NewClient(socketPath string) (PawSessionExtendedAgent, error) {
	a, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, err
	}

	c := &client{
		sshclient: sshagent.NewClient(a),
	}

	return c, nil
}

// Sessions returns the list of active sessions
func (c *client) Sessions() ([]Session, error) {
	request := bytes.Buffer{}
	request.WriteByte(SessionActionList)

	response, err := c.sshclient.Extension(SessionExtension, request.Bytes())
	if err != nil {
		return nil, err
	}
	sessions := []Session{}
	err = json.Unmarshal(response, &sessions)
	return sessions, err
}

// Lock locks vaultName removing from the agent all the active sessions from the agent
func (c *client) Lock(vaultName string) error {
	request := bytes.Buffer{}
	request.WriteByte(SessionActionLock)
	request.WriteString(vaultName)
	response, err := c.sshclient.Extension(SessionExtension, request.Bytes())
	if err != nil {
		return errors.New(string(response))
	}
	return nil
}

// Key returns a Paw key associated to the vaultName's session from the agent
func (c *client) Key(vaultName string, sessionID string) (*paw.Key, error) {
	session := &Session{
		ID:    sessionID,
		Vault: vaultName,
	}
	payload, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}
	request := bytes.Buffer{}
	request.WriteByte(SessionActionKey)
	request.Write(payload)

	response, err := c.sshclient.Extension(SessionExtension, request.Bytes())
	if err != nil {
		return nil, err
	}

	key := &paw.Key{}
	err = json.Unmarshal(response, key)
	return key, err
}

// Unlock unlocks the vault vaultName and adds a new session to the agent. Lifetime defines the session life, default to forever.
func (c *client) Unlock(vaultName string, key *paw.Key, lifetime time.Duration) (string, error) {
	session := &Session{
		Lifetime: lifetime,
		Key:      key,
		Vault:    vaultName,
	}
	payload, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	request := bytes.Buffer{}
	request.WriteByte(SessionActionUnlock)
	request.Write(payload)

	response, err := c.sshclient.Extension(SessionExtension, request.Bytes())
	if err != nil {
		return "", err
	}
	return string(response), err
}
