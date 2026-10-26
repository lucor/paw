// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/term"

	"go.lucor.dev/paw/internal/paw"
)

type prompter struct {
	input        *bufio.Reader
	output       io.Writer
	readPassword func() ([]byte, error)
}

func newPrompter(input io.Reader, output io.Writer) *prompter {
	reader := input
	fd, hasFD := input.(interface{ Fd() uintptr })
	if hasFD && term.IsTerminal(int(fd.Fd())) {
		// Do not read ahead past a public answer into a later password prompt.
		reader = singleByteReader{input}
	}
	return &prompter{
		input:  bufio.NewReader(reader),
		output: output,
		readPassword: func() ([]byte, error) {
			if !hasFD || !term.IsTerminal(int(fd.Fd())) {
				return nil, fmt.Errorf("password input is not a terminal")
			}
			return term.ReadPassword(int(fd.Fd()))
		},
	}
}

type singleByteReader struct{ io.Reader }

func (r singleByteReader) Read(b []byte) (int, error) {
	if len(b) > 1 {
		b = b[:1]
	}
	return r.Reader.Read(b)
}

func (p *prompter) readLine() (string, error) {
	line, err := p.input.ReadString('\n')
	if err != nil && !(err == io.EOF && len(line) > 0) {
		return "", fmt.Errorf("could not read input: %w", err)
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}

func (p *prompter) ask(prompt string) (string, error) {
	if _, err := fmt.Fprintf(p.output, "%s: ", prompt); err != nil {
		return "", err
	}
	return p.readLine()
}

func (p *prompter) askWithDefault(prompt, old string) (string, error) {
	if _, err := fmt.Fprintf(p.output, "%s [%s]: ", prompt, old); err != nil {
		return old, err
	}
	line, err := p.readLine()
	if err != nil || line == "" {
		return old, err
	}
	return line, nil
}

func (p *prompter) askPassword(prompt string) (string, error) {
	if _, err := fmt.Fprintf(p.output, "%s: ", prompt); err != nil {
		return "", err
	}
	password, err := p.readPassword()
	_, writeErr := fmt.Fprintln(p.output)
	if err != nil {
		return "", fmt.Errorf("could not read password: %w", err)
	}
	if writeErr != nil {
		return "", writeErr
	}
	return string(password), nil
}

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
		if _, err := fmt.Fprintln(p.output, "[✗] Passwords do not match"); err != nil {
			return "", err
		}
	}
}

func (p *prompter) askYesNo(prompt string, defaultYes bool) (bool, error) {
	def := "y/N"
	if defaultYes {
		def = "Y/n"
	}
	for {
		if _, err := fmt.Fprintf(p.output, "%s [%s]: ", prompt, def); err != nil {
			return false, err
		}
		line, err := p.readLine()
		if err != nil {
			return false, err
		}
		switch strings.ToLower(line) {
		case "":
			return defaultYes, nil
		case "y":
			return true, nil
		case "n":
			return false, nil
		}
	}
}

func (p *prompter) askIntWithDefaultAndRange(prompt string, def, min, max int) (int, error) {
	for {
		if _, err := fmt.Fprintf(p.output, "%s [%d]: ", prompt, def); err != nil {
			return 0, err
		}
		line, err := p.readLine()
		if err != nil {
			return 0, err
		}
		if line == "" {
			return def, nil
		}
		n, err := strconv.Atoi(line)
		if err == nil && n >= min && n <= max {
			return n, nil
		}
	}
}

func (p *prompter) askPasswordMode(prompt string, options []paw.PasswordMode, defaultMode paw.PasswordMode) (paw.PasswordMode, error) {
	if len(options) == 0 {
		return 0, fmt.Errorf("no password modes available")
	}
	defaultIdx := 0
	for i, mode := range options {
		if mode == defaultMode {
			defaultIdx = i
			break
		}
	}
	if _, err := fmt.Fprintf(p.output, "%s [%s]:\n", prompt, options[defaultIdx]); err != nil {
		return 0, err
	}
	for i, mode := range options {
		if _, err := fmt.Fprintf(p.output, "  [%d] %s\n", i, mode); err != nil {
			return 0, err
		}
	}
	for {
		if _, err := fmt.Fprint(p.output, "> "); err != nil {
			return 0, err
		}
		line, err := p.readLine()
		if err != nil {
			return 0, err
		}
		if line == "" {
			return options[defaultIdx], nil
		}
		i, err := strconv.Atoi(line)
		if err == nil && i >= 0 && i < len(options) {
			return options[i], nil
		}
	}
}
