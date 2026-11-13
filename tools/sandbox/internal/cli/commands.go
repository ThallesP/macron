package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/ThallesP/macron/tools/sandbox/internal/sandbox"
)

func ttlFlag(args arguments) (time.Duration, error) {
	if !args.has("ttl") {
		return sandbox.DefaultTTL, nil
	}
	ttl, err := time.ParseDuration(args.flags["ttl"])
	if err != nil {
		return 0, sandbox.Errorf(sandbox.CodeInvalidInput, "--ttl %q is not a duration; write it like 30m, 90s or 2h", args.flags["ttl"])
	}
	return ttl, nil
}

func (c CLI) new(ctx context.Context, args arguments) (int, error) {
	if len(args.positionals) != 0 {
		return 0, sandbox.Errorf(sandbox.CodeInvalidInput, "usage: sbx new [--ttl 30m]")
	}
	ttl, err := ttlFlag(args)
	if err != nil {
		return 0, err
	}

	asJSON := args.has("json")
	startedAt := time.Now()
	created, err := c.App.New(ctx, sandbox.NewInput{
		TTL: ttl,
		OnStarted: func(starting sandbox.Sandbox) {
			if asJSON {
				c.printJSON(map[string]any{"event": "started", "id": starting.ID, "sandbox": starting})
				return
			}
			fmt.Fprintf(c.Stderr, "Starting %s (%s)\n", starting.ID, starting.URL)
		},
	})
	if err != nil {
		return 0, err
	}

	readyAfter := time.Since(startedAt).Round(time.Second)
	if asJSON {
		c.printJSON(map[string]any{"event": "ready", "id": created.ID, "sandbox": created, "readyAfterSeconds": int(readyAfter.Seconds())})
		return 0, nil
	}
	fmt.Fprintf(c.Stdout, "%s is ready after %s. It stops itself at %s.\n", created.ID, readyAfter, clock(created.AutoStopAt))
	return 0, nil
}

func (c CLI) exec(ctx context.Context, args arguments) (int, error) {
	if len(args.positionals) < 2 {
		return 0, sandbox.Errorf(sandbox.CodeInvalidInput, "usage: sbx exec <id> -- <command>")
	}
	in := sandbox.ExecInput{
		ID:      args.positionals[0],
		Command: args.positionals[1:],
		Stdin:   c.Stdin,
		Stdout:  c.Stdout,
		Stderr:  c.Stderr,
	}
	if !args.has("json") {
		return c.App.Exec(ctx, in)
	}

	var stdout, stderr bytes.Buffer
	in.Stdout = &stdout
	in.Stderr = &stderr
	exitCode, err := c.App.Exec(ctx, in)
	if err != nil {
		return 0, err
	}
	c.printJSON(map[string]any{"id": in.ID, "exitCode": exitCode, "stdout": stdout.String(), "stderr": stderr.String()})
	return exitCode, nil
}

func (c CLI) ssh(ctx context.Context, args arguments) (int, error) {
	if len(args.positionals) == 0 {
		return 0, sandbox.Errorf(sandbox.CodeInvalidInput, "usage: sbx ssh <id> [-- <command>]")
	}
	id := args.positionals[0]
	argv, err := c.App.ShellCommand(ctx, id, args.positionals[1:], c.StdinIsTerminal)
	if err != nil {
		return 0, err
	}
	if args.has("json") {
		c.printJSON(map[string]any{"id": id, "command": argv})
		return 0, nil
	}

	ssh, err := exec.LookPath(argv[0])
	if err != nil {
		return 0, sandbox.Errorf(sandbox.CodeMissingTool, "ssh is not installed; sbx needs OpenSSH")
	}
	return 0, syscall.Exec(ssh, argv, os.Environ())
}

func (c CLI) scp(ctx context.Context, args arguments) (int, error) {
	if len(args.positionals) != 2 {
		return 0, sandbox.Errorf(sandbox.CodeInvalidInput, "usage: sbx scp [-r] <from> <to>, where a sandbox path is <id>:<path>")
	}
	in := sandbox.CopyInput{
		Source:      args.positionals[0],
		Destination: args.positionals[1],
		Recursive:   args.has("r"),
	}
	if err := c.App.Copy(ctx, in); err != nil {
		return 0, err
	}
	if args.has("json") {
		c.printJSON(map[string]any{"source": in.Source, "destination": in.Destination})
	}
	return 0, nil
}

func (c CLI) extend(ctx context.Context, args arguments) (int, error) {
	id, err := oneID(args, "sbx extend <id> [--ttl 30m]")
	if err != nil {
		return 0, err
	}
	ttl, err := ttlFlag(args)
	if err != nil {
		return 0, err
	}

	extended, err := c.App.Extend(ctx, sandbox.ExtendInput{ID: id, TTL: ttl})
	if err != nil {
		return 0, err
	}
	if args.has("json") {
		c.printJSON(map[string]any{"sandbox": extended})
		return 0, nil
	}
	fmt.Fprintf(c.Stdout, "%s now stops itself at %s.\n", extended.ID, clock(extended.AutoStopAt))
	return 0, nil
}

func (c CLI) stop(ctx context.Context, args arguments) (int, error) {
	id, err := oneID(args, "sbx stop <id>")
	if err != nil {
		return 0, err
	}

	stopped, err := c.App.Stop(ctx, id)
	if err != nil {
		return 0, err
	}
	if args.has("json") {
		c.printJSON(map[string]any{"sandbox": stopped})
		return 0, nil
	}
	fmt.Fprintf(c.Stdout, "%s is %s.\n", stopped.ID, stopped.State)
	return 0, nil
}

func (c CLI) list(ctx context.Context, args arguments) (int, error) {
	if len(args.positionals) != 0 {
		return 0, sandbox.Errorf(sandbox.CodeInvalidInput, "usage: sbx list [--all]")
	}

	sandboxes, err := c.App.List(ctx, sandbox.ListInput{IncludeStopped: args.has("all")})
	if err != nil {
		return 0, err
	}
	if args.has("json") {
		c.printJSON(map[string]any{"sandboxes": sandboxes})
		return 0, nil
	}
	c.printTable(sandboxes)
	return 0, nil
}

func (c CLI) info(ctx context.Context, args arguments) (int, error) {
	id, err := oneID(args, "sbx info <id>")
	if err != nil {
		return 0, err
	}

	found, err := c.App.Info(ctx, id)
	if err != nil {
		return 0, err
	}
	if args.has("json") {
		c.printJSON(map[string]any{"sandbox": found})
		return 0, nil
	}
	c.printDetails(found)
	return 0, nil
}

func (c CLI) proxy(ctx context.Context, args arguments) (int, error) {
	if len(args.positionals) != 2 {
		return 0, sandbox.Errorf(sandbox.CodeInvalidInput, "usage: sbx proxy <transport> <address>")
	}
	proxy, isKnown := c.Proxies[args.positionals[0]]
	if !isKnown {
		return 0, sandbox.Errorf(sandbox.CodeInvalidInput, "unknown transport %q", args.positionals[0])
	}
	return 0, proxy(ctx, args.positionals[1], c.Stdin, c.Stdout)
}
