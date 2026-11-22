// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/agent"
	"go.lucor.dev/paw/internal/storage"
)

type sessionsAgentStub struct {
	agent.PawAgent
	sessions []agent.Session
}

type agentTypeStub struct {
	agent.PawAgent
	typeName agent.Type
}

// agentSessionsFailingWriter records attempts and returns a configured error.
type agentSessionsFailingWriter struct {
	err    error
	writes int
}

// Write records a write attempt and returns the configured error.
func (writer *agentSessionsFailingWriter) Write([]byte) (int, error) {
	writer.writes++
	return 0, writer.err
}

func (stub agentTypeStub) Type() (agent.Type, error) {
	return stub.typeName, nil
}

func (stub sessionsAgentStub) Sessions() ([]agent.Session, error) {
	return stub.sessions, nil
}

func TestAgentCommandKeepsOnlySupportedSubcommands(t *testing.T) {
	cmd := newAgentCommand(&command{})
	require.Equal(t, []string{"start", "sessions"}, []string{cmd.Commands[0].Name, cmd.Commands[1].Name})
}

func TestAgentStartReturnsAlreadyRunningErrorWithoutOutput(t *testing.T) {
	store, err := storage.NewFileStorage(filepath.Join(t.TempDir(), "data"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	started := false
	base := &command{
		storage: store,
		agentClient: func(string) (agent.PawAgent, error) {
			return agentTypeStub{typeName: agent.CLI}, nil
		},
		startAgent: func(string) error {
			started = true
			return nil
		},
	}
	out, stderr, err := executeSubcommand(t, []string{"agent", "start"}, newAgentCommand, base, "")
	require.ErrorContains(t, err, "agent of type CLI is already running")
	require.Empty(t, out)
	require.Empty(t, stderr)
	require.False(t, started)
}

func TestAgentStartRunsInForegroundAndReturnsStartError(t *testing.T) {
	store, err := storage.NewFileStorage(filepath.Join(t.TempDir(), "data"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	startFailure := errors.New("agent stopped")
	started := false
	base := &command{
		storage: store,
		agentClient: func(string) (agent.PawAgent, error) {
			return nil, errors.New("agent unavailable")
		},
		startAgent: func(socketPath string) error {
			started = true
			require.Equal(t, store.SocketAgentPath(), socketPath)
			return startFailure
		},
	}
	out, stderr, err := executeSubcommand(t, []string{"agent", "start"}, newAgentCommand, base, "")
	require.ErrorIs(t, err, startFailure)
	require.Empty(t, out)
	require.Empty(t, stderr)
	require.True(t, started)
}

func TestAgentSessionsOutput(t *testing.T) {
	store, err := storage.NewFileStorage(filepath.Join(t.TempDir(), "data"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	vaultID := testVaultID(7, 1)
	base := &command{
		storage: store,
		agentClient: func(string) (agent.PawAgent, error) {
			return sessionsAgentStub{sessions: []agent.Session{{
				ID:       "session-id",
				Vault:    vaultID,
				Lifetime: 2*time.Minute + 400*time.Millisecond,
			}}}, nil
		},
	}
	out, stderr, err := executeSubcommand(t, []string{"agent", "sessions"}, newAgentCommand, base, "")
	require.NoError(t, err)
	require.Empty(t, stderr)
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	require.Len(t, lines, 2)
	require.Equal(t, []string{"Session", "ID", "Vault", "ID", "Lifetime"}, strings.Fields(lines[0]))
	prefix := storage.FormatVaultID(vaultID)[:7]
	require.Equal(t, []string{"session-id", prefix, "2m0s"}, strings.Fields(lines[1]))
	require.Equal(t, strings.Index(lines[0], "Session ID"), strings.Index(lines[1], "session-id"))
	require.Equal(t, strings.Index(lines[0], "Vault ID"), strings.Index(lines[1], prefix))
	require.Equal(t, strings.Index(lines[0], "Lifetime"), strings.Index(lines[1], "2m0s"))
	require.NotContains(t, out, "---")
	require.NotContains(t, out, "|")
}

func TestAgentSessionsWithNoSessionsHasNoOutput(t *testing.T) {
	store, err := storage.NewFileStorage(filepath.Join(t.TempDir(), "data"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	base := &command{
		storage: store,
		agentClient: func(string) (agent.PawAgent, error) {
			return sessionsAgentStub{}, nil
		},
	}
	out, stderr, err := executeSubcommand(t, []string{"agent", "sessions"}, newAgentCommand, base, "")
	require.NoError(t, err)
	require.Empty(t, out)
	require.Empty(t, stderr)
}

func TestAgentSessionsReturnsFlushError(t *testing.T) {
	store, err := storage.NewFileStorage(filepath.Join(t.TempDir(), "data"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	flushFailure := errors.New("output failed")
	writer := &agentSessionsFailingWriter{err: flushFailure}
	base := &command{
		storage: store,
		out:     writer,
		agentClient: func(string) (agent.PawAgent, error) {
			return sessionsAgentStub{sessions: []agent.Session{{
				ID:    "session-id",
				Vault: testVaultID(8, 1),
			}}}, nil
		},
	}
	cmd := &AgentCmd{command: base, agentCommand: agentSessionsSubCmd}
	require.ErrorIs(t, cmd.Run(context.Background(), nil), flushFailure)
	require.Equal(t, 1, writer.writes)
}
