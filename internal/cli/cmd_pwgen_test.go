// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/paw"
)

func TestPwgenNonInteractiveModes(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		pattern    string
		wantLength int
	}{
		{name: "random default", args: []string{"pwgen", "random"}, wantLength: paw.RandomPasswordDefaultLength},
		{name: "random minimum", args: []string{"pwgen", "random", "--length", "6"}, wantLength: 6},
		{name: "random maximum", args: []string{"pwgen", "random", "--length", "64"}, wantLength: 64},
		{name: "random custom without symbols", args: []string{"pwgen", "random", "--length", "24", "--no-symbols"}, pattern: `^[A-Za-z0-9]+$`, wantLength: 24},
		{name: "passphrase default", args: []string{"pwgen", "passphrase"}, wantLength: paw.PassphrasePasswordDefaultLength},
		{name: "passphrase minimum", args: []string{"pwgen", "passphrase", "--words", "2"}, wantLength: 2},
		{name: "passphrase maximum", args: []string{"pwgen", "passphrase", "--words", "12"}, wantLength: 12},
		{name: "passphrase custom", args: []string{"pwgen", "passphrase", "--words", "5"}, wantLength: 5},
		{name: "pin default", args: []string{"pwgen", "pin"}, pattern: `^\d{4}$`, wantLength: paw.PinPasswordDefaultLength},
		{name: "pin minimum", args: []string{"pwgen", "pin", "--length", "4"}, wantLength: 4},
		{name: "pin maximum", args: []string{"pwgen", "pin", "--length", "10"}, wantLength: 10},
		{name: "pin custom", args: []string{"pwgen", "pin", "--length", "6"}, pattern: `^\d{6}$`, wantLength: 6},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out, stderr, err := executeWithInput(t, test.args, nil, "")
			require.NoError(t, err)
			require.Empty(t, stderr)
			lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
			require.Len(t, lines, 1)
			require.Regexp(t, regexp.MustCompile(test.pattern), lines[0])
			if strings.HasPrefix(test.name, "random") || strings.HasPrefix(test.name, "pin") {
				require.Len(t, lines[0], test.wantLength)
			}
			if strings.HasPrefix(test.name, "passphrase") {
				require.Len(t, strings.Split(lines[0], "-"), test.wantLength)
			}
		})
	}
}

func TestPwgenSubcommandValidation(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"pwgen", "random", "--length", "5"}, "--length must be between 6 and 64"},
		{[]string{"pwgen", "random", "--length", "65"}, "--length must be between 6 and 64"},
		{[]string{"pwgen", "random", "--words", "5"}, "flag provided but not defined: -words"},
		{[]string{"pwgen", "random", "extra"}, "unexpected arguments"},
		{[]string{"pwgen", "passphrase", "--words", "1"}, "--words must be between 2 and 12"},
		{[]string{"pwgen", "passphrase", "--words", "13"}, "--words must be between 2 and 12"},
		{[]string{"pwgen", "passphrase", "--length", "5"}, "flag provided but not defined: -length"},
		{[]string{"pwgen", "passphrase", "extra"}, "unexpected arguments"},
		{[]string{"pwgen", "pin", "--length", "3"}, "--length must be between 4 and 10"},
		{[]string{"pwgen", "pin", "--length", "11"}, "--length must be between 4 and 10"},
		{[]string{"pwgen", "pin", "--words", "4"}, "flag provided but not defined: -words"},
		{[]string{"pwgen", "pin", "extra"}, "unexpected arguments"},
		{[]string{"pwgen", "custom"}, "unexpected arguments"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, "_"), func(t *testing.T) {
			out, stderr, err := executeWithInput(t, test.args, nil, "")
			require.Error(t, err)
			require.Contains(t, err.Error(), test.want)
			require.Empty(t, out)
			require.Empty(t, stderr)
		})
	}
}

func TestPwgenWriteError(t *testing.T) {
	writeErr := errors.New("stdout unavailable")
	base := &command{out: failingPwgenWriter{err: writeErr}, errOut: io.Discard}
	cmd := newPwGenCommand(base)
	cmd.ExitErrHandler = leaveExitToCaller
	cmd.Reader = strings.NewReader("")
	cmd.Writer = base.out
	cmd.ErrWriter = base.errOut
	err := cmd.Run(context.Background(), []string{"pwgen", "pin"})
	require.ErrorIs(t, err, writeErr)
}

func TestPwgenInteractiveFlowStillUsesHuh(t *testing.T) {
	out, stderr, err := executeWithInput(t, []string{"pwgen"}, nil, "2\n4\n")
	require.ErrorContains(t, err, "requires stdin to be a terminal")
	require.Empty(t, out)
	require.Empty(t, stderr)
}

func TestPwgenRandomGeneratorUsesDefaultFormat(t *testing.T) {
	password, err := generateRandomPassword(paw.RandomPasswordDefaultLength, true)
	require.NoError(t, err)
	require.Equal(t, paw.RandomPassword, password.Mode)
	require.Equal(t, paw.RandomPasswordDefaultFormat, password.Format)
	require.Len(t, password.Value, paw.RandomPasswordDefaultLength)
}

func TestPwgenClipboardForAllModes(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		match *regexp.Regexp
	}{
		{name: "random long flag", args: []string{"pwgen", "random", "--clip"}, match: regexp.MustCompile(`^.{16}$`)},
		{name: "passphrase inherited short flag", args: []string{"pwgen", "-c", "passphrase"}, match: regexp.MustCompile(`^[^-]+(-[^-]+){3}$`)},
		{name: "pin short flag", args: []string{"pwgen", "pin", "-c"}, match: regexp.MustCompile(`^\d{4}$`)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var copied string
			base := &command{clipboard: func(_ context.Context, data []byte) error {
				copied = string(data)
				return nil
			}}
			out, stderr, err := executeSubcommand(t, test.args, newPwGenCommand, base, "")
			require.NoError(t, err)
			require.Empty(t, out)
			require.Equal(t, "Password copied to clipboard\n", stderr)
			require.Regexp(t, test.match, copied)
		})
	}
}

func TestPwgenInteractiveClipboard(t *testing.T) {
	var copied string
	base := &command{
		prompter: testInteractivePrompter(t, "1\n16\ny\n"),
		clipboard: func(_ context.Context, data []byte) error {
			copied = string(data)
			return nil
		},
	}
	out, stderr, err := executeSubcommand(t, []string{"pwgen", "--clip"}, newPwGenCommand, base, "")
	require.NoError(t, err)
	require.Empty(t, out)
	require.Equal(t, "Password copied to clipboard\n", stderr)
	require.Len(t, copied, paw.RandomPasswordDefaultLength)
}

func TestPwgenClipboardErrorsAreReturned(t *testing.T) {
	clipboardErr := errors.New("clipboard unavailable")
	clipboard := func(context.Context, []byte) error { return clipboardErr }
	base := &command{clipboard: clipboard}
	out, stderr, err := executeSubcommand(t, []string{"pwgen", "random", "--clip"}, newPwGenCommand, base, "")
	require.ErrorIs(t, err, clipboardErr)
	require.Empty(t, out)
	require.Empty(t, stderr)

	t.Run("interactive", func(t *testing.T) {
		base := &command{
			prompter:  testInteractivePrompter(t, "1\n16\ny\n"),
			clipboard: clipboard,
		}
		out, stderr, err := executeSubcommand(t, []string{"pwgen", "--clip"}, newPwGenCommand, base, "")
		require.ErrorIs(t, err, clipboardErr)
		require.Empty(t, out)
		require.Empty(t, stderr)
	})
}

type failingPwgenWriter struct {
	err error
}

// Write returns the configured error for password output tests.
func (writer failingPwgenWriter) Write([]byte) (int, error) {
	return 0, writer.err
}
