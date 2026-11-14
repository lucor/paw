// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package browser

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMessageFromExtensionDoesNotLogInput(t *testing.T) {
	oldLogger := slog.Default()
	t.Cleanup(func() { slog.SetDefault(oldLogger) })
	var output bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))

	input := "chrome-extension://id?credential=private-value"
	require.True(t, MessageFromExtension([]string{input}))
	require.NotContains(t, output.String(), input)
	require.NotContains(t, output.String(), "private-value")
	require.Contains(t, output.String(), `component=browser`)
}

func TestMessageFromExtensionHandlesEmptyInput(t *testing.T) {
	require.False(t, MessageFromExtension(nil))
	require.False(t, MessageFromExtension([]string{nativeMessagingManifestFileName}))
}
