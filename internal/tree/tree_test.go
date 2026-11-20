// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package tree

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFprint(t *testing.T) {
	node := Node{Value: "paw", Child: []Node{
		{Value: "personal", Child: []Node{{Value: "login"}}},
		{Value: "work", Child: []Node{{Value: "note"}}},
	}}
	var out bytes.Buffer
	require.NoError(t, Fprint(&out, node))
	require.Equal(t, "paw\n├── personal\n│   └── login\n└── work\n    └── note\n", out.String())
	require.ErrorIs(t, Fprint(failingWriter{}, node), io.ErrClosedPipe)
	require.ErrorIs(t, Fprint(&limitWriter{remaining: 5}, node), io.ErrClosedPipe)
}

func TestFprintEscapesTerminalControlCharacters(t *testing.T) {
	var output bytes.Buffer
	err := Fprint(&output, Node{Value: "paw\x1b[31m", Child: []Node{{Value: "item\x07"}}})
	require.NoError(t, err)
	require.Equal(t, "paw\\u001b[31m\n└── item\\u0007\n", output.String())
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type limitWriter struct{ remaining int }

func (w *limitWriter) Write(b []byte) (int, error) {
	if len(b) > w.remaining {
		return 0, io.ErrClosedPipe
	}
	w.remaining -= len(b)
	return len(b), nil
}
