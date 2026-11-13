package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
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

func (a *App) batchCommand(ctx context.Context, id, remote string) *exec.Cmd {
	args := append(a.sshArgs(id, "-o", "BatchMode=yes"), "--", remote)
	return exec.CommandContext(ctx, args[0], args[1:]...)
}

func (a *App) awaitSSH(ctx context.Context, id string) error {
	for {
		if a.batchCommand(ctx, id, "true").Run() == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
