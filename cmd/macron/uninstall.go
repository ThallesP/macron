package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ThallesP/macron/internal/failure"
	"github.com/ThallesP/macron/internal/steamhook"
	"github.com/ThallesP/macron/internal/wineruntime"
)

const uninstallUsage = "usage: macron uninstall [--steam-dir <dir>]\n"

func uninstallCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	steamDir := flags.String("steam-dir", defaultSteamDir(), "")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprint(stderr, uninstallUsage)
		return exitUsage
	}
	if os.Geteuid() == 0 {
		return fail(stderr, failure.Errorf(failure.CodeInvalidInput, "run macron uninstall as yourself, not as root: it cleans your home folder and your Steam. sudo macron-uninstall does that for the logged-in user and also removes the program"))
	}

	layout, err := wineruntime.DefaultLayout()
	if err != nil {
		return fail(stderr, err)
	}
	hook := steamhook.Hook{SteamDir: *steamDir, IsSteamRunning: isSteamRunning}
	if err := uninstall(hook, layout, stdout); err != nil {
		return fail(stderr, err)
	}

	macronPath, err := currentExecutable()
	if err != nil {
		return fail(stderr, err)
	}
	uninstaller := filepath.Join(filepath.Dir(macronPath), "macron-uninstall")
	if _, err := os.Stat(uninstaller); err == nil {
		fmt.Fprintf(stdout, "The program itself is still at %s: sudo %s removes it.\n", macronPath, uninstaller)
		return 0
	}
	fmt.Fprintf(stdout, "The program itself is still at %s: delete it to finish.\n", macronPath)
	return 0
}

func uninstall(hook steamhook.Hook, layout wineruntime.Layout, stdout io.Writer) error {
	isHooked, err := hook.IsHooked()
	if err != nil {
		return err
	}
	if isHooked {
		if _, err := hook.Disable(); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Steam: removed macron's compatibility tool and put config.vdf's old mapping back (%s)\n", hook.SteamDir)
	}
	if !isHooked {
		fmt.Fprintf(stdout, "Steam: macron is not registered (%s)\n", hook.SteamDir)
	}

	var created []string
	for _, path := range layout.CreatedPaths() {
		if _, err := os.Lstat(path); err == nil {
			created = append(created, path)
		}
	}
	if len(created) == 0 {
		fmt.Fprintf(stdout, "Nothing of macron's in %s\n", layout.Home)
	}
	for _, path := range created {
		fmt.Fprintf(stdout, "removing %s\n", path)
	}
	for _, path := range created {
		if err := os.RemoveAll(path); err != nil {
			return failure.Errorf(failure.CodeFileFailed, "could not remove %s: %v", path, err)
		}
	}

	if err := os.Remove(layout.Home); err == nil {
		fmt.Fprintf(stdout, "removed %s\n", layout.Home)
		return nil
	}
	others, err := os.ReadDir(layout.Home)
	if err != nil {
		return nil
	}
	for _, other := range others {
		fmt.Fprintf(stdout, "left %s: macron did not create it\n", filepath.Join(layout.Home, other.Name()))
	}
	return nil
}
