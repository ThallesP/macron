package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"

	"github.com/ThallesP/macron/fixes"
	"github.com/ThallesP/macron/internal/check"
	"github.com/ThallesP/macron/internal/imagecheck"
	"github.com/ThallesP/macron/internal/scenario"
)

const usage = `macron-check runs a scenario: a Windows program started through macron,
checked against a screenshot of the same program on Windows.

  macron-check sandbox <scenario> [--keep]
                     from the dev machine: the whole check in a fresh sbx Mac sandbox
  macron-check run <scenario-dir> --out <dir>
                     on a Mac: install, launch, wait, capture, compare
  macron-check reference <scenario-dir> --out <dir>
                     on Windows: capture the reference from the original program
  macron-check compare <reference.png> <frame.png>
                     score two pictures with the scenario check's measure

A failed check exits 1, wrong usage 2, a broken setup 125.
`

const (
	exitCheckFailed = 1
	exitUsage       = 2
	exitBroken      = 125
)

type command func(ctx context.Context, args []string, stdout, stderr io.Writer) int

func main() {
	ctx, release := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer release()
	commands := map[string]command{
		"sandbox":   sandboxCommand,
		"run":       runCommand,
		"reference": referenceCommand,
		"compare":   compareCommand,
	}
	args := os.Args[1:]
	if len(args) == 0 || slices.Contains([]string{"help", "-h", "--help"}, args[0]) {
		fmt.Print(usage)
		return
	}
	selected, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(os.Stderr, "macron-check: unknown command %q\n", args[0])
		os.Exit(exitUsage)
	}
	os.Exit(selected(ctx, args[1:], os.Stdout, os.Stderr))
}

func parse(name string, args []string, stderr io.Writer, define func(*flag.FlagSet)) (*flag.FlagSet, bool) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	define(flags)
	positional := []string{}
	for len(args) > 0 {
		if err := flags.Parse(args); err != nil {
			return nil, false
		}
		args = flags.Args()
		if len(args) > 0 {
			positional = append(positional, args[0])
			args = args[1:]
		}
	}
	flags.Parse(append([]string{"--"}, positional...))
	return flags, true
}

func progressTo(w io.Writer) func(string) {
	return func(message string) {
		fmt.Fprintf(w, "macron-check: %s\n", message)
	}
}

func cacheDir() string {
	if dir := os.Getenv("MACRON_CHECK_CACHE"); dir != "" {
		return dir
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "macron-check-cache")
	}
	return filepath.Join(dir, "macron-check")
}

func runCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var out, macron string
	flags, ok := parse("run", args, stderr, func(f *flag.FlagSet) {
		f.StringVar(&out, "out", "", "where to put the evidence")
		f.StringVar(&macron, "macron", "", "the macron binary (default: next to macron-check)")
	})
	if !ok || flags.NArg() != 1 || out == "" {
		fmt.Fprint(stderr, "usage: macron-check run <scenario-dir> --out <dir> [--macron <path>]\n")
		return exitUsage
	}
	s, err := scenario.Load(flags.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "macron-check: %s\n", err)
		return exitBroken
	}
	if macron == "" {
		self, err := os.Executable()
		if err != nil {
			fmt.Fprintf(stderr, "macron-check: %s\n", err)
			return exitBroken
		}
		macron = filepath.Join(filepath.Dir(self), "macron")
	}
	otherGame := ""
	if s.OtherGame != "" {
		other, err := scenario.Load(filepath.Join(filepath.Dir(s.Dir), s.OtherGame))
		if err != nil {
			fmt.Fprintf(stderr, "macron-check: %s\n", err)
			return exitBroken
		}
		otherGame = other.ReferencePath()
	}

	report := check.RunOnMac(ctx, check.MacInput{
		Scenario:  s,
		Macron:    macron,
		Out:       out,
		CacheDir:  cacheDir(),
		OtherGame: otherGame,
		Progress:  progressTo(stderr),
	})
	if err := report.Write(out); err != nil {
		fmt.Fprintf(stderr, "macron-check: writing the report: %s\n", err)
		return exitBroken
	}
	fmt.Fprint(stdout, report.Summary())
	if !report.Passed {
		return exitCheckFailed
	}
	return 0
}

func referenceCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var out string
	flags, ok := parse("reference", args, stderr, func(f *flag.FlagSet) {
		f.StringVar(&out, "out", "", "where to put the captured reference")
	})
	if !ok || flags.NArg() != 1 || out == "" {
		fmt.Fprint(stderr, "usage: macron-check reference <scenario-dir> --out <dir>\n")
		return exitUsage
	}
	s, err := scenario.Load(flags.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "macron-check: %s\n", err)
		return exitBroken
	}
	db, err := fixes.Load()
	if err != nil {
		fmt.Fprintf(stderr, "macron-check: %s\n", err)
		return exitBroken
	}
	profile, err := db.Get(s.Game)
	if err != nil {
		fmt.Fprintf(stderr, "macron-check: %s\n", err)
		return exitBroken
	}

	report := check.CaptureReference(ctx, check.ReferenceInput{Scenario: s, Profile: profile, Out: out, CacheDir: cacheDir(), Progress: progressTo(stderr)})
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return exitBroken
	}
	os.WriteFile(filepath.Join(out, "reference-report.json"), content, 0o644)
	fmt.Fprintf(stdout, "%s\n", content)
	if !report.Captured {
		return exitCheckFailed
	}
	return 0
}

func compareCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprint(stderr, "usage: macron-check compare <reference.png> <frame.png>\n")
		return exitUsage
	}
	reference, err := check.ReadPNG(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "macron-check: %s\n", err)
		return exitBroken
	}
	frame, err := check.ReadPNG(args[1])
	if err != nil {
		fmt.Fprintf(stderr, "macron-check: %s\n", err)
		return exitBroken
	}
	score := imagecheck.Compare(reference, frame, nil)
	fmt.Fprintf(stdout, "SSIM %.3f (needs >= %.2f), colour distance %.3f (needs <= %.2f): passes=%t\n",
		score.SSIM, imagecheck.MinSSIM, score.ColorDistance, imagecheck.MaxColorDistance, score.Passes())
	if !score.Passes() {
		return exitCheckFailed
	}
	return 0
}
