// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/ui"
)

func TestDefaultWindowDimensions(t *testing.T) {
	require.Equal(t, 400, ui.DefaultWindowWidth)
	require.Equal(t, 640, ui.DefaultWindowHeight)
}
