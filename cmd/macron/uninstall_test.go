package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ThallesP/macron/internal/failure"
	"github.com/ThallesP/macron/internal/steamhook"
	"github.com/ThallesP/macron/internal/wineruntime"
)

func noTools(name string, args ...string) (string, error) {
	return "", errors.New(name + " is not on this machine")
}

const configBeforeMacron = `"InstallConfigStore"
{
	"Software"
	{
		"Valve"
		{
			"Steam"
			{
				"CompatToolMapping"
				{
					"0"
					{
						"name"		"someothertool"
						"config"		""
						"priority"		"75"
					}
				}
			}
		}
	}
}
`

const configWithMacron = `"InstallConfigStore"
{
	"Software"
	{
		"Valve"
		{
			"Steam"
			{
				"CompatToolMapping"
				{
					"0"
					{
						"name"		"macron"
						"config"		""
						"priority"		"75"
					}
				}
			}
		}
	}
}
`

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hookedSteam(t *testing.T) steamhook.Hook {
	t.Helper()
	steamDir := t.TempDir()
	clientDir := filepath.Join(steamDir, "Steam.AppBundle", "Steam", "Contents", "MacOS")
	writeTestFile(t, filepath.Join(clientDir, "steamclient.dylib"), "a Steam build macron does not know")
	writeTestFile(t, filepath.Join(clientDir, "steam_osx"), "a Steam build macron does not know")
	writeTestFile(t, filepath.Join(steamDir, "config", "config.vdf"), configWithMacron)
	writeTestFile(t, filepath.Join(steamDir, "config", "config.vdf.before-macron"), configBeforeMacron)
	writeTestFile(t, filepath.Join(steamDir, "compatibilitytools.d", "macron", "compatibilitytool.vdf"), "tool")
	return steamhook.Hook{SteamDir: steamDir, SteamApp: filepath.Join(steamDir, "Steam.app"), BackupDir: filepath.Join(steamDir, "backup"), IsSteamRunning: func() bool { return false }, Run: noTools}
}

func macronHome(t *testing.T) wineruntime.Layout {
	t.Helper()
	layout := wineruntime.Layout{Home: filepath.Join(t.TempDir(), "Macron")}
	writeTestFile(t, filepath.Join(layout.RuntimeDir("11.21"), "wine", "bin", "wine"), "wine")
	writeTestFile(t, filepath.Join(layout.PrefixDir("openttd"), "drive_c", "save.dat"), "save")
	writeTestFile(t, filepath.Join(layout.LogsDir(), "openttd", "run.log"), "log")
	writeTestFile(t, filepath.Join(layout.DownloadsDir(), "wine.tar.xz"), "archive")
	writeTestFile(t, layout.ShowFPSFile(), "")
	writeTestFile(t, layout.GameModeFile(), "")
	writeTestFile(t, layout.DLSSFile(), "")
	return layout
}

func TestUninstallPutsSteamBackAndRemovesMacronsFiles(t *testing.T) {
	hook := hookedSteam(t)
	layout := macronHome(t)
	var stdout bytes.Buffer

	if err := uninstall(hook, layout, &stdout); err != nil {
		t.Fatal(err)
	}

	config, err := os.ReadFile(filepath.Join(hook.SteamDir, "config", "config.vdf"))
	if err != nil || string(config) != configBeforeMacron {
		t.Errorf("config.vdf is not back to its old content (%v):\n%s", err, config)
	}
	for _, path := range []string{
		filepath.Join(hook.SteamDir, "config", "config.vdf.before-macron"),
		filepath.Join(hook.SteamDir, "compatibilitytools.d"),
		layout.Home,
	} {
		if _, err := os.Lstat(path); err == nil {
			t.Errorf("%s is still there", path)
		}
	}
	for _, path := range layout.CreatedPaths() {
		if !strings.Contains(stdout.String(), "removing "+path+"\n") {
			t.Errorf("did not say it removes %s:\n%s", path, stdout.String())
		}
	}
}

func TestUninstallLeavesWhatMacronDidNotCreate(t *testing.T) {
	layout := macronHome(t)
	notes := filepath.Join(layout.Home, "my-notes.txt")
	writeTestFile(t, notes, "mine")
	outside := filepath.Join(t.TempDir(), "documents")
	writeTestFile(t, filepath.Join(outside, "letter.txt"), "mine")
	if err := os.Symlink(outside, filepath.Join(layout.PrefixDir("openttd"), "dosdevices-z")); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer

	if err := uninstall(hookedSteam(t), layout, &stdout); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{notes, filepath.Join(outside, "letter.txt")} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s was removed: %v", path, err)
		}
	}
	if !strings.Contains(stdout.String(), "left "+notes) {
		t.Errorf("did not say it left %s:\n%s", notes, stdout.String())
	}
	for _, path := range layout.CreatedPaths() {
		if _, err := os.Lstat(path); err == nil {
			t.Errorf("%s is still there", path)
		}
	}
}

func TestUninstallRefusesWhileSteamRunsAndRemovesNothing(t *testing.T) {
	hook := hookedSteam(t)
	hook.IsSteamRunning = func() bool { return true }
	layout := macronHome(t)
	var stdout bytes.Buffer

	err := uninstall(hook, layout, &stdout)

	if failure.CodeOf(err) != failure.CodeSteamRunning {
		t.Fatalf("got %v, want %s", err, failure.CodeSteamRunning)
	}
	for _, path := range append(layout.CreatedPaths(), filepath.Join(hook.SteamDir, "compatibilitytools.d", "macron")) {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("%s was removed", path)
		}
	}
}

func TestUninstallWithoutSteamHookIgnoresRunningSteam(t *testing.T) {
	hook := steamhook.Hook{SteamDir: t.TempDir(), IsSteamRunning: func() bool { return true }, Run: noTools}
	layout := macronHome(t)
	var stdout bytes.Buffer

	if err := uninstall(hook, layout, &stdout); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(stdout.String(), "macron is not registered") {
		t.Errorf("stdout: %s", stdout.String())
	}
	if _, err := os.Lstat(layout.Home); err == nil {
		t.Errorf("%s is still there", layout.Home)
	}
}

func TestUninstallWhenNothingIsInstalled(t *testing.T) {
	hook := steamhook.Hook{SteamDir: t.TempDir(), IsSteamRunning: func() bool { return false }, Run: noTools}
	layout := wineruntime.Layout{Home: filepath.Join(t.TempDir(), "Macron")}
	var stdout bytes.Buffer

	if err := uninstall(hook, layout, &stdout); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(stdout.String(), "Nothing of macron's in "+layout.Home) {
		t.Errorf("stdout: %s", stdout.String())
	}
}
