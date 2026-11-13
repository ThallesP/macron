package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/ThallesP/macron/tools/sandbox/internal/sandbox"
)

const (
	exitUsage  = 2
	exitFailed = 125
)

const usage = `sbx gives you macOS sandboxes to run commands in.

  sbx new [--ttl 30m]              start a sandbox; returns when it accepts commands
  sbx exec <id> -- <command>       run a command; exits with the command's exit code
  sbx ssh <id> [-- <command>]      open a shell
  sbx scp [-r] <from> <to>         copy files; a sandbox path is <id>:<path>
  sbx extend <id> [--ttl 30m]      move the auto-stop time to this long from now
  sbx stop <id>                    stop a sandbox for good
  sbx list [--all]                 running sandboxes; --all adds stopped ones
  sbx info <id>                    one sandbox

Every command accepts --json. sbx's own failures exit 125, wrong usage exits 2.
`

type Proxy func(ctx context.Context, host string, in io.Reader, out io.Writer) error

type CLI struct {
	App             *sandbox.App
	Proxies         map[string]Proxy
	Stdin           io.Reader
	Stdout          io.Writer
	Stderr          io.Writer
	StdinIsTerminal bool
}

type command struct {
	flags map[string]flagKind
	run   func(ctx context.Context, args arguments) (int, error)
}

func (c CLI) commands() map[string]command {
	return map[string]command{
		"new":    {flags: map[string]flagKind{"ttl": valueFlag}, run: c.new},
		"exec":   {run: c.exec},
		"ssh":    {run: c.ssh},
		"scp":    {flags: map[string]flagKind{"r": boolFlag}, run: c.scp},
		"extend": {flags: map[string]flagKind{"ttl": valueFlag}, run: c.extend},
		"stop":   {run: c.stop},
		"list":   {flags: map[string]flagKind{"all": boolFlag}, run: c.list},
		"info":   {run: c.info},
		"proxy":  {run: c.proxy},
	}
}

func (c CLI) Run(ctx context.Context, raw []string) int {
	if len(raw) == 0 || slices.Contains([]string{"help", "-h", "--help"}, raw[0]) {
		fmt.Fprint(c.Stdout, usage)
		return 0
	}

	asJSON := slices.Contains(raw, "--json")
	selected, isKnown := c.commands()[raw[0]]
	if !isKnown {
		return c.fail(sandbox.Errorf(sandbox.CodeInvalidInput, "unknown command %q; run sbx help", raw[0]), asJSON)
	}

	known := map[string]flagKind{"json": boolFlag}
	for name, kind := range selected.flags {
		known[name] = kind
	}
	args, err := parseArguments(raw[1:], known)
	if err != nil {
		return c.fail(err, asJSON)
	}

	code, err := selected.run(ctx, args)
	if err != nil {
		return c.fail(err, args.has("json"))
	}
	return code
}

func (c CLI) fail(err error, asJSON bool) int {
	code := sandbox.Code("UNEXPECTED")
	var expected *sandbox.Error
	if errors.As(err, &expected) {
		code = expected.Code
	}

	if asJSON {
		json.NewEncoder(c.Stderr).Encode(map[string]any{"error": map[string]any{"code": code, "message": err.Error()}})
	} else {
		fmt.Fprintf(c.Stderr, "sbx: %s\n", err)
	}

	if code == sandbox.CodeInvalidInput {
		return exitUsage
	}
	return exitFailed
}

func (c CLI) printJSON(value any) {
	json.NewEncoder(c.Stdout).Encode(value)
}

func oneID(args arguments, usage string) (string, error) {
	if len(args.positionals) != 1 {
		return "", sandbox.Errorf(sandbox.CodeInvalidInput, "usage: %s", usage)
	}
	return args.positionals[0], nil
}
