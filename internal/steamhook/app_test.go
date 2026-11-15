package steamhook

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ThallesP/macron/internal/failure"
)

const (
	adHocMark     = "\nsigned ad hoc"
	valvePlist    = "LSEnvironment.LC_ALL=en_US.UTF-8\n"
	valveSteamOSX = "bootstrapper\nsigned by Valve"
	valveSeal     = "sealed by Valve"
)

// A Steam.app of text files, with the tools macron runs on it. Info.plist is
// one "key=value" line per key; an ad hoc signature is a mark at the end of a
// file, or CodeResources rewritten from the bundle's Info.plist.
type fakeApp struct {
	path     string
	failing  string
	commands []string
}

func newFakeApp(t *testing.T) *fakeApp {
	t.Helper()
	app := &fakeApp{path: filepath.Join(t.TempDir(), "Steam.app")}
	writeFixtureFile(t, filepath.Join(app.path, "Contents", "Info.plist"), valvePlist)
	writeFixtureFile(t, filepath.Join(app.path, "Contents", "MacOS", "steam_osx"), valveSteamOSX)
	writeFixtureFile(t, filepath.Join(app.path, "Contents", "_CodeSignature", "CodeResources"), valveSeal)
	writeFixtureFile(t, filepath.Join(app.path, "Contents", "Resources", "Steam.icns"), "icon")
	return app
}

func (a *fakeApp) run(name string, args ...string) (string, error) {
	command := name + " " + strings.Join(args, " ")
	a.commands = append(a.commands, command)
	if a.failing != "" && strings.HasPrefix(command, a.failing) {
		return a.failing + " failed", errors.New("exit status 1")
	}

	switch name {
	case "plutil":
		return a.plutil(args)
	case "codesign":
		return "", a.sign(args[len(args)-1])
	default:
		return "", nil
	}
}

func (a *fakeApp) plutil(args []string) (string, error) {
	plist := args[len(args)-1]
	data, err := os.ReadFile(plist)
	if err != nil {
		return err.Error(), err
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	at := slices.IndexFunc(lines, func(line string) bool {
		return strings.HasPrefix(line, args[1]+"=") || strings.HasPrefix(line, args[1]+".")
	})

	switch args[0] {
	case "-extract":
		if at < 0 {
			return "no such key", errors.New("exit status 1")
		}
		_, value, _ := strings.Cut(lines[at], "=")
		return value + "\n", nil
	case "-remove":
		if at < 0 {
			return "no such key", errors.New("exit status 1")
		}
		lines = slices.Delete(lines, at, at+1)
	case "-replace":
		if at >= 0 {
			lines = slices.Delete(lines, at, at+1)
		}
		lines = append(lines, args[1]+"="+args[3])
	default:
		return "unknown verb", errors.New("exit status 1")
	}
	return "", os.WriteFile(plist, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func (a *fakeApp) sign(path string) error {
	if path == a.path {
		plist, err := os.ReadFile(filepath.Join(a.path, "Contents", "Info.plist"))
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(a.path, "Contents", "_CodeSignature", "CodeResources"), []byte("sealed ad hoc over\n"+string(plist)), 0o644)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	unsigned, _, _ := strings.Cut(string(data), "\nsigned ")
	return os.WriteFile(path, []byte(unsigned+adHocMark), 0o755)
}

func (a *fakeApp) ran(prefix string) bool {
	return slices.ContainsFunc(a.commands, func(command string) bool { return strings.HasPrefix(command, prefix) })
}

func readFixtureFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestEnableSwitchesSteamPlayOnInSteamApp(t *testing.T) {
	hook := newFixture(t, "")

	status, err := hook.Enable()
	if err != nil {
		t.Fatal(err)
	}

	if !status.IsSwitchOn || !status.IsEnabled() {
		t.Errorf("status = %+v, want the switch on and the hook enabled", status)
	}
	plist := readFixtureFile(t, hook.appFile("Contents/Info.plist"))
	for _, line := range []string{
		"LSEnvironment.LC_ALL=en_US.UTF-8",
		"LSEnvironment.DYLD_INSERT_LIBRARIES=" + hook.appLibrary(),
		"LSEnvironment.STEAM_EXTRA_COMPAT_TOOLS_PATHS=" + filepath.Join(hook.SteamDir, "compatibilitytools.d"),
	} {
		if !strings.Contains(plist, line+"\n") {
			t.Errorf("Info.plist lacks %q:\n%s", line, plist)
		}
	}
	if got := readFixtureFile(t, hook.appLibrary()); got != "switch library"+adHocMark {
		t.Errorf("the library in Steam.app is %q", got)
	}
	if got := readFixtureFile(t, hook.appFile("Contents/MacOS/steam_osx")); got != "bootstrapper"+adHocMark {
		t.Errorf("steam_osx is %q, want it signed ad hoc", got)
	}
	backups := map[string]string{"Info.plist": valvePlist, "steam_osx": valveSteamOSX, "CodeResources": valveSeal}
	for name, want := range backups {
		if got := readFixtureFile(t, filepath.Join(hook.BackupDir, name)); got != want {
			t.Errorf("backup of %s = %q, want Valve's %q", name, got, want)
		}
	}
}

func TestDisablePutsValvesFilesBackByteForByte(t *testing.T) {
	hook := newFixture(t, "")
	before := snapshot(t, hook.SteamApp)
	if _, err := hook.Enable(); err != nil {
		t.Fatal(err)
	}

	status, err := hook.Disable()
	if err != nil {
		t.Fatal(err)
	}

	assertSameTree(t, before, snapshot(t, hook.SteamApp))
	if status.SwitchUndo != SwitchUndoRestored || status.IsSwitchOn {
		t.Errorf("status = %+v, want the switch off and Valve's files restored", status)
	}
	if fileExists(hook.BackupDir) {
		t.Errorf("%s is still there", hook.BackupDir)
	}
}

func TestEnableTwiceKeepsValvesFilesInTheBackup(t *testing.T) {
	hook := newFixture(t, "")
	before := snapshot(t, hook.SteamApp)
	if _, err := hook.Enable(); err != nil {
		t.Fatal(err)
	}
	if _, err := hook.Enable(); err != nil {
		t.Fatal(err)
	}

	if _, err := hook.Disable(); err != nil {
		t.Fatal(err)
	}

	assertSameTree(t, before, snapshot(t, hook.SteamApp))
}

func TestEnableRefusesASteamAppThatLoadsAnotherLibrary(t *testing.T) {
	hook := newFixture(t, "")
	writeFixtureFile(t, hook.appFile("Contents/Info.plist"), valvePlist+"LSEnvironment.DYLD_INSERT_LIBRARIES=/Applications/Steam.app/Contents/MacOS/other.dylib\n")
	before := snapshot(t, filepath.Dir(hook.SteamApp))
	steamBefore := snapshot(t, hook.SteamDir)

	_, err := hook.Enable()

	assertCode(t, err, failure.CodeSteamAppChanged)
	assertSameTree(t, before, snapshot(t, filepath.Dir(hook.SteamApp)))
	assertSameTree(t, steamBefore, snapshot(t, hook.SteamDir))
}

func TestEnableWithoutSteamAppOrLibraryChangesNothing(t *testing.T) {
	missing := map[failure.Code]func(*Hook){
		failure.CodeSteamAppNotFound: func(hook *Hook) { hook.SteamApp = filepath.Join(hook.SteamDir, "nowhere", "Steam.app") },
		failure.CodeSwitchMissing:    func(hook *Hook) { hook.SwitchLibrary = filepath.Join(hook.SteamDir, "nowhere.dylib") },
	}
	for code, remove := range missing {
		hook := newFixture(t, "")
		remove(&hook)
		before := snapshot(t, hook.SteamDir)

		_, err := hook.Enable()

		assertCode(t, err, code)
		assertSameTree(t, before, snapshot(t, hook.SteamDir))
	}
}

func TestEnableUndoesItselfWhenSigningFails(t *testing.T) {
	hook := newFixture(t, "")
	app := newFakeApp(t)
	app.failing = "codesign -f -s - " + app.path
	hook.SteamApp = app.path
	hook.Run = app.run
	before := snapshot(t, hook.SteamApp)
	steamBefore := snapshot(t, hook.SteamDir)

	_, err := hook.Enable()

	assertCode(t, err, failure.CodeSwitchFailed)
	assertSameTree(t, before, snapshot(t, hook.SteamApp))
	assertSameTree(t, steamBefore, snapshot(t, hook.SteamDir))
}

func TestDisableLeavesASteamAppThatSteamReplaced(t *testing.T) {
	hook := newFixture(t, "")
	if _, err := hook.Enable(); err != nil {
		t.Fatal(err)
	}
	replaced := newFakeApp(t)
	if err := os.RemoveAll(hook.SteamApp); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replaced.path, hook.SteamApp); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, hook.appFile("Contents/MacOS/steam_osx"), "newer bootstrapper\nsigned by Valve")
	before := snapshot(t, hook.SteamApp)

	status, err := hook.Disable()
	if err != nil {
		t.Fatal(err)
	}

	assertSameTree(t, before, snapshot(t, hook.SteamApp))
	if status.SwitchUndo != SwitchUndoNothing || fileExists(hook.BackupDir) {
		t.Errorf("status = %+v, backup kept = %t; want nothing undone and the stale backup gone", status, fileExists(hook.BackupDir))
	}
}

func TestDisableTakesItsSettingsOutOfASteamAppThatChanged(t *testing.T) {
	hook := newFixture(t, "")
	if _, err := hook.Enable(); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, hook.appFile("Contents/MacOS/steam_osx"), "bootstrapper changed by something else")

	status, err := hook.Disable()
	if err != nil {
		t.Fatal(err)
	}

	if status.SwitchUndo != SwitchUndoStripped || status.IsSwitchOn {
		t.Errorf("status = %+v, want macron's settings stripped", status)
	}
	if got := readFixtureFile(t, hook.appFile("Contents/Info.plist")); got != valvePlist {
		t.Errorf("Info.plist = %q, want Valve's keys only", got)
	}
	if fileExists(hook.appLibrary()) {
		t.Errorf("%s is still there", hook.appLibrary())
	}
	if got := readFixtureFile(t, hook.appFile("Contents/MacOS/steam_osx")); got != "bootstrapper changed by something else" {
		t.Errorf("steam_osx = %q, want it left alone", got)
	}
}

func TestSwitchingResetsSteamsInputPermissions(t *testing.T) {
	hook := newFixture(t, "")
	app := newFakeApp(t)
	hook.SteamApp = app.path
	hook.Run = app.run

	if _, err := hook.Enable(); err != nil {
		t.Fatal(err)
	}

	for _, service := range inputAccessServices {
		if !app.ran("tccutil reset " + service + " com.valvesoftware.steam") {
			t.Errorf("did not reset %s; ran %q", service, app.commands)
		}
	}
	if !app.ran(launchServices + " -f " + app.path) {
		t.Errorf("did not register %s with LaunchServices", app.path)
	}
}
