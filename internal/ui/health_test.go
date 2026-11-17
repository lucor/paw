// SPDX-FileCopyrightText: 2023-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/storage"
)

func TestHealthServiceCheckWithoutAddress(t *testing.T) {
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.False(t, HealthServiceCheck(store))
	require.NoError(t, store.WriteHealthAddress(""))
	require.False(t, HealthServiceCheck(store))
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
