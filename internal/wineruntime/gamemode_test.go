package wineruntime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := loaderDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"wine": "loader", "ntdll.so": "ntdll"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestEnsureGameModePutsTheLoaderInAGamesBundle(t *testing.T) {
	var signed []string
	signBundle = func(_ context.Context, app string) error {
		signed = append(signed, app)
		return nil
	}
	root := fakeTree(t)
	dir := loaderDir(root)

	if err := EnsureGameMode(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	if !GameModeApplied(root) {
		t.Fatal("not applied")
	}
	// The path Wine execs for every process reaches the bundle's copy, and
	// the copy finds ntdll.so beside the realpath of itself.
	target, err := filepath.EvalSymlinks(filepath.Join(dir, "wine"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "Macron Game.app", "Contents", "MacOS", "wine"); target != mustEval(t, want) {
		t.Errorf("wine resolves to %s, want %s", target, want)
	}
	if got := read(t, target); got != "loader" {
		t.Errorf("bundle loader = %q", got)
	}
	if got := read(t, filepath.Join(filepath.Dir(target), "ntdll.so")); got != "ntdll" {
		t.Errorf("ntdll.so beside the bundle loader = %q", got)
	}
	plist := read(t, filepath.Join(dir, "Macron Game.app", "Contents", "Info.plist"))
	for _, want := range []string{"<key>LSApplicationCategoryType</key><string>public.app-category.games</string>", "<key>CFBundleExecutable</key><string>wine</string>"} {
		if !strings.Contains(plist, want) {
			t.Errorf("Info.plist lacks %s", want)
		}
	}
	if len(signed) != 1 {
		t.Errorf("signed %v, want the bundle once", signed)
	}

	if err := EnsureGameMode(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	if len(signed) != 1 {
		t.Errorf("a second call signed again: %v", signed)
	}

	if err := RevertGameMode(root); err != nil {
		t.Fatal(err)
	}
	if GameModeApplied(root) {
		t.Error("still applied after revert")
	}
	if got := read(t, filepath.Join(dir, "wine")); got != "loader" {
		t.Errorf("loader after revert = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "Macron Game.app")); !os.IsNotExist(err) {
		t.Errorf("bundle left after revert: %v", err)
	}
}

func mustEval(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}
