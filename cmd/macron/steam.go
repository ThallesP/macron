package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ThallesP/macron/internal/steamhook"
)

const steamUsage = "usage: macron steam status|enable|disable [--steam-dir <dir>] [--json]\n"

func steamCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, steamUsage)
		return exitUsage
	}
	verb := args[0]

	flags := flag.NewFlagSet("steam", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	steamDir := flags.String("steam-dir", defaultSteamDir(), "")
	asJSON := flags.Bool("json", false, "")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
		fmt.Fprint(stderr, steamUsage)
		return exitUsage
	}

	macronPath, err := currentExecutable()
	if err != nil {
		return fail(stderr, err)
	}
	hook := steamhook.Hook{SteamDir: *steamDir, MacronPath: macronPath, IsSteamRunning: isSteamRunning}
	actions := map[string]func() (steamhook.Status, error){
		"status":  hook.Status,
		"enable":  hook.Enable,
		"disable": hook.Disable,
	}
	action, isKnown := actions[verb]
	if !isKnown {
		fmt.Fprint(stderr, steamUsage)
		return exitUsage
	}

	status, err := action()
	if err != nil {
		return fail(stderr, err)
	}
	if *asJSON {
		json.NewEncoder(stdout).Encode(status)
		return 0
	}
	fmt.Fprintf(stdout, "steam dir:       %s\n", status.SteamDir)
	fmt.Fprintf(stdout, "build:           %s (known: %t)\n", status.Build.ID, status.Build.IsKnown)
	fmt.Fprintf(stdout, "steamclient:     sha256 %s\n", status.Build.SHA256["steamclient.dylib"])
	fmt.Fprintf(stdout, "steam_osx:       sha256 %s\n", status.Build.SHA256["steam_osx"])
	fmt.Fprintf(stdout, "tool registered: %t\n", status.IsToolRegistered)
	fmt.Fprintf(stdout, "default tool:    %t\n", status.IsDefaultTool)
	fmt.Fprintf(stdout, "backup:          %t\n", status.HasBackup)
	return 0
}

func defaultSteamDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Application Support", "Steam")
}

func currentExecutable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

func isSteamRunning() bool {
	return exec.Command("pgrep", "-x", "steam_osx").Run() == nil
}
