package steamhook

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ThallesP/macron/internal/failure"
)

const fixtureConfig = `"InstallConfigStore"
{
	"Software"
	{
		"Valve"
		{
			"Steam"
			{
				"AutoUpdateWindowEnabled"		"0"
				"CompatToolMapping"
				{
					"0"
					{
						"name"		"someothertool"
						"config"		""
						"priority"		"75"
					}
					"440"
					{
						"name"		"anothertool"
						"config"		""
						"priority"		"250"
					}
				}
			}
		}
	}
	"Music"
	{
		"CrawlSteamInstallFolders"		"1"
	}
}
`

const fixtureManifest = `"osx"
{
	"version"		"1700000001"
	"ostype"		"macos1015"
}
`

func newFixture(t *testing.T, config string) Hook {
	t.Helper()
	steamDir := t.TempDir()
	clientDir := filepath.Join(steamDir, "Steam.AppBundle", "Steam", "Contents", "MacOS")
	writeFixtureFile(t, filepath.Join(clientDir, "steamclient.dylib"), "fake steamclient")
	writeFixtureFile(t, filepath.Join(clientDir, "steam_osx"), "fake steam_osx")
	writeFixtureFile(t, filepath.Join(clientDir, "package", "steam_client_osx.manifest"), fixtureManifest)
	if config != "" {
		writeFixtureFile(t, filepath.Join(steamDir, "config", "config.vdf"), config)
	}

	known := KnownBuilds
	KnownBuilds = []KnownBuild{{ID: "1700000001", Files: []KnownFile{
		{Path: "steamclient.dylib", SHA256: sha256Text("fake steamclient")},
		{Path: "steam_osx", SHA256: sha256Text("fake steam_osx")},
	}}}
	t.Cleanup(func() { KnownBuilds = known })

	return Hook{SteamDir: steamDir, MacronPath: "/opt/macron/bin/macron", IsSteamRunning: func() bool { return false }}
}

func writeFixtureFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sha256Text(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func assertSameTree(t *testing.T, before, after map[string]string) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatalf("file count changed from %d to %d", len(before), len(after))
	}
	for path, content := range before {
		if after[path] != content {
			t.Fatalf("%s changed", path)
		}
	}
}

func assertCode(t *testing.T, err error, code failure.Code) {
	t.Helper()
	var expected *failure.Error
	if !errors.As(err, &expected) || expected.Code != code {
		t.Fatalf("got error %v, want code %s", err, code)
	}
}

func TestIdentifyKnownBuild(t *testing.T) {
	hook := newFixture(t, fixtureConfig)

	build, err := hook.Identify()
	if err != nil {
		t.Fatal(err)
	}
	if !build.IsKnown || build.ID != "1700000001" {
		t.Fatalf("got %+v, want known build 1700000001", build)
	}
}

func TestIdentifyAlteredClientIsUnknown(t *testing.T) {
	hook := newFixture(t, fixtureConfig)
	writeFixtureFile(t, filepath.Join(hook.clientDir(), "steamclient.dylib"), "fake steamclient!")

	build, err := hook.Identify()
	if err != nil {
		t.Fatal(err)
	}
	if build.IsKnown {
		t.Fatal("an altered steamclient.dylib was identified as a known build")
	}
}

func TestIdentifyWithoutClient(t *testing.T) {
	hook := Hook{SteamDir: t.TempDir(), IsSteamRunning: func() bool { return false }}

	_, err := hook.Identify()
	assertCode(t, err, failure.CodeSteamNotFound)
}

func TestEnableRefusesUnknownBuildAndChangesNothing(t *testing.T) {
	hook := newFixture(t, fixtureConfig)
	writeFixtureFile(t, filepath.Join(hook.clientDir(), "steam_osx"), "patched steam_osx")
	before := snapshot(t, hook.SteamDir)

	_, err := hook.Enable()
	assertCode(t, err, failure.CodeUnknownBuild)
	if !strings.Contains(err.Error(), "1700000001") || !strings.Contains(err.Error(), sha256Text("patched steam_osx")) {
		t.Fatalf("error %q does not name the observed build and hash", err)
	}
	assertSameTree(t, before, snapshot(t, hook.SteamDir))
}

func TestEnableRefusesWhileSteamRuns(t *testing.T) {
	hook := newFixture(t, fixtureConfig)
	hook.IsSteamRunning = func() bool { return true }
	before := snapshot(t, hook.SteamDir)

	_, err := hook.Enable()
	assertCode(t, err, failure.CodeSteamRunning)
	assertSameTree(t, before, snapshot(t, hook.SteamDir))
}

func TestEnableRefusesRelativeMacronPath(t *testing.T) {
	hook := newFixture(t, fixtureConfig)
	hook.MacronPath = "macron"

	_, err := hook.Enable()
	assertCode(t, err, failure.CodeInvalidInput)
}

func TestEnableRegistersToolAndBacksUpConfig(t *testing.T) {
	hook := newFixture(t, fixtureConfig)

	status, err := hook.Enable()
	if err != nil {
		t.Fatal(err)
	}
	if !status.IsEnabled() || !status.HasBackup {
		t.Fatalf("got %+v, want enabled with a backup", status)
	}

	backup, err := os.ReadFile(hook.backupPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != fixtureConfig {
		t.Fatal("the backup is not the original config.vdf")
	}

	target, err := os.Readlink(filepath.Join(hook.toolDir(), "macron"))
	if err != nil || target != hook.MacronPath {
		t.Fatalf("launcher links to %q (%v), want %q", target, err, hook.MacronPath)
	}

	manifest := readVDF(t, filepath.Join(hook.toolDir(), "toolmanifest.vdf"))
	assertValue(t, manifest, []string{"manifest", "version"}, "2")
	assertValue(t, manifest, []string{"manifest", "commandline"}, "/macron %verb%")

	tool := readVDF(t, filepath.Join(hook.toolDir(), "compatibilitytool.vdf"))
	assertValue(t, tool, []string{"compatibilitytools", "compat_tools", "macron", "install_path"}, ".")
	assertValue(t, tool, []string{"compatibilitytools", "compat_tools", "macron", "from_oslist"}, "windows")
	assertValue(t, tool, []string{"compatibilitytools", "compat_tools", "macron", "to_oslist"}, "macos")

	config := readVDF(t, hook.configPath())
	assertValue(t, config, append(mappingPath, "0", "name"), "macron")
	assertValue(t, config, append(mappingPath, "0", "priority"), "75")
	assertValue(t, config, append(mappingPath, "440", "name"), "anothertool")
	assertValue(t, config, []string{"InstallConfigStore", "Music", "CrawlSteamInstallFolders"}, "1")
}

func TestEnableTwiceKeepsTheOriginalBackup(t *testing.T) {
	hook := newFixture(t, fixtureConfig)

	if _, err := hook.Enable(); err != nil {
		t.Fatal(err)
	}
	if _, err := hook.Enable(); err != nil {
		t.Fatal(err)
	}

	backup, err := os.ReadFile(hook.backupPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != fixtureConfig {
		t.Fatal("a second enable overwrote the backup")
	}
}

func TestDisableRestoresTheOriginalMapping(t *testing.T) {
	hook := newFixture(t, fixtureConfig)
	if _, err := hook.Enable(); err != nil {
		t.Fatal(err)
	}

	status, err := hook.Disable()
	if err != nil {
		t.Fatal(err)
	}
	if status.IsToolRegistered || status.IsDefaultTool || status.HasBackup {
		t.Fatalf("got %+v, want nothing of macron left", status)
	}
	if fileExists(filepath.Dir(hook.toolDir())) {
		t.Fatal("the empty compatibilitytools.d folder enable made is still there")
	}

	config := readVDF(t, hook.configPath())
	original, err := ParseVDF(fixtureConfig)
	if err != nil {
		t.Fatal(err)
	}
	if config.String() != original.String() {
		t.Fatalf("config.vdf after disable:\n%s\nwant:\n%s", config, original)
	}
}

func TestDisableWithoutPreviousMappingRemovesIt(t *testing.T) {
	hook := newFixture(t, `"InstallConfigStore"
{
	"Software"
	{
		"Valve"
		{
			"Steam"
			{
				"AutoUpdateWindowEnabled"		"0"
			}
		}
	}
}
`)
	if _, err := hook.Enable(); err != nil {
		t.Fatal(err)
	}

	if _, err := hook.Disable(); err != nil {
		t.Fatal(err)
	}

	config := readVDF(t, hook.configPath())
	if config.Path(mappingPath...) != nil {
		t.Fatalf("CompatToolMapping is still there:\n%s", config)
	}
}

func TestEnableWithoutConfigThenDisable(t *testing.T) {
	hook := newFixture(t, "")

	if _, err := hook.Enable(); err != nil {
		t.Fatal(err)
	}
	assertValue(t, readVDF(t, hook.configPath()), append(mappingPath, "0", "name"), "macron")

	if _, err := hook.Disable(); err != nil {
		t.Fatal(err)
	}
	if fileExists(hook.toolDir()) {
		t.Fatal("the tool folder is still there")
	}
}

func TestDisableKeepsOtherTools(t *testing.T) {
	hook := newFixture(t, fixtureConfig)
	other := filepath.Join(hook.SteamDir, "compatibilitytools.d", "othertool", "toolmanifest.vdf")
	writeFixtureFile(t, other, "\"manifest\" { }")
	if _, err := hook.Enable(); err != nil {
		t.Fatal(err)
	}

	if _, err := hook.Disable(); err != nil {
		t.Fatal(err)
	}
	if !fileExists(other) {
		t.Fatal("disable removed another compatibility tool")
	}
}

func TestDisableWhenNotEnabled(t *testing.T) {
	hook := newFixture(t, fixtureConfig)

	_, err := hook.Disable()
	assertCode(t, err, failure.CodeHookNotEnabled)
}

func TestDisableWorksAfterSteamUpdates(t *testing.T) {
	hook := newFixture(t, fixtureConfig)
	if _, err := hook.Enable(); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(hook.clientDir(), "steamclient.dylib"), "a newer steamclient")

	if _, err := hook.Disable(); err != nil {
		t.Fatal(err)
	}
}

func TestDisableRefusesWhileSteamRuns(t *testing.T) {
	hook := newFixture(t, fixtureConfig)
	if _, err := hook.Enable(); err != nil {
		t.Fatal(err)
	}
	hook.IsSteamRunning = func() bool { return true }

	_, err := hook.Disable()
	assertCode(t, err, failure.CodeSteamRunning)
}

func readVDF(t *testing.T, path string) *Node {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	node, err := ParseVDF(string(data))
	if err != nil {
		t.Fatal(err)
	}
	return node
}

func assertValue(t *testing.T, root *Node, path []string, want string) {
	t.Helper()
	node := root.Path(path...)
	if node == nil {
		t.Fatalf("%s is missing", strings.Join(path, "/"))
	}
	if node.Value != want {
		t.Fatalf("%s is %q, want %q", strings.Join(path, "/"), node.Value, want)
	}
}
