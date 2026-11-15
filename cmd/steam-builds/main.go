package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"github.com/ThallesP/macron/internal/steamhook"
)

const usage = `steam-builds reads Valve's update manifest for the Mac Steam client, hashes
steamclient.dylib and the steam_osx of every variant of the current build, and
prints their rows for KnownBuilds (internal/steamhook/builds.go).

  steam-builds [--check] [--write <builds.go>]

  --check    exit 1 when KnownBuilds lacks a row of the current build
  --write    rewrite that builds.go with the missing rows added

A missing row with --check exits 1, wrong usage 2, any other failure 125.
`

const (
	exitMissingRows = 1
	exitUsage       = 2
	exitFailed      = 125
)

func main() {
	ctx, release := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer release()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("steam-builds", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	isCheck := flags.Bool("check", false, "")
	writePath := flags.String("write", "", "")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}

	cacheDir, err := os.MkdirTemp("", "steam-builds-")
	if err != nil {
		fmt.Fprintf(stderr, "steam-builds: %s\n", err)
		return exitFailed
	}
	defer os.RemoveAll(cacheDir)

	release, err := steamhook.FetchRelease(ctx, steamhook.UpdateCDN, cacheDir)
	if err != nil {
		fmt.Fprintf(stderr, "steam-builds: %s\n", err)
		return exitFailed
	}
	rows, err := steamhook.RenderRows(release.Builds)
	if err != nil {
		fmt.Fprintf(stderr, "steam-builds: %s\n", err)
		return exitFailed
	}
	fmt.Fprint(stdout, rows)

	fmt.Fprintf(stderr, "steam-builds: Valve's current Mac build is %s\n", release.ID)
	for _, variant := range release.SkippedVariants {
		fmt.Fprintf(stderr, "steam-builds: %s: its package has no steam_osx, so no row\n", variant)
	}
	var missing []steamhook.KnownBuild
	for _, build := range release.Builds {
		if build.IsIn(steamhook.KnownBuilds) {
			fmt.Fprintf(stderr, "steam-builds: %s: in KnownBuilds\n", build.Variant)
			continue
		}
		fmt.Fprintf(stderr, "steam-builds: %s: not in KnownBuilds\n", build.Variant)
		missing = append(missing, build)
	}

	if *writePath != "" && len(missing) > 0 {
		table, err := steamhook.RenderTable(slices.Concat(steamhook.KnownBuilds, missing))
		if err != nil {
			fmt.Fprintf(stderr, "steam-builds: %s\n", err)
			return exitFailed
		}
		if err := os.WriteFile(*writePath, table, 0o644); err != nil {
			fmt.Fprintf(stderr, "steam-builds: %s\n", err)
			return exitFailed
		}
		fmt.Fprintf(stderr, "steam-builds: added the missing rows to %s\n", *writePath)
	}
	if *isCheck && len(missing) > 0 {
		return exitMissingRows
	}
	return 0
}
