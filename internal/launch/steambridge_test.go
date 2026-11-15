package launch

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ThallesP/macron/fixes"
	"github.com/ThallesP/macron/internal/wineruntime"
)

func addSteamBridge(t *testing.T, runtime wineruntime.Installed, suffix string) {
	t.Helper()
	for _, tree := range []wineruntime.Tree{wineruntime.TreePlain, wineruntime.TreeDXMT} {
		for path, content := range map[string]string{
			"lib/wine/i386-windows/lsteamclient.dll":   "lsteamclient 32",
			"lib/wine/x86_64-windows/lsteamclient.dll": "lsteamclient 64",
			"lib/wine/x86_64-windows/steam.exe":        "steam helper",
			"lib/wine/x86_64-unix/lsteamclient.so":     "lsteamclient unix",
		} {
			file := filepath.Join(runtime.Root(tree), path)
			if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte(content+suffix), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func steamInput(t *testing.T, g game, runtime wineruntime.Installed) Input {
	t.Helper()
	return Input{
		Runtime:     runtime,
		Profile:     fixes.Default("871720"),
		Prefix:      g.prefix,
		Exe:         g.exe,
		Args:        []string{"-from-steam"},
		Output:      &bytes.Buffer{},
		Progress:    func(string) {},
		SteamBridge: true,
	}
}

func TestSteamStartsTheGameThroughSteamExe(t *testing.T) {
	g := newGame(t)
	runtime := fakeRuntime(t)
	addSteamBridge(t, runtime, "")

	exitCode, err := Run(context.Background(), steamInput(t, g, runtime))
	if err != nil {
		t.Fatal(err)
	}

	if exitCode != 3 {
		t.Errorf("exit code %d, want the game's 3 passed on by steam.exe", exitCode)
	}
	if calls := g.read(t, "wine-calls.txt"); !strings.Contains(calls, `C:\windows\system32\steam.exe `+g.exe+" -from-steam") {
		t.Errorf("the game was not started through steam.exe:\n%s", calls)
	}
	for path, want := range map[string]string{
		"Program Files (x86)/Steam/steamclient.dll":   "lsteamclient 32",
		"Program Files (x86)/Steam/steamclient64.dll": "lsteamclient 64",
		"Program Files (x86)/Steam/steam.exe":         "steam helper",
		"windows/system32/steam.exe":                  "steam helper",
	} {
		if got := g.read(t, filepath.Join("prefix/drive_c", path)); got != want {
			t.Errorf("%s = %q, want %q", path, got, want)
		}
	}
}

func TestSteamBridgeRegistryIsImportedOnce(t *testing.T) {
	g := newGame(t)
	runtime := fakeRuntime(t)
	addSteamBridge(t, runtime, "")

	for range 2 {
		if _, err := Run(context.Background(), steamInput(t, g, runtime)); err != nil {
			t.Fatal(err)
		}
	}

	if imports := strings.Count(g.read(t, "wine-calls.txt"), `regedit /S C:\macron-steam-bridge.reg`); imports != 1 {
		t.Errorf("the Steam bridge's registry keys were imported %d times, want 1", imports)
	}
}

func TestTheBridgeInThePrefixFollowsTheRuntime(t *testing.T) {
	g := newGame(t)
	runtime := fakeRuntime(t)
	addSteamBridge(t, runtime, "")
	if _, err := Run(context.Background(), steamInput(t, g, runtime)); err != nil {
		t.Fatal(err)
	}
	addSteamBridge(t, runtime, ", newer")

	if _, err := Run(context.Background(), steamInput(t, g, runtime)); err != nil {
		t.Fatal(err)
	}

	if got := g.read(t, "prefix/drive_c/Program Files (x86)/Steam/steamclient64.dll"); got != "lsteamclient 64, newer" {
		t.Errorf("steamclient64.dll = %q, want the runtime's newer build", got)
	}
}

func TestARuntimeWithoutTheBridgeStartsTheGameDirectly(t *testing.T) {
	g := newGame(t)
	var messages []string
	in := steamInput(t, g, fakeRuntime(t))
	in.Progress = func(message string) { messages = append(messages, message) }

	if _, err := Run(context.Background(), in); err != nil {
		t.Fatal(err)
	}

	if calls := g.read(t, "wine-calls.txt"); strings.Contains(calls, "steam.exe") || !strings.Contains(calls, g.exe+" -from-steam") {
		t.Errorf("wine calls:\n%s", calls)
	}
	if _, err := os.Stat(filepath.Join(g.prefix, "drive_c", "Program Files (x86)", "Steam")); err == nil {
		t.Error("a Steam folder was made without a bridge to put in it")
	}
	if !slices.Contains(messages, "this runtime has no Steam bridge, so the game cannot reach Steam") {
		t.Errorf("the missing bridge was not reported: %q", messages)
	}
}

func TestGamesNotStartedBySteamDoNotGetTheBridge(t *testing.T) {
	g := newGame(t)
	runtime := fakeRuntime(t)
	addSteamBridge(t, runtime, "")
	in := steamInput(t, g, runtime)
	in.SteamBridge = false

	if _, err := Run(context.Background(), in); err != nil {
		t.Fatal(err)
	}

	if calls := g.read(t, "wine-calls.txt"); strings.Contains(calls, "steam.exe") {
		t.Errorf("wine calls:\n%s", calls)
	}
}

func TestSteamBridgeEnvFillsWhatSteamDidNotSet(t *testing.T) {
	got := steamBridgeEnv([]string{"HOME=/Users/player", "SteamAppId=871720"})

	want := []string{"SteamGameId=871720", "STEAM_COMPAT_CLIENT_INSTALL_PATH=/Users/player/Library/Application Support/Steam"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSteamBridgeEnvKeepsWhatSteamSet(t *testing.T) {
	got := steamBridgeEnv([]string{
		"HOME=/Users/player",
		"SteamAppId=871720",
		"SteamGameId=871720",
		"STEAM_COMPAT_CLIENT_INSTALL_PATH=/Users/player/Library/Application Support/Steam",
	})

	if len(got) != 0 {
		t.Errorf("got %q, want nothing added", got)
	}
}

func TestSteamBridgeEnvTakesTheAppIDFromSteamsCompatVariable(t *testing.T) {
	got := steamBridgeEnv([]string{"HOME=/Users/player", "STEAM_COMPAT_APP_ID=480", "STEAM_COMPAT_CLIENT_INSTALL_PATH=/steam"})

	want := []string{"SteamGameId=480", "SteamAppId=480"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSteamBridgeRegistryPointsGamesAtTheBridge(t *testing.T) {
	reg := RegFile(steamBridgeRegistry)

	for _, want := range []string{
		"[HKEY_CURRENT_USER\\Software\\Valve\\Steam\\ActiveProcess]\r\n",
		`"SteamClientDll"="C:\\Program Files (x86)\\Steam\\steamclient.dll"`,
		`"SteamClientDll64"="C:\\Program Files (x86)\\Steam\\steamclient64.dll"`,
		"[HKEY_LOCAL_MACHINE\\Software\\Wow6432Node\\Valve\\Steam]\r\n",
	} {
		if !strings.Contains(reg, want) {
			t.Errorf("the registry file lacks %s:\n%s", want, reg)
		}
	}
	if strings.Count(reg, "[HKEY_CURRENT_USER\\Software\\Valve\\Steam\\ActiveProcess]") != 1 {
		t.Errorf("ActiveProcess is split into several sections:\n%s", reg)
	}
}
