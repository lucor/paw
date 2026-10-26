// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"go.lucor.dev/paw/internal/paw"
)

func TestPrompterSequentialAnswers(t *testing.T) {
	var out bytes.Buffer
	p := newPrompter(strings.NewReader("alice\r\n\nmaybe\nY\nabc\n99\n8\n-1\n1\nlast"), &out)
	value, err := p.ask("Username")
	require.NoError(t, err)
	require.Equal(t, "alice", value)
	value, err = p.askWithDefault("Note", "old note")
	require.NoError(t, err)
	require.Equal(t, "old note", value)
	yes, err := p.askYesNo("Continue?", false)
	require.NoError(t, err)
	require.True(t, yes)
	n, err := p.askIntWithDefaultAndRange("Length", 12, 6, 64)
	require.NoError(t, err)
	require.Equal(t, 8, n)
	mode, err := p.askPasswordMode("Mode", []paw.PasswordMode{paw.RandomPassword, paw.PinPassword}, paw.RandomPassword)
	require.NoError(t, err)
	require.Equal(t, paw.PinPassword, mode)
	value, err = p.ask("Last")
	require.NoError(t, err)
	require.Equal(t, "last", value)
	_, err = p.ask("End")
	require.ErrorIs(t, err, io.EOF)
	require.Contains(t, out.String(), "Username: Note [old note]: Continue? [y/N]: ")
}

func TestPrompterDefaultsAndEOF(t *testing.T) {
	for _, input := range []string{"", "invalid\n"} {
		t.Run("confirm/"+input, func(t *testing.T) {
			p := newPrompter(strings.NewReader(input), io.Discard)
			_, err := p.askYesNo("Continue?", true)
			require.ErrorIs(t, err, io.EOF)
		})
		t.Run("length/"+input, func(t *testing.T) {
			p := newPrompter(strings.NewReader(input), io.Discard)
			_, err := p.askIntWithDefaultAndRange("Length", 12, 6, 64)
			require.ErrorIs(t, err, io.EOF)
		})
		t.Run("mode/"+input, func(t *testing.T) {
			p := newPrompter(strings.NewReader(input), io.Discard)
			_, err := p.askPasswordMode("Mode", []paw.PasswordMode{paw.RandomPassword}, paw.RandomPassword)
			require.ErrorIs(t, err, io.EOF)
		})
	}
	p := newPrompter(strings.NewReader("\n\n\n\n"), io.Discard)
	yes, err := p.askYesNo("Continue?", true)
	require.NoError(t, err)
	require.True(t, yes)
	yes, err = p.askYesNo("Continue?", false)
	require.NoError(t, err)
	require.False(t, yes)
	n, err := p.askIntWithDefaultAndRange("Length", 12, 6, 64)
	require.NoError(t, err)
	require.Equal(t, 12, n)
	mode, err := p.askPasswordMode("Mode", []paw.PasswordMode{paw.PinPassword}, paw.RandomPassword)
	require.NoError(t, err)
	require.Equal(t, paw.PinPassword, mode)
	_, err = p.askPasswordMode("Mode", nil, paw.RandomPassword)
	require.Error(t, err)
}

func TestPasswordConfirmation(t *testing.T) {
	var out bytes.Buffer
	p := newPrompter(strings.NewReader(""), &out)
	answers := []string{"first-secret", "different-secret", "confirmed-secret", "confirmed-secret"}
	p.readPassword = func() ([]byte, error) {
		if len(answers) == 0 {
			return nil, io.EOF
		}
		answer := answers[0]
		answers = answers[1:]
		return []byte(answer), nil
	}
	password, err := p.askPasswordWithConfirm()
	require.NoError(t, err)
	require.Equal(t, "confirmed-secret", password)
	require.Contains(t, out.String(), "Passwords do not match")
	require.NotContains(t, out.String(), "secret")
	_, err = p.askPasswordWithConfirm()
	require.ErrorIs(t, err, io.EOF)
}

func TestPasswordRejectsNonTerminalInput(t *testing.T) {
	input := strings.NewReader("plaintext-secret\n")
	var out bytes.Buffer
	p := newPrompter(input, &out)
	_, err := p.askPassword("Password")
	require.ErrorContains(t, err, "not a terminal")
	require.Equal(t, len("plaintext-secret\n"), input.Len())
	require.NotContains(t, out.String(), "plaintext-secret")
}

func TestTerminalTextDoesNotReadAheadIntoPassword(t *testing.T) {
	input := strings.NewReader("alice\nsecret\n")
	p := &prompter{input: bufio.NewReader(singleByteReader{input}), output: io.Discard}
	name, err := p.ask("Name")
	require.NoError(t, err)
	require.Equal(t, "alice", name)
	remaining, err := io.ReadAll(input)
	require.NoError(t, err)
	require.Equal(t, "secret\n", string(remaining))
}

type failingInput struct{ err error }

func (r failingInput) Read([]byte) (int, error) { return 0, r.err }

type failingOutput struct{ err error }

func (w failingOutput) Write([]byte) (int, error) { return 0, w.err }

func TestPrompterIOErrors(t *testing.T) {
	failure := errors.New("I/O failed")
	p := newPrompter(failingInput{failure}, io.Discard)
	_, err := p.ask("Name")
	require.ErrorIs(t, err, failure)
	p = newPrompter(strings.NewReader("answer\n"), failingOutput{failure})
	_, err = p.ask("Name")
	require.ErrorIs(t, err, failure)
	p = newPrompter(strings.NewReader(""), io.Discard)
	p.readPassword = func() ([]byte, error) { return nil, failure }
	_, err = p.askPassword("Password")
	require.ErrorIs(t, err, failure)
}
