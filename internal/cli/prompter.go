// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"charm.land/huh/v2"
	"golang.org/x/term"

	"go.lucor.dev/paw/internal/paw"
)

type prompter struct {
	input         *bufio.Reader
	terminalInput io.Reader
	output        io.Writer
	interactive   bool
	readPassword  func() ([]byte, error)
	runForm       func(*huh.Form) error
}

// newPrompter creates a prompt reader and writer for an interactive command.
func newPrompter(input io.Reader, output io.Writer) *prompter {
	reader := input
	fd, hasFD := input.(interface{ Fd() uintptr })
	inputIsTerminal := hasFD && term.IsTerminal(int(fd.Fd()))
	if inputIsTerminal {
		// Do not read ahead past a public answer into a later password prompt.
		reader = singleByteReader{input}
	}
	return &prompter{
		input:         bufio.NewReader(reader),
		terminalInput: input,
		output:        output,
		interactive:   inputIsTerminal,
	}
}

type singleByteReader struct{ io.Reader }

// Read returns one byte at a time to prevent buffered text input from consuming a password.
func (r singleByteReader) Read(b []byte) (int, error) {
	if len(b) > 1 {
		b = b[:1]
	}
	return r.Reader.Read(b)
}

// ask displays a prompt and returns the entered line.
func (p *prompter) ask(prompt string) (string, error) {
	var value string
	field := huh.NewInput().Title(safeTerminalText(prompt)).Value(&value)
	if err := p.runHuh(field); err != nil {
		return "", err
	}
	return value, nil
}

// selectOption displays a keyboard-navigable selection from fixed options.
func (p *prompter) selectOption(prompt string, options ...string) (string, error) {
	if len(options) == 0 {
		return "", errors.New("no options available")
	}
	choices := make([]huh.Option[string], 0, len(options))
	for _, option := range options {
		choices = append(choices, huh.NewOption(safeTerminalText(option), option))
	}
	value := options[0]
	field := huh.NewSelect[string]().Title(safeTerminalText(prompt)).Options(choices...).Value(&value)
	if err := p.runHuh(field); err != nil {
		return "", err
	}
	return value, nil
}

// selectOptionValue displays labels while returning their associated values.
func (p *prompter) selectOptionValue(prompt string, labels, values []string) (string, error) {
	if len(labels) == 0 || len(labels) != len(values) {
		return "", errors.New("selection labels and values do not match")
	}
	choices := make([]huh.Option[string], 0, len(labels))
	for index, label := range labels {
		choices = append(choices, huh.NewOption(safeTerminalText(label), values[index]))
	}
	value := values[0]
	field := huh.NewSelect[string]().Title(safeTerminalText(prompt)).Options(choices...).Value(&value)
	if err := p.runHuh(field); err != nil {
		return "", err
	}
	return value, nil
}

// editMultiline edits multiline text in the terminal.
func (p *prompter) editMultiline(prompt, initial string) (string, error) {
	value := initial
	field := huh.NewText().
		Title(safeTerminalText(prompt)).
		Description("Esc or Ctrl+C to cancel").
		CharLimit(0).
		Value(&value).
		ExternalEditor(false)
	// Huh normalizes some characters when it loads the text. Keep the
	// original value if the user submits the displayed text unchanged.
	field.Blur()
	displayed := value
	if err := p.runHuh(field); err != nil {
		return "", err
	}
	if value == displayed {
		return initial, nil
	}
	return value, nil
}

// askWithDefault displays the previous value and keeps it when input is empty.
func (p *prompter) askWithDefault(prompt, old string) (string, error) {
	value := old
	field := huh.NewInput().Title(safeTerminalText(prompt)).Value(&value)
	if err := p.runHuh(field); err != nil {
		return old, err
	}
	return value, nil
}

// askPassword reads a password without echoing it to the terminal.
func (p *prompter) askPassword(prompt string) (string, error) {
	if p.readPassword != nil {
		password, err := p.readPassword()
		return string(password), err
	}
	var password string
	field := huh.NewInput().Title(safeTerminalText(prompt)).EchoMode(huh.EchoModeNone).Value(&password)
	if err := p.runHuh(field); err != nil {
		return "", err
	}
	return password, nil
}

// askPasswordWithConfirm reads a password twice and rejects mismatched values.
func (p *prompter) askPasswordWithConfirm() (string, error) {
	for {
		password, err := p.askPassword("Password")
		if err != nil {
			return "", err
		}
		confirm, err := p.askPassword("Confirm password")
		if err != nil {
			return "", err
		}
		if password == confirm {
			return password, nil
		}
		if _, err := fmt.Fprintln(p.output, "Passwords do not match"); err != nil {
			return "", err
		}
	}
}

// askYesNo prompts for a yes-or-no answer using the provided default.
func (p *prompter) askYesNo(prompt string, defaultYes bool) (bool, error) {
	value := defaultYes
	field := huh.NewConfirm().Title(safeTerminalText(prompt)).Value(&value)
	if err := p.runHuh(field); err != nil {
		return false, err
	}
	return value, nil
}

// askIntWithDefaultAndRange reads an integer, applies a default, and enforces its bounds.
func (p *prompter) askIntWithDefaultAndRange(prompt string, def, min, max int) (int, error) {
	value := strconv.Itoa(def)
	field := huh.NewInput().Title(safeTerminalText(prompt)).Value(&value).Validate(func(input string) error {
		n, err := strconv.Atoi(input)
		if err != nil || n < min || n > max {
			return fmt.Errorf("enter a number between %d and %d", min, max)
		}
		return nil
	})
	if err := p.runHuh(field); err != nil {
		return 0, err
	}
	return strconv.Atoi(value)
}

// askPasswordMode displays available password modes and reads the selected mode.
func (p *prompter) askPasswordMode(prompt string, options []paw.PasswordMode, defaultMode paw.PasswordMode) (paw.PasswordMode, error) {
	if len(options) == 0 {
		return 0, fmt.Errorf("no password modes available")
	}
	value := options[0]
	choices := make([]huh.Option[paw.PasswordMode], 0, len(options))
	for _, mode := range options {
		choices = append(choices, huh.NewOption(mode.String(), mode))
		if mode == defaultMode {
			value = mode
		}
	}
	field := huh.NewSelect[paw.PasswordMode]().Title(safeTerminalText(prompt)).Options(choices...).Value(&value)
	if err := p.runHuh(field); err != nil {
		return 0, err
	}
	return value, nil
}

// runHuh displays one Huh field after checking terminal availability.
func (p *prompter) runHuh(field huh.Field) error {
	if err := p.checkTerminal(); err != nil {
		return err
	}
	form := p.makeForm(huh.NewGroup(field))
	if _, ok := field.(*huh.Text); ok {
		form.WithShowHelp(true)
	}
	var err error
	if p.runForm != nil {
		err = p.runForm(form)
	} else {
		err = form.Run()
	}
	if errors.Is(err, huh.ErrUserAborted) {
		return errCanceled
	}
	return err
}

// checkTerminal reports whether the invocation can display a prompt.
func (p *prompter) checkTerminal() error {
	if !p.interactive {
		return errors.New("interactive prompt requires stdin to be a terminal")
	}
	if os.Getenv("TERM") == "dumb" {
		return errors.New("interactive prompt is not supported when TERM=dumb")
	}
	return nil
}

// makeForm applies the shared prompt configuration.
func (p *prompter) makeForm(groups ...*huh.Group) *huh.Form {
	form := huh.NewForm(groups...)
	form.WithKeyMap(prompterKeyMap())
	form.WithShowHelp(false)
	form.WithInput(p.terminalInput)
	form.WithOutput(p.output)
	form.WithTheme(huh.ThemeFunc(huh.ThemeBase))
	return form
}

// prompterKeyMap configures cancellation and disables the external editor shortcut.
func prompterKeyMap() *huh.KeyMap {
	keymap := huh.NewDefaultKeyMap()
	keymap.Quit.SetKeys("esc", "ctrl+c")
	keymap.Text.Editor.SetEnabled(false)
	return keymap
}
