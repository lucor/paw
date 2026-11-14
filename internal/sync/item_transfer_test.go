// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"bytes"
	"context"
	"crypto/mldsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

func TestTransferItemValidationAndExpectedState(t *testing.T) {
	store, vault := pairingVault(t)
	defer store.Close()
	item, err := model.NewNote("Sync", "original")
	require.NoError(t, err)
	hash, err := item.StateHash()
	require.NoError(t, err)
	require.NoError(t, validateTransferItem(item, item.ID, hash))
	require.Error(t, validateTransferItem(item, "other", hash))
	require.Error(t, validateTransferItem(item, item.ID, hashByte(1)))
	absent, err := checkedItem(context.Background(), store, vault, item.ID, "")
	require.NoError(t, err)
	require.Nil(t, absent)
	require.NoError(t, store.StoreItem(vault, item))
	loaded, err := checkedItem(context.Background(), store, vault, item.ID, hash)
	require.NoError(t, err)
	require.Equal(t, item.ID, loaded.ID)
	_, err = checkedItem(context.Background(), store, vault, item.ID, "")
	require.ErrorContains(t, err, "rebuild")
	_, err = checkedItem(context.Background(), store, vault, item.ID, hashByte(2))
	require.ErrorContains(t, err, "rebuild")
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = checkedItem(canceled, store, vault, item.ID, hash)
	require.ErrorIs(t, err, context.Canceled)
	fileID, err := model.NewFileID()
	require.NoError(t, err)
	digest := sha256.Sum256(nil)
	require.NoError(t, item.AddFile(cxf.File{
		Type:          cxf.CredentialTypeFile,
		ID:            fileID,
		Name:          "attachment",
		IntegrityHash: base64.RawURLEncoding.EncodeToString(digest[:]),
	}))
	hash, err = item.StateHash()
	require.NoError(t, err)
	require.NoError(t, validateTransferItem(item, item.ID, hash))
	history := model.History{}
	require.NoError(t, history.AddSnapshot(item, 1))
	require.NoError(t, item.RemoveFile(fileID))
	require.NoError(t, item.SetHistory(history))
	hash, err = item.StateHash()
	require.NoError(t, err)
	require.NoError(t, validateTransferItem(item, item.ID, hash))
	missingStore, missingVault := pairingVault(t)
	defer missingStore.Close()
	missingHistoryItem, err := model.NewNote("missing", "attachment reference")
	require.NoError(t, err)
	file := cxf.File{
		Type:          cxf.CredentialTypeFile,
		ID:            fileID,
		Name:          "attachment",
		IntegrityHash: base64.RawURLEncoding.EncodeToString(digest[:]),
	}
	require.NoError(t, missingHistoryItem.AddFile(file))
	require.NoError(t, missingStore.StoreItem(missingVault, missingHistoryItem))
	valid, err := missingStore.VerifyAttachment(missingVault, missingHistoryItem.ID, file)
	require.NoError(t, err)
	require.False(t, valid)
}

func TestServeItemRejectsTrustedPeerWhileAnotherIsActive(t *testing.T) {
	store, vault := pairingVault(t)
	defer store.Close()
	activeIdentity := testSyncIdentity(t)
	requestIdentity := testSyncIdentity(t)
	for _, identity := range []*mldsa.PrivateKey{activeIdentity, requestIdentity} {
		require.NoError(t, store.AddSyncPeer(vault, storage.SyncPeer{
			PublicKey: identity.PublicKey().Bytes(),
			Name:      "Trusted device",
		}))
	}
	server := newPairingServer(store, vault, "Local", testSyncIdentity(t).PublicKey())
	require.True(t, server.beginSync(activeIdentity.PublicKey(), "Active device"))
	certificate, err := sessionCertificate(requestIdentity)
	require.NoError(t, err)
	peerCertificate, err := x509.ParseCertificate(certificate.Certificate[0])
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, itemPath, nil)
	request.TLS = &tls.ConnectionState{
		Version:          tls.VersionTLS13,
		CurveID:          tls.X25519MLKEM768,
		PeerCertificates: []*x509.Certificate{peerCertificate},
	}
	response := httptest.NewRecorder()
	server.serveItem(response, request)
	require.Equal(t, http.StatusConflict, response.Code)
}

func TestServeAttachmentRejectsOversizedUpload(t *testing.T) {
	store, vault := pairingVault(t)
	defer store.Close()
	peerIdentity := testSyncIdentity(t)
	require.NoError(t, store.AddSyncPeer(vault, storage.SyncPeer{
		PublicKey: peerIdentity.PublicKey().Bytes(),
		Name:      "Trusted device",
	}))
	server := newPairingServer(store, vault, "Local", testSyncIdentity(t).PublicKey())
	item, err := model.NewNote("Sync", "attachment upload")
	require.NoError(t, err)
	fileID, err := model.NewFileID()
	require.NoError(t, err)
	digest := sha256.Sum256([]byte("x"))
	file := cxf.File{
		Type:          cxf.CredentialTypeFile,
		ID:            fileID,
		Name:          "attachment",
		DecryptedSize: 1,
		IntegrityHash: base64.RawURLEncoding.EncodeToString(digest[:]),
	}
	request := httptest.NewRequest(http.MethodPut, attachmentURL("https://peer.test", file, item.ID), bytes.NewReader([]byte("xx")))
	certificate, err := sessionCertificate(peerIdentity)
	require.NoError(t, err)
	peerCertificate, err := x509.ParseCertificate(certificate.Certificate[0])
	require.NoError(t, err)
	request.TLS = &tls.ConnectionState{
		Version:          tls.VersionTLS13,
		CurveID:          tls.X25519MLKEM768,
		PeerCertificates: []*x509.Certificate{peerCertificate},
	}
	response := httptest.NewRecorder()
	server.serveAttachment(response, request)
	require.Equal(t, http.StatusConflict, response.Code)
	stored, err := store.VerifyAttachment(vault, item.ID, file)
	require.NoError(t, err)
	require.False(t, stored)
}

func TestAttachmentTimeoutScalesWithSize(t *testing.T) {
	const maxDuration = time.Duration(math.MaxInt64)
	maxSeconds := uint64((maxDuration - attachmentBaseTimeout) / time.Second)
	maxSafeTimeout := attachmentBaseTimeout + time.Duration(maxSeconds)*time.Second
	cases := []struct {
		name string
		size uint64
		want time.Duration
	}{
		{name: "no body", size: 0, want: attachmentBaseTimeout},
		{name: "one mebibyte", size: 1 << 20, want: attachmentBaseTimeout + time.Second},
		{name: "sixty-four mebibytes", size: 64 << 20, want: attachmentBaseTimeout + 64*time.Second},
		{
			name: "largest unclamped timeout",
			size: maxSeconds * attachmentMinRate,
			want: maxSafeTimeout,
		},
		{
			name: "first clamped timeout",
			size: (maxSeconds + 1) * attachmentMinRate,
			want: maxSafeTimeout,
		},
		{name: "maximum signed size", size: uint64(math.MaxInt64), want: maxSafeTimeout},
		{name: "maximum unsigned size", size: ^uint64(0), want: maxSafeTimeout},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := attachmentTimeout(tc.size)
			require.Equal(t, tc.want, got)
			require.Greater(t, got, time.Duration(0))
		})
	}
}

func TestAttachmentClientUsesSizeDerivedTimeout(t *testing.T) {
	client, err := NewClient(testSyncIdentity(t))
	require.NoError(t, err)
	defer client.CloseIdleConnections()
	public := testSyncIdentity(t).PublicKey()
	pinned, transport := client.attachmentClient(public, 2<<20)
	defer transport.CloseIdleConnections()
	require.Equal(t, attachmentTimeout(2<<20), pinned.Timeout)
	require.NotSame(t, client.http.Transport, pinned.Transport)
}
