// SPDX-FileCopyrightText: 2023-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later


package agent

import (
	"bytes"
	"crypto"
	"encoding/json"
	"errors"
	"time"

	"golang.org/x/crypto/ssh"
	sshagent "golang.org/x/crypto/ssh/agent"
	"lucor.dev/paw/internal/paw"
)

const (
	dialTimeout = 100 * time.Millisecond
)

type PawAgent interface {
	SSHAgent
	PawSessionExtendedAgent
	PawTypeExtendedAgent
}

// SSHAgent wraps the method for the Paw agent client to handle SSH keys
type SSHAgent interface {
	AddSSHKey(key crypto.PrivateKey, comment string) error
	RemoveSSHKey(key ssh.PublicKey) error
}

// PawSessionExtendedAgent wraps the method for the Paw agent client to handle sessions
type PawSessionExtendedAgent interface {
	Key(vaultName string, sessionID string) (*paw.Key, error)
	Lock(vaultName string) error
	Sessions() ([]Session, error)
	Unlock(vaultName string, key *paw.Key, lifetime time.Duration) (string, error)
}

// PawSessionExtendedAgent wraps the method for the Paw agent client to handle sessions
type PawTypeExtendedAgent interface {
	Type() (Type, error)
}

var _ PawAgent = &client{}

type client struct {
	sshclient sshagent.ExtendedAgent
}

// NewClient returns a Paw agent client to manage sessions and SSH keys
// The communication with agent is done using the SSH agent protocol.
func NewClient(socketPath string) (PawAgent, error) {
	a, err := dialWithTimeout(socketPath, dialTimeout)
	if err != nil {
		return nil, err
	}

	c := &client{
		sshclient: sshagent.NewClient(a),
	}

	return c, nil
}

// AddSSHKey adds an SSH key to agent along with a comment
func (c *client) AddSSHKey(key crypto.PrivateKey, comment string) error {
	return c.sshclient.Add(sshagent.AddedKey{
		PrivateKey: key,
		Comment:    comment,
	})
}

// RemoveSSHKey removes an SSH key from the agent
func (c *client) RemoveSSHKey(key ssh.PublicKey) error {
	return c.sshclient.Remove(key)
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

// Type implements PawAgent
func (c *client) Type() (Type, error) {
	response, err := c.sshclient.Extension(TypeExtension, nil)
	if err != nil {
		return "", err
	}
	return Type(response), err
}
