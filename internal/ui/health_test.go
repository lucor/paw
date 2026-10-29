// SPDX-FileCopyrightText: 2023-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHealthServiceCheckWithoutAddress(t *testing.T) {
	lockFile := filepath.Join(t.TempDir(), "paw.lock")
	require.False(t, HealthServiceCheck(lockFile))
	require.NoError(t, os.WriteFile(lockFile, nil, 0o600))
	require.False(t, HealthServiceCheck(lockFile))
}

func TestHealthServiceCheckConn(t *testing.T) {
	client, server := net.Pipe()
	go handleConnection(server)
	require.True(t, healthServiceCheckConn(client))

	client, server = net.Pipe()
	go func() {
		defer server.Close()
		_, _ = server.Write([]byte("other\n"))
	}()
	require.False(t, healthServiceCheckConn(client))
}
