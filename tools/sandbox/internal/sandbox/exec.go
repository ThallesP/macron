package sandbox

import (
	"context"
	"io"
	"os"
)

type ExecInput struct {
	ID      string
	Command []string
	Stdin   *os.File
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

	exitCode, err := a.run(ctx, remoteCommand{
		ID:     in.ID,
		Line:   RemoteCommand(in.Command),
		Stdin:  in.Stdin,
		Stdout: in.Stdout,
		Stderr: in.Stderr,
	})
	if err != nil || exitCode != sshFailureExitCode {
		return exitCode, err
	}
	if err := a.stoppedError(ctx, in.ID); err != nil {
		return 0, err
	}
	return exitCode, nil
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
