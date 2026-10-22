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

	"lucor.dev/paw/internal/paw"
	"lucor.dev/paw/internal/sshkey"
)

func execute(t *testing.T, args []string, s paw.Storage) (string, error) {
	t.Helper()
	out, _, err := executeWithInput(t, args, newCommand(s), "")
	return out, err
}

func executeWithInput(t *testing.T, args []string, cmd *urfave.Command, input string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd.Reader = strings.NewReader(input)
	cmd.Writer = &out
	cmd.ErrWriter = &errOut
	if cmd.Name != "paw cli" {
		args = args[1:]
	}
	err := cmd.Run(context.Background(), append([]string{cmd.Name}, args...))
	return out.String(), errOut.String(), err
}

func TestHelp(t *testing.T) {
	commands := []string{"agent", "add", "edit", "init", "ls", "lock", "pwgen", "rm", "show", "unlock", "version"}
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

func TestInvalidArguments(t *testing.T) {
	type testCase struct {
		args []string
		want string
	}
	cases := []testCase{
		{nil, "a command is required"},
		{[]string{"unknown"}, `unknown command "unknown"`},
		{[]string{"agent"}, "a command is required"},
		{[]string{"agent", "unknown"}, `unknown command "unknown"`},
		{[]string{"agent", "start", "extra"}, "invalid arguments"},
		{[]string{"agent", "sessions", "extra"}, "invalid arguments"},
		{[]string{"ls", "a", "b"}, "invalid arguments"},
		{[]string{"pwgen", "extra"}, "invalid arguments"},
		{[]string{"version", "extra"}, "invalid arguments"},
		{[]string{"unlock", "-t", "invalid", "vault"}, "invalid value"},
		{[]string{"show", "--unknown"}, "flag provided but not defined"},
		{[]string{"show", "vault/unknown/item"}, "invalid item type"},
		{[]string{"ls", "vault/login/item/extra"}, "invalid vault item path"},
	}
	for _, name := range []string{"add", "edit", "init", "lock", "rm", "show", "unlock"} {
		cases = append(cases,
			testCase{[]string{name}, "invalid arguments"},
			testCase{[]string{name, "a", "b"}, "invalid arguments"},
		)
	}
	for _, name := range []string{"add", "edit", "rm", "show"} {
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
		{"add", []string{"-i", "key.pem"}, func(t *testing.T, c *urfave.Command) { require.Equal(t, "key.pem", c.String("input")) }},
		{"add", []string{"--input=key.pem"}, func(t *testing.T, c *urfave.Command) { require.Equal(t, "key.pem", c.String("input")) }},
		{"show", []string{"-c"}, func(t *testing.T, c *urfave.Command) { require.True(t, c.Bool("clip")) }},
		{"show", []string{"--clip"}, func(t *testing.T, c *urfave.Command) { require.True(t, c.Bool("clip")) }},
		{"unlock", []string{"-t", "15m"}, func(t *testing.T, c *urfave.Command) { require.Equal(t, 15*time.Minute, c.Duration("lifetime")) }},
		{"unlock", []string{"--lifetime=15m"}, func(t *testing.T, c *urfave.Command) { require.Equal(t, 15*time.Minute, c.Duration("lifetime")) }},
	}
	for _, tc := range cases {
		for _, after := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v/after=%t", tc.name, tc.flags, after), func(t *testing.T) {
				cmd := newCommand(nil)
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

func TestHelpDoesNotExposeSession(t *testing.T) {
	t.Setenv(paw.ENV_SESSION, "private-session-value")
	for _, name := range []string{"add", "edit", "ls", "rm", "show"} {
		out, err := execute(t, []string{name, "--session=explicit-session-value", "--help"}, nil)
		require.NoError(t, err)
		require.Contains(t, out, "--session")
		require.NotContains(t, out, "private-session-value")
		require.NotContains(t, out, "explicit-session-value")
	}
}

func TestSessionFlag(t *testing.T) {
	for _, name := range []string{"add", "edit", "ls", "rm", "show"} {
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
				cmd := newCommand(nil)
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
	failure := errors.New("storage unavailable")
	s := &paw.StorageMock{VaultStorageMock: paw.VaultStorageMock{OnVaults: func() ([]string, error) {
		return nil, failure
	}}}
	require.ErrorIs(t, Run([]string{"paw", "cli", "ls", "--session=override"}, s), failure)
	require.Equal(t, "environment-session", os.Getenv(paw.ENV_SESSION))
	require.ErrorIs(t, Run([]string{"paw", "cli", "ls"}, s), failure)
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

func TestListVaults(t *testing.T) {
	calls := 0
	s := &paw.StorageMock{VaultStorageMock: paw.VaultStorageMock{OnVaults: func() ([]string, error) {
		calls++
		return []string{"personal", "work"}, nil
	}}}
	_, err := execute(t, []string{"ls"}, s)
	require.NoError(t, err)
	require.Equal(t, 1, calls)
}

func TestSSHKeyImportFailureAndCancellation(t *testing.T) {
	base := &command{readFile: os.ReadFile}
	base.out = io.Discard
	base.prompter = newPrompter(strings.NewReader("n\n"), io.Discard)
	cmd := &AddCmd{command: base, importPath: filepath.Join(t.TempDir(), "missing-key")}
	err := cmd.addSSHKeyItem(paw.NewSSHKey())
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))

	key, err := sshkey.GenerateKey()
	require.NoError(t, err)
	cmd.importPath = filepath.Join(t.TempDir(), "key")
	require.NoError(t, os.WriteFile(cmd.importPath, key.MarshalPrivateKey(), 0600))
	require.ErrorIs(t, cmd.addSSHKeyItem(paw.NewSSHKey()), errCanceled)
}
