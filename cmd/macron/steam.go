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
	"github.com/ThallesP/macron/internal/wineruntime"
)

const steamUsage = "usage: macron steam status|enable|disable [--steam-dir <dir>] [--steam-app <Steam.app>] [--json]\n"

var switchUndoLines = map[steamhook.SwitchUndo]string{
	steamhook.SwitchUndoRestored: "Steam.app:       Valve's files are back, byte for byte",
	steamhook.SwitchUndoStripped: "Steam.app:       changed since macron enabled it, so macron took its own settings out and signed it ad hoc; reinstall Steam from Valve's DMG for Valve's signature",
	steamhook.SwitchUndoNothing:  "Steam.app:       nothing of macron's to undo",
}

func steamCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, steamUsage)
		return exitUsage
	}
	verb := args[0]

	flags := flag.NewFlagSet("steam", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	steamDir := flags.String("steam-dir", defaultSteamDir(), "")
	steamApp := flags.String("steam-app", defaultSteamApp, "")
	asJSON := flags.Bool("json", false, "")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
		fmt.Fprint(stderr, steamUsage)
		return exitUsage
	}

	hook, err := newHook(*steamDir, *steamApp)
	if err != nil {
		return fail(stderr, err)
	}
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
	fmt.Fprintf(stdout, "steam app:       %s\n", status.SteamApp)
	fmt.Fprintf(stdout, "steam play:      %s\n", map[bool]string{true: "on", false: "off"}[status.IsSwitchOn])
	if verb == "disable" {
		fmt.Fprintln(stdout, switchUndoLines[status.SwitchUndo])
	}
	return 0
}

const defaultSteamApp = "/Applications/Steam.app"

func newHook(steamDir string, steamApp string) (steamhook.Hook, error) {
	macronPath, err := currentExecutable()
	if err != nil {
		return steamhook.Hook{}, err
	}
	layout, err := wineruntime.DefaultLayout()
	if err != nil {
		return steamhook.Hook{}, err
	}
	return steamhook.Hook{
		SteamDir:       steamDir,
		SteamApp:       steamApp,
		MacronPath:     macronPath,
		SwitchLibrary:  switchLibraryPath(macronPath),
		BackupDir:      layout.SteamAppBackupDir(),
		IsSteamRunning: isSteamRunning,
		Run:            runTool,
	}, nil
}

func switchLibraryPath(macronPath string) string {
	if fromEnv := os.Getenv("MACRON_STEAMPLAY_LIBRARY"); fromEnv != "" {
		return fromEnv
	}
	beside := filepath.Join(filepath.Dir(macronPath), steamhook.SwitchLibraryName)
	if _, err := os.Stat(beside); err == nil {
		return beside
	}
	return filepath.Join(filepath.Dir(macronPath), "..", "lib", "macron", steamhook.SwitchLibraryName)
}

func runTool(name string, args ...string) (string, error) {
	output, err := exec.Command(name, args...).CombinedOutput()
	return string(output), err
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
