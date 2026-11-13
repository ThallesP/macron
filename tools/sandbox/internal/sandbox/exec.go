package sandbox

import (
	"context"
	"errors"
	"io"
	"os/exec"
)

const sshFailureExitCode = 255

type ExecInput struct {
	ID      string
	Command []string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

func (a *App) Exec(ctx context.Context, in ExecInput) (int, error) {
	if len(in.Command) == 0 {
		return 0, Errorf(CodeInvalidInput, "no command given; use: sbx exec <id> -- <command>")
	}
	if _, err := a.connectable(ctx, in.ID); err != nil {
		return 0, err
	}

	command := a.batchCommand(ctx, in.ID, RemoteCommand(in.Command))
	command.Stdin = in.Stdin
	command.Stdout = in.Stdout
	command.Stderr = in.Stderr

	err := command.Run()
	if err == nil {
		return 0, nil
	}

	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() < 0 {
		return 0, err
	}
	if exit.ExitCode() != sshFailureExitCode {
		return exit.ExitCode(), nil
	}
	if err := a.stoppedError(ctx, in.ID); err != nil {
		return 0, err
	}
	return sshFailureExitCode, nil
}

func (a *App) ShellCommand(ctx context.Context, id string, command []string, withTerminal bool) ([]string, error) {
	if _, err := a.connectable(ctx, id); err != nil {
		return nil, err
	}

	if len(command) == 0 {
		return a.sshArgs(id), nil
	}
	if withTerminal {
		return append(a.sshArgs(id, "-t"), "--", RemoteCommand(command)), nil
	}
	return append(a.sshArgs(id), "--", RemoteCommand(command)), nil
}
