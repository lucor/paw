// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/stretchr/testify/require"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

func TestPrompterRequiresTerminalForPrompt(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	input := strings.NewReader("answer\n")
	var output bytes.Buffer
	p := newPrompter(input, &output)

	_, err := p.ask("Username")
	require.ErrorContains(t, err, "requires stdin to be a terminal")
	require.Equal(t, len("answer\n"), input.Len(), "a prompt must not consume piped input")
	require.Empty(t, output.String())
}

func TestRemoveChecksTerminalBeforeAction(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	base, vault := addTestCommand(t)
	item, err := model.NewNote("example", "content")
	require.NoError(t, err)
	require.NoError(t, base.storage.StoreItem(vault, item))
	base.prompter = newPrompter(strings.NewReader("y\n"), io.Discard)
	_, _, err = executeSubcommand(t, []string{"rm", "--session", "active", "Personal/note/example"}, newRemoveCommand, base, "")
	require.ErrorContains(t, err, "requires stdin to be a terminal")
}

func TestPrompterRejectsTermDumb(t *testing.T) {
	t.Setenv("TERM", "dumb")
	p := newPrompter(strings.NewReader("answer\n"), io.Discard)
	p.interactive = true
	var output bytes.Buffer
	p.output = &output

	_, err := p.ask("Username")
	require.ErrorContains(t, err, "not supported when TERM=dumb")
	require.Empty(t, output.String())
}

func TestPrompterHuhInputAndDefault(t *testing.T) {
	p := testInteractivePrompter(t, "alice\n\n")
	value, err := p.ask("Username")
	require.NoError(t, err)
	require.Equal(t, "alice", value)
	value, err = p.askWithDefault("Note", "old note")
	require.NoError(t, err)
	require.Equal(t, "old note", value)
}

func TestPrompterHonorsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	p := testInteractivePrompter(t, "value\n")
	var output bytes.Buffer
	p.output = &output
	_, err := p.ask("Name")
	require.NoError(t, err)
	require.NotContains(t, output.String(), "\x1b[")
}

func TestPrompterHuhConfirmAndSelection(t *testing.T) {
	p := testInteractivePrompter(t, "Y\nn\n2\n")
	yes, err := p.askYesNo("Continue?", false)
	require.NoError(t, err)
	require.True(t, yes)
	yes, err = p.askYesNo("Continue?", true)
	require.NoError(t, err)
	require.False(t, yes)
	mode, err := p.askPasswordMode("Mode", []paw.PasswordMode{paw.RandomPassword, paw.PinPassword}, paw.RandomPassword)
	require.NoError(t, err)
	require.Equal(t, paw.PinPassword, mode)
}

func TestPrompterSelectOption(t *testing.T) {
	p := testInteractivePrompter(t, "2\n")
	value, err := p.selectOption("Edit item", "Standard fields", "Custom fields", "Tags")
	require.NoError(t, err)
	require.Equal(t, "Custom fields", value)

	input := strings.NewReader("1\n")
	p = newPrompter(input, io.Discard)
	_, err = p.selectOption("Edit item", "Standard fields", "Custom fields")
	require.ErrorContains(t, err, "requires stdin to be a terminal")
	require.Equal(t, len("1\n"), input.Len())
}

func TestPrompterHuhIntegerValidation(t *testing.T) {
	p := testInteractivePrompter(t, "abc\n8\n")
	n, err := p.askIntWithDefaultAndRange("Length", 12, 6, 64)
	require.NoError(t, err)
	require.Equal(t, 8, n)
}

func TestPrompterHuhCancellation(t *testing.T) {
	for name, key := range map[string]tea.Key{
		"escape": {Code: tea.KeyEscape},
		"ctrl+c": {Code: 'c', Mod: tea.ModCtrl},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("TERM", "xterm-256color")
			p := newPrompter(strings.NewReader(""), io.Discard)
			form := p.makeForm(huh.NewGroup(huh.NewInput()))
			form.CancelCmd = tea.Interrupt
			updated, cancel := form.Update(tea.KeyPressMsg(key))
			require.Equal(t, huh.StateAborted, updated.(*huh.Form).State)
			require.NotNil(t, cancel)
		})
	}
}

// testInteractivePrompter supplies scripted answers and an optional replacement for multiline text.
func testInteractivePrompter(t *testing.T, input string, multiline ...string) *prompter {
	t.Helper()
	t.Setenv("TERM", "xterm-256color")
	p := newPrompter(strings.NewReader(input), io.Discard)
	p.terminalInput = singleByteReader{p.terminalInput}
	p.interactive = true
	// Command tests use scripted answers to exercise the command flow.
	p.runForm = func(form *huh.Form) error {
		if field, ok := form.GetFocusedField().(*huh.Text); ok {
			require.Len(t, multiline, 1)
			form.Init()
			form.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnd, Mod: tea.ModCtrl}))
			for range field.GetValue().(string) {
				form.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
			}
			form.Update(tea.PasteMsg{Content: multiline[0]})
			form.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
			return nil
		}
		return form.WithAccessible(true).Run()
	}
	return p
}

func TestPrompterEditsMultilineTextWithoutTrimming(t *testing.T) {
	initial := "  first line  \nsecond line\n\n" + strings.Repeat("x", 5000)
	p := testInteractivePrompter(t, "")
	p.runForm = func(form *huh.Form) error {
		form.Init()
		require.Contains(t, form.View(), "new line")
		require.Contains(t, form.View(), "submit")
		require.Contains(t, form.View(), "Esc or Ctrl+C to cancel")
		require.NotContains(t, form.View(), "open editor")
		form.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnd, Mod: tea.ModCtrl}))
		form.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Mod: tea.ModCtrl}))
		form.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
		return nil
	}
	value, err := p.editMultiline("Note", initial)
	require.NoError(t, err)
	require.Equal(t, initial+"\n", value)

	initial = "\tindented\r\n"
	p.runForm = func(form *huh.Form) error {
		form.Init()
		form.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
		return nil
	}
	value, err = p.editMultiline("Note", initial)
	require.NoError(t, err)
	require.Equal(t, initial, value)

	p.runForm = func(form *huh.Form) error {
		form.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
		require.Equal(t, huh.StateAborted, form.State)
		return huh.ErrUserAborted
	}
	value, err = p.editMultiline("Note", initial)
	require.ErrorIs(t, err, errCanceled)
	require.Empty(t, value)
}

func TestPasswordConfirmation(t *testing.T) {
	var out bytes.Buffer
	p := newPrompter(strings.NewReader(""), &out)
	p.output = &out
	answers := []string{"first-secret", "different-secret", "confirmed-secret", "confirmed-secret"}
	p.readPassword = func() ([]byte, error) {
		value := answers[0]
		answers = answers[1:]
		return []byte(value), nil
	}
	password, err := p.askPasswordWithConfirm()
	require.NoError(t, err)
	require.Equal(t, "confirmed-secret", password)
	require.Equal(t, "Passwords do not match\n", out.String())
	require.NotContains(t, out.String(), "secret")
}

func TestPasswordRejectsNonTerminalInput(t *testing.T) {
	input := strings.NewReader("plaintext-secret\n")
	var out bytes.Buffer
	p := newPrompter(input, &out)
	_, err := p.askPassword("Password")
	require.ErrorContains(t, err, "requires stdin to be a terminal")
	require.Equal(t, len("plaintext-secret\n"), input.Len())
	require.NotContains(t, out.String(), "plaintext-secret")
}

func TestTerminalTextDoesNotReadAheadIntoPassword(t *testing.T) {
	input := strings.NewReader("alice\nsecret\n")
	p := testInteractivePrompter(t, "alice\nsecret\n")
	p.terminalInput = singleByteReader{input}
	name, err := p.ask("Name")
	require.NoError(t, err)
	require.Equal(t, "alice", name)
	remaining, err := io.ReadAll(input)
	require.NoError(t, err)
	require.Equal(t, "secret\n", string(remaining))
}
