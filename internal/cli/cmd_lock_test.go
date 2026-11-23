// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/agent"
	"go.lucor.dev/paw/internal/storage"
)

type lockAgentStub struct {
	agent.PawAgent
	locked string
	err    error
}

func (stub *lockAgentStub) Lock(vaultID string) error {
	stub.locked = vaultID
	return stub.err
}

func TestLockInvalidatesSelectedVaultWithoutPromptOrOutput(t *testing.T) {
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	vault, err := store.CreateVault("Personal", "password")
	require.NoError(t, err)
	agentStub := &lockAgentStub{}
	p := newPrompter(strings.NewReader(""), io.Discard)
	p.readPassword = func() ([]byte, error) {
		t.Fatal("lock must not request the vault password")
		return nil, nil
	}
	base := &command{
		storage:  store,
		prompter: p,
		out:      &bytes.Buffer{},
		errOut:   &bytes.Buffer{},
		agentClient: func(string) (agent.PawAgent, error) {
			return agentStub, nil
		},
	}
	out, stderr, err := executeSubcommand(t, []string{"lock", "Personal"}, newLockCommand, base, "")
	require.NoError(t, err)
	require.Empty(t, out)
	require.Empty(t, stderr)
	require.Equal(t, vault.Accounts[0].ID, agentStub.locked)
}

func TestLockReturnsAgentError(t *testing.T) {
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	_, err = store.CreateVault("Personal", "password")
	require.NoError(t, err)
	wantErr := errors.New("agent lock failed")
	agentStub := &lockAgentStub{err: wantErr}
	base := &command{
		storage: store,
		agentClient: func(string) (agent.PawAgent, error) {
			return agentStub, nil
		},
	}
	_, _, err = executeSubcommand(t, []string{"lock", "Personal"}, newLockCommand, base, "")
	require.ErrorIs(t, err, wantErr)
}
