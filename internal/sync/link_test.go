// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const linkFingerprint = "7a2a9d534f4b092dcc8e5a53f644508ed498fdd0203f387f717c74f8b63c118d"

func TestParseConnectionLink(t *testing.T) {
	for _, host := range []string{"192.168.0.9:61654", "[2001:db8::1]:61654", "paw.example.test:61654"} {
		t.Run(host, func(t *testing.T) {
			link, err := ParseConnectionLink("paw://" + host + "/sync?fp=" + linkFingerprint)
			require.NoError(t, err)
			require.Equal(t, linkFingerprint, link.Fingerprint)
			require.Equal(t, 61654, link.Port)
		})
	}
}

func TestParseConnectionLinkRejectsMalformedValues(t *testing.T) {
	for _, value := range []string{
		"https://paw.example:1/sync?fp=" + linkFingerprint,
		"paw://paw.example/sync?fp=" + linkFingerprint,
		"paw://paw.example:0/sync?fp=" + linkFingerprint,
		"paw://paw.example:65536/sync?fp=" + linkFingerprint,
		"paw://paw.example:1/other?fp=" + linkFingerprint,
		"paw://paw.example:1/sync?fp=" + linkFingerprint + "&x=y",
		"paw://paw.example:1/sync?fp=bad",
		"paw://user@paw.example:1/sync?fp=" + linkFingerprint,
	} {
		t.Run(value, func(t *testing.T) { _, err := ParseConnectionLink(value); require.Error(t, err) })
	}
}
