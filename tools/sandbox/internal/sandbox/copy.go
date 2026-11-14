package sandbox

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type CopyInput struct {
	Source      string
	Destination string
	Recursive   bool
}

var remotePathPattern = regexp.MustCompile(`^(sbx_[a-z0-9]{8}):`)

func (a *App) Copy(ctx context.Context, in CopyInput) error {
	fromSandbox := remotePathPattern.FindStringSubmatch(in.Source)
	toSandbox := remotePathPattern.FindStringSubmatch(in.Destination)
	if (fromSandbox == nil) == (toSandbox == nil) {
		return Errorf(CodeInvalidInput, "exactly one of the two paths must be in a sandbox, written <id>:<path>")
	}

	id := append(fromSandbox, toSandbox...)[1]
	if _, err := a.connectable(ctx, id); err != nil {
		return err
	}

	args := []string{"-F", a.Store.SSHConfigPath(id), "-o", "BatchMode=yes", "-o", "LogLevel=VERBOSE"}
	if in.Recursive {
		args = append(args, "-r")
	}
	args = append(args, "--", in.Source, in.Destination)

	result, err := a.overSSH(ctx, id, func(ctx context.Context, log *os.File) *exec.Cmd {
		command := exec.CommandContext(ctx, "scp", args...)
		command.Stderr = log
		return command
	})
	if err != nil {
		return err
	}
	if result.ExitCode == 0 {
		return nil
	}
	if err := a.stoppedError(ctx, id); err != nil {
		return err
	}
	return Errorf(CodeCopyFailed, "copy failed: %s", scpComplaint(result.Complaints))
}

func scpComplaint(output string) string {
	complaint := ""
	for line := range strings.Lines(output) {
		if strings.HasPrefix(line, "scp: ") || strings.Contains(line, "Host key verification failed") {
			complaint += line
		}
	}
	if complaint == "" {
		return lastLine(output)
	}
	return strings.TrimSpace(complaint)
}
