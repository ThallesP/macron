package wineruntime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ThallesP/macron/internal/failure"
)

// Wine starts every Windows process, the game included, by running
// lib/wine/x86_64-unix/wine (ntdll's wineloader: the realpath of ntdll.so's
// folder plus "wine"), and that loader opens the ntdll.so beside the
// realpath of itself. macOS's gamepolicyd labels a process a game only from
// its app bundle's LSApplicationCategoryType; a bare executable is
// "anon<wine>" to RunningBoard and never gets Game Mode, even with the
// category in its embedded Info.plist. So the loader is copied into
// "Macron Game.app/Contents/MacOS/wine" with ntdll.so linked beside it, and
// the loader's old path becomes a symlink to the copy. docs/perf/README.md
// has what a sandbox showed.
const (
	gameModeApp    = "Macron Game.app"
	gameModeBackup = "wine.before-gamemode"
)

const gameModeInfoPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key><string>English</string>
  <key>CFBundleExecutable</key><string>wine</string>
  <key>CFBundleIdentifier</key><string>org.winehq.wine</string>
  <key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
  <key>CFBundleName</key><string>Wine</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>11.21</string>
  <key>CFBundleVersion</key><string>11.21</string>
  <key>NSPrincipalClass</key><string>WineApplication</string>
  <key>LSUIElement</key><string>1</string>
  <key>LSApplicationCategoryType</key><string>public.app-category.games</string>
  <key>LSSupportsGameMode</key><true/>
</dict>
</plist>
`

// signBundle signs the bundle ad hoc, so its Info.plist is bound to the
// loader inside it. A variable so tests can run where there is no codesign.
var signBundle = func(ctx context.Context, app string) error {
	output, err := exec.CommandContext(ctx, "codesign", "-f", "-s", "-", app).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, output)
	}
	return nil
}

func loaderDir(root string) string {
	return filepath.Join(root, "lib", "wine", "x86_64-unix")
}

// GameModeApplied reports whether the tree's Wine processes run from the
// games-category bundle.
func GameModeApplied(root string) bool {
	info, err := os.Lstat(filepath.Join(loaderDir(root), "wine"))
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

// EnsureGameMode puts the tree's Wine loader into the games-category bundle.
// It does nothing when that is already done.
func EnsureGameMode(ctx context.Context, root string) error {
	if GameModeApplied(root) {
		return nil
	}
	dir := loaderDir(root)
	loader := filepath.Join(dir, "wine")
	if _, err := os.Stat(filepath.Join(dir, "ntdll.so")); err != nil {
		return failure.Errorf(failure.CodeRuntimeMissing, "%s has no ntdll.so", dir)
	}
	app := filepath.Join(dir, gameModeApp)
	macOS := filepath.Join(app, "Contents", "MacOS")
	if err := os.RemoveAll(app); err != nil {
		return err
	}
	if err := os.MkdirAll(macOS, 0o755); err != nil {
		return err
	}
	content, err := os.ReadFile(loader)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(macOS, "wine"), content, 0o755); err != nil {
		return err
	}
	if err := os.Symlink(filepath.Join("..", "..", "..", "ntdll.so"), filepath.Join(macOS, "ntdll.so")); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(gameModeInfoPlist), 0o644); err != nil {
		return err
	}
	if err := signBundle(ctx, app); err != nil {
		return failure.Errorf(failure.CodeFileFailed, "signing %s: %s", app, err)
	}
	backup := filepath.Join(dir, gameModeBackup)
	if err := os.Rename(loader, backup); err != nil {
		return err
	}
	if err := os.Symlink(filepath.Join(gameModeApp, "Contents", "MacOS", "wine"), loader); err != nil {
		return errors.Join(err, os.Rename(backup, loader))
	}
	return nil
}

// RevertGameMode puts the tree's own Wine loader back.
func RevertGameMode(root string) error {
	if !GameModeApplied(root) {
		return nil
	}
	dir := loaderDir(root)
	loader := filepath.Join(dir, "wine")
	if err := os.Remove(loader); err != nil {
		return err
	}
	if err := os.Rename(filepath.Join(dir, gameModeBackup), loader); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(dir, gameModeApp))
}
