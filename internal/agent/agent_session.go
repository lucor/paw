// SPDX-FileCopyrightText: 2023-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later


package agent

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"lucor.dev/paw/internal/age/bech32"
	"lucor.dev/paw/internal/paw"
)

const (
	// SessionExtension is the Session Extension type for the Paw Agent
	SessionExtension = "session@paw"

	// SessionIDPrefix is the prefix of the Paw Session ID
	SessionIDPrefix = "PAW-SID-"
)

const (
	SessionActionLock uint8 = iota
	SessionActionUnlock
	SessionActionKey
	SessionActionList
)

// Session is the payload used to perform agent's requests
type Session struct {
	ID       string
	Lifetime time.Duration
	Key      *paw.Key
	Vault    string
}

// processSessionRequest process the custom agent request.
// Note: we are using the sshagent server implentation that always returns a SSH_AGENT_EXTENSION_FAILURE in accord to SSH PROTOCOL spec.
// So even if we return a detailed error, the client will always receive a "generic extension failure" error.
// See https://cs.opensource.google/go/x/crypto/+/refs/tags/v0.5.0:ssh/agent/server.go;l=183
func (a *Agent) processSessionRequest(contents []byte) ([]byte, error) {
	action := contents[0]
	data := contents[1:]

	switch action {
	case SessionActionKey:
		request := &Session{}
		err := json.Unmarshal(data, request)
		if err != nil {
			return nil, err
		}

		a.mu.Lock()
		defer a.mu.Unlock()
		session, ok := a.sessions[request.ID]
		if !ok {
			return nil, fmt.Errorf("session invalid")
		}
		if session.expire != nil && session.expire.Before(time.Now().UTC()) {
			// session expired
			delete(a.sessions, session.id)
			return nil, fmt.Errorf("session expired")
		}
		if session.vaultName != request.Vault {
			return nil, fmt.Errorf("session invalid")
		}
		return json.Marshal(session.key)
	case SessionActionList:
		sessions := []Session{}
		a.mu.Lock()
		defer a.mu.Unlock()
		for _, session := range a.sessions {
			v := Session{
				ID:    session.id,
				Vault: session.vaultName,
			}
			if session.expire != nil {
				v.Lifetime = time.Until(*session.expire)
			}
			sessions = append(sessions, v)
		}
		return json.Marshal(sessions)
	case SessionActionLock:
		vaultName := string(data)
		a.mu.Lock()
		defer a.mu.Unlock()
		for sid, session := range a.sessions {
			if session.vaultName == vaultName {
				delete(a.sessions, sid)
			}
		}
		return nil, nil
	case SessionActionUnlock:
		request := &Session{}
		err := json.Unmarshal(data, request)
		if err != nil {
			return nil, err
		}
		if request.Key == nil || request.Vault == "" {
			return nil, fmt.Errorf("invalid request")
		}

		buf := make([]byte, 2)
		if _, err := rand.Read(buf); err != nil {
			return nil, err
		}

		id, err := bech32.Encode(SessionIDPrefix, buf)
		if err != nil {
			return nil, err
		}

		s := session{
			id:        id,
			key:       request.Key,
			vaultName: request.Vault,
		}
		if request.Lifetime > 0 {
			t := time.Now().UTC().Add(request.Lifetime)
			s.expire = &t
		}

		a.mu.Lock()
		a.sessions[id] = s
		a.mu.Unlock()

		return []byte(id), nil
	}
	return nil, fmt.Errorf("invalid action")
}
