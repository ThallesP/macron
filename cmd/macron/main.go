package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"syscall"
)

const usage = `macron runs Windows games on macOS.

  macron runtime install|path       download and unpack the pinned runtime, or print where it is
  macron launch --game <id> -- <exe> [args]
                                    start a game with its profile from the fixes database
  macron kill --game <id>           stop everything running in a game's Windows environment
  macron waitforexitandrun <exe> [args]
  macron run <exe> [args]           what Steam calls; the profile comes from SteamAppId

Wrong usage exits 2. A failure inside macron exits 125; otherwise macron exits
with the game's exit code.
`

const (
	exitUsage  = 2
	exitFailed = 125
)

type command func(ctx context.Context, args []string, stdout, stderr io.Writer) int

func commands() map[string]command {
	return map[string]command{
		"runtime":           runtimeCommand,
		"launch":            launchCommand,
		"kill":              killCommand,
		"waitforexitandrun": steamVerb(true),
		"run":               steamVerb(false),
		"steam":             steamCommand,
	}
}

func main() {
	ctx, release := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer release()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || slices.Contains([]string{"help", "-h", "--help"}, args[0]) {
		fmt.Fprint(stdout, usage)
		return 0
	}

	selected, isKnown := commands()[args[0]]
	if !isKnown {
		fmt.Fprintf(stderr, "macron: unknown command %q; run macron help\n", args[0])
		return exitUsage
	}
	return selected(ctx, args[1:], stdout, stderr)
}
