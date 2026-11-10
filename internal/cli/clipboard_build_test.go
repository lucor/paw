// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"go/build"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestClipboardBuildConstraints checks which clipboard implementation is
// compiled for each target platform.
//
// The Go toolchain satisfies the "linux" build tag on Android and the "darwin"
// build tag on iOS, so the mobile platforms must be excluded explicitly.
// Otherwise the mobile builds would pull in golang.design/x/clipboard and,
// through it, the golang.org/x/mobile dependency that needs a gomobile bridge.
func TestClipboardBuildConstraints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		goos            string
		goarch          string
		wantClipboard   bool
		wantUnsupported bool
	}{
		{
			name:            "macOS",
			goos:            "darwin",
			goarch:          "arm64",
			wantClipboard:   true,
			wantUnsupported: false,
		},
		{
			name:            "Linux",
			goos:            "linux",
			goarch:          "amd64",
			wantClipboard:   true,
			wantUnsupported: false,
		},
		{
			name:            "Windows",
			goos:            "windows",
			goarch:          "amd64",
			wantClipboard:   true,
			wantUnsupported: false,
		},
		{
			name:            "FreeBSD",
			goos:            "freebsd",
			goarch:          "amd64",
			wantClipboard:   true,
			wantUnsupported: false,
		},
		{
			name:            "Android",
			goos:            "android",
			goarch:          "arm64",
			wantClipboard:   false,
			wantUnsupported: true,
		},
		{
			name:            "iOS",
			goos:            "ios",
			goarch:          "arm64",
			wantClipboard:   false,
			wantUnsupported: true,
		},
		{
			name:            "OpenBSD",
			goos:            "openbsd",
			goarch:          "amd64",
			wantClipboard:   false,
			wantUnsupported: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctxt := build.Default
			ctxt.GOOS = tt.goos
			ctxt.GOARCH = tt.goarch

			match, err := ctxt.MatchFile(".", "clipboard.go")
			require.NoError(t, err)
			require.Equal(t, tt.wantClipboard, match)

			match, err = ctxt.MatchFile(".", "clipboard_unsupported.go")
			require.NoError(t, err)
			require.Equal(t, tt.wantUnsupported, match)
		})
	}
}
