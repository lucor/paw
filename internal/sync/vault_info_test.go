// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"crypto/tls"
	"crypto/x509"
	json "encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/storage"
)

func TestVaultInfoRequiresTrustedPeer(t *testing.T) {
	store, vault := pairingVault(t)
	defer store.Close()
	serverKey := testSyncIdentity(t)
	clientKey := testSyncIdentity(t)
	certificate, err := sessionCertificate(clientKey)
	require.NoError(t, err)
	peerCertificate, err := x509.ParseCertificate(certificate.Certificate[0])
	require.NoError(t, err)
	server := newPairingServer(store, vault, "Source", serverKey.PublicKey())
	server.vaultRef = VaultReference{Name: "Personal", ID: vault.Accounts[0].ID}
	httpServer, err := newServer(serverKey, server)
	require.NoError(t, err)
	mux := httpServer.http.Handler
	request := httptest.NewRequest(http.MethodGet, vaultInfoPath, nil)
	request.TLS = &tls.ConnectionState{
		Version:          tls.VersionTLS13,
		CurveID:          tls.X25519MLKEM768,
		PeerCertificates: []*x509.Certificate{peerCertificate},
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	require.Equal(t, http.StatusForbidden, response.Code)
	require.NoError(t, store.AddSyncPeer(vault, storage.SyncPeer{
		PublicKey: clientKey.PublicKey().Bytes(),
		Name:      "Destination",
	}))
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	var got VaultReference
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &got))
	require.Equal(t, server.vaultRef, got)
}
