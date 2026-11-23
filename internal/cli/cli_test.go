// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	urfave "github.com/urfave/cli/v3"

	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
)

func execute(t *testing.T, args []string, s *storage.FileStorage) (string, error) {
	t.Helper()
	out, _, err := executeWithInput(t, args, s, "")
	return out, err
}

func executeWithInput(t *testing.T, args []string, s *storage.FileStorage, input string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := newCommand(s, strings.NewReader(input), &out, &errOut)
	err := cmd.Run(context.Background(), append([]string{cmd.Name}, args...))
	return out.String(), errOut.String(), err
}

func executeSubcommand(t *testing.T, args []string, constructor func(*command) *urfave.Command, base *command, input string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	in := strings.NewReader(input)
	testCommand := *base
	testCommand.out = &out
	testCommand.errOut = &errOut
	if testCommand.prompter == nil {
		testCommand.prompter = newPrompter(in, &errOut)
	}
	cmd := constructor(&testCommand)
	cmd.ExitErrHandler = leaveExitToCaller
	cmd.Reader = in
	cmd.Writer = &out
	cmd.ErrWriter = &errOut
	err := cmd.Run(context.Background(), args)
	return out.String(), errOut.String(), err
}

func TestHelp(t *testing.T) {
	commands := []string{"agent", "add", "attachment", "edit", "history", "init", "ls", "lock", "pwgen", "rm", "show", "sync", "unlock", "version"}
	for _, flag := range []string{"--help", "-h", "-help"} {
		t.Run("root/"+flag, func(t *testing.T) {
			out, err := execute(t, []string{flag}, nil)
			require.NoError(t, err)
			require.Contains(t, out, "paw cli")
			for _, name := range commands {
				require.Contains(t, out, name)
			}
		})
		for _, name := range commands {
			t.Run(name+"/"+flag, func(t *testing.T) {
				out, err := execute(t, []string{name, flag}, nil)
				require.NoError(t, err)
				require.Contains(t, out, "paw cli "+name)
				if name == "unlock" {
					require.Contains(t, out, "VAULT")
					require.Contains(t, out, "VAULT can be a vault name, full ID, or unique ID prefix (at least 7 characters).")
					require.NotContains(t, out, "paw cli session")
				}
			})
		}
	}
	for _, sub := range []string{"start", "sessions"} {
		out, err := execute(t, []string{"agent", sub, "--help"}, nil)
		require.NoError(t, err)
		require.Contains(t, out, "paw cli agent "+sub)
	}
}

func TestUnlockRejectsNegativeLifetime(t *testing.T) {
	_, _, err := executeSubcommand(t, []string{"unlock", "--lifetime", "-1s", "personal"}, newUnlockCommand, &command{}, "")
	require.ErrorContains(t, err, "lifetime cannot be negative")
}

func TestCommandGroupHelpAndUnknownSubcommand(t *testing.T) {
	for _, group := range []string{"attachment", "agent", "history"} {
		t.Run(group, func(t *testing.T) {
			out, err := execute(t, []string{group}, nil)
			require.NoError(t, err)
			require.Contains(t, out, "paw cli "+group)
			out, err = execute(t, []string{group, "--help"}, nil)
			require.NoError(t, err)
			require.Contains(t, out, "paw cli "+group)
			_, err = execute(t, []string{group, "unexpected"}, nil)
			require.ErrorContains(t, err, "unknown "+group+" subcommand")
			require.NotContains(t, err.Error(), "No help topic")
		})
	}
	_, err := execute(t, []string{"attachment", "list", "--help"}, nil)
	require.NoError(t, err)
	out, err := execute(t, []string{"history", "list", "--help"}, nil)
	require.NoError(t, err)
	require.Contains(t, out, "paw cli history list")
}

func TestInvalidArguments(t *testing.T) {
	type testCase struct {
		args []string
		want string
	}
	cases := []testCase{
		{nil, "a command is required"},
		{[]string{"unknown"}, `unknown command "unknown"`},
		{[]string{"agent", "unknown"}, `unknown agent subcommand "unknown"`},
		{[]string{"agent", "start", "extra"}, "invalid arguments"},
		{[]string{"agent", "sessions", "extra"}, "invalid arguments"},
		{[]string{"ls", "a", "b"}, "invalid arguments"},
		{[]string{"pwgen", "extra"}, "invalid arguments"},
		{[]string{"version", "extra"}, "invalid arguments"},
		{[]string{"unlock", "-t", "invalid", "vault"}, "invalid value"},
		{[]string{"show", "--unknown"}, "flag provided but not defined"},
		{[]string{"show", "vault/unknown/item"}, "invalid item type"},
		{[]string{"edit", "--name=", "vault/login/item"}, "--name cannot be empty"},
		{[]string{"edit", "--input=", "vault/ssh_key/key"}, "--input requires a file path"},
		{[]string{"edit", "--input=key.pem", "vault/login/item"}, "only supported for SSH key items"},
		{[]string{"edit", "--generate-key", "vault/login/item"}, "only supported for SSH key items"},
		{[]string{"edit", "--generate-key", "--input=key.pem", "vault/ssh_key/key"}, "cannot be combined"},
		{[]string{"edit", "--totp-stdin", "vault/password/item"}, "only supported for login items"},
		{[]string{"edit", "--totp-stdin", "--field=Token", "vault/login/item"}, "cannot be combined"},
		{[]string{"edit", "--password-stdin", "--totp-stdin", "vault/login/item"}, "cannot be combined"},
		{[]string{"edit", "--password-stdin", "--field=Token", "vault/login/item"}, "cannot be combined"},
		{[]string{"edit", "--username=mario", "vault/password/item"}, "only supported for login items"},
		{[]string{"edit", "--password-stdin", "vault/note/item"}, "only supported for login and password items"},
		{[]string{"add", "note", "--tag=", "vault", "item"}, "--tag cannot be empty"},
		{[]string{"add", "vault/note/item"}, "unknown add subcommand"},
		{[]string{"add", "note", "vault"}, "invalid arguments"},
		{[]string{"sync"}, "invalid arguments"},
		{[]string{"sync", "vault", "extra"}, "invalid arguments"},
		{[]string{"ls", "vault/login/item/extra"}, "invalid vault item path"},
	}
	for _, name := range []string{"edit", "init", "lock", "rm", "show", "unlock"} {
		cases = append(cases,
			testCase{[]string{name}, "invalid arguments"},
			testCase{[]string{name, "a", "b"}, "invalid arguments"},
		)
	}
	for _, name := range []string{"edit", "rm", "show"} {
		for _, path := range []string{"vault", "vault/login", "vault/login/"} {
			cases = append(cases, testCase{[]string{name, path}, "path"})
		}
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.args), func(t *testing.T) {
			_, err := execute(t, tc.args, nil)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestOptionAliases(t *testing.T) {
	cases := []struct {
		name  string
		flags []string
		check func(*testing.T, *urfave.Command)
	}{
		{"show", []string{"-c"}, func(t *testing.T, c *urfave.Command) { require.True(t, c.Bool("clip")) }},
		{"show", []string{"--clip"}, func(t *testing.T, c *urfave.Command) { require.True(t, c.Bool("clip")) }},
		{"unlock", []string{"-t", "15m"}, func(t *testing.T, c *urfave.Command) { require.Equal(t, 15*time.Minute, c.Duration("lifetime")) }},
		{"unlock", []string{"--lifetime=15m"}, func(t *testing.T, c *urfave.Command) { require.Equal(t, 15*time.Minute, c.Duration("lifetime")) }},
	}
	for _, tc := range cases {
		for _, after := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v/after=%t", tc.name, tc.flags, after), func(t *testing.T) {
				cmd := newCommand(nil, strings.NewReader(""), io.Discard, io.Discard)
				checked := false
				stop := errors.New("stop before command execution")
				cmd.Command(tc.name).Before = func(ctx context.Context, c *urfave.Command) (context.Context, error) {
					checked = true
					tc.check(t, c)
					require.Equal(t, []string{"vault/login/item"}, c.Args().Slice())
					return ctx, stop
				}
				args := []string{"paw cli", tc.name}
				if after {
					args = append(args, "vault/login/item")
				}
				args = append(args, tc.flags...)
				if !after {
					args = append(args, "vault/login/item")
				}
				require.ErrorIs(t, cmd.Run(context.Background(), args), stop)
				require.True(t, checked)
			})
		}
	}
}

func TestCommandHelpRunsWithoutTerminal(t *testing.T) {
	for _, args := range [][]string{
		{"rm", "--help"},
		{"history", "show", "--help"},
	} {
		t.Run(strings.Join(args, "/"), func(t *testing.T) {
			out, _, err := executeWithInput(t, args, nil, "")
			require.NoError(t, err)
			require.Contains(t, out, "USAGE:")
		})
	}
}

func TestMigratedCommandStaticValidation(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"export", "--plaintext", "--paw", "vault", "backup"}, "cannot be combined"},
		{[]string{"sync", "--clone", "--reconcile"}, "reconcile cannot be combined with clone"},
		{[]string{"history", "show", "vault/note/item", "invalid"}, "invalid history index"},
		{[]string{"agent", "sessions", "extra"}, "invalid arguments"},
		{[]string{"attachment", "list", "vault/unknown/item"}, "invalid item type"},
		{[]string{"edit", "--field=Token", "--type=unknown", "vault/login/item"}, "unsupported custom field type"},
		{[]string{"restore"}, "invalid arguments"},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, "/"), func(t *testing.T) {
			_, err := execute(t, tc.args, nil)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestShowRejectsRevealWithClipboard(t *testing.T) {
	_, _, err := executeSubcommand(t, []string{"show", "--reveal", "--clip", "vault/login/item"}, newShowCommand, &command{}, "")
	require.ErrorContains(t, err, "cannot be combined")
}

func TestHelpDoesNotExposeSession(t *testing.T) {
	t.Setenv(paw.ENV_SESSION, "private-session-value")
	for _, name := range []string{"add", "edit", "export", "ls", "rm", "show"} {
		out, err := execute(t, []string{name, "--session=explicit-session-value", "--help"}, nil)
		require.NoError(t, err)
		require.Contains(t, out, "--session")
		require.NotContains(t, out, "private-session-value")
		require.NotContains(t, out, "explicit-session-value")
	}
}

func TestSessionFlag(t *testing.T) {
	for _, name := range []string{"add", "edit", "export", "ls", "rm", "show"} {
		for _, tc := range []struct {
			name  string
			flags []string
			want  string
		}{
			{"environment", nil, "environment-session"},
			{"override", []string{"--session=flag-session"}, "flag-session"},
			{"empty override", []string{"--session="}, ""},
		} {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				t.Setenv(paw.ENV_SESSION, "environment-session")
				cmd := newCommand(nil, strings.NewReader(""), io.Discard, io.Discard)
				stop := errors.New("stop before command execution")
				checked := false
				cmd.Command(name).Before = func(ctx context.Context, c *urfave.Command) (context.Context, error) {
					checked = true
					require.Equal(t, tc.want, c.String("session"))
					return ctx, stop
				}
				args := append([]string{"paw cli", name}, tc.flags...)
				require.ErrorIs(t, cmd.Run(context.Background(), args), stop)
				require.True(t, checked)
				require.Equal(t, "environment-session", os.Getenv(paw.ENV_SESSION))
			})
		}
	}
}

func TestRunRepeatedly(t *testing.T) {
	t.Setenv(paw.ENV_SESSION, "environment-session")
	s, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, s.Close())
	require.Error(t, Run([]string{"paw", "cli", "ls", "--session=override"}, s))
	require.Equal(t, "environment-session", os.Getenv(paw.ENV_SESSION))
	require.Error(t, Run([]string{"paw", "cli", "ls"}, s))
	require.Error(t, Run([]string{"paw", "cli", "show"}, nil))
	require.NoError(t, Run([]string{"paw", "cli", "version"}, nil))
	for _, args := range [][]string{nil, {"paw"}, {"paw", "gui"}} {
		require.Error(t, Run(args, nil))
	}
}

func TestVersion(t *testing.T) {
	out, err := execute(t, []string{"version"}, nil)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("paw cli version %s\n", paw.Version()), out)
}

func TestCommandConstructorRoutesPromptDiagnostics(t *testing.T) {
	out, diagnostics, err := executeWithInput(t, []string{"pwgen"}, nil, "2\n4\n")
	require.ErrorContains(t, err, "interactive prompt requires stdin to be a terminal")
	require.Empty(t, out)
	require.Empty(t, diagnostics)
}

func TestSSHKeyImportMissingFile(t *testing.T) {
	base := &command{readFile: os.ReadFile, errOut: io.Discard}
	base.out = io.Discard
	base.prompter = testInteractivePrompter(t, "n\n")
	cmd := &AddCmd{command: base, importPath: filepath.Join(t.TempDir(), "missing-key")}
	_, err := cmd.newSSHKey()
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))

}
