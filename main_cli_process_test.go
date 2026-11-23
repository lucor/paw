// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin || linux

package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/agent"
	"go.lucor.dev/paw/internal/cli"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
	sshagent "golang.org/x/crypto/ssh/agent"
)

const (
	cliProcessHelperEnv = "PAW_TEST_CLI_PROCESS"
	cliProcessArgsEnv   = "PAW_TEST_CLI_ARGS"
	cliProcessAgentEnv  = "PAW_TEST_CLI_AGENT"
)

func TestCLIProcessHelper(t *testing.T) {
	if os.Getenv(cliProcessHelperEnv) != "1" {
		return
	}
	args := strings.Split(os.Getenv(cliProcessArgsEnv), "\x1f")
	os.Args = append([]string{"paw", "cli"}, args...)

	if os.Getenv(cliProcessAgentEnv) == "1" {
		store, err := storage.NewFileStorage(filepath.Join(os.Getenv(paw.ENV_HOME), ".paw"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(cli.ExitCodeApplicationError)
		}
		listener, err := net.Listen("unix", store.SocketAgentPath())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(cli.ExitCodeApplicationError)
		}
		server := agent.NewCLI()
		go func() {
			for {
				connection, err := listener.Accept()
				if err != nil {
					return
				}
				go func() {
					defer connection.Close()
					_ = sshagent.ServeAgent(server, connection)
				}()
			}
		}()
	}
	os.Exit(run())
}

// TestCLIProcessShellCompletion checks native completion through the CLI entry point.
func TestCLIProcessShellCompletion(t *testing.T) {
	code, stdout, stderr := runCLIProcess(t, []string{"--generate-shell-completion"}, false)
	require.Equal(t, cli.ExitCodeSuccess, code)
	require.Contains(t, stdout, "\nunlock:Unlock a vault")
	require.Empty(t, stderr)

	code, stdout, stderr = runCLIProcess(t, []string{"agent", "--generate-shell-completion"}, false)
	require.Equal(t, cli.ExitCodeSuccess, code)
	require.Contains(t, stdout, "sessions:List active sessions")
	require.Empty(t, stderr)

	for shell, registration := range map[string]string{
		"bash": "-F __paw_bash_autocomplete paw",
		"zsh":  "compdef _paw paw",
		"fish": "complete -c paw -f",
	} {
		t.Run(shell, func(t *testing.T) {
			code, stdout, stderr := runCLIProcess(t, []string{"completion", shell}, false)
			require.Equal(t, cli.ExitCodeSuccess, code)
			require.Contains(t, stdout, registration)
			require.Empty(t, stderr)
		})
	}
}

func TestCLIProcessExitCodes(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		startAgent bool
		wantCode   int
		wantOutput string
		wantError  string
	}{
		{
			name:       "success",
			args:       []string{"version"},
			wantCode:   cli.ExitCodeSuccess,
			wantOutput: "paw cli version",
		},
		{
			name:       "empty results",
			args:       []string{"agent", "sessions"},
			startAgent: true,
			wantCode:   cli.ExitCodeSuccess,
		},
		{
			name:      "missing argument",
			args:      []string{"init"},
			wantCode:  cli.ExitCodeInvalidArguments,
			wantError: "Error: invalid arguments: missing required arguments: NAME",
		},
		{
			name:      "extra argument",
			args:      []string{"agent", "start", "extra"},
			wantCode:  cli.ExitCodeInvalidArguments,
			wantError: "Error: invalid arguments: unexpected arguments; usage: paw cli agent start",
		},
		{
			name:      "invalid flag",
			args:      []string{"version", "--invalid"},
			wantCode:  cli.ExitCodeInvalidArguments,
			wantError: "Error: flag provided but not defined: -invalid",
		},
		{
			name:      "invalid flag value",
			args:      []string{"unlock", "--lifetime=-1s", "vault"},
			wantCode:  cli.ExitCodeInvalidArguments,
			wantError: "Error: lifetime cannot be negative",
		},
		{
			name:      "application error",
			args:      []string{"ls"},
			wantCode:  cli.ExitCodeApplicationError,
			wantError: "Error: no vaults found.",
		},
		{
			name:      "terminal unavailable",
			args:      []string{"init", "vault"},
			wantCode:  cli.ExitCodeApplicationError,
			wantError: "Error: interactive prompt requires stdin to be a terminal",
		},
		{
			name:      "sync terminal unavailable",
			args:      []string{"sync", "vault"},
			wantCode:  cli.ExitCodeApplicationError,
			wantError: "Error: interactive prompt requires stdin to be a terminal",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, stdout, stderr := runCLIProcess(t, test.args, test.startAgent)
			require.Equal(t, test.wantCode, code, "stdout: %q; stderr: %q", stdout, stderr)
			require.Contains(t, stdout, test.wantOutput)
			if test.wantError == "" {
				require.Empty(t, stderr)
			} else {
				require.Empty(t, stdout)
				require.Contains(t, stderr, test.wantError)
			}
		})
	}
}

func TestCLIProcessCtrlCExitCode(t *testing.T) {
	cmd, home := newCLIProcess(t, []string{"agent", "start"}, false)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Start())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	socketPath := filepath.Join(home, ".paw", "agent.sock")
	deadline := time.NewTimer(10 * time.Second)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()
	for {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("agent process exited before opening its socket: %v; stdout: %s; stderr: %s", err, stdout.String(), stderr.String())
		case <-deadline.C:
			_ = cmd.Process.Kill()
			<-done
			t.Fatal("agent process did not open its socket")
		case <-ticker.C:
		}
	}
	require.NoError(t, cmd.Process.Signal(os.Interrupt))
	select {
	case err := <-done:
		var exitError *exec.ExitError
		require.ErrorAs(t, err, &exitError)
		require.True(t, exitError.ProcessState.Sys().(syscall.WaitStatus).Signaled())
		require.Equal(t, syscall.SIGINT, exitError.ProcessState.Sys().(syscall.WaitStatus).Signal())
		require.Empty(t, stdout.String())
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		t.Fatal("agent process did not exit after Ctrl+C")
	}
}

func TestCLIProcessHuhEscapeKeepsSuccessExitCode(t *testing.T) {
	script, err := exec.LookPath("script")
	require.NoError(t, err, "the process test requires the system script utility")
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("script command syntax is supported on macOS and Linux")
	}

	_, home := newCLIProcess(t, []string{"init", "cancelled"}, false)
	var process *exec.Cmd
	command := shellQuote(os.Args[0]) + " -test.run=^TestCLIProcessHelper$"
	if runtime.GOOS == "darwin" {
		process = exec.Command(script, "-q", "/dev/null", os.Args[0], "-test.run=^TestCLIProcessHelper$")
	} else {
		process = exec.Command(script, "-q", "-e", "-c", command, "/dev/null")
	}
	process.Env = cliProcessEnvironment(home, []string{"init", "cancelled"}, false)
	stdin, err := process.StdinPipe()
	require.NoError(t, err)
	stdout, err := process.StdoutPipe()
	require.NoError(t, err)
	var output bytes.Buffer
	var stderr bytes.Buffer
	process.Stderr = &stderr
	require.NoError(t, process.Start())

	formReady := make(chan struct{}, 1)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		buf := make([]byte, 512)
		for {
			n, readErr := stdout.Read(buf)
			if n > 0 {
				_, _ = output.Write(buf[:n])
				if bytes.Contains(output.Bytes(), []byte("\x1b[?25l")) {
					select {
					case formReady <- struct{}{}:
					default:
					}
				}
			}
			if readErr != nil {
				return
			}
		}
	}()
	done := make(chan error, 1)
	go func() { done <- process.Wait() }()
	select {
	case <-formReady:
		_, err = stdin.Write([]byte{0x1b})
		require.NoError(t, err)
		_ = stdin.Close()
	case err = <-done:
		<-readerDone
		t.Fatalf("interactive form exited before it could be canceled: %v; output: %s; stderr: %s", err, output.String(), stderr.String())
	case <-time.After(10 * time.Second):
		_ = process.Process.Kill()
		<-done
		<-readerDone
		t.Fatalf("interactive form did not start; output bytes: %d; stderr: %s", output.Len(), stderr.String())
	}
	select {
	case err = <-done:
		<-readerDone
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		_ = process.Process.Kill()
		<-done
		<-readerDone
		t.Fatal("interactive form did not exit after Esc")
	}
}

// runCLIProcess executes the CLI helper and captures its exit code and output.
func runCLIProcess(t *testing.T, args []string, startAgent bool) (int, string, string) {
	t.Helper()
	cmd, _ := newCLIProcess(t, args, startAgent)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return cli.ExitCodeSuccess, stdout.String(), stderr.String()
	}
	var exitError *exec.ExitError
	require.ErrorAs(t, err, &exitError)
	return exitError.ExitCode(), stdout.String(), stderr.String()
}

// newCLIProcess prepares an isolated CLI subprocess environment.
func newCLIProcess(t *testing.T, args []string, startAgent bool) (*exec.Cmd, string) {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "paw-cli-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(home)) })
	require.NoError(t, os.MkdirAll(filepath.Join(home, "Library", "Caches"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".cache"), 0o700))
	cmd := exec.Command(os.Args[0], "-test.run=^TestCLIProcessHelper$")
	cmd.Env = cliProcessEnvironment(home, args, startAgent)
	return cmd, home
}

// cliProcessEnvironment returns environment variables for a CLI subprocess.
func cliProcessEnvironment(home string, args []string, startAgent bool) []string {
	env := os.Environ()
	env = replaceProcessEnv(env, cliProcessHelperEnv, "1")
	env = replaceProcessEnv(env, cliProcessArgsEnv, strings.Join(args, "\x1f"))
	agentValue := "0"
	if startAgent {
		agentValue = "1"
	}
	env = replaceProcessEnv(env, cliProcessAgentEnv, agentValue)
	env = replaceProcessEnv(env, paw.ENV_HOME, home)
	env = replaceProcessEnv(env, "HOME", home)
	return replaceProcessEnv(env, "TERM", "xterm-256color")
}

// replaceProcessEnv sets one variable in an environment slice.
func replaceProcessEnv(env []string, name, value string) []string {
	updated := make([]string, 0, len(env)+1)
	prefix := name + "="
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			updated = append(updated, entry)
		}
	}
	return append(updated, prefix+value)
}

// shellQuote quotes a value for use in a shell command.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
