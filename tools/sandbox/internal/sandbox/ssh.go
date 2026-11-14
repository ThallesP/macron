package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	ConnectAttempts    = 4
	LoginTimeout       = 10 * time.Second
	sshFailureExitCode = 255
)

var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

func RemoteCommand(args []string) string {
	if len(args) == 1 {
		return args[0]
	}

	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = QuoteForShell(arg)
	}
	return strings.Join(quoted, " ")
}

func QuoteForShell(arg string) string {
	if shellSafe.MatchString(arg) {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}

func (a *App) generateKey(ctx context.Context, id string) (string, error) {
	path := a.Store.KeyPath(id)
	output, err := exec.CommandContext(ctx, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", id, "-f", path).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ssh-keygen: %s: %w", strings.TrimSpace(string(output)), err)
	}

	public, err := os.ReadFile(path + ".pub")
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(public))
	if len(fields) < 2 {
		return "", fmt.Errorf("ssh-keygen wrote an unreadable public key")
	}
	return fields[0] + " " + fields[1], nil
}

func (a *App) writeSSHFiles(id string, announcement Announcement, route Route) error {
	knownHosts := fmt.Sprintf("%s %s\n", id, announcement.HostKey)
	if err := os.WriteFile(a.Store.KnownHostsPath(id), []byte(knownHosts), 0o600); err != nil {
		return err
	}

	lines := []string{
		"Host " + id,
		"  HostName " + route.HostName,
		"  User " + announcement.User,
		"  HostKeyAlias " + id,
		fmt.Sprintf("  IdentityFile %q", a.Store.KeyPath(id)),
		"  IdentitiesOnly yes",
		"  IdentityAgent none",
		fmt.Sprintf("  UserKnownHostsFile %q", a.Store.KnownHostsPath(id)),
		"  GlobalKnownHostsFile /dev/null",
		"  StrictHostKeyChecking yes",
		"  UpdateHostKeys no",
		"  PreferredAuthentications publickey",
		"  PasswordAuthentication no",
		"  KbdInteractiveAuthentication no",
		"  ForwardAgent no",
		"  ServerAliveInterval 15",
		"  ServerAliveCountMax 4",
		"  ConnectTimeout 20",
		"  LogLevel ERROR",
	}
	if route.ProxyCommand != "" {
		lines = append(lines, "  ProxyCommand "+route.ProxyCommand)
	}
	config := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(a.Store.SSHConfigPath(id), []byte(config), 0o600)
}

func (a *App) sshArgs(id string, options ...string) []string {
	args := []string{"ssh", "-F", a.Store.SSHConfigPath(id)}
	args = append(args, options...)
	return append(args, id)
}

type sshResult struct {
	ExitCode    int
	HasLoggedIn bool
	Complaints  string
}

type sshCommand func(ctx context.Context, log *os.File) *exec.Cmd

func (a *App) overSSH(ctx context.Context, id string, build sshCommand) (sshResult, error) {
	for attempt := 1; ; attempt++ {
		result, err := a.attempt(ctx, id, build)
		if ctx.Err() != nil {
			return sshResult{}, ctx.Err()
		}
		if err != nil {
			return sshResult{}, err
		}
		if result.ExitCode == 0 || result.HasLoggedIn {
			return result, nil
		}
		if strings.Contains(result.Complaints, "Host key verification failed") {
			return sshResult{}, Errorf(CodeHostKeyMismatch, "%s answered with a different SSH host key than the one GitHub published for it; refusing to connect", id)
		}
		if strings.Contains(result.Complaints, "Permission denied") {
			return sshResult{}, Errorf(CodeKeyRefused, "%s refused this machine's SSH key", id)
		}
		if err := a.stoppedError(ctx, id); err != nil {
			return sshResult{}, err
		}
		if attempt == ConnectAttempts {
			return sshResult{}, Errorf(CodeUnreachable, "could not connect to %s: %s", id, lastLine(result.Complaints))
		}
	}
}

func (a *App) attempt(ctx context.Context, id string, build sshCommand) (sshResult, error) {
	log, err := os.CreateTemp(filepath.Dir(a.Store.SSHConfigPath(id)), "ssh-*.log")
	if err != nil {
		return sshResult{}, err
	}
	defer os.Remove(log.Name())
	defer log.Close()

	ctx, kill := context.WithCancel(ctx)
	defer kill()

	command := build(ctx, log)
	if err := command.Start(); err != nil {
		return sshResult{}, err
	}
	finished := make(chan error, 1)
	go func() { finished <- command.Wait() }()

	var runErr error
	select {
	case runErr = <-finished:
	case <-time.After(LoginTimeout):
		if result, err := readSSHLog(log.Name()); err != nil || !result.HasLoggedIn {
			kill()
		}
		runErr = <-finished
	}

	result, err := readSSHLog(log.Name())
	if err != nil {
		return sshResult{}, err
	}
	var exit *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exit) {
		return sshResult{}, runErr
	}
	if runErr != nil {
		result.ExitCode = exit.ExitCode()
	}
	if result.ExitCode < 0 && !result.HasLoggedIn {
		result.Complaints += fmt.Sprintf("no login after %s\n", LoginTimeout)
	}
	return result, nil
}

func readSSHLog(path string) (sshResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return sshResult{}, err
	}

	result := sshResult{}
	for line := range strings.Lines(string(data)) {
		if strings.HasPrefix(line, "Authenticated to ") {
			result.HasLoggedIn = true
			continue
		}
		if strings.HasPrefix(line, "Transferred: ") || strings.HasPrefix(line, "Bytes per second: ") {
			continue
		}
		result.Complaints += line
	}
	return result, nil
}

func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

type remoteCommand struct {
	ID     string
	Line   string
	Stdin  *os.File
	Stdout io.Writer
	Stderr io.Writer
}

func (a *App) run(ctx context.Context, in remoteCommand) (int, error) {
	result, err := a.overSSH(ctx, in.ID, func(ctx context.Context, log *os.File) *exec.Cmd {
		args := append(a.sshArgs(in.ID, "-o", "BatchMode=yes", "-o", "LogLevel=VERBOSE", "-E", log.Name()), "--", in.Line)
		command := exec.CommandContext(ctx, args[0], args[1:]...)
		if in.Stdin != nil {
			command.Stdin = in.Stdin
		}
		command.Stdout = in.Stdout
		command.Stderr = in.Stderr
		return command
	})
	if err != nil {
		return 0, err
	}
	if result.ExitCode == sshFailureExitCode && in.Stderr != nil {
		fmt.Fprint(in.Stderr, result.Complaints)
	}
	return result.ExitCode, nil
}

func (a *App) awaitSSH(ctx context.Context, id string) error {
	for {
		exitCode, err := a.run(ctx, remoteCommand{ID: id, Line: "true"})
		if err == nil && exitCode == 0 {
			return nil
		}

		var expected *Error
		if err != nil && (!errors.As(err, &expected) || expected.Code != CodeUnreachable) {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
