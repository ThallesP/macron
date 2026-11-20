package launch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ThallesP/macron/fixes"
	"github.com/ThallesP/macron/internal/failure"
	"github.com/ThallesP/macron/internal/wineruntime"
)

const fakeWine = `#!/bin/sh
echo "$*" >> "$FAKE_WINE_ROOT/wine-calls.txt"
if [ "$1" = wineboot ]; then
	echo "$WINEPREFIX" >> "$FAKE_WINE_ROOT/wineboot-prefixes.txt"
	home="$WINEPREFIX/drive_c/users/$USER"
	mkdir -p "$home/AppData/Roaming" "$FAKE_WINE_ROOT/mac-documents" "$WINEPREFIX/drive_c/windows/system32" "$WINEPREFIX/drive_c/windows/syswow64"
	[ -e "$home/Documents" ] || ln -s "$FAKE_WINE_ROOT/mac-documents" "$home/Documents"
	echo "registry" > "$WINEPREFIX/system.reg"
	date -r "$(dirname "$0")/../share/wine/wine.inf" +%s > "$WINEPREFIX/.update-timestamp"
	exit 0
fi
[ "$1" = regedit ] && exit 0
env | grep -E '^(WINE|DXMT|GALLIUM)' | sort > "$FAKE_WINE_ROOT/game-env.txt"
pwd > "$FAKE_WINE_ROOT/game-cwd.txt"
echo "game output"
exit 3
`

func TestRunEndsTheWindowsEnvironmentWhenAskedToStop(t *testing.T) {
	g := newGame(t)
	runtime := fakeRuntime(t)
	gameInAnotherProcess := "sleep 60 &\necho $! > \"$FAKE_WINE_ROOT/game.pid\"\ntouch \"$FAKE_WINE_ROOT/game-started\"\nexec sleep 60"
	waitingGame := strings.Replace(fakeWine, "echo \"game output\"\nexit 3", gameInAnotherProcess, 1)
	recordingServer := "#!/bin/sh\necho \"$*\" >> \"$FAKE_WINE_ROOT/wineserver-calls.txt\"\n[ \"$1\" = -k ] && kill \"$(cat \"$FAKE_WINE_ROOT/game.pid\")\"\nexit 0\n"
	for _, tree := range []wineruntime.Tree{wineruntime.TreePlain, wineruntime.TreeDXMT} {
		bin := filepath.Join(runtime.Root(tree), "bin")
		if err := os.WriteFile(filepath.Join(bin, "wine"), []byte(waitingGame), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(bin, "wineserver"), []byte(recordingServer), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ctx, stop := context.WithCancel(context.Background())
	go func() {
		for !fileExists(filepath.Join(g.root, "game-started")) {
			time.Sleep(10 * time.Millisecond)
		}
		stop()
	}()
	started := time.Now()

	exitCode, err := Run(ctx, Input{
		Runtime:     runtime,
		Profile:     fixes.Profile{ID: "some-game", Backend: fixes.BackendWineD3D},
		Prefix:      g.prefix,
		Exe:         g.exe,
		Output:      &bytes.Buffer{},
		Progress:    func(string) {},
		WaitForWine: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if exitCode != ExitStopped {
		t.Errorf("exit code %d, want %d for a game that was stopped", exitCode, ExitStopped)
	}
	if calls := g.read(t, "wineserver-calls.txt"); !strings.Contains(calls, "-k\n") {
		t.Errorf("the Wine server was not told to end its programs; calls:\n%s", calls)
	}
	if took := time.Since(started); took > 5*time.Second {
		t.Errorf("stopping took %s: macron waited for the game's output instead of ending it", took)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

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
		writeWineInf(t, installed.Root(tree), wineInfTime)
	}
	return installed
}

var wineInfTime = time.Unix(1793450195, 0)

func writeWineInf(t *testing.T, root string, modified time.Time) {
	t.Helper()
	inf := filepath.Join(root, "share", "wine", "wine.inf")
	if err := os.MkdirAll(filepath.Dir(inf), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inf, []byte("[Version]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(inf, modified, modified); err != nil {
		t.Fatal(err)
	}
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
	t.Setenv("FAKE_WINE_ROOT", root)
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
	for _, want := range []string{"WINEDLLOVERRIDES=d3d9=n,b;mscoree=;mshtml=;winedbg.exe=;winemenubuilder.exe=", "WINEPREFIX=" + g.prefix, "DXMT_LOG_LEVEL=info", "WINEDEBUG=-all"} {
		if !strings.Contains(env, want) {
			t.Errorf("game environment lacks %s:\n%s", want, env)
		}
	}
	gameDir, err := filepath.EvalSymlinks(filepath.Dir(g.exe))
	if err != nil {
		t.Fatal(err)
	}
	if cwd := strings.TrimSpace(g.read(t, "game-cwd.txt")); cwd != gameDir {
		t.Errorf("game started in %s, want %s", cwd, gameDir)
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

func TestANewGameIsClonedFromTheRuntimesTemplate(t *testing.T) {
	g := newGame(t)
	runtime := fakeRuntime(t)
	in := Input{Runtime: runtime, Profile: fixes.Default("x"), Prefix: g.prefix, Exe: g.exe, Output: &bytes.Buffer{}, Progress: func(string) {}}
	if _, err := Run(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	in.Prefix = filepath.Join(g.root, "another-prefix")

	if _, err := Run(context.Background(), in); err != nil {
		t.Fatal(err)
	}

	boots := strings.Fields(g.read(t, "wineboot-prefixes.txt"))
	template := filepath.Join(runtime.Dir, templatesFolder, string(wineruntime.TreeDXMT))
	if len(boots) != 1 || !strings.HasPrefix(boots[0], template+".partial-") {
		t.Errorf("wineboot ran in %v, want once in a new template next to %s", boots, template)
	}
	for _, prefix := range []string{"prefix", "another-prefix"} {
		if got := g.read(t, prefix+"/system.reg"); got != "registry\n" {
			t.Errorf("%s/system.reg = %q, want the template's", prefix, got)
		}
		if _, hasRecord := readPrefixRecord(filepath.Join(g.root, prefix)); !hasRecord {
			t.Errorf("%s has no %s", prefix, prefixMarker)
		}
	}
	if fileExists(filepath.Join(template, prefixMarker)) {
		t.Errorf("the template has a %s, so a clone would look made for a game", prefixMarker)
	}
}

func TestAPrefixInAFolderWithFilesIsBootedInPlace(t *testing.T) {
	g := newGame(t)
	if err := os.MkdirAll(g.prefix, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g.prefix, "keep.txt"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(context.Background(), Input{Runtime: fakeRuntime(t), Profile: fixes.Default("x"), Prefix: g.prefix, Exe: g.exe, Output: &bytes.Buffer{}, Progress: func(string) {}}); err != nil {
		t.Fatal(err)
	}

	if boots := strings.Fields(g.read(t, "wineboot-prefixes.txt")); len(boots) != 1 || boots[0] != g.prefix {
		t.Errorf("wineboot ran in %v, want once in %s", boots, g.prefix)
	}
	if got := g.read(t, "prefix/keep.txt"); got != "mine" {
		t.Errorf("keep.txt = %q", got)
	}
}

func TestANewRuntimeWithTheSameWineInfDoesNotUpdateThePrefix(t *testing.T) {
	g := newGame(t)
	runtime := fakeRuntime(t)
	profile := fixes.Default("x")
	profile.Backend = fixes.BackendDXVK
	writeDXVK(t, runtime, "old dxvk")
	in := Input{Runtime: runtime, Profile: profile, Prefix: g.prefix, Exe: g.exe, Output: &bytes.Buffer{}, Progress: func(string) {}}
	if _, err := Run(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	in.Runtime = fakeRuntime(t)
	in.Runtime.Version = "test-2"
	writeDXVK(t, in.Runtime, "new dxvk")

	if _, err := Run(context.Background(), in); err != nil {
		t.Fatal(err)
	}

	if boots := strings.Fields(g.read(t, "wineboot-prefixes.txt")); len(boots) != 1 {
		t.Errorf("wineboot ran in %v, want only for the first runtime's template", boots)
	}
	if got := g.read(t, "prefix/drive_c/windows/system32/d3d11.dll"); got != "new dxvk 64" {
		t.Errorf("system32/d3d11.dll = %q, want the new runtime's DXVK", got)
	}
	if record, _ := readPrefixRecord(g.prefix); record.Runtime != "test-2" {
		t.Errorf("the prefix is recorded for %q, want test-2", record.Runtime)
	}
}

func TestANewRuntimeWithAnotherWineInfUpdatesThePrefix(t *testing.T) {
	g := newGame(t)
	in := Input{Runtime: fakeRuntime(t), Profile: fixes.Default("x"), Prefix: g.prefix, Exe: g.exe, Output: &bytes.Buffer{}, Progress: func(string) {}}
	if _, err := Run(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	in.Runtime = fakeRuntime(t)
	in.Runtime.Version = "test-2"
	writeWineInf(t, in.Runtime.Root(wineruntime.TreeDXMT), wineInfTime.Add(time.Hour))

	if _, err := Run(context.Background(), in); err != nil {
		t.Fatal(err)
	}

	boots := strings.Fields(g.read(t, "wineboot-prefixes.txt"))
	if len(boots) != 2 || boots[1] != g.prefix {
		t.Errorf("wineboot ran in %v, want a second time in %s", boots, g.prefix)
	}
}

func writeDXVK(t *testing.T, runtime wineruntime.Installed, content string) {
	t.Helper()
	for bits, suffix := range map[wineruntime.DXVKBits]string{wineruntime.DXVK64: " 64", wineruntime.DXVK32: " 32"} {
		if err := os.MkdirAll(runtime.DXVK(bits), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runtime.DXVK(bits), "d3d11.dll"), []byte(content+suffix), 0o644); err != nil {
			t.Fatal(err)
		}
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
	if env := g.read(t, "game-env.txt"); !strings.Contains(env, "WINEDLLOVERRIDES=d3d10core=n;d3d11=n;d3d9=n;dxgi=n;mscoree=;mshtml=;winedbg.exe=;winemenubuilder.exe=") {
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

func TestRegFile(t *testing.T) {
	got := RegFile([]fixes.RegistryValue{
		{Key: `HKEY_CURRENT_USER\Software\Wine\Mac Driver`, Name: "AllowSoftwareRendering", Type: "sz", Value: "y"},
		{Key: `HKEY_CURRENT_USER\Software\Wine\Mac Driver`, Name: "Path", Type: "sz", Value: `C:\a "b"`},
		{Key: `HKEY_CURRENT_USER\Software\Game`, Name: "Width", Type: "dword", Value: "320"},
	})

	want := "Windows Registry Editor Version 5.00\r\n" +
		"\r\n[HKEY_CURRENT_USER\\Software\\Wine\\Mac Driver]\r\n" +
		"\"AllowSoftwareRendering\"=\"y\"\r\n" +
		"\"Path\"=\"C:\\\\a \\\"b\\\"\"\r\n" +
		"\r\n[HKEY_CURRENT_USER\\Software\\Game]\r\n" +
		"\"Width\"=dword:00000320\r\n"
	if got != want {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
}

func TestRegistryIsImportedOnceUntilItChanges(t *testing.T) {
	g := newGame(t)
	profile := fixes.Default("x")
	profile.Registry = []fixes.RegistryValue{{Key: `HKEY_CURRENT_USER\Software\Wine\Mac Driver`, Name: "AllowSoftwareRendering", Type: "sz", Value: "y"}}
	in := Input{Runtime: fakeRuntime(t), Profile: profile, Prefix: g.prefix, Exe: g.exe, Output: &bytes.Buffer{}, Progress: func(string) {}}

	for range 2 {
		if _, err := Run(context.Background(), in); err != nil {
			t.Fatal(err)
		}
	}
	in.Profile.Registry[0].Value = "n"
	if _, err := Run(context.Background(), in); err != nil {
		t.Fatal(err)
	}

	if imports := strings.Count(g.read(t, "wine-calls.txt"), "regedit /S"); imports != 2 {
		t.Errorf("regedit ran %d times, want 2", imports)
	}
}

func TestSoftwareOpenGLPutsMesaInThePrefix(t *testing.T) {
	g := newGame(t)
	runtime := fakeRuntime(t)
	for _, bits := range []wineruntime.MesaBits{wineruntime.Mesa64, wineruntime.Mesa32} {
		if err := os.MkdirAll(runtime.Mesa(bits), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runtime.Mesa(bits), "opengl32.dll"), []byte("mesa "+string(bits)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(runtime.Dir, "mesa", wineruntime.Mesa.SHA256), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	in := Input{Runtime: runtime, Profile: fixes.Default("x"), Prefix: g.prefix, Exe: g.exe, Output: &bytes.Buffer{}, Progress: func(string) {}, SoftwareOpenGL: true}

	if _, err := Run(context.Background(), in); err != nil {
		t.Fatal(err)
	}

	if got := g.read(t, "prefix/drive_c/windows/system32/opengl32.dll"); got != "mesa x64" {
		t.Errorf("system32/opengl32.dll = %q", got)
	}
	if got := g.read(t, "prefix/drive_c/windows/syswow64/opengl32.dll"); got != "mesa x86" {
		t.Errorf("syswow64/opengl32.dll = %q", got)
	}
	env := g.read(t, "game-env.txt")
	if !strings.Contains(env, "opengl32=n,b") {
		t.Errorf("opengl32 is not overridden:\n%s", env)
	}
}

func TestTheUsersWineDebugChannelsWin(t *testing.T) {
	env := Environment(fixes.Default("x"), "/prefix", []string{"WINEDEBUG=fps", "WINEPREFIX=/elsewhere"})

	if got := lastValue(env, "WINEDEBUG"); got != "fps" {
		t.Errorf("WINEDEBUG=%q, want the user's fps", got)
	}
	if got := lastValue(env, "WINEPREFIX"); got != "/prefix" {
		t.Errorf("WINEPREFIX=%q, want macron's prefix", got)
	}
	if got := lastValue(Environment(fixes.Default("x"), "/prefix", nil), "WINEDEBUG"); got != "-all" {
		t.Errorf("WINEDEBUG=%q without the user's, want -all", got)
	}
}

func TestEnvironmentJoinsTheProfilesAndTheUsersDXMTOptions(t *testing.T) {
	profile := fixes.Default("x")
	profile.Env = map[string]string{"DXMT_CONFIG": "dxmt.logLatency=True"}

	both := Environment(profile, "/prefix", []string{"DXMT_CONFIG=dxmt.latencySleep=False"})
	onlyProfile := Environment(profile, "/prefix", nil)
	onlyUser := Environment(fixes.Default("x"), "/prefix", []string{"DXMT_CONFIG=dxmt.latencySleep=False"})

	if got, want := lastValue(both, "DXMT_CONFIG"), "dxmt.logLatency=True;dxmt.latencySleep=False"; got != want {
		t.Errorf("both: DXMT_CONFIG=%q, want the profile's options and then the user's, %q", got, want)
	}
	if got := lastValue(onlyProfile, "DXMT_CONFIG"); got != "dxmt.logLatency=True" {
		t.Errorf("only the profile: DXMT_CONFIG=%q", got)
	}
	if got := lastValue(onlyUser, "DXMT_CONFIG"); got != "dxmt.latencySleep=False" {
		t.Errorf("only the user: DXMT_CONFIG=%q", got)
	}
}

func lastValue(env []string, name string) string {
	value := ""
	for _, entry := range env {
		if after, ok := strings.CutPrefix(entry, name+"="); ok {
			value = after
		}
	}
	return value
}

func TestAProgramThatLeavesItsOutputOpenStillExitsWithItsCode(t *testing.T) {
	var output bytes.Buffer
	program := exec.Command("sh", "-c", "sleep 5 & echo started; exit 0")
	program.Stdout = &output
	program.WaitDelay = 200 * time.Millisecond

	exitCode, err := exitCodeOf(program.Run())

	if err != nil || exitCode != 0 {
		t.Fatalf("exit code %d, error %v; want 0 and no error when only a background program kept the output open", exitCode, err)
	}
}

func TestTheUsersWined3dOptionsWinAndTheProfilesOthersStay(t *testing.T) {
	profile := fixes.Default("steam-1")
	profile.Env = map[string]string{"WINE_D3D_CONFIG": "VideoPciVendorID=0x10de,VideoPciDeviceID=0x2684"}

	alone := Environment(profile, "/prefix", []string{"HOME=/Users/me"})
	both := Environment(profile, "/prefix", []string{"HOME=/Users/me", "WINE_D3D_CONFIG=csmt=0"})

	if got := lastValue(alone, "WINE_D3D_CONFIG"); got != "VideoPciVendorID=0x10de,VideoPciDeviceID=0x2684" {
		t.Errorf("without the user's: %q", got)
	}
	if got := lastValue(both, "WINE_D3D_CONFIG"); got != "csmt=0,VideoPciVendorID=0x10de,VideoPciDeviceID=0x2684" {
		t.Errorf("with the user's: %q, want the user's options first (wined3d takes the first value of each)", got)
	}
}

func TestRenderPixelsIsForTheProfilesChipsUnlessTheUserSetIt(t *testing.T) {
	profile := fixes.Default("x")
	profile.RenderPixels = 1342080
	profile.RenderPixelsChips = []string{"Apple M3"}
	everyMac := fixes.Default("y")
	everyMac.RenderPixels = 1000000

	cases := map[string]struct {
		profile fixes.Profile
		env     []string
		chip    string
		want    int
	}{
		"the profile's chip":         {profile, nil, "Apple M3", 1342080},
		"another chip":               {profile, nil, "Apple M3 Max", 0},
		"no chip known":              {profile, nil, "", 0},
		"the user turned it off":     {profile, []string{"MACRON_RENDER_PIXELS=0"}, "Apple M3", 0},
		"the user chose a size":      {profile, []string{"MACRON_RENDER_PIXELS=900000"}, "Apple M3", 0},
		"a profile without chips":    {everyMac, nil, "Apple M2 Pro", 1000000},
		"a profile without the size": {fixes.Default("z"), nil, "Apple M3", 0},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := renderPixels(c.profile, c.env, c.chip)

			if got != c.want || ok != (c.want != 0) {
				t.Errorf("renderPixels = %d, %v; want %d", got, ok, c.want)
			}
		})
	}
}
