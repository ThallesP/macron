package launch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ThallesP/macron/fixes"
	"github.com/ThallesP/macron/internal/failure"
	"github.com/ThallesP/macron/internal/wineruntime"
)

const fakeWine = `#!/bin/sh
echo "$*" >> "$WINEPREFIX/../wine-calls.txt"
if [ "$1" = wineboot ]; then
	home="$WINEPREFIX/drive_c/users/$USER"
	mkdir -p "$home/AppData/Roaming" "$WINEPREFIX/../mac-documents" "$WINEPREFIX/drive_c/windows/system32" "$WINEPREFIX/drive_c/windows/syswow64"
	ln -s "$WINEPREFIX/../mac-documents" "$home/Documents"
	exit 0
fi
env | grep -E '^(WINE|DXMT)' | sort > "$WINEPREFIX/../game-env.txt"
pwd > "$WINEPREFIX/../game-cwd.txt"
echo "game output"
exit 3
`

func fakeRuntime(t *testing.T) wineruntime.Installed {
	t.Helper()
	installed := wineruntime.Installed{Version: "test-1", Dir: t.TempDir()}
	for _, tree := range []wineruntime.Tree{wineruntime.TreePlain, wineruntime.TreeDXMT} {
		bin := filepath.Join(installed.Root(tree), "bin")
		if err := os.MkdirAll(bin, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(bin, "wine"), []byte(fakeWine), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(bin, "wineserver"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return installed
}

type game struct {
	root   string
	prefix string
	exe    string
}

func newGame(t *testing.T) game {
	t.Helper()
	t.Setenv("USER", "player")
	root := t.TempDir()
	exe := filepath.Join(root, "Game", "game.exe")
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	return game{root: root, prefix: filepath.Join(root, "prefix"), exe: exe}
}

func (g game) read(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(g.root, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func TestRunPassesTheGamesExitCodeAndProfile(t *testing.T) {
	g := newGame(t)
	var output bytes.Buffer
	profile := fixes.Profile{
		ID:           "some-game",
		Backend:      fixes.BackendDXMT,
		Args:         []string{"-windowed"},
		Env:          map[string]string{"DXMT_LOG_LEVEL": "info"},
		DLLOverrides: map[string]string{"d3d9": "n,b"},
		Files:        []fixes.File{{Path: "{appdata}/Some Game/settings.ini", Content: "width=800"}},
	}

	exitCode, err := Run(context.Background(), Input{
		Runtime:  fakeRuntime(t),
		Profile:  profile,
		Prefix:   g.prefix,
		Exe:      g.exe,
		Args:     []string{"-from-steam"},
		Output:   &output,
		Progress: func(string) {},
	})
	if err != nil {
		t.Fatal(err)
	}

	if exitCode != 3 {
		t.Errorf("exit code %d, want the game's 3", exitCode)
	}
	if !strings.Contains(output.String(), "game output") {
		t.Errorf("game output not passed on: %q", output.String())
	}
	calls := g.read(t, "wine-calls.txt")
	if !strings.Contains(calls, g.exe+" -from-steam -windowed") {
		t.Errorf("wine calls:\n%s", calls)
	}
	env := g.read(t, "game-env.txt")
	for _, want := range []string{"WINEDLLOVERRIDES=d3d9=n,b;mscoree=;mshtml=", "WINEPREFIX=" + g.prefix, "DXMT_LOG_LEVEL=info", "WINEDEBUG=-all"} {
		if !strings.Contains(env, want) {
			t.Errorf("game environment lacks %s:\n%s", want, env)
		}
	}
	if cwd := strings.TrimSpace(g.read(t, "game-cwd.txt")); cwd != filepath.Dir(g.exe) {
		t.Errorf("game started in %s", cwd)
	}
	if settings := g.read(t, "prefix/drive_c/users/player/AppData/Roaming/Some Game/settings.ini"); settings != "width=800" {
		t.Errorf("settings.ini = %q", settings)
	}
}

func TestPrefixIsPrivateAndBootedOnce(t *testing.T) {
	g := newGame(t)
	runtime := fakeRuntime(t)
	in := Input{Runtime: runtime, Profile: fixes.Default("x"), Prefix: g.prefix, Exe: g.exe, Output: &bytes.Buffer{}, Progress: func(string) {}}

	for range 2 {
		if _, err := Run(context.Background(), in); err != nil {
			t.Fatal(err)
		}
	}

	if boots := strings.Count(g.read(t, "wine-calls.txt"), "wineboot"); boots != 1 {
		t.Errorf("wineboot ran %d times", boots)
	}
	documents := filepath.Join(g.prefix, "drive_c/users/player/Documents")
	info, err := os.Lstat(documents)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		t.Errorf("Documents is still linked to the Mac's folder: %s", info.Mode())
	}
}

func TestChangingBackendRefreshesThePrefix(t *testing.T) {
	g := newGame(t)
	runtime := fakeRuntime(t)
	in := Input{Runtime: runtime, Profile: fixes.Default("x"), Prefix: g.prefix, Exe: g.exe, Output: &bytes.Buffer{}, Progress: func(string) {}}
	if _, err := Run(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	in.Profile.Backend = fixes.BackendWineD3D

	if _, err := Run(context.Background(), in); err != nil {
		t.Fatal(err)
	}

	if boots := strings.Count(g.read(t, "wine-calls.txt"), "wineboot"); boots != 2 {
		t.Errorf("wineboot ran %d times, want 2", boots)
	}
}

func TestDXVKBackendPutsItsDLLsInThePrefix(t *testing.T) {
	g := newGame(t)
	runtime := fakeRuntime(t)
	for bits, content := range map[wineruntime.DXVKBits]string{wineruntime.DXVK64: "dxvk 64", wineruntime.DXVK32: "dxvk 32"} {
		if err := os.MkdirAll(runtime.DXVK(bits), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runtime.DXVK(bits), "d3d11.dll"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	profile := fixes.Default("x")
	profile.Backend = fixes.BackendDXVK

	if _, err := Run(context.Background(), Input{Runtime: runtime, Profile: profile, Prefix: g.prefix, Exe: g.exe, Output: &bytes.Buffer{}, Progress: func(string) {}}); err != nil {
		t.Fatal(err)
	}

	if got := g.read(t, "prefix/drive_c/windows/system32/d3d11.dll"); got != "dxvk 64" {
		t.Errorf("system32/d3d11.dll = %q", got)
	}
	if got := g.read(t, "prefix/drive_c/windows/syswow64/d3d11.dll"); got != "dxvk 32" {
		t.Errorf("syswow64/d3d11.dll = %q", got)
	}
	if env := g.read(t, "game-env.txt"); !strings.Contains(env, "WINEDLLOVERRIDES=d3d10core=n;d3d11=n;dxgi=n;mscoree=;mshtml=") {
		t.Errorf("DXVK overrides missing:\n%s", env)
	}
}

func TestProfileFilesNeverOverwriteTheUsersChanges(t *testing.T) {
	g := newGame(t)
	places := Places(g.prefix, filepath.Dir(g.exe))
	files := []fixes.File{{Path: "{game}/config.cfg", Content: "fullscreen 0"}}
	if err := os.WriteFile(filepath.Join(filepath.Dir(g.exe), "config.cfg"), []byte("fullscreen 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := WriteFiles(files, places); err != nil {
		t.Fatal(err)
	}

	if got := g.read(t, "Game/config.cfg"); got != "fullscreen 1" {
		t.Errorf("config.cfg = %q", got)
	}
}

func TestMissingExe(t *testing.T) {
	g := newGame(t)

	_, err := Run(context.Background(), Input{Runtime: fakeRuntime(t), Profile: fixes.Default("x"), Prefix: g.prefix, Exe: g.exe + ".missing", Output: &bytes.Buffer{}, Progress: func(string) {}})

	if failure.CodeOf(err) != failure.CodeLaunchFailed {
		t.Fatalf("got %v", err)
	}
}

func TestExtraInstallsRunOncePerPrefix(t *testing.T) {
	g := newGame(t)
	installer := []byte("an installer")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(installer) }))
	t.Cleanup(server.Close)
	hash := sha256.Sum256(installer)
	installers := map[string]fixes.Installer{"thing": {Steps: []fixes.InstallStep{{
		Archive: wineruntime.Archive{Name: "setup.exe", URL: server.URL + "/setup.exe", SHA256: hex.EncodeToString(hash[:])},
		Args:    []string{"/quiet"},
	}}, GoodExitCodes: []int{0, 3}}}
	profile := fixes.Default("x")
	profile.Installs = []string{"thing"}
	in := Input{Runtime: fakeRuntime(t), Installers: installers, DownloadsDir: filepath.Join(g.root, "downloads"), Profile: profile, Prefix: g.prefix, Exe: g.exe, Output: &bytes.Buffer{}, Progress: func(string) {}}

	for range 2 {
		if _, err := Run(context.Background(), in); err != nil {
			t.Fatal(err)
		}
	}

	if runs := strings.Count(g.read(t, "wine-calls.txt"), "setup.exe /quiet"); runs != 1 {
		t.Errorf("the installer ran %d times:\n%s", runs, g.read(t, "wine-calls.txt"))
	}
}
