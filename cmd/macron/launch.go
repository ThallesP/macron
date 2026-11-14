package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ThallesP/macron/fixes"
	"github.com/ThallesP/macron/internal/failure"
	"github.com/ThallesP/macron/internal/launch"
	"github.com/ThallesP/macron/internal/wineruntime"
)

func launchCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("launch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	gameID := flags.String("game", "", "profile id from the fixes database")
	if err := flags.Parse(args); err != nil || *gameID == "" || flags.NArg() == 0 {
		fmt.Fprint(stderr, "usage: macron launch --game <id> -- <exe> [args]\n")
		return exitUsage
	}

	db, err := fixes.Load()
	if err != nil {
		return fail(stderr, err)
	}
	profile, err := db.Get(*gameID)
	if err != nil {
		return fail(stderr, err)
	}
	return startGame(ctx, gameStart{Profile: profile, Exe: flags.Arg(0), Args: flags.Args()[1:], WaitForWine: true}, stderr)
}

func killCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("kill", flag.ContinueOnError)
	flags.SetOutput(stderr)
	gameID := flags.String("game", "", "profile id whose Windows environment to stop")
	if err := flags.Parse(args); err != nil || *gameID == "" {
		fmt.Fprint(stderr, "usage: macron kill --game <id>\n")
		return exitUsage
	}

	layout, err := wineruntime.DefaultLayout()
	if err != nil {
		return fail(stderr, err)
	}
	installed, err := wineruntime.Find(layout, wineruntime.Pinned)
	if err != nil {
		return fail(stderr, err)
	}
	if err := launch.Kill(ctx, installed, layout.PrefixDir(*gameID)); err != nil {
		return fail(stderr, err)
	}
	return 0
}

type gameStart struct {
	Profile     fixes.Profile
	Prefix      string
	Exe         string
	Args        []string
	WaitForWine bool
}

func startGame(ctx context.Context, in gameStart, stderr io.Writer) int {
	layout, err := wineruntime.DefaultLayout()
	if err != nil {
		return fail(stderr, err)
	}
	prefix := in.Prefix
	if prefix == "" {
		prefix = layout.PrefixDir(in.Profile.ID)
	}
	logDir := filepath.Join(layout.LogsDir(), in.Profile.ID)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fail(stderr, err)
	}
	logFile, err := os.Create(filepath.Join(logDir, time.Now().UTC().Format("20060102T150405Z")+".log"))
	if err != nil {
		return fail(stderr, err)
	}
	defer logFile.Close()
	output := io.MultiWriter(stderr, logFile)
	progress := progressTo(output)
	progress(fmt.Sprintf("game %s, log %s", in.Profile.ID, logFile.Name()))

	installed, err := wineruntime.Install(ctx, layout, wineruntime.Pinned, progress)
	if err != nil {
		return fail(output, err)
	}
	exitCode, err := launch.Run(ctx, launch.Input{
		Runtime:     installed,
		Profile:     in.Profile,
		Prefix:      prefix,
		Exe:         in.Exe,
		Args:        in.Args,
		Output:      output,
		Progress:    progress,
		WaitForWine: in.WaitForWine,
	})
	if err != nil {
		return fail(output, err)
	}
	return exitCode
}

func progressTo(w io.Writer) func(string) {
	return func(message string) {
		fmt.Fprintf(w, "macron: %s\n", message)
	}
}

func fail(w io.Writer, err error) int {
	code := failure.CodeOf(err)
	if code == "" {
		code = "UNEXPECTED"
	}
	fmt.Fprintf(w, "macron: %s: %s\n", code, err)
	return exitFailed
}
